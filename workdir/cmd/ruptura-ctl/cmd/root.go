package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/benfradjselim/ruptura/pkg/client"
	"github.com/spf13/cobra"
)

// CTLVersion is the ruptura-ctl release version — versioned independently of the server.
const CTLVersion = "1.2.0"

var (
	cfgURL     string
	cfgAPIKey  string
	cfgOutput  string
	cfgNS      string
	cfgNoColor bool
	cfgTimeout int
	noColor    bool // resolved at PersistentPreRun
)

var rootCmd = &cobra.Command{
	Use:   "ruptura-ctl",
	Short: "Command-line interface for Ruptura",
	Long: `ruptura-ctl — control and observe a running Ruptura instance.

Environment variables:
  RUPTURA_URL      Ruptura API base URL  (default: http://localhost:8080)
  RUPTURA_API_KEY  Bearer token for authentication

Examples:
  ruptura-ctl status
  ruptura-ctl get workloads -n production
  ruptura-ctl describe workload production/Deployment/payment-api
  ruptura-ctl explain <rupture-id>
  ruptura-ctl actions approve <action-id>
  ruptura-ctl suppress create payment-api 30m --reason "rolling deploy"
  ruptura-ctl weights set --selector "production/*" --stress 0.4
  ruptura-ctl sim inject cascade-failure --workload default/Deployment/api
  ruptura-ctl health`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		noColor = cfgNoColor
		// env overrides
		if v := os.Getenv("RUPTURA_URL"); v != "" {
			if !cmd.Flags().Changed("url") {
				cfgURL = v
			}
		}
		if v := os.Getenv("RUPTURA_API_KEY"); v != "" && cfgAPIKey == "" {
			cfgAPIKey = v
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print ruptura-ctl version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ruptura-ctl v%s\n", CTLVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().StringVarP(&cfgURL, "url", "u", "http://localhost:8080", "Ruptura API URL [$RUPTURA_URL]")
	rootCmd.PersistentFlags().StringVarP(&cfgAPIKey, "api-key", "k", "", "API key [$RUPTURA_API_KEY]")
	rootCmd.PersistentFlags().StringVarP(&cfgOutput, "output", "o", "table", "Output format: table|json|wide")
	rootCmd.PersistentFlags().StringVarP(&cfgNS, "namespace", "n", "", "Filter by namespace")
	rootCmd.PersistentFlags().BoolVar(&cfgNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().IntVar(&cfgTimeout, "timeout", 15, "Request timeout in seconds")
}

func newClient() *client.Client {
	return client.New(client.Config{
		BaseURL: cfgURL,
		APIKey:  cfgAPIKey,
		Timeout: time.Duration(cfgTimeout) * time.Second,
	})
}

func ctx() context.Context {
	// Use cfgTimeout so --timeout flag is respected everywhere.
	// The cancel func is intentionally not deferred here — each command
	// is short-lived and os.Exit() cleans up.
	c, _ := context.WithTimeout(context.Background(), //nolint:govet
		time.Duration(cfgTimeout)*time.Second)
	return c
}

func fatal(err error) {
	errLine(err.Error())
	fmt.Fprintln(os.Stderr, dim("  Run with --help for usage."))
	os.Exit(1)
}
