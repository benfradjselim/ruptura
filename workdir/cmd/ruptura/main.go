package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/alerter"
	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/api"
	apicontext "github.com/benfradjselim/ruptura/internal/context"
	"github.com/benfradjselim/ruptura/internal/correlator"
	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/fusion"
	"github.com/benfradjselim/ruptura/internal/ingest"
	pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
)

const version = "6.1.0"

// Config holds all runtime configuration parsed from CLI flags.
type Config struct {
	Port        int
	OTLPPort    int
	StoragePath string
	APIKey      string
	ShowVersion bool
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("ruptura", flag.ContinueOnError)
	cfg := Config{}
	fs.IntVar(&cfg.Port, "port", 8080, "HTTP port")
	fs.IntVar(&cfg.OTLPPort, "otlp-port", 4317, "OTLP ingest HTTP port")
	fs.StringVar(&cfg.StoragePath, "storage", "/var/lib/ruptura/data", "storage directory")
	fs.StringVar(&cfg.APIKey, "api-key", "", "API bearer token")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	err := fs.Parse(args)
	return cfg, err
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Printf("ruptura v%s\n", version)
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runWithContext(ctx, cfg); err != nil {
		logger.Default.Error("server error", "err", err)
		os.Exit(1)
	}
}

// run starts the server with a signal-based context (used by main).
func run(cfg Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, cfg)
}

// burstLogSink wraps a BurstDetector to satisfy the ingest.LogSink interface.
// It classifies log lines by scanning for "error" or "warn" level keywords.
type burstLogSink struct {
	detector *correlator.BurstDetector
}

func (b *burstLogSink) IngestLine(service string, line []byte, ts time.Time) {
	lower := bytes.ToLower(line)
	switch {
	case bytes.Contains(lower, []byte("error")):
		b.detector.Observe(service, "error", ts)
	case bytes.Contains(lower, []byte("warn")):
		b.detector.Observe(service, "warn", ts)
	}
}

// runWithContext is the testable entrypoint — it uses the provided context for shutdown.
func runWithContext(ctx context.Context, cfg Config) error {
	logger.Default.Info("ruptura starting", "version", version, "port", cfg.Port)

	store, err := storage.Open(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("open storage failed: %w", err)
	}
	defer store.Close()

	bus := eventbus.NewWithKafka(ctx, os.Getenv("KAFKA_BROKERS"), "ruptura")
	defer bus.Close()

	// --- core pipeline ---
	pipelineEngine := pipelinemetrics.NewEngine()
	burstDet := correlator.NewBurstDetector(256)
	logSink := &burstLogSink{detector: burstDet}
	ingestEngine := ingest.New(pipelineEngine, logSink, nil)
	fusionEngine := fusion.NewEngine()
	analyzerEngine := analyzer.NewAnalyzer()

	// Pipe burst events into fusion as logR
	go fusionEngine.StartLogWatcher(ctx, burstDet.Events())

	// Start OTLP ingest listener
	otlpAddr := fmt.Sprintf(":%d", cfg.OTLPPort)
	if err := ingestEngine.StartHTTP(otlpAddr); err != nil {
		return fmt.Errorf("start OTLP ingest failed: %w", err)
	}
	defer ingestEngine.Stop(context.Background()) //nolint:errcheck

	// 15-second analyzer ticker: pipeline → analyzer → store → fusion
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				for _, host := range pipelineEngine.AllHosts() {
					rawMetrics := pipelineEngine.LatestByHost(host)
					if len(rawMetrics) == 0 {
						continue
					}
					ref := models.WorkloadRefFromHost(host)
					snap := analyzerEngine.Update(ref, rawMetrics)
					store.StoreSnapshot(snap)

					// Feed metricR into fusion using the rupture index of the primary metric.
					// We use whichever metric is available; cpu_percent is preferred.
					metricName := pickPrimaryMetric(rawMetrics)
					if metricName != "" {
						if r, err := pipelineEngine.RuptureIndex(host, metricName); err == nil {
							fusionEngine.SetMetricR(host, r, now)
						}
					}
				}
			}
		}
	}()

	actionEngine, err := engine.New(nil, bus)
	if err != nil {
		return fmt.Errorf("init action engine failed: %w", err)
	}

	// Forward critical anomaly events from the alerter channel to the action engine.
	al := alerter.NewAlerter(256)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case alert := <-al.Alerts():
				if alert.Severity == models.SeverityCritical {
					ev := models.AnomalyEvent{
						Host:      alert.Host,
						Metric:    alert.Metric,
						Value:     alert.Value,
						Score:     alert.Value / (alert.Threshold + 1e-9),
						Severity:  models.SeverityCritical,
						Timestamp: alert.CreatedAt,
					}
					if _, err := actionEngine.RecommendFromAnomaly(ev); err != nil {
						logger.Default.Warn("action recommend failed", "err", err)
					}
				}
			}
		}
	}()

	explainer := explain.NewEngine()
	ctxStore := apicontext.NewManualContextStore()
	detector := apicontext.NewDeploymentDetector()
	metrics := telemetry.NewRegistry(version)
	healthCheck := telemetry.NewHealthChecker()

	handlers := api.New(store, actionEngine, explainer, al, ctxStore, detector, metrics, healthCheck, cfg.APIKey)
	handlers.SetReady(true)

	router := handlers.NewRouter()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Default.Info("listening", "addr", srv.Addr, "otlp", otlpAddr)

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		logger.Default.Info("shutdown complete")
		return nil
	case err := <-errCh:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

// pickPrimaryMetric returns the best available metric name for RuptureIndex computation.
func pickPrimaryMetric(metrics map[string]float64) string {
	preferred := []string{"cpu_percent", "memory_percent", "latency_p99", "error_rate", "request_rate"}
	for _, name := range preferred {
		if _, ok := metrics[name]; ok {
			return name
		}
	}
	// fallback: any metric
	for name := range metrics {
		return name
	}
	return ""
}

