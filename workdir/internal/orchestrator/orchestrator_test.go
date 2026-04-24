package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != "central" {
		t.Errorf("default mode want 'central', got %q", cfg.Mode)
	}
	if cfg.Port != 8080 {
		t.Errorf("default port want 8080, got %d", cfg.Port)
	}
	if cfg.CollectInterval != 15*time.Second {
		t.Errorf("default collect interval want 15s, got %v", cfg.CollectInterval)
	}
	if cfg.BufferSize <= 0 {
		t.Errorf("default buffer size should be positive, got %d", cfg.BufferSize)
	}
	if cfg.JWTSecret == "" {
		t.Error("default JWT secret should not be empty")
	}
}

func TestNewEngine_AuthEnabledWithoutSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuthEnabled = true
	cfg.JWTSecret = ""
	cfg.StoragePath = t.TempDir()

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for auth_enabled with empty jwt_secret")
	}
}

func TestNewEngine_AuthEnabledWithDefaultSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuthEnabled = true
	cfg.JWTSecret = "change-me-in-production"
	cfg.StoragePath = t.TempDir()

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for auth_enabled with default jwt_secret")
	}
}

func TestNewEngine_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	cfg.Port = 0
	cfg.DogStatsDAddr = "" // disable UDP listener

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := eng.store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
}

func TestNewEngine_AuthEnabledWithCustomSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuthEnabled = true
	cfg.JWTSecret = "my-super-secret-key-longer-than-32chars"
	cfg.StoragePath = t.TempDir()
	cfg.DogStatsDAddr = ""

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error with valid auth config: %v", err)
	}
	_ = eng.store.Close()
}

func TestSeedAdminIfEmpty_SeedsThenSkips(t *testing.T) {
	dir := t.TempDir()

	// Import storage inline via New to get a store
	cfg := DefaultConfig()
	cfg.StoragePath = dir
	cfg.DogStatsDAddr = ""
	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer eng.store.Close()

	// Seeding happened inside New(); verify admin user exists
	count := 0
	_ = eng.store.ListUsers(func([]byte) error {
		count++
		return nil
	})
	if count == 0 {
		t.Fatal("expected admin user to be seeded on first boot")
	}

	// A second call to seedAdminIfEmpty should be a no-op (count > 0, no env var)
	before := count
	_ = seedAdminIfEmpty(eng.store)
	after := 0
	_ = eng.store.ListUsers(func([]byte) error {
		after++
		return nil
	})
	if after != before {
		t.Errorf("seedAdminIfEmpty should be idempotent; before=%d after=%d", before, after)
	}
}

func TestSeedAdminIfEmpty_UsesEnvPassword(t *testing.T) {
	t.Setenv("OHE_ADMIN_PASSWORD", "test-password-123")

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.StoragePath = dir
	cfg.DogStatsDAddr = ""
	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer eng.store.Close()

	count := 0
	_ = eng.store.ListUsers(func([]byte) error { count++; return nil })
	if count == 0 {
		t.Fatal("expected seeded user even with env password")
	}
}

func TestPushBatch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("want application/json Content-Type, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	if err := pushBatch(context.Background(), client, srv.URL, models.MetricBatch{AgentID: "test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushBatch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := pushBatch(context.Background(), client, srv.URL, models.MetricBatch{})
	if err == nil {
		t.Fatal("expected error for 5xx response")
	}
}

func TestPushBatch_NetworkError(t *testing.T) {
	client := &http.Client{Timeout: 100 * time.Millisecond}
	err := pushBatch(context.Background(), client, "http://127.0.0.1:1", models.MetricBatch{})
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestPushBatch_ContextCancelled(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := &http.Client{}
	err := pushBatch(ctx, client, srv.URL, models.MetricBatch{})
	close(done) // unblock handler so srv.Close() returns quickly
	srv.Close()
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestBuildMetricsMap_ReturnsMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	cfg.DogStatsDAddr = ""
	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer eng.store.Close()

	m := eng.buildMetricsMap(cfg.Host)
	if m == nil {
		t.Fatal("buildMetricsMap returned nil")
	}
	// No data fed yet — map should be empty or populated but never nil
}

func TestEngineRun_AgentMode(t *testing.T) {
	// Mock central server so collectAndPush has somewhere to POST
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.Mode = "agent"
	cfg.StoragePath = t.TempDir()
	cfg.Port = 0
	cfg.DogStatsDAddr = ""
	cfg.CentralURL = srv.URL
	cfg.CollectInterval = 50 * time.Millisecond

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New agent: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run agent: %v", err)
	}
}

func TestEngineRun_TLSHalfConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	cfg.DogStatsDAddr = ""
	cfg.TLSCertFile = "/tmp/cert.pem" // only cert, no key

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer eng.store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = eng.Run(ctx)
	if err == nil {
		t.Fatal("expected error for half-TLS config (cert without key)")
	}
}

func TestEngineRun_ShutdownClean(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping engine run test in CI — requires port binding")
	}
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	cfg.Port = 18765
	cfg.DogStatsDAddr = ""

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run should return cleanly after ctx timeout
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}
