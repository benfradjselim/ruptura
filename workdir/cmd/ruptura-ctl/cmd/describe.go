package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show the details of a workload",
}

var describeWorkloadCmd = &cobra.Command{
	Use:   "workload <namespace/Kind/name or name>",
	Short: "Show a workload's state and its failures, with evidence and action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleet, err := newClient().Fleet(ctx())
		if err != nil {
			return err
		}
		needle := strings.ToLower(args[0])
		found := false
		for _, w := range fleet.Hosts {
			if !strings.Contains(strings.ToLower(w.Host), needle) {
				continue
			}
			found = true
			if cfgOutput == "json" {
				if err := printJSON(w); err != nil {
					return err
				}
				continue
			}
			fmt.Printf("\n%s  %s %s\n", bold(w.Host), stateIcon(w.State), w.State)
			if w.RestartCount > 0 {
				fmt.Printf("  Restarts      %d\n", w.RestartCount)
			}
			if w.Failure == nil {
				fmt.Println(dim("  No known failure"))
				continue
			}
			f := w.Failure
			fmt.Printf("  Failure       %s, %s\n", cyan(f.FailureMode), failureWhen(*f))
			for _, e := range f.Evidence {
				fmt.Printf("  Evidence      %s\n", e)
			}
			fmt.Printf("  Action        %s\n", f.RecommendedAction)
			if len(f.Affects) > 0 {
				fmt.Printf("  Can hit       %s\n", strings.Join(f.Affects, ", "))
			}
		}
		if !found {
			return fmt.Errorf("no workload matches %q", args[0])
		}
		fmt.Println()
		return nil
	},
}

func init() {
	describeCmd.AddCommand(describeWorkloadCmd)
	rootCmd.AddCommand(describeCmd)
}
