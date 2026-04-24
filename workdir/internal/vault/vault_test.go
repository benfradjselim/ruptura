package vault_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/internal/vault"
)

func vaultResponse(data map[string]string) interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"data":     data,
			"metadata": map[string]interface{}{"version": 1},
		},
	}
}

func stubVault(t *testing.T, secrets map[string]map[string]string) (*vault.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// Strip leading /v1/ and ?version=0
		path := r.URL.Path[4:] // remove "/v1/"
		data, ok := secrets[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(vaultResponse(data))
	}))
	t.Cleanup(srv.Close)
	c, err := vault.New(vault.Config{Addr: srv.URL, Token: "test-token"})
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}
	return c, srv
}

func TestSecretFound(t *testing.T) {
	c, _ := stubVault(t, map[string]map[string]string{
		"ohe/jwt_secret": {"value": "super-secret-jwt"},
	})
	val, err := c.Secret(context.Background(), "ohe/jwt_secret", "value")
	if err != nil {
		t.Fatalf("Secret: %v", err)
	}
	if val != "super-secret-jwt" {
		t.Errorf("got %q; want super-secret-jwt", val)
	}
}

func TestSecretCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(vaultResponse(map[string]string{"value": "cached-val"}))
	}))
	t.Cleanup(srv.Close)

	c, _ := vault.New(vault.Config{Addr: srv.URL, Token: "tok", CacheTTL: time.Minute})
	ctx := context.Background()
	_, _ = c.Secret(ctx, "path/secret", "value")
	_, _ = c.Secret(ctx, "path/secret", "value")
	if calls != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", calls)
	}
}

func TestSecretNotFound(t *testing.T) {
	c, _ := stubVault(t, map[string]map[string]string{})
	_, err := c.Secret(context.Background(), "missing/secret", "value")
	if err == nil {
		t.Error("expected error for missing secret")
	}
}

func TestSecretWrongKey(t *testing.T) {
	c, _ := stubVault(t, map[string]map[string]string{
		"ohe/db": {"password": "dbpass"},
	})
	_, err := c.Secret(context.Background(), "ohe/db", "nonexistent-key")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestSecretInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "valid" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		json.NewEncoder(w).Encode(vaultResponse(map[string]string{"value": "ok"}))
	}))
	t.Cleanup(srv.Close)
	c, _ := vault.New(vault.Config{Addr: srv.URL, Token: "bad-token"})
	_, err := c.Secret(context.Background(), "any/path", "value")
	if err == nil {
		t.Error("expected permission denied error")
	}
}

func TestInvalidate(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(vaultResponse(map[string]string{"value": "val"}))
	}))
	t.Cleanup(srv.Close)

	c, _ := vault.New(vault.Config{Addr: srv.URL, Token: "tok", CacheTTL: time.Hour})
	ctx := context.Background()
	_, _ = c.Secret(ctx, "p/s", "value")
	c.Invalidate("p/s", "value")
	_, _ = c.Secret(ctx, "p/s", "value")
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls after invalidation, got %d", calls)
	}
}

func TestLoader(t *testing.T) {
	c, _ := stubVault(t, map[string]map[string]string{
		"ohe/jwt": {"secret": "from-vault"},
	})
	loader := c.Loader(context.Background(), "ohe/jwt", "secret")

	// When existing is set, it wins
	if got := loader("already-set"); got != "already-set" {
		t.Errorf("loader with existing: got %q", got)
	}
	// When empty, fetches from Vault
	if got := loader(""); got != "from-vault" {
		t.Errorf("loader without existing: got %q", got)
	}
}

func TestNewMissingAddr(t *testing.T) {
	_, err := vault.New(vault.Config{Token: "tok"})
	if err == nil {
		t.Error("expected error for missing Addr")
	}
}

func TestNewMissingToken(t *testing.T) {
	_, err := vault.New(vault.Config{Addr: "http://vault:8200"})
	if err == nil {
		t.Error("expected error for missing Token")
	}
}
