package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benfradjselim/ohe/internal/orchestrator"
	"github.com/benfradjselim/ohe/pkg/logger"
	"gopkg.in/yaml.v3"
)

const banner = `
 ██████╗ ██╗  ██╗███████╗
██╔═══██╗██║  ██║██╔════╝
██║   ██║███████║█████╗
██║   ██║██╔══██║██╔══╝
╚██████╔╝██║  ██║███████╗
 ╚═════╝ ╚═╝  ╚═╝╚══════╝
Observability Holistic Engine v5.0.0
"Prevention is better than cure"
`

func main() {
	fmt.Print(banner)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "agent":
		runMode("agent", os.Args[2:])
	case "central":
		runMode("central", os.Args[2:])
	case "version":
		fmt.Println("ohe version 5.0.0")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: ohe <command> [flags]

Commands:
  agent    Run in agent mode (collect metrics, push to central)
  central  Run in central mode (API server, UI, storage)
  version  Print version

Flags (agent & central):
  --config    Path to config YAML (default: /etc/ohe/config.yaml)
  --port      HTTP/HTTPS port (default: 8080)
  --host      Hostname override
  --storage   Storage directory (default: /var/lib/ohe/data)
  --tls-cert  Path to TLS certificate PEM (enables HTTPS when paired with --tls-key)
  --tls-key   Path to TLS private key PEM  (enables HTTPS when paired with --tls-cert)
  --auth      Enable JWT authentication
  --jwt-secret JWT signing secret

Agent-specific:
  --central-url  Central server URL (default: http://localhost:8080)

Examples:
  ohe central --port 8080 --storage /var/lib/ohe
  ohe central --port 443 --tls-cert /etc/ohe/tls.crt --tls-key /etc/ohe/tls.key
  ohe agent   --central-url https://central:443`)
}

func runMode(mode string, args []string) {
	fs := flag.NewFlagSet(mode, flag.ExitOnError)

	configFile := fs.String("config", "", "path to config YAML")
	port := fs.Int("port", 8080, "HTTP port")
	host := fs.String("host", "", "hostname override")
	storagePath := fs.String("storage", "/var/lib/ohe/data", "storage directory")
	centralURL := fs.String("central-url", "http://localhost:8080", "central server URL (agent mode)")
	authEnabled := fs.Bool("auth", false, "enable JWT authentication")
	jwtSecret := fs.String("jwt-secret", "", "JWT signing secret")
	collectInterval := fs.Duration("interval", 15*time.Second, "metric collection interval")
	tlsCert := fs.String("tls-cert", "", "path to TLS certificate PEM file (enables HTTPS when paired with --tls-key)")
	tlsKey := fs.String("tls-key", "", "path to TLS private key PEM file (enables HTTPS when paired with --tls-cert)")
	replicaURL := fs.String("replica-url", "", "Litestream replica URL for HA replication (e.g. s3://mybucket/ohe)")

	_ = fs.Parse(args)

	// Start with defaults
	cfg := orchestrator.DefaultConfig()
	cfg.Mode = mode

	// Load from file if provided
	if *configFile != "" {
		if err := loadConfigFile(*configFile, &cfg); err != nil {
			logger.Default.Error("load config failed", "err", err)
			os.Exit(1)
		}
	}

	// Override with flags
	if *port != 8080 || cfg.Port == 0 {
		cfg.Port = *port
	}
	if *host != "" {
		cfg.Host = *host
	}
	if *storagePath != "/var/lib/ohe/data" {
		cfg.StoragePath = *storagePath
	}
	if *centralURL != "http://localhost:8080" {
		cfg.CentralURL = *centralURL
	}
	if *authEnabled {
		cfg.AuthEnabled = true
	}
	if *jwtSecret != "" {
		cfg.JWTSecret = *jwtSecret
	}
	if *collectInterval != 15*time.Second {
		cfg.CollectInterval = *collectInterval
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 10000
	}
	if *tlsCert != "" {
		cfg.TLSCertFile = *tlsCert
	}
	if *tlsKey != "" {
		cfg.TLSKeyFile = *tlsKey
	}
	if *replicaURL != "" {
		cfg.ReplicaURL = *replicaURL
	}

	// Resolve hostname if not set
	if cfg.Host == "" {
		h, err := os.Hostname()
		if err == nil {
			cfg.Host = h
		} else {
			cfg.Host = "localhost"
		}
	}

	// Security: Ensure JWT secret is set if auth is enabled
	if cfg.AuthEnabled && cfg.JWTSecret == "" {
		logger.Default.Warn("Auth enabled but no JWT secret set; generating a random one. Change this in production!")
		cfg.JWTSecret = generateSecureSecret(32)
	}

	logger.Default.Info("ohe starting", "mode", cfg.Mode, "host", cfg.Host, "port", cfg.Port, "storage", cfg.StoragePath)

	engine, err := orchestrator.New(cfg)
	if err != nil {
		logger.Default.Error("init engine failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := engine.Run(ctx); err != nil {
		logger.Default.Error("engine error", "err", err)
		os.Exit(1)
	}
	logger.Default.Info("shutdown complete")
}

func loadConfigFile(path string, cfg *orchestrator.Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return yaml.Unmarshal(data, cfg)
}

func generateSecureSecret(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		logger.Default.Error("failed to generate random secret", "err", err)
		os.Exit(1)
	}
	return hex.EncodeToString(b)
}
