// ruptura-sim injects synthetic degradation patterns into a running Ruptura instance.
// Use it to demo the tool, run load tests, or validate alerting without a real incident.
//
// Usage:
//
//	ruptura-sim inject --pattern memory-leak --workload demo/deployment/api --duration 30m
//	ruptura-sim patterns
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/sim"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "patterns":
		fmt.Println("Available patterns:")
		for _, p := range sim.AllPatterns {
			fmt.Println(" ", p)
		}

	case "inject":
		cfg, err := parseInject(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ruptura-sim: injecting pattern %q into %s for %s\n", cfg.Pattern, cfg.Workload, cfg.Duration)
		if err := sim.Run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("ruptura-sim: done")

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func parseInject(args []string) (sim.Config, error) {
	cfg := sim.Config{
		Target:   "http://localhost:8080",
		Duration: 30 * time.Minute,
		Interval: 5 * time.Second,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--pattern":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--pattern requires a value")
			}
			cfg.Pattern = args[i]
		case "--workload":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--workload requires a value")
			}
			cfg.Workload = args[i]
		case "--origin":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--origin requires a value")
			}
			cfg.Origin = args[i]
		case "--duration":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--duration requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return cfg, fmt.Errorf("invalid --duration %q: %w", args[i], err)
			}
			cfg.Duration = d
		case "--target":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--target requires a value")
			}
			cfg.Target = strings.TrimRight(args[i], "/")
		case "--interval":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("--interval requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return cfg, fmt.Errorf("invalid --interval %q: %w", args[i], err)
			}
			cfg.Interval = d
		case "--verbose", "-v":
			cfg.Verbose = true
		default:
			return cfg, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if cfg.Pattern == "" {
		return cfg, fmt.Errorf("--pattern is required (run 'ruptura-sim patterns' to list)")
	}
	if cfg.Workload == "" {
		cfg.Workload = "demo/deployment/api"
	}
	return cfg, nil
}

func printUsage() {
	fmt.Print(`ruptura-sim — inject synthetic degradation patterns into Ruptura

Commands:
  inject    Inject a degradation pattern into a workload
  patterns  List available patterns

inject flags:
  --pattern   <name>      Pattern to inject (required)
  --workload  <ref>       Workload ref, e.g. "demo/deployment/api" (default: demo/deployment/api)
  --origin    <ref>       Cascade source workload (cascade-failure pattern only)
  --duration  <duration>  How long to run, e.g. 30m (default: 30m)
  --target    <url>       Ruptura API base URL (default: http://localhost:8080)
  --interval  <duration>  Tick interval (default: 5s)
  --verbose               Print metrics per tick

Examples:
  ruptura-sim inject --pattern memory-leak --workload payments/deployment/checkout --duration 30m --verbose
  ruptura-sim inject --pattern cascade-failure --origin demo/deployment/payment-db --workload demo/deployment/payment-api
  ruptura-sim inject --pattern slow-burn --duration 2h --target http://ruptura.my-cluster:8080
`)
}
