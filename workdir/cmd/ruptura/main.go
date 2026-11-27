package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/actions/providers"
	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/internal/callgraph"
	"github.com/benfradjselim/ruptura/internal/collector/infra/admission"
	"github.com/benfradjselim/ruptura/internal/collector/infra/co"
	"github.com/benfradjselim/ruptura/internal/collector/infra/dag"
	"github.com/benfradjselim/ruptura/internal/collector/infra/mcp"
	"github.com/benfradjselim/ruptura/internal/collector/infra/networking"
	"github.com/benfradjselim/ruptura/internal/collector/infra/node"
	infraoperator "github.com/benfradjselim/ruptura/internal/collector/infra/operator"
	infrastorage "github.com/benfradjselim/ruptura/internal/collector/infra/storage"
	"github.com/benfradjselim/ruptura/internal/collector/infra/tenancy"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/ingest"
	"github.com/benfradjselim/ruptura/internal/namespaces"
	"github.com/benfradjselim/ruptura/internal/sim"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
)

const version = "8.7.12"

// Config holds all runtime configuration parsed from CLI flags.
type Config struct {
	Port         int
	OTLPGRPCPort int
	OTLPHTTPPort int
	StoragePath  string
	APIKey       string
	Edition      string // "community" (default) or "autopilot"
	ShowVersion  bool
	DemoMode     bool // --demo / RUPTURA_DEMO_MODE=true: seed synthetic data, no cluster required
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("ruptura", flag.ContinueOnError)
	cfg := Config{}
	fs.IntVar(&cfg.Port, "port", 8080, "HTTP port")
	fs.IntVar(&cfg.OTLPGRPCPort, "otlp-grpc-port", 4317, "OTLP gRPC ingest port")
	fs.IntVar(&cfg.OTLPHTTPPort, "otlp-http-port", 4318, "OTLP HTTP and Prometheus remote write ingest port")
	fs.StringVar(&cfg.StoragePath, "storage", "/var/lib/ruptura/data", "storage directory")
	fs.StringVar(&cfg.APIKey, "api-key", "", "API bearer token")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	fs.BoolVar(&cfg.DemoMode, "demo", false, "seed synthetic demo data on startup; no cluster required")
	err := fs.Parse(args)
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("RUPTURA_API_KEY")
	}
	if cfg.Edition == "" {
		if e := os.Getenv("RUPTURA_EDITION"); e != "" {
			cfg.Edition = e
		} else {
			cfg.Edition = "community"
		}
	}
	if !cfg.DemoMode && os.Getenv("RUPTURA_DEMO_MODE") == "true" {
		cfg.DemoMode = true
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

// runWithContext is the testable entrypoint — it uses the provided context for shutdown.
func runWithContext(ctx context.Context, cfg Config) error {
	logger.Default.Info("ruptura starting", "version", version, "port", cfg.Port)

	store, err := storage.Open(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("open storage failed: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	// cancel must run before store.Close so the GC goroutine stops first.
	defer cancel()
	defer store.Close()
	startProfiling(ctx, os.Getenv(profilingEnv))

	// Periodic BadgerDB value-log GC reclaims space from TTL-expired entries.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for store.RunGC() == nil {
				}
			}
		}
	}()

	bus := eventbus.NewWithKafka(ctx, os.Getenv("KAFKA_BROKERS"), "ruptura")
	defer bus.Close()

	// OTLP and Prometheus remote write: metrics, logs and spans are stored.
	ingestEngine := ingest.New(store, store)
	ingestEngine.SetLogStore(store)

	var inf *discovery.Informer
	if disc, err := discovery.NewInformer(); err == nil {
		logger.Default.Info("k8s auto-discovery active — watching Deployments/StatefulSets/DaemonSets")
		inf = disc
		ingestEngine.SetOwnerResolver(disc.ResolvePodOwner)
		go disc.Run(ctx, func(models.WorkloadRef) {}, func(models.WorkloadRef) {}, func() {
			logger.Default.Info("discovery: first sync complete", "known_workloads", len(disc.KnownRefs()))
		})
	} else {
		logger.Default.Info("k8s auto-discovery skipped (not in-cluster)", "reason", err.Error())
	}

	// Infra collectors probe in-cluster objects; empty out of cluster.
	infraRegistry := dag.NewRegistry()
	if nodeCol, err := node.New(); err == nil {
		infraRegistry.Add(nodeCol)
	}
	if coCol, err := co.New(); err == nil {
		infraRegistry.Add(coCol)
	}
	if mcpCol, err := mcp.New(); err == nil {
		infraRegistry.Add(mcpCol)
	}
	if netCol, err := networking.New(); err == nil {
		infraRegistry.Add(netCol)
	}
	if storageCol, err := infrastorage.New(); err == nil {
		infraRegistry.Add(storageCol)
	}
	if admissionCol, err := admission.New(); err == nil {
		infraRegistry.Add(admissionCol)
	}
	if opCol, err := infraoperator.New(); err == nil {
		infraRegistry.Add(opCol)
	}
	if tenancyCol, err := tenancy.New(); err == nil {
		infraRegistry.Add(tenancyCol)
	}
	infraRegistry.SetPersister(store)
	go infraRegistry.Run(ctx)

	otlpAddr := fmt.Sprintf(":%d", cfg.OTLPHTTPPort)
	if certFile := os.Getenv("RUPTURA_OTLP_TLS_CERT_FILE"); certFile != "" {
		tlsConfig, err := ingest.ServerTLS(certFile, os.Getenv("RUPTURA_OTLP_TLS_KEY_FILE"), os.Getenv("RUPTURA_OTLP_TLS_CLIENT_CA_FILE"))
		if err != nil {
			return fmt.Errorf("OTLP TLS: %w", err)
		}
		ingestEngine.SetTLS(tlsConfig)
	}
	if err := ingestEngine.StartHTTP(otlpAddr); err != nil {
		return fmt.Errorf("start OTLP ingest failed: %w", err)
	}
	if err := ingestEngine.StartOTLPGRPC(fmt.Sprintf(":%d", cfg.OTLPGRPCPort)); err != nil {
		return fmt.Errorf("OTLP gRPC: %w", err)
	}
	defer ingestEngine.Stop(context.Background()) //nolint:errcheck

	metricsReg := telemetry.NewRegistry(version)
	ingestEngine.SetIngestHook(metricsReg.IncIngestTotal)

	// The action engine stays dormant in community: its endpoints answer,
	// approving or rolling back returns 402, and nothing triggers it yet.
	var k8sActuator *providers.KubernetesActuator
	if a, err := providers.NewKubernetesActuator(); err == nil {
		k8sActuator = a
	}
	_ = providers.NewKubernetesProviderWithActuator(k8sActuator)
	var rulesYAML []byte
	if p := os.Getenv("RUPTURA_ACTION_RULES_PATH"); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			rulesYAML = data
		} else {
			logger.Default.Warn("RUPTURA_ACTION_RULES_PATH: failed to read file", "path", p, "err", err)
		}
	}
	actionEngine, err := engine.New(rulesYAML, bus)
	if err != nil {
		return fmt.Errorf("init action engine failed: %w", err)
	}

	handlers := api.New(store, actionEngine, metricsReg, telemetry.NewHealthChecker(), cfg.APIKey)
	handlers.SetIngest(ingestEngine)
	if inf != nil {
		handlers.SetDiscovery(inf)
	}
	handlers.SetInfraRegistry(infraRegistry)
	if graph, err := callgraph.New(); err == nil {
		if url := os.Getenv("RUPTURA_THANOS_URL"); url != "" {
			if source, err := sources.NewPrometheus(prometheusConfig(url)); err == nil {
				graph.SetTraceSource(source)
			} else {
				logger.Default.Warn("call graph: trace metrics disabled", "err", err)
			}
		}
		go graph.Run(ctx)
		handlers.SetCallGraph(graph)
	} else {
		logger.Default.Info("call graph disabled", "reason", err.Error())
	}
	limits := newCeilings(os.Getenv("RUPTURA_DEFAULT_MEMORY_LIMITS"))
	objects := failuremodes.NewObjects()
	tracker := startFailureModes(ctx, os.Getenv("RUPTURA_THANOS_URL"), inf, store, limits, objects)
	switch {
	case cfg.DemoMode:
		// --demo: synthetic failures, no cluster required.
		demo := sim.NewDemo(time.Now())
		handlers.SetFailureModes(demo)
		handlers.SetInjector(demo)
		logger.Default.Info("demo mode: synthetic failures on 12 workloads")
	case tracker != nil:
		handlers.SetFailureModes(tracker)
		handlers.SetObjectFindings(objects, cardInfo())
	}
	if inputs := startInputs(ctx, os.Getenv("RUPTURA_THANOS_URL"), tracker, inf, limits); inputs != nil {
		handlers.SetInputs(inputs)
	}
	handlers.SetLevelScope(os.Getenv("RUPTURA_LEVEL_SCOPE"))
	handlers.SetLevelSelector(api.ParseSelector(os.Getenv("RUPTURA_LEVEL_SELECTOR")))
	if selector := api.ParseSelector(os.Getenv("RUPTURA_NAMESPACE_SELECTOR")); len(selector) > 0 {
		var source api.NamespaceLabels
		if labels, err := namespaces.New(); err != nil {
			logger.Default.Warn("namespace selector set but namespace labels unavailable, no failure is reported", "err", err)
		} else {
			go labels.Run(ctx)
			source = labels
		}
		handlers.SetNamespaceSelector(selector, source)
	}
	startAlerts(ctx, handlers)
	if horizon := os.Getenv("RUPTURA_HORIZON"); horizon != "" {
		parsed, err := time.ParseDuration(horizon)
		if err != nil {
			logger.Default.Warn("invalid RUPTURA_HORIZON, using the default", "err", err)
		} else {
			handlers.SetHorizon(parsed)
		}
	}
	handlers.SetEdition(cfg.Edition)
	handlers.SetVersion(version)
	handlers.SetDemoMode(cfg.DemoMode)
	handlers.SetReady(true)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handlers.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Default.Info("listening", "addr", srv.Addr, "otlp_grpc", cfg.OTLPGRPCPort, "otlp_http", cfg.OTLPHTTPPort)

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			logger.Default.Error("shutdown error", "err", err)
		}
		// Save the failure mode readings so the next start resumes its forecasts.
		saveTrackerState(tracker, store, time.Now())
		logger.Default.Info("shutdown complete")
		return nil
	case err := <-errCh:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}
