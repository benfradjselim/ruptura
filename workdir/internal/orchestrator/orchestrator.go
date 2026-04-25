package orchestrator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/internal/alerter"
	"github.com/benfradjselim/kairo-core/internal/analyzer"
	"github.com/benfradjselim/kairo-core/internal/api"
	"github.com/benfradjselim/kairo-core/internal/billing"
	"github.com/benfradjselim/kairo-core/internal/collector"
	"github.com/benfradjselim/kairo-core/internal/grpcserver"
	"github.com/benfradjselim/kairo-core/internal/predictor"
	"github.com/benfradjselim/kairo-core/internal/processor"
	"github.com/benfradjselim/kairo-core/internal/receiver"
	"github.com/benfradjselim/kairo-core/internal/storage"
	"github.com/benfradjselim/kairo-core/pkg/logger"
	"github.com/benfradjselim/kairo-core/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// PredictorConfig holds CA-ILR tuning knobs. All fields are optional — omitting
// them from the YAML file leaves the v5.0 defaults in place.
type PredictorConfig struct {
	StableWindow     time.Duration `yaml:"stable_window"`    // ILR stable model window; default 60m → λ=0.995
	BurstWindow      time.Duration `yaml:"burst_window"`     // ILR burst model window;  default 5m  → λ=0.80
	RuptureThreshold float64       `yaml:"rupture_threshold"` // R > threshold → ExponentialFailure; default 3.0
}

