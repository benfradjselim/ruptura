package orchestrator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/internal/alerter"
	"github.com/benfradjselim/ohe/internal/analyzer"
	"github.com/benfradjselim/ohe/internal/api"
	"github.com/benfradjselim/ohe/internal/collector"
	"github.com/benfradjselim/ohe/internal/predictor"
	"github.com/benfradjselim/ohe/internal/processor"
	"github.com/benfradjselim/ohe/internal/storage"
	"github.com/benfradjselim/ohe/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// Config holds all runtime configuration
type Config struct {
	Mode            string        `yaml:"mode"`             // "agent" or "central"
	Host            string        `yaml:"host"`             // hostname override
	Port            int           `yaml:"port"`             // HTTP port
	StoragePath     string        `yaml:"storage_path"`     // Badger directory
	CentralURL      string        `yaml:"central_url"`      // agent→central endpoint
	JWTSecret       string        `yaml:"jwt_secret"`
	AuthEnabled     bool          `yaml:"auth_enabled"`
	CollectInterval time.Duration `yaml:"collect_interval"` // default 15s
	BufferSize      int           `yaml:"buffer_size"`      // circular buffer size
	AllowedOrigins  []string      `yaml:"allowed_origins"`  // CORS origins; empty = wildcard
}

// DefaultConfig returns sensible production defaults
func DefaultConfig() Config {
	hostname, _ := os.Hostname()
	return Config{
		Mode:            "central",
		Host:            hostname,
		Port:            8080,
		StoragePath:     "/var/lib/ohe/data",
		JWTSecret:       "change-me-in-production",
		AuthEnabled:     false,
		CollectInterval: 15 * time.Second,
		BufferSize:      10000,
	}
}

// Engine is the main orchestrator that wires all internal components
type Engine struct {
	cfg             Config
	store           *storage.Store
	proc            *processor.Processor
	ana             *analyzer.Analyzer
	pred            *predictor.Predictor
	alrt            *alerter.Alerter
	sysColl         *collector.SystemCollector
	containerColl   *collector.ContainerCollector
	logColl         *collector.LogCollector
	server          *http.Server
	handlers        *api.Handlers
	wg              sync.WaitGroup
	cancel          context.CancelFunc
}

// New creates a fully-wired engine
func New(cfg Config) (*Engine, error) {
	if cfg.AuthEnabled && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("auth_enabled=true requires jwt_secret to be set (use --jwt-secret or OHE_JWT_SECRET env var)")
	}
	if cfg.AuthEnabled && cfg.JWTSecret == "change-me-in-production" {
		return nil, fmt.Errorf("jwt_secret must be changed from the default value before enabling auth")
	}
	if err := os.MkdirAll(cfg.StoragePath, 0o750); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	store, err := storage.Open(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	proc := processor.NewProcessor(cfg.BufferSize)
	ana := analyzer.NewAnalyzer()
	pred := predictor.NewPredictor()
	alrt := alerter.NewAlerter(1000)
	sysColl := collector.NewSystemCollector(cfg.Host)
	containerColl := collector.NewContainerCollector(cfg.Host)
	logColl := collector.NewLogCollector(cfg.Host, nil) // nil = default log sources

	// Seed admin user on first boot if no users exist
	if err := seedAdminIfEmpty(store); err != nil {
		log.Printf("[ohe] admin seed warning: %v", err)
	}

	handlers := api.NewHandlers(store, proc, ana, pred, alrt, cfg.JWTSecret, cfg.AuthEnabled, cfg.AllowedOrigins)
	router := api.NewRouter(handlers, cfg.JWTSecret, cfg.AuthEnabled, cfg.AllowedOrigins)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Engine{
		cfg:           cfg,
		store:         store,
		proc:          proc,
		ana:           ana,
		pred:          pred,
		alrt:          alrt,
		sysColl:       sysColl,
		containerColl: containerColl,
		logColl:       logColl,
		server:        srv,
		handlers:      handlers,
	}, nil
}

