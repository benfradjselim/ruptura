package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <resource>",
	Short: "List resources (workloads, ruptures, actions, suppressions, anomalies)",
	Example: `  ruptura-ctl get workloads
  ruptura-ctl get workloads -n production
  ruptura-ctl get ruptures
  ruptura-ctl get actions
  ruptura-ctl get suppressions
  ruptura-ctl get anomalies`,
}

// --- get workloads ---

var getWorkloadsCmd = &cobra.Command{
	Use:     "workloads",
	Aliases: []string{"workload", "wl"},
	Short:   "List all monitored workloads",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		snaps, err := c.Snapshots(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(snaps)
		}
		if len(snaps) == 0 {
			fmt.Println(dim("\n  No workloads found.\n"))
			return nil
		}

		headers := []string{"WORKLOAD", "HEALTH", "STATE", "FUSEDR", "CALIB", "STRESS", "FATIGUE", "MOOD", "CONTAGION"}
		if cfgOutput == "wide" {
			headers = append(headers, "PRESSURE", "RESILIENCE", "ENTROPY", "VELOCITY")
		}
		t := newTable(headers...)
		t.alignRight(1)
		t.alignRight(3)

		for _, s := range snaps {
			if cfgNS != "" && s.Workload.Namespace != cfgNS {
				continue
			}
			ref := s.Host
			if s.Workload.Namespace != "" {
				ref = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			}
			state := s.WorkloadStatus
			if state == "" {
				if s.CalibrationProgress < 100 {
					state = "calibrating"
				} else {
					state = "active"
				}
			}
			row := []string{
				ref,
				healthColor(s.HealthScore.Value),
				stateIcon(state) + " " + state,
				fusedRColor(s.FusedRuptureIndex),
				calibBar(s.CalibrationProgress),
				fmt.Sprintf("%.2f", s.Stress.Value),
				fmt.Sprintf("%.2f", s.Fatigue.Value),
				fmt.Sprintf("%.2f", s.Mood.Value),
				fmt.Sprintf("%.2f", s.Contagion.Value),
			}
			if cfgOutput == "wide" {
				row = append(row,
					fmt.Sprintf("%.2f", s.Pressure.Value),
					fmt.Sprintf("%.2f", s.Resilience.Value),
					fmt.Sprintf("%.2f", s.Entropy.Value),
					fmt.Sprintf("%.2f", s.Velocity.Value),
				)
			}
			t.add(row...)
		}
		t.print()
		return nil
	},
}

// --- get ruptures ---

var getRupturesCmd = &cobra.Command{
	Use:     "ruptures",
	Aliases: []string{"rupture"},
	Short:   "List all active rupture events",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		// Use the dedicated /api/v2/ruptures endpoint — server-side filtered,
		// much faster on large fleets than fetching all snapshots client-side.
		active, err := c.Ruptures(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(active)
		}
		if len(active) == 0 {
			fmt.Println(green("\n  ✓  No active ruptures.\n"))
			return nil
		}
		t := newTable("WORKLOAD", "FUSEDR", "STATE", "HEALTH", "BLAST RADIUS", "PATTERN MATCH")
		t.alignRight(1)
		for _, s := range active {
			ref := s.Host
			if s.Workload.Namespace != "" {
				ref = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			}
			blast := dim("—")
			if s.Business != nil {
				blast = fmt.Sprintf("%d", s.Business.BlastRadius)
			}
			match := dim("—")
			if s.PatternMatch != nil {
				match = cyan(fmt.Sprintf("%.0f%% similarity", s.PatternMatch.Similarity*100))
			}
			state := "warning"
			if s.FusedRuptureIndex >= 5 {
				state = "emergency"
			} else if s.FusedRuptureIndex >= 3 {
				state = "critical"
			}
			t.add(ref, fusedRColor(s.FusedRuptureIndex), stateIcon(state)+" "+state,
				healthColor(s.HealthScore.Value), blast, match)
		}
		t.print()
		return nil
	},
}

// --- get actions ---

var getActionsCmd = &cobra.Command{
	Use:     "actions",
	Aliases: []string{"action"},
	Short:   "List pending and recent actions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		actions, err := c.Actions(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(actions)
		}
		if len(actions) == 0 {
			fmt.Println(dim("\n  No pending actions.\n"))
			return nil
		}
		t := newTable("ID", "WORKLOAD", "TYPE", "TIER", "STATUS", "FUSEDR", "CREATED")
		for _, a := range actions {
			tierLabel := fmt.Sprintf("Tier-%d", a.Tier)
			switch a.Tier {
			case 1:
				tierLabel = red(tierLabel)
			case 2:
				tierLabel = yellow(tierLabel)
			default:
				tierLabel = dim(tierLabel)
			}
			status := a.Status
			switch status {
			case "pending":
				status = cyan(status)
			case "executed":
				status = green(status)
			case "rejected":
				status = dim(status)
			}
			t.add(
				cyan(a.ID),
				a.WorkloadID,
				a.Type,
				tierLabel,
				status,
				fusedRColor(a.FusedR),
				dim(a.CreatedAt.Format(time.RFC3339)),
			)
		}
		t.print()
		fmt.Printf("  %s   %s   %s\n\n",
			cyan("ruptura-ctl actions approve <id>"),
			cyan("ruptura-ctl actions reject <id>"),
			red("ruptura-ctl emergency-stop"),
		)
		return nil
	},
}

// --- get suppressions ---

var getSuppressionsCmd = &cobra.Command{
	Use:     "suppressions",
	Aliases: []string{"suppression", "supp"},
	Short:   "List active maintenance windows",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		supps, err := c.Suppressions(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(supps)
		}
		if len(supps) == 0 {
			fmt.Println(dim("\n  No active suppressions.\n"))
			return nil
		}
		t := newTable("ID", "WORKLOAD", "START", "END", "REASON")
		for _, s := range supps {
			remaining := ""
			if time.Now().Before(s.End) {
				remaining = yellow("  (active)")
			}
			t.add(
				cyan(s.ID),
				s.Workload,
				s.Start.Format("2006-01-02 15:04"),
				s.End.Format("2006-01-02 15:04")+remaining,
				dim(s.Reason),
			)
		}
		t.print()
		return nil
	},
}

// --- get anomalies ---

var getAnomaliesCmd = &cobra.Command{
	Use:   "anomalies",
	Short: "List anomaly detection totals",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		anomalies, err := c.Anomalies(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(anomalies)
		}
		if len(anomalies) == 0 {
			fmt.Println(dim("\n  No anomalies recorded.\n"))
			return nil
		}
		t := newTable("HOST", "SEVERITY", "TOTAL")
		t.alignRight(2)
		for _, a := range anomalies {
			sev := a.Severity
			switch sev {
			case "critical":
				sev = red(sev)
			case "warning":
				sev = yellow(sev)
			}
			t.add(a.Host, sev, fmtNum(a.Total))
		}
		t.print()
		return nil
	},
}

func init() {
	getCmd.AddCommand(getWorkloadsCmd)
	getCmd.AddCommand(getRupturesCmd)
	getCmd.AddCommand(getActionsCmd)
	getCmd.AddCommand(getSuppressionsCmd)
	getCmd.AddCommand(getAnomaliesCmd)
	rootCmd.AddCommand(getCmd)

	// shorthand aliases at root level
	rootCmd.AddCommand(&cobra.Command{
		Use:    "workloads",
		Hidden: true,
		RunE:   getWorkloadsCmd.RunE,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:    "ruptures",
		Hidden: true,
		RunE:   getRupturesCmd.RunE,
	})

}
