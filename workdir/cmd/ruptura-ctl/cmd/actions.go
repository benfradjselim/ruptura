package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cfgForce bool

var actionsCmd = &cobra.Command{
	Use:   "actions",
	Short: "Manage Ruptura actions (list, approve, reject, emergency-stop)",
	Example: `  ruptura-ctl actions
  ruptura-ctl actions approve act_abc123
  ruptura-ctl actions reject  act_abc123
  ruptura-ctl actions emergency-stop`,
	RunE: getActionsCmd.RunE,
}

var actionsApproveCmd = &cobra.Command{
	Use:   "approve <action-id>",
	Short: "Approve a pending Tier-2 action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		id := args[0]
		if err := c.ApproveAction(ctx(), id); err != nil {
			return fmt.Errorf("approve %q: %w", id, err)
		}
		successLine(fmt.Sprintf("Action %s approved — executing now.", cyan(id)))
		fmt.Println()
		return nil
	},
}

var actionsRejectCmd = &cobra.Command{
	Use:   "reject <action-id>",
	Short: "Reject a pending Tier-2 action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		id := args[0]
		if err := c.RejectAction(ctx(), id); err != nil {
			return fmt.Errorf("reject %q: %w", id, err)
		}
		successLine(fmt.Sprintf("Action %s rejected.", cyan(id)))
		fmt.Println()
		return nil
	},
}

var actionsEmergencyStopCmd = &cobra.Command{
	Use:   "emergency-stop",
	Short: "Halt all Tier-1 automatic actions globally",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cfgForce {
			fmt.Println()
			fmt.Printf("  %s  This will halt ALL Tier-1 auto-actions immediately.\n", yellow("⚠"))
			fmt.Printf("  Type %s to confirm: ", bold("yes"))
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println(dim("  Cancelled."))
				fmt.Println()
				return nil
			}
		}
		c := newClient()
		if err := c.EmergencyStop(ctx()); err != nil {
			return fmt.Errorf("emergency stop: %w", err)
		}
		fmt.Println()
		fmt.Printf("  %s  All Tier-1 auto-actions halted.\n", red("■"))
		fmt.Printf("  %s  Resume with: %s\n\n",
			dim("→"),
			cyan("POST /api/v2/actions/emergency-stop/clear"),
		)
		return nil
	},
}

// Top-level shortcuts
var approveCmd = &cobra.Command{
	Use:    "approve <action-id>",
	Short:  "Approve a pending action",
	Args:   cobra.ExactArgs(1),
	Hidden: false,
	RunE:   actionsApproveCmd.RunE,
}

var rejectCmd = &cobra.Command{
	Use:    "reject <action-id>",
	Short:  "Reject a pending action",
	Args:   cobra.ExactArgs(1),
	Hidden: false,
	RunE:   actionsRejectCmd.RunE,
}

var emergencyStopCmd = &cobra.Command{
	Use:   "emergency-stop",
	Short: "Halt all Tier-1 auto-actions globally",
	RunE:  actionsEmergencyStopCmd.RunE,
}

func init() {
	actionsCmd.AddCommand(actionsApproveCmd)
	actionsCmd.AddCommand(actionsRejectCmd)
	actionsCmd.AddCommand(actionsEmergencyStopCmd)
	rootCmd.AddCommand(actionsCmd)
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(rejectCmd)
	rootCmd.AddCommand(emergencyStopCmd)

	// --force/-f skips the "yes" confirmation prompt
	for _, c := range []*cobra.Command{actionsEmergencyStopCmd, emergencyStopCmd} {
		c.Flags().BoolVarP(&cfgForce, "force", "f", false, "Skip confirmation prompt")
	}
}
