package cmd

import (
	"fmt"
	"time"

	"github.com/benfradjselim/ruptura/pkg/client"
	"github.com/spf13/cobra"
)

var (
	ctxType     string
	ctxNote     string
	ctxDuration string
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage operational context entries (maintenance windows, load tests, deploys)",
	Long: `Context entries tell Ruptura what is happening in your environment.
They suppress false positives during planned events and are recorded in the XAI audit trail.

Examples:
  ruptura-ctl context add "production/payment-api" --type maintenance --note "DB upgrade"
  ruptura-ctl context add "production/*"           --type load-test   --duration 2h
  ruptura-ctl context list
  ruptura-ctl context delete ctx_abc123`,
	RunE: contextListCmd.RunE,
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active context entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		entries, err := c.ListContexts(ctx())
		if err != nil {
			return fmt.Errorf("list contexts: %w", err)
		}
		if cfgOutput == "json" {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println(dim("\n  No active context entries.\n"))
			return nil
		}
		t := newTable("ID", "WORKLOAD", "TYPE", "NOTE", "EXPIRES")
		for _, e := range entries {
			expires := dim("—")
			if !e.ExpiresAt.IsZero() {
				if time.Now().Before(e.ExpiresAt) {
					d := time.Until(e.ExpiresAt).Truncate(time.Minute)
					expires = yellow(fmt.Sprintf("%s remaining", d))
				} else {
					expires = dim("expired")
				}
			}
			t.add(
				cyan(e.ID),
				e.Service,
				e.Type,
				dim(e.Note),
				expires,
			)
		}
		t.print()
		return nil
	},
}

var contextAddCmd = &cobra.Command{
	Use:   "add <workload>",
	Short: "Add a context entry for a workload or namespace",
	Long: `Add an operational context entry. Use workload selectors:
  exact:     production/Deployment/payment-api
  namespace: production/*
  global:    *

Context types: maintenance, load-test, deploy, incident, other`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workload := args[0]

		if ctxType == "" {
			return fmt.Errorf("--type is required (maintenance, load-test, deploy, incident, other)")
		}
		validTypes := map[string]bool{
			"maintenance": true, "load-test": true,
			"deploy": true, "incident": true, "other": true,
		}
		if !validTypes[ctxType] {
			return fmt.Errorf("unknown type %q — valid: maintenance, load-test, deploy, incident, other", ctxType)
		}

		entry := client.ContextEntry{
			Service: workload,
			Type:    ctxType,
			Note:    ctxNote,
		}

		if ctxDuration != "" {
			dur, err := time.ParseDuration(ctxDuration)
			if err != nil {
				return fmt.Errorf("invalid duration %q (use 30m, 1h, 2h30m): %w", ctxDuration, err)
			}
			entry.ExpiresAt = time.Now().UTC().Add(dur)
		}

		c := newClient()
		created, err := c.AddContext(ctx(), entry)
		if err != nil {
			return fmt.Errorf("add context: %w", err)
		}

		successLine(fmt.Sprintf("Context entry created: %s", cyan(created.ID)))
		fmt.Printf("  %-16s %s\n", dim("workload"), created.Service)
		fmt.Printf("  %-16s %s\n", dim("type"), created.Type)
		if created.Note != "" {
			fmt.Printf("  %-16s %s\n", dim("note"), dim(created.Note))
		}
		if !created.ExpiresAt.IsZero() {
			fmt.Printf("  %-16s %s\n", dim("expires"), created.ExpiresAt.Format("15:04 (Jan 2)"))
		}
		fmt.Println()
		return nil
	},
}

var contextDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Remove a context entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		id := args[0]
		if err := c.DeleteContext(ctx(), id); err != nil {
			return fmt.Errorf("delete context %q: %w", id, err)
		}
		successLine(fmt.Sprintf("Context entry %s deleted.", cyan(id)))
		fmt.Println()
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	rootCmd.AddCommand(contextCmd)

	contextAddCmd.Flags().StringVar(&ctxType, "type", "", "Context type: maintenance, load-test, deploy, incident, other")
	contextAddCmd.Flags().StringVarP(&ctxNote, "note", "n", "", "Human-readable note (optional)")
	contextAddCmd.Flags().StringVarP(&ctxDuration, "duration", "d", "", "Auto-expire after duration (e.g. 30m, 2h)")
	_ = contextAddCmd.MarkFlagRequired("type")
}
