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
	"github.com/benfradjselim/ruptura/internal/actions/providers"
	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/fusion"
	"github.com/benfradjselim/ruptura/internal/ingest"
	pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/internal/predictor"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
)

const version = "6.2.2"

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

// busSentimentSink publishes sentiment log counts to the event bus.
// Downstream subscribers can use these counts to compute the Sentiment KPI.
type busSentimentSink struct {
	bus eventbus.Bus
	ctx context.Context
}

func (s *busSentimentSink) UpdateSentiment(service string, positive, negative int) {
	_ = s.bus.Publish(s.ctx, "ruptura.sentiment.update", "", map[string]interface{}{
		"service":  service,
		"positive": positive,
		"negative": negative,
	})
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
	topoBuilder := correlator.NewTopologyBuilder()
	logSink := &burstLogSink{detector: burstDet}
	fusionEngine := fusion.NewEngine()
	predictorEngine := predictor.NewPredictor()
	sentSink := &busSentimentSink{bus: bus, ctx: ctx}
	ingestEngine := ingest.New(pipelineEngine, logSink, nil, sentSink, fusionEngine)
	analyzerEngine := analyzer.NewAnalyzer()
	analyzerEngine.SetTopology(topoBuilder)

	// Pipe burst events into fusion as logR
	go fusionEngine.StartLogWatcher(ctx, burstDet.Events())

	// Start OTLP ingest listener
	otlpAddr := fmt.Sprintf(":%d", cfg.OTLPPort)
	if err := ingestEngine.StartHTTP(otlpAddr); err != nil {
		return fmt.Errorf("start OTLP ingest failed: %w", err)
	}
	defer ingestEngine.Stop(context.Background()) //nolint:errcheck

	al := alerter.NewAlerter(256)
	metricsReg := telemetry.NewRegistry(version)
	explainer := explain.NewEngine()

	// 15-second analyzer ticker: pipeline → analyzer → store → fusion → predictor → explain
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

					// Feed metricR into fusion BEFORE storing snapshot so FusedR is current.
					metricName := pickPrimaryMetric(rawMetrics)
					var metricR float64
					if metricName != "" {
						if r, err := pipelineEngine.RuptureIndex(host, metricName); err == nil {
							metricR = r
							fusionEngine.SetMetricR(host, r, now)
						}
					}
					// Annotate snapshot with current FusedR before persisting.
					var fusedR float64
					if fr, _, err := fusionEngine.FusedR(host); err == nil {
						fusedR = fr
						snap.FusedRuptureIndex = fusedR
					}

					store.StoreSnapshot(snap)
					metricsReg.RecordKPISnapshot(snap)

					// Feed each raw metric into the predictor ensemble.
					for metric, val := range rawMetrics {
						predictorEngine.Feed(host, metric, val, now)
					}
					// Also feed the composite KPI signals.
					predictorEngine.Feed(host, "health_score", snap.HealthScore.Value, now)
					predictorEngine.Feed(host, "stress", snap.Stress.Value, now)
					predictorEngine.Feed(host, "fatigue", snap.Fatigue.Value, now)

					// Record an explain entry for any workload in warning or worse state.
					if fusedR >= 1.5 {
						rec := buildExplainRecord(host, snap, fusedR, metricR, fusionEngine, topoBuilder, now)
						explainer.Record(rec)
					}

					// Forward recent critical anomalies to the alerter for rule evaluation.
					for _, anom := range pipelineEngine.RecentAnomalies(host, now.Add(-15*time.Second)) {
						if anom.Severity == models.SeverityCritical {
							al.Evaluate(host, map[string]float64{anom.Metric: anom.Value})
						}
					}
				}
			}
		}
	}()

	// Try to build a real Kubernetes actuator (only works inside a K8s pod).
	// If not in-cluster, the provider silently no-ops instead of failing startup.
	var k8sActuator *providers.KubernetesActuator
	if a, err := providers.NewKubernetesActuator(); err == nil {
		k8sActuator = a
		logger.Default.Info("kubernetes actuator initialised — scale/restart/cordon actions enabled")
	} else {
		logger.Default.Info("kubernetes actuator unavailable (not in-cluster) — K8s actions will be queued only", "reason", err.Error())
	}
	_ = providers.NewKubernetesProviderWithActuator(k8sActuator) // registered; used by action dispatcher

	actionEngine, err := engine.New(nil, bus)
	if err != nil {
		return fmt.Errorf("init action engine failed: %w", err)
	}

	// Forward critical anomaly events from the alerter channel to the action engine.
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

	ctxStore := apicontext.NewManualContextStore()
	detector := apicontext.NewDeploymentDetector()
	healthCheck := telemetry.NewHealthChecker()

	handlers := api.New(store, actionEngine, explainer, al, predictorEngine, pipelineEngine, ctxStore, detector, metricsReg, healthCheck, cfg.APIKey)
	handlers.SetAnalyzer(analyzerEngine)
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
			logger.Default.Error("shutdown error", "err", err)
		}
		// NFR-05: flush all in-memory snapshots to BadgerDB before exit.
		if err := store.FlushSnapshots(); err != nil {
			logger.Default.Error("snapshot flush failed", "err", err)
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

// buildExplainRecord constructs a RuptureRecord from live pipeline state.
// Called from the 15-second ticker when FusedR >= 1.5.
func buildExplainRecord(
	host string,
	snap models.KPISnapshot,
	fusedR, metricR float64,
	fe *fusion.Engine,
	topo interface{ Edges() []models.ServiceEdge },
	now time.Time,
) explain.RuptureRecord {
	id := utils.GenerateID(8)

	// Collect contagion sources: upstream services with error rate > 10%.
	var contagionSources []string
	if snap.Contagion.Value >= 0.3 {
		for _, edge := range topo.Edges() {
			if edge.To == host && edge.Calls > 0 {
				errRate := float64(edge.Errors) / float64(edge.Calls)
				if errRate > 0.1 {
					contagionSources = append(contagionSources, edge.From)
				}
			}
		}
	}

	// Pull logR and traceR from fusion engine internals via Snapshot.
	fusionSnap := fe.Snapshot()
	_ = fusionSnap // FusedR already computed; we use snap.FusedRuptureIndex

	return explain.RuptureRecord{
		ID:               id,
		Host:             host,
		R:                fusedR,
		Confidence:       0.75,
		Timestamp:        now,
		FusedR:           fusedR,
		MetricR:          metricR,
		KPISnapshot:      snap,
		ContagionSources: contagionSources,
	}
}

