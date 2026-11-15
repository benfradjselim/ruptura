package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const engineFailures = `[{"workload":"shop/Deployment/a","failureMode":"saturation","affects":["hidden/Deployment/z"]},{"workload":"hidden/Deployment/b"}]`

// engineServer plays the Ruptura engine: it requires the API key and serves
// canned answers.
func engineServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer the-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v2/failures", "/api/v2/failures/closest":
			_, _ = io.WriteString(w, engineFailures)
		case "/api/v2/failures/series":
			_, _ = io.WriteString(w, `{"workload":"`+r.URL.Query().Get("workload")+`","pods":[]}`)
		case "/api/v2/callgraph":
			_, _ = io.WriteString(w, `{"nodes":[{"id":"n","namespace":"shop"}],"edges":[]}`)
		case "/api/v2/health":
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestHandler(t *testing.T) {
	engine, seen := engineServer(t)
	checker := &mapChecker{allowed: map[string]bool{"shop": true}}
	h := newHandler(engine.URL, "the-key", checker, 5*time.Second)

	tests := []struct {
		name, method, target, token string
		status                      int
		wantBody                    string // substring, or "" to skip
		absent                      string // substring that must not appear
	}{
		{"failures of visible namespaces only", "GET", "/api/v2/failures", "user", 200, `"shop/Deployment/a"`, "hidden"},
		{"closest pods filtered the same way", "GET", "/api/v2/failures/closest", "user", 200, `"shop/Deployment/a"`, "hidden"},
		{"series of a visible workload", "GET", "/api/v2/failures/series?workload=shop/Deployment/a", "user", 200, `"shop/Deployment/a"`, ""},
		{"series of a hidden workload", "GET", "/api/v2/failures/series?workload=hidden/Deployment/b", "user", 403, "", "pods"},
		{"series without a workload", "GET", "/api/v2/failures/series", "user", 403, "", ""},
		{"call graph", "GET", "/api/v2/callgraph", "user", 200, `"nodes":[{"id":"n"`, ""},
		{"no token", "GET", "/api/v2/failures", "", 401, "", "shop"},
		{"health is not proxied", "GET", "/api/v2/health", "user", 403, "", "ok"},
		{"anything else is not proxied", "GET", "/api/v2/auth/login", "user", 403, "", ""},
		{"writes are refused", "POST", "/api/v2/failures", "user", 405, "", "shop"},
		{"trailing slash is another path", "GET", "/api/v2/failures/", "user", 403, "", "shop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			body := rec.Body.String()
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, body)
			}
			if tt.wantBody != "" && !strings.Contains(body, tt.wantBody) {
				t.Fatalf("body %s lacks %s", body, tt.wantBody)
			}
			if tt.absent != "" && strings.Contains(body, tt.absent) {
				t.Fatalf("body %s must not contain %s", body, tt.absent)
			}
		})
	}
	for _, uri := range *seen {
		if strings.HasPrefix(uri, "/api/v2/health") || strings.HasPrefix(uri, "/api/v2/auth") {
			t.Fatalf("engine was asked %s", uri)
		}
	}
}

func TestHandler_AffectsOfHiddenCallersDropped(t *testing.T) {
	engine, _ := engineServer(t)
	h := newHandler(engine.URL, "the-key", &mapChecker{allowed: map[string]bool{"shop": true}}, 5*time.Second)
	req := httptest.NewRequest("GET", "/api/v2/failures", nil)
	req.Header.Set("Authorization", "Bearer user")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var got []struct {
		Workload string   `json:"workload"`
		Affects  []string `json:"affects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Affects) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestHandler_Upstream(t *testing.T) {
	engine, _ := engineServer(t)
	tests := []struct {
		name    string
		key     string
		checker AccessChecker
		status  int
	}{
		{"wrong API key is a gateway error", "bad", &mapChecker{allowed: map[string]bool{"shop": true}}, 502},
		{"refused user token", "the-key", &mapChecker{err: errUnauthenticated}, 401},
		{"API server down", "the-key", &mapChecker{err: io.ErrUnexpectedEOF}, 502},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler(engine.URL, tt.key, tt.checker, 5*time.Second)
			req := httptest.NewRequest("GET", "/api/v2/failures", nil)
			req.Header.Set("Authorization", "Bearer user")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "shop/") {
				t.Fatal("no data must leak on an error")
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct{ header, want string }{
		{"Bearer abc", "abc"},
		{"bearer abc ", "abc"},
		{"Basic abc", ""},
		{"", ""},
		{"Bearer", ""},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		if tt.header != "" {
			req.Header.Set("Authorization", tt.header)
		}
		if got := bearerToken(req); got != tt.want {
			t.Errorf("bearerToken(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
