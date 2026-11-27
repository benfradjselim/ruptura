// ruptura-sim asks a Ruptura engine in demo mode to report a synthetic failure
// on a workload, to demo the tool or check alert routing without a real incident.
//
// Usage:
//
//	ruptura-sim inject --pattern memory-leak --workload demo/Deployment/api
//	ruptura-sim patterns
package main

import (
	"fmt"
	"os"

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
		if err := sim.Run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ruptura-sim: %s reported on %s\n", cfg.Pattern, cfg.Workload)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func parseInject(args []string) (sim.Config, error) {
	cfg := sim.Config{Target: "http://localhost:8080", APIKey: os.Getenv("RUPTURA_API_KEY")}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		if i+1 >= len(args) {
			return cfg, fmt.Errorf("%s requires a value", flag)
		}
		i++
		switch flag {
		case "--pattern":
			cfg.Pattern = args[i]
		case "--workload":
			cfg.Workload = args[i]
		case "--target":
			cfg.Target = args[i]
		case "--api-key":
			cfg.APIKey = args[i]
		default:
			return cfg, fmt.Errorf("unknown flag %q", flag)
		}
	}
	if cfg.Pattern == "" || cfg.Workload == "" {
		return cfg, fmt.Errorf("--pattern and --workload are required")
	}
	return cfg, nil
}

func printUsage() {
	fmt.Println(`ruptura-sim: report synthetic failures on a Ruptura engine running with --demo

Commands:
  inject --pattern <pattern> --workload <namespace/Kind/name> [--target URL] [--api-key KEY]
  patterns`)
}
