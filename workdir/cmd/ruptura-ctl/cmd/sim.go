package cmd

import (
	"fmt"

	"github.com/benfradjselim/ruptura/internal/sim"
	"github.com/spf13/cobra"
)

var (
	simWorkload string
)

var simCmd = &cobra.Command{
	Use:   "sim",
	Short: "Control ruptura-sim synthetic load injection",
	Example: `  ruptura-ctl sim inject cascade-failure
  ruptura-ctl sim inject memory-leak --workload demo/Deployment/payment-api
  ruptura-ctl sim patterns`,
}

var simInjectCmd = &cobra.Command{
	Use:   "inject <pattern>",
	Short: "Report a synthetic failure on a demo engine",
	Long: `Ask a Ruptura engine running with --demo to report a synthetic failure on a
workload, to try the console, alerts or the MCP server without a real incident.
Engines connected to a cluster refuse injections.

Available patterns:
  memory-leak       Memory heading to its limit, out of memory in about 22 minutes
  cascade-failure   Crash loop
  traffic-surge     CPU throttled by its limit
  slow-burn         Memory heading to its limit, in about 50 minutes`,
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

		fmt.Println()
		fmt.Printf("  Reporting %s on %s\n\n", cyan(pattern), cyan(workload))

		cfg := sim.Config{
			Target:   cfgURL,
			APIKey:   cfgAPIKey,
			Workload: workload,
			Pattern:  pattern,
		}
		if err := sim.Run(cfg); err != nil {
			return fmt.Errorf("sim inject: %w", err)
		}

		fmt.Println()
		successLine(fmt.Sprintf("Pattern %s reported", cyan(pattern)))
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
}