// Run starts the engine and blocks until ctx is cancelled
func (e *Engine) Run(ctx context.Context) error {
	ctx, e.cancel = context.WithCancel(ctx)

	// Start alert log goroutine
	e.wg.Add(1)
	go e.logAlerts(ctx)

	// Start GC goroutine
	e.wg.Add(1)
	go e.runGC(ctx)

	// In agent mode, collect and push to central
	// In central mode, collect locally AND serve API
	switch e.cfg.Mode {
	case "agent":
		e.wg.Add(1)
		go e.collectAndPush(ctx)
		log.Printf("[agent] started on %s, pushing to %s every %s", e.cfg.Host, e.cfg.CentralURL, e.cfg.CollectInterval)
	default: // central
		e.wg.Add(1)
		go e.collectLocally(ctx)
		log.Printf("[central] started on :%d", e.cfg.Port)
	}

	// HTTP server (both modes expose API)
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[ohe] HTTP server listening on :%d", e.cfg.Port)
		e.handlers.SetReady(true)
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			e.handlers.SetReady(false)
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("[ohe] shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = e.server.Shutdown(shutCtx)
	case err := <-errCh:
		e.cancel()
		return err
	}

	e.wg.Wait()
	return e.store.Close()
}

// collectLocally runs the collection loop in central mode
func (e *Engine) collectLocally(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.cfg.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics, err := e.sysColl.Collect()
			if err != nil {
				log.Printf("[collector] system: %v", err)
				continue
			}

			// Container metrics (best-effort)
			if containerMetrics, err := e.containerColl.Collect(); err == nil {
				metrics = append(metrics, containerMetrics...)
			}

			// Log-derived metrics: inject error_rate and timeout_rate
			if logEntries, err := e.logColl.Collect(); err == nil && len(logEntries) > 0 {
				errRate := collector.ErrorRate(logEntries)
				now := time.Now()
				metrics = append(metrics,
					models.Metric{Name: "error_rate", Value: errRate, Timestamp: now, Host: e.cfg.Host},
					models.Metric{Name: "timeout_rate", Value: 0, Timestamp: now, Host: e.cfg.Host},
				)
			}

			e.proc.Ingest(metrics)

			// Persist to storage
			for _, m := range metrics {
				if err := e.store.SaveMetric(m.Host, m.Name, m.Value, m.Timestamp); err != nil {
					log.Printf("[store] SaveMetric %s/%s: %v", m.Host, m.Name, err)
				}
			}

			// Compute and store KPIs
			mmap := e.buildMetricsMap(e.cfg.Host)
			snapshot := e.ana.Update(e.cfg.Host, mmap)
			now := time.Now()
			for kpiName, kpiVal := range map[string]float64{
				"stress":    snapshot.Stress.Value,
				"fatigue":   snapshot.Fatigue.Value,
				"mood":      snapshot.Mood.Value,
				"pressure":  snapshot.Pressure.Value,
				"humidity":  snapshot.Humidity.Value,
				"contagion": snapshot.Contagion.Value,
			} {
				if err := e.store.SaveKPI(e.cfg.Host, kpiName, kpiVal, now); err != nil {
					log.Printf("[store] SaveKPI %s/%s: %v", e.cfg.Host, kpiName, err)
				}
			}

			// Feed predictor
			for _, m := range metrics {
				e.pred.Feed(m.Host, m.Name, m.Value, now)
			}
			e.pred.Feed(e.cfg.Host, "stress", snapshot.Stress.Value, now)
			e.pred.Feed(e.cfg.Host, "fatigue", snapshot.Fatigue.Value, now)

			// Evaluate alerts
			kpiMap := map[string]float64{
				"stress":    snapshot.Stress.Value,
				"fatigue":   snapshot.Fatigue.Value,
				"mood":      snapshot.Mood.Value,
				"pressure":  snapshot.Pressure.Value,
				"humidity":  snapshot.Humidity.Value,
				"contagion": snapshot.Contagion.Value,
			}
			for _, m := range metrics {
				kpiMap[m.Name] = m.Value
			}
			e.alrt.Evaluate(e.cfg.Host, kpiMap)
		}
	}
}

