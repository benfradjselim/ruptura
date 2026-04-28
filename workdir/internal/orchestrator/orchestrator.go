package orchestrator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
)

// Config holds all runtime configuration for Ruptura.
type Config struct {
	Mode            string        `yaml:"mode"`
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	StoragePath     string        `yaml:"storage_path"`
	CentralURL      string        `yaml:"central_url"`
	JWTSecret       string        `yaml:"jwt_secret"`
	AuthEnabled     bool          `yaml:"auth_enabled"`
	CollectInterval time.Duration `yaml:"collect_interval"`
	BufferSize      int           `yaml:"buffer_size"`
	AllowedOrigins  []string      `yaml:"allowed_origins"`
	DogStatsDAddr   string        `yaml:"dogstatsd_addr"`
	TLSCertFile     string        `yaml:"tls_cert"`
	TLSKeyFile      string        `yaml:"tls_key"`
	ReplicaURL      string        `yaml:"replica_url"`
	APIKey          string        `yaml:"api_key"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	secret := generateSecret(32)
	h, _ := os.Hostname()
	if h == "" {
		h = "localhost"
	}
	return Config{
		Mode:            "central",
		Host:            h,
		Port:            8080,
		StoragePath:     "/var/lib/ruptura/data",
		CentralURL:      "http://localhost:8080",
		JWTSecret:       secret,
		CollectInterval: 15 * time.Second,
		BufferSize:      10000,
		DogStatsDAddr:   "",
	}
}

// Engine is the v6 Ruptura runtime.
type Engine struct {
	store *storage.Store
	cfg   Config
}

// New creates a new Engine, validating the config and opening storage.
func New(cfg Config) (*Engine, error) {
	if cfg.AuthEnabled {
		if cfg.JWTSecret == "" {
			return nil, errors.New("auth_enabled requires a non-empty jwt_secret")
		}
		if cfg.JWTSecret == "change-me-in-production" {
			return nil, errors.New("jwt_secret must be changed from the default value")
		}
	}

	if cfg.StoragePath == "" {
		cfg.StoragePath = "/var/lib/ruptura/data"
	}

	store, err := storage.Open(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	if err := seedAdminIfEmpty(store); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("seed admin: %w", err)
	}

	return &Engine{store: store, cfg: cfg}, nil
}

// Run starts the Ruptura server. It blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	defer e.store.Close()

	if (e.cfg.TLSCertFile == "") != (e.cfg.TLSKeyFile == "") {
		return errors.New("TLS requires both cert and key files to be set")
	}

	if e.cfg.Mode == "agent" {
		return e.runAgent(ctx)
	}
	return e.runCentral(ctx)
}

func (e *Engine) runCentral(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ready","message":"ruptura v6"}`)
	})

	addr := fmt.Sprintf(":%d", e.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := &http.Server{Handler: mux}
	logger.Default.Info("ruptura central listening", "addr", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func (e *Engine) runAgent(ctx context.Context) error {
	client := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(e.cfg.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			batch := models.MetricBatch{
				AgentID:   e.cfg.Host,
				Host:      e.cfg.Host,
				Timestamp: time.Now(),
			}
			if err := pushBatch(ctx, client, e.cfg.CentralURL+"/api/v2/write", batch); err != nil {
				logger.Default.Warn("push batch failed", "err", err)
			}
		}
	}
}

// buildMetricsMap returns a map of current metric values for the host.
func (e *Engine) buildMetricsMap(host string) map[string]float64 {
	_ = host
	return map[string]float64{}
}

// seedAdminIfEmpty is a no-op in v6; auth is handled by the v6 API layer.
func seedAdminIfEmpty(store *storage.Store) error {
	_ = store
	return nil
}

// pushBatch serialises batch as JSON and POSTs it to url.
func pushBatch(ctx context.Context, client *http.Client, url string, batch models.MetricBatch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}
	return nil
}

func generateSecret(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("x", length*2)
	}
	return hex.EncodeToString(b)
}
