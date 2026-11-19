package cmd

import (
	"fmt"
	"time"

	"github.com/benfradjselim/ruptura/internal/sim"
	"github.com/spf13/cobra"
)

var (
	simWorkload string
	simDuration int
)

var simCmd = &cobra.Command{
	Use:   "sim",
	Short: "Control ruptura-sim synthetic load injection",
	Example: `  ruptura-ctl sim inject cascade-failure
  ruptura-ctl sim inject memory-leak --workload production/Deployment/payment-api --duration 120
  ruptura-ctl sim patterns`,
}

var simInjectCmd = &cobra.Command{
	Use:   "inject <pattern>",
	Short: "Inject a synthetic load pattern",
	Long: `Inject a synthetic load pattern into Ruptura to test alerting and action pipelines.

Available patterns:
  memory-leak       Slowly increasing memory pressure
  cascade-failure   Error propagation across service dependencies
  traffic-surge     Sudden spike in throughput and latency
  slow-burn         Gradual multi-signal degradation over time`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		valid := map[string]bool{
			"memory-leak":     true,
			"cascade-failure": true,
			"traffic-surge":   true,
			"slow-burn":       true,
		}
		if !valid[pattern] {
			return fmt.Errorf("unknown pattern %q — valid: memory-leak, cascade-failure, traffic-surge, slow-burn", pattern)
		}

		workload := simWorkload
		if workload == "" {
			workload = "demo/Deployment/api"
		}

		dur := time.Duration(simDuration) * time.Second
		if dur == 0 {
			dur = 60 * time.Second
		}

		fmt.Println()
		fmt.Printf("  Injecting %s into %s for %s\n", cyan(pattern), cyan(workload), dim(dur.String()))
		fmt.Printf("  %s\n\n", dim("Ticking every 5s — Ctrl+C to stop early"))

		cfg := sim.Config{
			Target:   cfgURL,
			APIKey:   cfgAPIKey,
			Workload: workload,
			Pattern:  pattern,
			Duration: dur,
			Verbose:  true,
		}
		if err := sim.Run(cfg); err != nil {
			return fmt.Errorf("sim inject: %w", err)
		}

		fmt.Println()
		successLine(fmt.Sprintf("Pattern %s complete", cyan(pattern)))
		infoLine("Watch the effect: " + cyan("ruptura-ctl status"))
		fmt.Println()
		return nil
	},
}

var simPatternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "List available simulation patterns",
	RunE: func(cmd *cobra.Command, args []string) error {
		patterns := []struct{ name, desc string }{
			{"memory-leak", "Slowly increasing memory pressure — triggers fatigue accumulation"},
			{"cascade-failure", "Error propagation across service edges — triggers contagion signal"},
			{"traffic-surge", "Sudden throughput spike + latency — triggers stress + pressure"},
			{"slow-burn", "Gradual multi-signal degradation — tests forecast accuracy"},
		}
		fmt.Println()
		fmt.Printf("  %s\n\n", bold("Simulation Patterns"))
		for _, p := range patterns {
			fmt.Printf("  %s\n    %s\n\n", cyan(p.name), dim(p.desc))
		}
		fmt.Printf("  Usage: %s\n\n", cyan("ruptura-ctl sim inject <pattern> [--workload <ref>] [--duration <seconds>]"))
		return nil
	},
}

func init() {
	simCmd.AddCommand(simInjectCmd)
	simCmd.AddCommand(simPatternsCmd)
	rootCmd.AddCommand(simCmd)

	simInjectCmd.Flags().StringVar(&simWorkload, "workload", "", "Target workload ref (namespace/Kind/name). Omit for auto-select.")
	simInjectCmd.Flags().IntVar(&simDuration, "duration", 0, "Duration in seconds (default: pattern default)")
}
