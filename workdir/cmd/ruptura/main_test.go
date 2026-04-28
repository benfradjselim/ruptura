package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func TestParseFlags_Defaults(t *testing.T) {
	cfg, err := parseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 {
		t.Errorf("want 8080 got %d", cfg.Port)
	}
	if cfg.StoragePath != "/var/lib/ruptura/data" {
		t.Errorf("wrong default storage")
	}
	if cfg.APIKey != "" {
		t.Errorf("want empty api-key")
	}
	if cfg.ShowVersion {
		t.Error("show-version should default false")
	}
}

func TestParseFlags_Custom(t *testing.T) {
	cfg, err := parseFlags([]string{"--port=9090", "--storage=/tmp/test", "--api-key=secret"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9090 {
		t.Errorf("want 9090 got %d", cfg.Port)
	}
	if cfg.StoragePath != "/tmp/test" {
		t.Errorf("wrong storage")
	}
	if cfg.APIKey != "secret" {
		t.Errorf("wrong api key")
	}
}

func TestParseFlags_Version(t *testing.T) {
	cfg, err := parseFlags([]string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShowVersion {
		t.Error("want show-version true")
	}
}

func TestParseFlags_UnknownFlag(t *testing.T) {
	_, err := parseFlags([]string{"--unknown-flag=x"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestVersion_Constant(t *testing.T) {
	if version != "6.0.0" {
		t.Errorf("want version 6.0.0 got %s", version)
	}
}

func TestRunWithContext_InvalidStorage(t *testing.T) {
	ctx := context.Background()
	err := runWithContext(ctx, Config{StoragePath: "/proc/nonexistent/ruptura", Port: 0})
	if err == nil {
		t.Error("expected error for invalid storage path")
	}
}

// TestRunWithContext_ShutdownClean starts the server with a cancelled context
// and verifies runWithContext returns nil on clean shutdown.
func TestRunWithContext_ShutdownClean(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := Config{
		Port:        port,
		StoragePath: t.TempDir(),
	}

	done := make(chan error, 1)
	go func() { done <- runWithContext(ctx, cfg) }()

	// Give the server time to bind then cancel
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithContext returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWithContext did not return after context cancel")
	}
}

// TestRunWithContext_HealthEndpoint starts the server, hits /api/v2/health and
// /api/v2/metrics, then shuts down via context cancellation.
func TestRunWithContext_HealthEndpoint(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := Config{
		Port:        port,
		StoragePath: t.TempDir(),
	}

	done := make(chan error, 1)
	go func() { done <- runWithContext(ctx, cfg) }()

	// Wait for server
	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	var resp *http.Response
	for i := 0; i < 25; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err = http.Get(addr + "/api/v2/health")
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("server never came up: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health want 200 got %d", resp.StatusCode)
	}

	// Hit metrics endpoint
	resp2, err := http.Get(addr + "/api/v2/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("metrics want 200 got %d", resp2.StatusCode)
	}

	// Hit ready endpoint
	resp3, err := http.Get(addr + "/api/v2/ready")
	if err != nil {
		t.Fatalf("ready request failed: %v", err)
	}
	resp3.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithContext returned error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWithContext did not return after context cancel")
	}
}

// TestRunWithContext_WithAPIKey verifies the server starts with an API key configured.
func TestRunWithContext_WithAPIKey(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := Config{
		Port:        port,
		StoragePath: t.TempDir(),
		APIKey:      "test-secret",
	}

	done := make(chan error, 1)
	go func() { done <- runWithContext(ctx, cfg) }()

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 200; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err2 := http.Get(addr + "/api/v2/health")
		if err2 == nil {
			resp.Body.Close()
			break
		}
	}

	// Request without auth should return 401
	resp, err := http.Get(addr + "/api/v2/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 without auth, got %d", resp.StatusCode)
	}

	// Request with correct auth should return 200
	req, _ := http.NewRequest("GET", addr+"/api/v2/health", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("auth request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("want 200 with valid auth, got %d", resp2.StatusCode)
	}

	cancel()
	<-done
}

// TestRunWithContext_PortInUse covers the errCh path when ListenAndServe fails
// immediately because the port is already bound.
func TestRunWithContext_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	ctx := context.Background()
	cfg := Config{
		Port:        port,
		StoragePath: t.TempDir(),
	}
	err = runWithContext(ctx, cfg)
	if err == nil {
		t.Fatal("expected error when port is already in use")
	}
}

// TestRun_SIGTERM covers the run() wrapper by sending SIGTERM after the server starts.
func TestRun_SIGTERM(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := Config{
		Port:        port,
		StoragePath: t.TempDir(),
	}

	done := make(chan error, 1)
	go func() { done <- run(cfg) }()

	// Wait for the server to bind then signal shutdown
	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 30; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, e := http.Get(addr + "/api/v2/health")
		if e == nil {
			resp.Body.Close()
			break
		}
	}
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return after SIGTERM")
	}
}
