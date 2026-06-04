package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/alerter"
	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/api"
	apicontext "github.com/benfradjselim/ruptura/internal/context"
	"github.com/benfradjselim/ruptura/internal/correlator"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/actions/providers"
	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/internal/events"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/fusion"
	"github.com/benfradjselim/ruptura/internal/history"
	"github.com/benfradjselim/ruptura/internal/ingest"
	"github.com/benfradjselim/ruptura/internal/k8smetrics"
	pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/internal/predictor"
	"github.com/benfradjselim/ruptura/internal/scraper"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
)

const version = "7.0.27"

// Config holds all runtime configuration parsed from CLI flags.
type Config struct {
	Port          int
	OTLPPort      int
	StoragePath   string
	APIKey        string
	Edition       string // "community" (default) or "autopilot"
	ShowVersion   bool
	PrometheusURL string // cluster Prometheus URL; if set, seeded as a persistent datasource
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("ruptura", flag.ContinueOnError)
	cfg := Config{}
	fs.IntVar(&cfg.Port, "port", 8080, "HTTP port")
	fs.IntVar(&cfg.OTLPPort, "otlp-port", 4317, "OTLP ingest HTTP port")
	fs.StringVar(&cfg.StoragePath, "storage", "/var/lib/ruptura/data", "storage directory")
	fs.StringVar(&cfg.APIKey, "api-key", "", "API bearer token")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	fs.StringVar(&cfg.PrometheusURL, "prometheus-url", "", "cluster Prometheus URL to seed as default datasource")
	err := fs.Parse(args)
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("RUPTURA_API_KEY")
	}
	if cfg.PrometheusURL == "" {
		cfg.PrometheusURL = os.Getenv("RUPTURA_PROMETHEUS_URL")
	}
	if cfg.Edition == "" {
		if e := os.Getenv("RUPTURA_EDITION"); e != "" {
			cfg.Edition = e
		} else {
			cfg.Edition = "community"
		}
	}
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

	// Restore in-memory snapshots from BadgerDB so the analyzer sees prior state
	// immediately after restart (data continuity across pod restarts and upgrades).
	if n, err := store.LoadSnapshots(); err != nil {
		logger.Default.Warn("snapshot restore partial", "loaded", n, "err", err)
	} else {
		logger.Default.Info("snapshots restored", "count", n)
	}

	// Restore user-configured retention policy (applies TTL to all new writes).
	store.LoadRetentionConfig()

	// Periodic BadgerDB maintenance: value-log GC (reclaim deleted/expired vlog space)
	// + SST compaction (compact raw→5m→1h tiers, evict expired keys from SSTables).
	// GC runs every 10 min; compaction runs every 30 min.
	// Without compaction, TTL-expired SST entries accumulate on disk indefinitely.
	go func() {
		gcTicker := time.NewTicker(10 * time.Minute)
		compactTicker := time.NewTicker(30 * time.Minute)
		defer gcTicker.Stop()
		defer compactTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-gcTicker.C:
				for store.RunGC() == nil {
				}
			case <-compactTicker.C:
				store.Compact()
			}
		}
	}()

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
	ingestEngine.SetLogStore(store)
	analyzerEngine := analyzer.NewAnalyzer()
	analyzerEngine.SetTopology(topoBuilder)
	if raw := os.Getenv("RUPTURA_WORKLOAD_WEIGHTS"); raw != "" {
		var cfgs []models.SignalWeights
		if err := json.Unmarshal([]byte(raw), &cfgs); err == nil {
			analyzerEngine.SetWeightConfigs(cfgs)
			logger.Default.Info("loaded workload weight overrides", "count", len(cfgs))
		} else {
			logger.Default.Warn("RUPTURA_WORKLOAD_WEIGHTS parse error — using defaults", "err", err)
		}
	}

	// k8sKnown tracks only workloads discovered via the k8s informer. The
	// analyzer's workload registry also includes telemetry-driven workloads
	// (re-added by the Prometheus poller from historical data), so we cannot
	// use AllWorkloadRefs() for GC decisions.
	var (
		k8sKnown   = map[string]bool{}
		k8sKnownMu sync.RWMutex
	)
	registerK8s := func(ref models.WorkloadRef) {
		k8sKnownMu.Lock()
		k8sKnown[ref.Key()] = true
		if ref.Namespace != "" && ref.Name != "" {
			k8sKnown[ref.Namespace+"/"+ref.Name] = true
		}
		k8sKnownMu.Unlock()
		analyzerEngine.RegisterWorkload(ref)
	}
	unregisterK8s := func(ref models.WorkloadRef) {
		k8sKnownMu.Lock()
		delete(k8sKnown, ref.Key())
		if ref.Namespace != "" && ref.Name != "" {
			delete(k8sKnown, ref.Namespace+"/"+ref.Name)
		}
		k8sKnownMu.Unlock()
		analyzerEngine.UnregisterWorkload(ref)
	}

	// k8s workload auto-discovery — no-op when not running inside a cluster.
	var inf *discovery.Informer
	if disc, err := discovery.NewInformer(); err == nil {
		logger.Default.Info("k8s auto-discovery active — watching Deployments/StatefulSets/DaemonSets")
		inf = disc
		go disc.Run(ctx, registerK8s, unregisterK8s)

		// Workload GC: remove stale snapshots for workloads deleted from the cluster.
		// LoadSnapshots() restores ALL persisted snapshots on startup, including those
		// for workloads that were deleted while the engine was offline. The informer
		// only sends ADDED events for current workloads, so stale ones never get an
		// UnregisterWorkload call. This goroutine reconciles every 60s.
		//
		// We compare against k8sKnown (informer-only set) rather than
		// analyzerEngine.AllWorkloadRefs(), because the Prometheus scraper
		// re-registers deleted workloads that still have historical Prometheus
		// data — which would otherwise defeat the GC.
		go func() {
			// First tick after 30s — gives the informer LIST phase time to complete.
			timer := time.NewTimer(30 * time.Second)
			defer timer.Stop()
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				case <-ticker.C:
				}
				k8sKnownMu.RLock()
				current := make(map[string]bool, len(k8sKnown))
				for k := range k8sKnown {
					current[k] = true
				}
				k8sKnownMu.RUnlock()

				var purged int
				for _, snap := range store.AllSnapshots() {
					canonical := snap.Workload.Key()
					if canonical == "" || canonical == "default/host/" {
						canonical = snap.Host
					}
					if canonical == "" || current[canonical] || current[snap.Host] {
						continue
					}
					analyzerEngine.UnregisterWorkload(snap.Workload)
					store.DeleteSnapshot(snap.Host, canonical)
					purged++
				}
				if purged > 0 {
					logger.Default.Info("workload gc: removed stale workloads", "count", purged)
				}
			}
		}()
	} else {
		logger.Default.Info("k8s auto-discovery skipped (not in-cluster)", "reason", err.Error())
	}

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
	ingestEngine.SetIngestHook(metricsReg.IncIngestTotal)
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
					// When k8s discovery is active, skip Prometheus-scraped metrics for
					// workloads that are not in the informer's known set. This prevents
					// the Prometheus poller from re-registering deleted workloads via
					// historical Prometheus data and defeating the stale-workload GC.
					if inf != nil {
						parts := strings.SplitN(host, "/", 3)
						if len(parts) == 3 {
							kind := parts[1]
							if kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
								k8sKnownMu.RLock()
								known := k8sKnown[host]
								k8sKnownMu.RUnlock()
								if !known {
									continue
								}
							}
						} else if len(parts) == 1 {
							// Bare-host name (no namespace prefix). When k8s discovery has
							// populated k8sKnown, skip hosts that don't match any known
							// workload — prevents historical Prometheus metrics for deleted
							// workloads from continuously re-registering stale entries.
							k8sKnownMu.RLock()
							knownCount := len(k8sKnown)
							known := k8sKnown[host] || k8sKnown["default/"+host]
							k8sKnownMu.RUnlock()
							if knownCount > 0 && !known {
								continue
							}
						}
					}
					rawMetrics := pipelineEngine.LatestByHost(host)
					if len(rawMetrics) == 0 {
						continue
					}
					ref := models.WorkloadRefFromKey(host)
					snap := analyzerEngine.Update(ref, rawMetrics)

					// Feed each raw metric into the predictor ensemble first so that
					// health_score CAILR is current when we compute metricR below.
					for metric, val := range rawMetrics {
						predictorEngine.Feed(host, metric, val, now)
					}
					// Also feed the composite KPI signals.
					predictorEngine.Feed(host, "health_score", snap.HealthScore.Value, now)
					predictorEngine.Feed(host, "stress", snap.Stress.Value, now)
					predictorEngine.Feed(host, "fatigue", snap.Fatigue.Value, now)

					// Feed metricR into fusion BEFORE storing snapshot so FusedR is current.
					// Primary: CAILR on the raw pipeline metric (fatigue/stress/cpu_percent…)
					// Secondary: CAILR on health_score decline from the predictor ensemble —
					// fires when health_score is dropping faster recently than its long-term
					// trend, which is a leading indicator of impending failure.
					metricName := pickPrimaryMetric(rawMetrics)
					var metricR float64
					if metricName != "" {
						if r, err := pipelineEngine.RuptureIndex(host, metricName); err == nil {
							metricR = r
						}
					}
					// Health-score decline detector: use whichever R is larger.
					if hr := predictorEngine.RuptureIndex(host, "health_score"); hr > metricR {
						metricR = hr
					}
					fusionEngine.SetMetricR(host, metricR, now)

					// Annotate snapshot with current FusedR before persisting.
					var fusedR float64
					if fr, _, err := fusionEngine.FusedR(host); err == nil {
						fusedR = fr
						snap.FusedRuptureIndex = fusedR
					}

					store.StoreSnapshot(snap)
					metricsReg.RecordKPISnapshot(snap)
					metricsReg.IncIngestTotal("metrics")

					// v6.4: near-miss tracking + fingerprint recording (requires FusedR).
					analyzerEngine.UpdateFusedR(ref, fusedR)
					analyzerEngine.MaybeRecordFingerprint(snap, fusedR)

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

	// k8s metrics-server poller — provides pod CPU/memory signals directly via the
	// metrics.k8s.io API. Falls back silently when metrics-server is not installed.
	if poller, err := k8smetrics.New(pipelineEngine, 30*time.Second); err == nil {
		logger.Default.Info("k8s metrics-server poller active — injecting pod CPU/memory signals")
		go poller.Run(ctx)
	} else {
		logger.Default.Info("k8s metrics-server poller skipped", "reason", err.Error())
	}

	// Try to build a real Kubernetes actuator (only works inside a K8s pod).
	// If not in-cluster, the provider silently no-ops instead of failing startup.
	var k8sActuator *providers.KubernetesActuator
	if a, err := providers.NewKubernetesActuator(); err == nil {
		k8sActuator = a
		logger.Default.Info("kubernetes actuator initialised — scale/restart/cordon actions enabled")
	} else {
		logger.Default.Info("kubernetes actuator unavailable (not in-cluster) — K8s actions will be queued only", "reason", err.Error())
	}
	k8sProvider := providers.NewKubernetesProviderWithActuator(k8sActuator)

	actionEngine, err := engine.New(nil, bus)
	if err != nil {
		return fmt.Errorf("init action engine failed: %w", err)
	}

	// Tier-1 auto-execute: autopilot edition only.
	// Every 15s, approve and execute any Tier-1 actions that haven't been executed yet.
	if cfg.Edition == "autopilot" {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if actionEngine.IsEmergencyStopped() {
						continue
					}
					for _, a := range actionEngine.PendingActions() {
						if a.Tier != engine.Tier1 || a.Executed {
							continue
						}
						actionEngine.Approve(a.ID)
						execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
						if err := k8sProvider.Execute(execCtx, a); err != nil {
							logger.Default.Warn("tier-1 action execute failed", "id", a.ID, "err", err)
						} else {
							actionEngine.MarkExecuted(a.ID)
							logger.Default.Info("tier-1 action executed", "id", a.ID, "action_type", a.ActionType, "host", a.Host)
						}
						cancel()
					}
				}
			}
		}()
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
	handlers.SetIngest(ingestEngine)
	handlers.SetFusion(fusionEngine)
	handlers.SetTopology(topoBuilder)
	if inf != nil {
		handlers.SetDiscovery(inf)
	}
	handlers.SetEdition(cfg.Edition)
	handlers.SetVersion(version)
	histMgr := history.New()
	evBus := events.New()
	handlers.SetHistoryMgr(histMgr)
	handlers.SetEventBus(evBus)

	// Active scrape engine — pulls data from Prometheus servers and direct /metrics endpoints.
	scraperMgr := scraper.New(pipelineEngine, store)
	scraperMgr.Start()
	defer scraperMgr.Stop()
	handlers.SetScraper(scraperMgr)

	// Seed built-in datasources. Each is idempotent — existing user configs are not overwritten.
	seedDS := func(ds scraper.DatasourceConfig) {
		if _, ok := scraperMgr.Get(ds.ID); ok {
			return // already loaded from storage
		}
		if err := scraperMgr.Put(ds); err != nil {
			logger.Default.Warn("seed datasource failed", "id", ds.ID, "err", err)
		}
	}

	now := time.Now()
	seedDS(scraper.DatasourceConfig{
		ID:                "self-ruptura-metrics",
		Name:              "Ruptura Engine (self)",
		Type:              scraper.TypeDirect,
		URL:               fmt.Sprintf("http://localhost:%d/metrics", cfg.Port),
		Enabled:           true,
		ScrapeIntervalSec: 30,
		WorkloadKey:       "ruptura-system/Deployment/ruptura",
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	if cfg.PrometheusURL != "" {
		seedDS(scraper.DatasourceConfig{
			ID:                "prometheus-cluster",
			Name:              "Prometheus (cluster)",
			Type:              scraper.TypePrometheus,
			URL:               cfg.PrometheusURL,
			Enabled:           true,
			ScrapeIntervalSec: 30,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
	}

	handlers.SetReady(true)

	router := handlers.NewRouter()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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
	// k8smetrics poller injects "fatigue" (restart count), "stress" (cpu), "pressure" (mem).
	// Prometheus scraper also injects these. Prefer them alongside classic sensor names.
	preferred := []string{"fatigue", "stress", "cpu_percent", "memory_percent", "latency_p99", "error_rate", "request_rate"}
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
	// Use workload key as ID so the UI can look up explain by workload reference.
	// Using a random ID made records unreachable since the UI always queries by key.
	id := host
	if id == "" {
		id = utils.GenerateID(8)
	}

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

