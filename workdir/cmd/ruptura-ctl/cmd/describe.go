package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe <type> <ref>",
	Short: "Show detailed information about a resource",
	Example: `  ruptura-ctl describe workload production/Deployment/payment-api
  ruptura-ctl describe workload payment-api`,
}

var describeWorkloadCmd = &cobra.Command{
	Use:     "workload <ref>",
	Aliases: []string{"wl"},
	Short:   "Show full KPI snapshot for a workload",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		ref := args[0]
		snap, err := c.Snapshot(ctx(), ref)
		if err != nil {
			return fmt.Errorf("fetch workload %q: %w", ref, err)
		}
		if cfgOutput == "json" {
			return printJSON(snap)
		}

		displayRef := snap.Host
		if snap.Workload.Namespace != "" {
			displayRef = snap.Workload.Namespace + "/" + snap.Workload.Kind + "/" + snap.Workload.Name
		}

		state := snap.WorkloadStatus
		if state == "" {
			if snap.CalibrationProgress < 100 {
				state = "calibrating"
			} else {
				state = "active"
			}
		}

		fmt.Println()
		fmt.Printf("  %s  %s\n", bold(displayRef), dim("[workload]"))
		fmt.Println()

		// status row
		fmt.Printf("  %-20s %s %s\n", dim("state"), stateIcon(state), bold(state))
		healthLine := kpiHealth(snap.HealthScore)
		if !snap.HealthScore.Unavailable {
			healthLine += " / 100"
		}
		fmt.Printf("  %-20s %s\n", dim("health score"), healthLine)
		fmt.Printf("  %-20s %s\n", dim("fused rupture idx"), fusedRColor(snap.FusedRuptureIndex))
		fmt.Printf("  %-20s %s  %s\n", dim("calibration"),
			calibBar(snap.CalibrationProgress),
			progressBar(snap.CalibrationProgress, 20),
		)

		// forecast
		if snap.HealthForecast != nil {
			f := snap.HealthForecast
			fmt.Println()
			fmt.Printf("  %s\n", bold("Forecast"))
			fmt.Printf("  %-20s %s  →  %s (15m)  →  %s (30m)\n",
				dim("health trajectory"),
				kpiHealth(snap.HealthScore),
				healthColor(f.In15Min),
				healthColor(f.In30Min),
			)
			if f.CriticalETAMinutes > 0 {
				fmt.Printf("  %-20s %s\n", dim("critical ETA"), red(fmt.Sprintf("⚠  %dm", f.CriticalETAMinutes)))
			} else {
				fmt.Printf("  %-20s %s\n", dim("critical ETA"), green("stable — no degradation projected"))
			}
		}

		// KPI signals grid
		fmt.Println()
		fmt.Printf("  %s\n", bold("KPI Signals"))
		signals := []struct {
			name string
			val  float64
		}{
			{"stress", snap.Stress.Value},
			{"fatigue", snap.Fatigue.Value},
			{"mood", snap.Mood.Value},
			{"pressure", snap.Pressure.Value},
			{"humidity", snap.Humidity.Value},
			{"contagion", snap.Contagion.Value},
			{"resilience", snap.Resilience.Value},
			{"entropy", snap.Entropy.Value},
			{"velocity", snap.Velocity.Value},
		}
		for i := 0; i < len(signals); i += 2 {
			sig := signals[i]
			bar := progressBar(int(sig.val*100), 12)
			line := fmt.Sprintf("  %-12s %s  %.3f", dim(sig.name), bar, sig.val)
			if i+1 < len(signals) {
				next := signals[i+1]
				nextBar := progressBar(int(next.val*100), 12)
				line += fmt.Sprintf("     %-12s %s  %.3f", dim(next.name), nextBar, next.val)
			}
			fmt.Println(line)
		}

		// business signals
		if snap.Business != nil {
			b := snap.Business
			fmt.Println()
			fmt.Printf("  %s\n", bold("Business Signals"))
			burnLabel := fmt.Sprintf("%.3f", b.SLOBurnVelocity)
			if b.SLOBurnVelocity > 1 {
				burnLabel = red("🔥 " + burnLabel + " (burning budget)")
			} else if b.SLOBurnVelocity > 0.5 {
				burnLabel = yellow(burnLabel + " (elevated)")
			} else {
				burnLabel = green(burnLabel + " (healthy)")
			}
			fmt.Printf("  %-20s %s\n", dim("SLO burn velocity"), burnLabel)
			fmt.Printf("  %-20s %d downstream service(s)\n", dim("blast radius"), b.BlastRadius)
			debtLabel := fmt.Sprintf("%d near-miss(es) in 7d", b.RecoveryDebt)
			if b.RecoveryDebt >= 7 {
				debtLabel = red(debtLabel + " — fragile")
			} else if b.RecoveryDebt >= 3 {
				debtLabel = yellow(debtLabel)
			} else {
				debtLabel = green(debtLabel)
			}
			fmt.Printf("  %-20s %s\n", dim("recovery debt"), debtLabel)
		}

		// pattern match
		if snap.PatternMatch != nil {
			pm := snap.PatternMatch
			fmt.Println()
			fmt.Printf("  %s\n", bold("Pattern Match"))
			fmt.Printf("  %-20s %s\n", dim("similarity"), cyan(fmt.Sprintf("%.1f%%", pm.Similarity*100)))
			if pm.Resolution != "" {
				wrapped := wrapText(pm.Resolution, 60)
				fmt.Printf("  %-20s %s\n", dim("prior resolution"), dim(wrapped[0]))
				for _, l := range wrapped[1:] {
					fmt.Printf("  %-20s %s\n", "", dim(l))
				}
			}
		}

		fmt.Println()

		if snap.FusedRuptureIndex >= 1.5 {
			fmt.Printf("  %s  run %s for a human-readable explanation\n\n",
				yellow("→"),
				cyan("ruptura-ctl explain "+displayRef),
			)
		}
		return nil
	},
}

func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	var lines []string
	line := ""
	for _, w := range words {
		if len(line)+len(w)+1 > width && line != "" {
			lines = append(lines, line)
			line = w
		} else {
			if line != "" {
				line += " "
			}
			line += w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func init() {
	describeCmd.AddCommand(describeWorkloadCmd)
	rootCmd.AddCommand(describeCmd)
}