// FatigueConfig holds dissipative-fatigue tuning knobs. Optional — defaults to v5.0 spec.
type FatigueConfig struct {
	RThreshold float64 `yaml:"r_threshold"` // stress floor; default 0.3
	Lambda     float64 `yaml:"lambda"`      // dissipation per 15 s tick; default 0.05
}

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
	DogStatsDAddr   string        `yaml:"dogstatsd_addr"`   // UDP addr for DogStatsD; empty = disabled
	TLSCertFile       string          `yaml:"tls_cert"`            // path to TLS certificate (PEM); enables HTTPS when both set
	TLSKeyFile        string          `yaml:"tls_key"`             // path to TLS private key (PEM)
	ReplicaURL        string          `yaml:"replica_url"`         // Litestream replica URL (s3://bucket/path, gcs://, etc.); empty = no replication
	BillingWebhookURL string          `yaml:"billing_webhook_url"` // optional webhook for usage metering (Stripe, Lago, etc.)
	GRPCAddr          string          `yaml:"grpc_addr"`           // gRPC agent ingest address, e.g. ":9090"; empty = disabled
	Predictor         PredictorConfig `yaml:"predictor"`           // CA-ILR tuning (v5.0); all fields optional
	Fatigue           FatigueConfig   `yaml:"fatigue"`             // dissipative-fatigue tuning (v5.0); all fields optional
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
		DogStatsDAddr:   ":8125",
		Predictor: PredictorConfig{
			StableWindow:     60 * time.Minute,
			BurstWindow:      5 * time.Minute,
			RuptureThreshold: 3.0,
		},
		Fatigue: FatigueConfig{
			RThreshold: 0.3,
			Lambda:     0.05,
		},
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
	meter           *billing.Meter
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

	// Apply v5.0 config knobs (defaults are set if YAML fields are zero-valued).
	if cfg.Fatigue.RThreshold != 0 || cfg.Fatigue.Lambda != 0 {
		rThreshold := cfg.Fatigue.RThreshold
		lambda := cfg.Fatigue.Lambda
		if rThreshold == 0 {
			rThreshold = 0.3
		}
		if lambda == 0 {
			lambda = 0.05
		}
		ana.SetDefaultFatigueConfig(rThreshold, lambda)
	}
	if cfg.Predictor.RuptureThreshold != 0 {
		pred.SetRuptureThreshold(cfg.Predictor.RuptureThreshold)
	}
	meter := billing.New(cfg.BillingWebhookURL, 10000, time.Minute)
	sysColl := collector.NewSystemCollector(cfg.Host)
	containerColl := collector.NewContainerCollector(cfg.Host)
	logColl := collector.NewLogCollector(cfg.Host, nil) // nil = default log sources

	// Seed admin user on first boot if no users exist
	if err := seedAdminIfEmpty(store); err != nil {
		logger.Default.Warn("admin seed warning", "err", err)
	}

	handlers := api.NewHandlers(store, proc, ana, pred, alrt, cfg.Host, cfg.JWTSecret, cfg.AuthEnabled, cfg.AllowedOrigins)

	// Wire API key lookup so AuthMiddleware can validate ohe_* tokens.
	// The lookup scans the key's org (encoded in its prefix) to find and verify the key.
	api.SetAPIKeyLookup(func(_ string, rawKey string) (*api.JWTClaims, bool) {
		// The org is unknown at lookup time — we scan all orgs' "ak:" namespaces.
		// In practice, the key prefix encodes enough entropy that collisions are impossible.
		// For high-scale deployments, include the orgID in the key itself (e.g. ohe_{orgSlug}_{secret}).
		// For now, scan the default org first, then all registered orgs.
		orgs, _ := store.ListOrgIDs()
		orgs = append([]string{"default"}, orgs...)
		for _, orgID := range orgs {
			os := store.ForOrg(orgID)
			if claims, ok := api.ValidateAPIKey(os, rawKey); ok {
				return claims, true
			}
		}
		return nil, false
	})

	// Wire JWT revocation checker so AuthMiddleware rejects logged-out tokens.
	api.SetTokenRevokedChecker(store.IsTokenRevoked)

	// Wire billing meter into handlers so ingest/predict events are metered.
	handlers.SetUsageRecorder(func(orgID, eventType string, value float64) {
		meter.Record(orgID, billing.EventType(eventType), value)
	})

	// Log replication status. The Go process itself does not run Litestream —
	// it must be deployed as a sidecar container that replicates Badger's
	// data directory to the configured replica URL (S3, GCS, Azure Blob, SFTP).
	// See deploy/central-deployment.yaml for the sidecar spec.
	if cfg.ReplicaURL != "" {
		logger.Default.Info("HA replication configured", "replica_url", cfg.ReplicaURL, "note", "Litestream sidecar required")
	} else {
		logger.Default.Warn("no replica_url configured — single-node mode, data loss on pod restart")
	}

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
		meter:         meter,
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

	// Start billing meter flush goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.meter.Run(ctx)
	}()

	// Start gRPC agent ingest server (optional — enabled when GRPCAddr is set)
	if e.cfg.GRPCAddr != "" {
		grpcSrv, err := grpcserver.New(e.store, grpcserver.Config{
			TLSCert:    e.cfg.TLSCertFile,
			TLSKey:     e.cfg.TLSKeyFile,
			DefaultOrg: "default",
		})
		if err != nil {
			return fmt.Errorf("grpc: %w", err)
		}
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			if err := grpcSrv.Serve(ctx, e.cfg.GRPCAddr); err != nil {
				logger.Default.Error("grpc serve", "err", err)
			}
		}()
	}

	// Start GC goroutine
	e.wg.Add(1)
	go e.runGC(ctx)

	// Start retention/downsampling compaction goroutine
	e.wg.Add(1)
	go e.runCompaction(ctx)

	// In agent mode, collect and push to central
	// In central mode, collect locally AND serve API
	switch e.cfg.Mode {
	case "agent":
		e.wg.Add(1)
		go e.collectAndPush(ctx)
		logger.Default.Info("agent started", "host", e.cfg.Host, "central_url", e.cfg.CentralURL, "interval", e.cfg.CollectInterval)
	default: // central
		e.wg.Add(1)
		go e.collectLocally(ctx)
		logger.Default.Info("central started", "port", e.cfg.Port)
	}

	// Start DogStatsD UDP receiver (drop-in for Datadog StatsD endpoint)
	if e.cfg.DogStatsDAddr != "" {
		bus := receiver.NewBus(e.store, e.handlers.TopologyAnalyzer())
		dsd := receiver.NewDogStatsDReceiver(e.cfg.DogStatsDAddr, e.store, bus, e.cfg.Host)
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			if err := dsd.Run(ctx); err != nil {
				logger.Default.Error("dogstatsd error", "err", err)
			}
		}()
	}

	// HTTP server (both modes expose API)
	// When --tls-cert and --tls-key are both provided the server uses HTTPS.
	// Providing only one of the two is a configuration error and causes an
	// immediate fatal shutdown.
	tlsEnabled := e.cfg.TLSCertFile != "" || e.cfg.TLSKeyFile != ""
	if tlsEnabled && (e.cfg.TLSCertFile == "" || e.cfg.TLSKeyFile == "") {
		return fmt.Errorf("TLS requires both --tls-cert and --tls-key; only one was provided")
	}

	errCh := make(chan error, 1)
	go func() {
		e.handlers.SetReady(true)
		var serveErr error
		if tlsEnabled {
			logger.Default.Info("HTTPS server started", "port", e.cfg.Port, "tls", true)
			serveErr = e.server.ListenAndServeTLS(e.cfg.TLSCertFile, e.cfg.TLSKeyFile)
		} else {
			logger.Default.Info("HTTP server started", "port", e.cfg.Port)
			serveErr = e.server.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			e.handlers.SetReady(false)
			errCh <- serveErr
		}
	}()

	select {
	case <-ctx.Done():
		logger.Default.Info("shutting down")
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
				logger.Default.Error("collector system error", "err", err)
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
					logger.Default.Error("SaveMetric failed", "host", m.Host, "metric", m.Name, "err", err)
				}
			}

			// Compute and store KPIs
			mmap := e.buildMetricsMap(e.cfg.Host)
			snapshot := e.ana.Update(e.cfg.Host, mmap)
			now := time.Now()
			for kpiName, kpiVal := range map[string]float64{
				"stress":       snapshot.Stress.Value,
				"fatigue":      snapshot.Fatigue.Value,
				"mood":         snapshot.Mood.Value,
				"pressure":     snapshot.Pressure.Value,
				"humidity":     snapshot.Humidity.Value,
				"contagion":    snapshot.Contagion.Value,
				"resilience":   snapshot.Resilience.Value,
				"entropy":      snapshot.Entropy.Value,
				"velocity":     snapshot.Velocity.Value,
				"health_score": snapshot.HealthScore.Value,
			} {
				if err := e.store.SaveKPI(e.cfg.Host, kpiName, kpiVal, now); err != nil {
					logger.Default.Error("SaveKPI failed", "host", e.cfg.Host, "kpi", kpiName, "err", err)
				}
			}

			// Feed predictor
			for _, m := range metrics {
				e.pred.Feed(m.Host, m.Name, m.Value, now)
			}
			e.pred.Feed(e.cfg.Host, "stress", snapshot.Stress.Value, now)
			e.pred.Feed(e.cfg.Host, "fatigue", snapshot.Fatigue.Value, now)
			e.pred.Feed(e.cfg.Host, "health_score", snapshot.HealthScore.Value, now)
			e.pred.Feed(e.cfg.Host, "resilience", snapshot.Resilience.Value, now)

			// Evaluate alerts
			kpiMap := map[string]float64{
				"stress":       snapshot.Stress.Value,
				"fatigue":      snapshot.Fatigue.Value,
				"mood":         snapshot.Mood.Value,
				"pressure":     snapshot.Pressure.Value,
				"humidity":     snapshot.Humidity.Value,
				"contagion":    snapshot.Contagion.Value,
				"resilience":   snapshot.Resilience.Value,
				"entropy":      snapshot.Entropy.Value,
				"velocity":     snapshot.Velocity.Value,
				"health_score": snapshot.HealthScore.Value / 100.0,
			}
			for _, m := range metrics {
				kpiMap[m.Name] = m.Value
			}
			e.alrt.Evaluate(e.cfg.Host, kpiMap)

			// v5.0: CA-ILR rupture detection — fire ExponentialFailure alerts
			for _, ev := range e.pred.AcceleratingMetrics(e.cfg.Host) {
				e.alrt.FireRupture(ev)
			}
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
				logger.Default.Error("agent collect error", "err", err)
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
				logger.Default.Error("agent push error", "err", err)
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
	_, _ = io.Copy(io.Discard, resp.Body)
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
			logger.Default.Warn("alert fired",
				"severity", alert.Severity,
				"host", alert.Host,
				"description", alert.Description,
				"metric", alert.Metric,
				"value", alert.Value,
				"threshold", alert.Threshold,
			)
			// Fan out to configured notification channels (Slack, webhook, PagerDuty)
			e.handlers.DispatchAlertToChannels(alert)
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
				logger.Default.Warn("gc", "err", err)
			}
		}
	}
}

// runCompaction periodically downsizes raw time-series into 5m and 1h rollups.
func (e *Engine) runCompaction(ctx context.Context) {
	defer e.wg.Done()
	// Run after startup to compact any existing data, then every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logger.Default.Info("compaction started")
			e.store.Compact()
			logger.Default.Info("compaction done")
		}
	}
}

// seedAdminIfEmpty creates the first admin user when the store has no users.
// The generated password is printed to stdout and logged at startup.
// If OHE_ADMIN_PASSWORD is set, it is used as the admin password regardless
// of whether users already exist (allows forced password reset via env var).
func seedAdminIfEmpty(store *storage.Store) error {
	forcePwd := os.Getenv("OHE_ADMIN_PASSWORD")

	count := 0
	_ = store.ListUsers(func([]byte) error {
		count++
		return nil
	})
	if count > 0 && forcePwd == "" {
		return nil
	}

	// Use OHE_ADMIN_PASSWORD if set, otherwise generate a random one
	var password string
	if forcePwd != "" {
		password = forcePwd
	} else {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		password = hex.EncodeToString(b)
	}

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

	logger.Default.Warn("FIRST BOOT — admin credentials generated",
		"username", "admin",
		"password", password,
		"action", "change this password immediately after login",
	)
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
