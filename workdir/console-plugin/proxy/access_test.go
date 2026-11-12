package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// reviewServer answers SelfSubjectAccessReviews: allowed for the namespaces
// in allow, 401 for the token "expired".
func reviewServer(t *testing.T, allow map[string]bool) (*ssarChecker, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var review accessReview
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			t.Errorf("decode review: %v", err)
		}
		attrs := review.Spec.ResourceAttributes
		if attrs.Verb != "list" || attrs.Resource != "pods" {
			t.Errorf("review asks %s %s, want list pods", attrs.Verb, attrs.Resource)
		}
		review.Status.Allowed = allow[attrs.Namespace]
		_ = json.NewEncoder(w).Encode(review)
	}))
	t.Cleanup(srv.Close)
	return &ssarChecker{apiServer: srv.URL, client: srv.Client()}, &calls
}

func TestSSARChecker(t *testing.T) {
	checker, _ := reviewServer(t, map[string]bool{"shop": true})
	tests := []struct {
		name, token, namespace string
		want                   bool
		wantErr                error
	}{
		{"allowed namespace", "good", "shop", true, nil},
		{"denied namespace", "good", "hidden", false, nil},
		{"refused token", "expired", "shop", false, errUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checker.Allowed(context.Background(), tt.token, tt.namespace)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("allowed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCachedChecker(t *testing.T) {
	inner, calls := reviewServer(t, map[string]bool{"shop": true})
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	cache := newCachedChecker(inner, 30*time.Second)
	cache.now = func() time.Time { return now }
	ctx := context.Background()

	ask := func(token, ns string, want bool, wantCalls int) {
		t.Helper()
		got, err := cache.Allowed(ctx, token, ns)
		if err != nil {
			t.Fatal(err)
		}
		if got != want || *calls != wantCalls {
			t.Fatalf("%s/%s: allowed %v (want %v), calls %d (want %d)", token, ns, got, want, *calls, wantCalls)
		}
	}
	ask("a", "shop", true, 1)
	ask("a", "shop", true, 1)     // fresh: answered from the cache
	ask("a", "hidden", false, 2)  // another namespace
	ask("b", "shop", true, 3)     // another user
	now = now.Add(31 * time.Second) // expired
	ask("a", "shop", true, 4)
	if len(cache.seen) != 1 {
		t.Fatalf("expired decisions kept: %d", len(cache.seen))
	}
	if _, err := cache.Allowed(ctx, "expired", "shop"); !errors.Is(err, errUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
	if _, cached := cache.seen[cacheKey("expired", "shop")]; cached {
		t.Fatal("an error must not be cached")
	}
}

func TestCacheKeyHidesTheToken(t *testing.T) {
	key := cacheKey("secret-token", "shop")
	if len(key) != 64+len("/shop") || key[:6] == "secret" {
		t.Fatalf("key %q must be a hash", key)
	}
}
