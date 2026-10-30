package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vec(rows ...string) string {
	return `{"status":"success","data":{"resultType":"vector","result":[` + strings.Join(rows, ",") + `]}}`
}

func row(ns, pod, v string) string {
	return `{"metric":{"namespace":"` + ns + `","pod":"` + pod + `"},"value":[1700000000,"` + v + `"]}`
}

func TestCollect(t *testing.T) {
	sig := Signal{Name: "s", Mode: "saturation", Usage: "U", Capacity: "C", Restarts: "R"}
	tests := []struct {
		name    string
		resp    map[string]string
		status  int
		wantErr bool
		want    map[string]Reading
	}{
		{name: "join", resp: map[string]string{
			"U": vec(row("a", "p1", "50"), row("a", "p2", "10")),
			"C": vec(row("a", "p1", "100"), row("a", "p2", "20")),
			"R": vec(row("a", "p1", "3")),
		}, want: map[string]Reading{
			"a/p1": {Namespace: "a", Pod: "p1", Usage: 50, Capacity: 100, Restarts: 3},
			"a/p2": {Namespace: "a", Pod: "p2", Usage: 10, Capacity: 20},
		}},
		{name: "missing capacity", resp: map[string]string{
			"U": vec(row("a", "p1", "50")), "C": vec(), "R": vec(),
		}, want: map[string]Reading{"a/p1": {Namespace: "a", Pod: "p1", Usage: 50}}},
		{name: "missing usage keeps restarts only", resp: map[string]string{
			"U": vec(), "C": vec(row("a", "p1", "100")), "R": vec(row("a", "p1", "1")),
		}, want: map[string]Reading{"a/p1": {Namespace: "a", Pod: "p1", Restarts: 1, RestartsOnly: true}}},
		{name: "http error", status: 500, wantErr: true},
		{name: "prometheus error status", resp: map[string]string{
			"U": `{"status":"error","errorType":"bad","error":"boom"}`, "C": vec(), "R": vec(),
		}, wantErr: true},
		{name: "bad json", resp: map[string]string{"U": "{", "C": vec(), "R": vec()}, wantErr: true},
		{name: "bad value", resp: map[string]string{
			"U": vec(row("a", "p1", "xx")), "C": vec(), "R": vec(),
		}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var auth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth = r.Header.Get("Authorization")
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				_, _ = w.Write([]byte(tc.resp[r.URL.Query().Get("query")]))
			}))
			defer srv.Close()
			th := NewThanos(srv.URL, "tok", srv.Client())
			got, err := th.Collect(context.Background(), sig)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if auth != "Bearer tok" {
				t.Errorf("auth header = %q", auth)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d readings, want %d", len(got), len(tc.want))
			}
			for _, r := range got {
				w := tc.want[r.Namespace+"/"+r.Pod]
				if r.Usage != w.Usage || r.Capacity != w.Capacity || r.Restarts != w.Restarts {
					t.Errorf("reading %+v, want %+v", r, w)
				}
				if r.At.Unix() != 1700000000 {
					t.Errorf("At = %v", r.At)
				}
			}
		})
	}
}

func TestCollectConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	c := srv.Client()
	srv.Close()
	if _, err := NewThanos(url, "t", c).Collect(context.Background(), Signal{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadCatalog(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Signals) == 0 {
		t.Fatal("empty catalog")
	}
	s := cat.Signals[0]
	if s.Name != "memory_saturation" || s.Mode != "saturation" {
		t.Errorf("first = %+v", s)
	}
	if !strings.Contains(s.Usage, "container_memory_working_set_bytes") ||
		!strings.Contains(s.Capacity, "kube_pod_container_resource_limits") ||
		!strings.Contains(s.Restarts, "kube_pod_container_status_restarts_total") {
		t.Errorf("queries wrong: %+v", s)
	}
}

func TestNewInClusterThanos(t *testing.T) {
	const pemCA = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`
	tests := []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{"both CAs", map[string]string{"token": "t", "ca.crt": pemCA, "service-ca.crt": pemCA}, false},
		{"service-ca absent", map[string]string{"token": "t", "ca.crt": pemCA}, false},
		{"missing token", map[string]string{"ca.crt": pemCA}, true},
		{"missing ca", map[string]string{"token": "t"}, true},
		{"invalid ca", map[string]string{"token": "t", "ca.crt": "junk"}, true},
		{"invalid service-ca", map[string]string{"token": "t", "ca.crt": pemCA, "service-ca.crt": "junk"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, c := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			th, err := newInClusterThanos("https://x", dir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && th == nil {
				t.Fatal("nil thanos")
			}
		})
	}
	if _, err := NewInClusterThanos("https://x"); err == nil {
		t.Skip("running in cluster")
	}
}

func TestLoadCatalog_CPUThrottling(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cat.Signals {
		if s.Name != "cpu_throttling" {
			continue
		}
		if s.Mode != "condition" || s.Capacity != "" || s.Restarts != "" {
			t.Fatalf("signal = %+v", s)
		}
		for _, part := range []string{"container_cpu_cfs_throttled_periods_total", "container_cpu_cfs_periods_total", `container!=""`, "[5m]"} {
			if !strings.Contains(s.Usage, part) {
				t.Errorf("usage query lacks %q: %s", part, s.Usage)
			}
		}
		return
	}
	t.Fatal("cpu_throttling signal missing")
}

func TestCollect_ConditionSignalRunsOnlyUsageQuery(t *testing.T) {
	var queries int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		_, _ = w.Write([]byte(vec(row("n", "p", "0.4"))))
	}))
	defer srv.Close()
	got, err := NewThanos(srv.URL, "", srv.Client()).Collect(context.Background(), Signal{Name: "cpu_throttling", Mode: "condition", Usage: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if queries != 1 || len(got) != 1 || got[0].Usage != 0.4 {
		t.Fatalf("queries = %d readings = %+v", queries, got)
	}
}
