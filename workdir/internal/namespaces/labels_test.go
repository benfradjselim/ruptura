package namespaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("continue") == "" {
			_, _ = w.Write([]byte(`{"metadata":{"continue":"next"},"items":[{"metadata":{"name":"shop","labels":{"ruptura.io/enabled":"true"}}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"metadata":{},"items":[{"metadata":{"name":"batch"}}]}`))
	}))
	defer server.Close()
	l := &Labels{apiBase: server.URL, token: "token", client: server.Client()}
	if _, ok := l.Of("shop"); ok {
		t.Fatal("labels must be unknown before the first refresh")
	}
	if err := l.refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if labels, ok := l.Of("shop"); !ok || labels["ruptura.io/enabled"] != "true" {
		t.Errorf("shop labels %v %v", labels, ok)
	}
	if _, ok := l.Of("batch"); !ok {
		t.Error("namespace of the second page is missing")
	}
	if _, ok := l.Of("gone"); ok {
		t.Error("unknown namespace reported as known")
	}
}

func TestRefreshKeepsLabelsOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	l := &Labels{apiBase: server.URL, token: "token", client: server.Client(), labels: map[string]map[string]string{"shop": {"a": "b"}}}
	if err := l.refresh(context.Background()); err == nil {
		t.Fatal("want an error on 403")
	}
	if _, ok := l.Of("shop"); !ok {
		t.Error("a failed refresh must keep the last labels")
	}
}
