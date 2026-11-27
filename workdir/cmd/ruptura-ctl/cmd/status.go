package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the fleet at a glance and the most urgent failures",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		health, err := c.Health(ctx())
		if err != nil {
			return err
		}
		fleet, err := c.Fleet(ctx())
		if err != nil {
			return err
		}
		failures, err := c.Failures(ctx())
		if err != nil {
			return err
		}
		if cfgOutput == "json" {
			return printJSON(map[string]interface{}{"health": health, "fleet": fleet, "failures": failures})
		}
		fmt.Printf("\n%s %s  %s\n", bold("Ruptura"), health.Version, dim(health.Status))
		if health.DemoMode {
			fmt.Println(yellow("  Demo mode: synthetic failures, no cluster connected"))
		}
		fmt.Printf("  %d workloads: %s critical, %s degraded, %s healthy\n\n", fleet.TotalHosts,
			red(fmt.Sprint(fleet.CriticalHosts)), yellow(fmt.Sprint(fleet.DegradedHosts)), green(fmt.Sprint(fleet.HealthyHosts)))
		if len(failures) == 0 {
			successLine("No failure predicted or happening")
			return nil
		}
		t := newTable("TARGET", "FAILURE", "WHEN")
		for i, f := range failures {
			if i == 10 {
				break
			}
			t.add(f.Target, f.FailureMode, failureWhen(f))
		}
		t.print()
		if len(failures) > 10 {
			infoLine(fmt.Sprintf("%d more: ruptura-ctl get failures", len(failures)-10))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