// collectAndPush runs the agent collection loop, pushing to central
func (e *Engine) collectAndPush(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.cfg.CollectInterval)
	defer ticker.Stop()
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics, err := e.sysColl.Collect()
			if err != nil {
				log.Printf("[agent] collect error: %v", err)
				continue
			}
			if containerMetrics, err := e.containerColl.Collect(); err == nil {
				metrics = append(metrics, containerMetrics...)
			}
			if logEntries, err := e.logColl.Collect(); err == nil && len(logEntries) > 0 {
				errRate := collector.ErrorRate(logEntries)
				now := time.Now()
				metrics = append(metrics,
					models.Metric{Name: "error_rate", Value: errRate, Timestamp: now, Host: e.cfg.Host},
					models.Metric{Name: "timeout_rate", Value: 0, Timestamp: now, Host: e.cfg.Host},
				)
			}

			batch := models.MetricBatch{
				AgentID:   e.cfg.Host,
				Host:      e.cfg.Host,
				Metrics:   metrics,
				Timestamp: time.Now(),
			}

			if err := pushBatch(ctx, client, e.cfg.CentralURL+"/api/v1/ingest", batch); err != nil {
				log.Printf("[agent] push error: %v", err)
			}
		}
	}
}

func pushBatch(ctx context.Context, client *http.Client, url string, batch models.MetricBatch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck — drain for connection reuse
	if resp.StatusCode >= 400 {
		return fmt.Errorf("central returned %d", resp.StatusCode)
	}
	return nil
}

func (e *Engine) logAlerts(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case alert := <-e.alrt.Alerts():
			log.Printf("[ALERT] [%s] [%s] %s — %s=%.4f (threshold=%.2f)",
				alert.Severity, alert.Host, alert.Description,
				alert.Metric, alert.Value, alert.Threshold)
		}
	}
}

func (e *Engine) runGC(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.store.RunGC(); err != nil {
				// ErrNoRewrite is expected when nothing to GC
				log.Printf("[gc] %v", err)
			}
		}
	}
}

// seedAdminIfEmpty creates the first admin user when the store has no users.
// The generated password is printed to stdout and logged at startup.
func seedAdminIfEmpty(store *storage.Store) error {
	count := 0
	_ = store.ListUsers(func([]byte) error {
		count++
		return nil
	})
	if count > 0 {
		return nil
	}

	// Generate a random 16-byte password
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate password: %w", err)
	}
	password := hex.EncodeToString(b)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		ID:       "admin",
		Username: "admin",
		Password: string(hash),
		Role:     "admin",
	}
	if err := store.SaveUser("admin", user); err != nil {
		return fmt.Errorf("save admin: %w", err)
	}

	log.Printf("╔══════════════════════════════════════════════════╗")
	log.Printf("║  FIRST BOOT — admin credentials generated         ║")
	log.Printf("║  Username : admin                                  ║")
	log.Printf("║  Password : %-35s ║", password)
	log.Printf("║  Change this password immediately after login!     ║")
	log.Printf("╚══════════════════════════════════════════════════╝")
	return nil
}

func (e *Engine) buildMetricsMap(host string) map[string]float64 {
	names := []string{
		"cpu_percent", "memory_percent", "disk_percent",
		"load_avg_1", "error_rate", "timeout_rate",
		"request_rate", "uptime_seconds",
	}
	m := make(map[string]float64, len(names))
	for _, name := range names {
		if v, ok := e.proc.GetNormalized(host, name); ok {
			m[name] = v
		}
	}
	return m
}
