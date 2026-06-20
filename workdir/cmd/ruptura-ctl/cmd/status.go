package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show health status of all workloads",
	Long:  `Display a live summary of all workloads Ruptura is monitoring, their health scores, Fused Rupture Index, and calibration state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()

		health, err := c.Health(ctx())
		if err != nil {
			return fmt.Errorf("cannot reach Ruptura at %s: %w", cfgURL, err)
		}

		snapshots, err := c.Snapshots(ctx())
		if err != nil {
			return fmt.Errorf("fetch snapshots: %w", err)
		}

		actions, actErr := c.Actions(ctx())
		if actErr != nil {
			fmt.Printf("  %s\n", dim("(pending actions unavailable: "+actErr.Error()+")"))
		}

		// header box
		edition := health.Edition
		if edition == "" {
			edition = "community"
		}
		editionLabel := dim("[" + edition + "]")
		statusDot := green("●")
		if health.Status != "ok" && health.Status != "healthy" {
			statusDot = yellow("●")
		}

		fmt.Println()
		fmt.Printf("  %s %s  %s  %s\n",
			bold(cyan("RUPTURA")),
			dim("v"+health.Version),
			editionLabel,
			cfgURL,
		)
		fmt.Printf("  %s %s   uptime %s\n",
			statusDot,
			bold(health.Status),
			dim(uptimeStr(health.UptimeSeconds)),
		)
		fmt.Println()

		// filter by namespace
		filtered := snapshots
		if cfgNS != "" {
			filtered = make([]models.KPISnapshot, 0, len(snapshots))
			for _, s := range snapshots {
				if s.Workload.Namespace == cfgNS || s.Host == cfgNS {
					filtered = append(filtered, s)
				}
			}
		}

		if len(filtered) == 0 {
			fmt.Println(dim("  No workloads found. Send telemetry to start monitoring."))
			fmt.Println()
			return nil
		}

		t := newTable("WORKLOAD", "HEALTH", "STATE", "FUSEDR", "CALIB", "ETA", "SLO BURN", "DEBT")
		t.alignRight(1)
		t.alignRight(3)
		t.alignRight(5)
		t.alignRight(6)

		warnings, criticals := 0, 0

		for _, s := range filtered {
			ref := s.Host
			if s.Workload.Namespace != "" {
				ref = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			}
			if cfgNS != "" {
				// strip namespace prefix for readability
				ref = strings.TrimPrefix(ref, cfgNS+"/")
			}

			state := s.WorkloadStatus
			if state == "" {
				if s.CalibrationProgress < 100 {
					state = "calibrating"
				} else {
					state = "active"
				}
			}

			eta := 0
			if s.HealthForecast != nil {
				eta = s.HealthForecast.CriticalETAMinutes
			}

			sloBurn := dim("—")
			debt := dim("—")
			if s.Business != nil {
				if s.Business.SLOBurnVelocity > 0 {
					sloBurn = fmt.Sprintf("%.2f", s.Business.SLOBurnVelocity)
					if s.Business.SLOBurnVelocity > 1 {
						sloBurn = red(sloBurn)
					} else if s.Business.SLOBurnVelocity > 0.5 {
						sloBurn = yellow(sloBurn)
					}
				}
				debt = fmt.Sprintf("%d", s.Business.RecoveryDebt)
				if s.Business.RecoveryDebt >= 7 {
					debt = red(debt)
				} else if s.Business.RecoveryDebt >= 3 {
					debt = yellow(debt)
				}
			}

			if s.FusedRuptureIndex >= 5 {
				criticals++
			} else if s.FusedRuptureIndex >= 1.5 {
				warnings++
			}

			t.add(
				ref,
				healthColor(s.HealthScore.Value),
				stateIcon(state)+" "+state,
				fusedRColor(s.FusedRuptureIndex),
				calibBar(s.CalibrationProgress),
				etaStr(eta),
				sloBurn,
				debt,
			)
		}

		t.print()

		// summary line
		pending := len(actions)
		parts := []string{fmt.Sprintf("%d workload", len(filtered))}
		if len(filtered) != 1 {
			parts[0] += "s"
		}
		if warnings > 0 {
			parts = append(parts, yellow(fmt.Sprintf("%d warning", warnings)))
		}
		if criticals > 0 {
			parts = append(parts, red(fmt.Sprintf("%d critical", criticals)))
		}
		if pending > 0 {
			parts = append(parts, cyan(fmt.Sprintf("%d pending action", pending)))
			if pending != 1 {
				parts[len(parts)-1] += "s"
			}
		}
		fmt.Printf("  %s\n\n", strings.Join(parts, dim("  ·  ")))

		if criticals > 0 {
			fmt.Printf("  %s Run %s or %s\n\n",
				yellow("⚠"),
				cyan("ruptura-ctl actions"),
				cyan("ruptura-ctl describe workload <ref>"),
			)
		}
		return nil
	},
}

var statusWatchInterval int

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().IntVarP(&statusWatchInterval, "watch", "w", 0,
		"Refresh every N seconds (e.g. -w 5). 0 = run once.")
}

// runStatusOnce executes the status command body. Called directly or on a ticker when --watch is set.
func runStatusWatch(cmd *cobra.Command) {
	if statusWatchInterval <= 0 {
		return
	}
	// Clear screen on each refresh
	fmt.Print("\033[H\033[2J")
	_ = statusCmd.RunE(cmd, nil)

	ticker := time.NewTicker(time.Duration(statusWatchInterval) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fmt.Print("\033[H\033[2J")
		_ = statusCmd.RunE(cmd, nil)
	}
}
