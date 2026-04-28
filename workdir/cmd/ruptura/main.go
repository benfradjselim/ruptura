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
	"github.com/benfradjselim/ruptura/internal/api"
	apicontext "github.com/benfradjselim/ruptura/internal/context"
	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const version = "6.0.0"

// Config holds all runtime configuration parsed from CLI flags.
type Config struct {
	Port        int
	StoragePath string
	APIKey      string
	ShowVersion bool
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("ruptura", flag.ContinueOnError)
	cfg := Config{}
	fs.IntVar(&cfg.Port, "port", 8080, "HTTP port")
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

	actionEngine, err := engine.New(nil, bus)
	if err != nil {
		return fmt.Errorf("init action engine failed: %w", err)
	}

	explainer := explain.NewEngine()
	ctxStore := apicontext.NewManualContextStore()
	detector := apicontext.NewDeploymentDetector()
	metrics := telemetry.NewRegistry(version)
	healthCheck := telemetry.NewHealthChecker()

	handlers := api.New(store, actionEngine, explainer, ctxStore, detector, metrics, healthCheck, cfg.APIKey)
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
	logger.Default.Info("listening", "addr", srv.Addr)

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
