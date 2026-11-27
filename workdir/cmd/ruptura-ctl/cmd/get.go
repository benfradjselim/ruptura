package cmd

import (
	"fmt"
	"strings"

	"github.com/benfradjselim/ruptura/pkg/client"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "List workloads, failures or actions",
}

var getWorkloadsCmd = &cobra.Command{
	Use:     "workloads",
	Aliases: []string{"fleet", "wl"},
	Short:   "List workloads with their state",
	RunE: func(cmd *cobra.Command, args []string) error {
		fleet, err := newClient().Fleet(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(fleet)
		}
		t := newTable("WORKLOAD", "STATE", "FAILURE", "WHEN")
		for _, w := range fleet.Hosts {
			mode, when := "-", "-"
			if w.Failure != nil {
				mode, when = w.Failure.FailureMode, failureWhen(*w.Failure)
			}
			t.add(w.Host, stateIcon(w.State)+" "+w.State, mode, when)
		}
		t.print()
		fmt.Printf("\n%d workloads: %d critical, %d degraded, %d healthy\n", fleet.TotalHosts, fleet.CriticalHosts, fleet.DegradedHosts, fleet.HealthyHosts)
		return nil
	},
}

var getFailuresCmd = &cobra.Command{
	Use:     "failures",
	Aliases: []string{"failure", "f"},
	Short:   "List the failures Ruptura names now, most urgent first",
	RunE: func(cmd *cobra.Command, args []string) error {
		failures, err := newClient().Failures(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(failures)
		}
		if len(failures) == 0 {
			successLine("No failure predicted or happening")
			return nil
		}
		t := newTable("TARGET", "FAILURE", "KIND", "WHEN", "ACTION")
		for _, f := range failures {
			t.add(f.Target, f.FailureMode, f.Kind, failureWhen(f), f.RecommendedAction)
		}
		t.print()
		return nil
	},
}

var getActionsCmd = &cobra.Command{
	Use:   "actions",
	Short: "List action recommendations",
	RunE: func(cmd *cobra.Command, args []string) error {
		actions, err := newClient().Actions(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(actions)
		}
		t := newTable("ID", "WORKLOAD", "TYPE", "TIER", "STATUS")
		for _, a := range actions {
			t.add(a.ID, a.WorkloadID, a.Type, fmt.Sprintf("T%d", a.Tier), a.Status)
		}
		t.print()
		return nil
	},
}

// failureWhen says when a failure happens: "now" or the time left.
func failureWhen(f client.Failure) string {
	if f.Kind != "predicted" || f.ETASeconds <= 0 {
		return "now"
	}
	return "in " + strings.TrimSpace(etaStr((f.ETASeconds+59)/60))
}

func init() {
	getCmd.AddCommand(getWorkloadsCmd, getFailuresCmd, getActionsCmd)
	rootCmd.AddCommand(getCmd)
}
