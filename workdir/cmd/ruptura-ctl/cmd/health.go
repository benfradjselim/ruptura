package cmd

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show Ruptura server health and ingestion statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()

		health, err := c.Health(ctx())
		if err != nil {
			return fmt.Errorf("cannot reach Ruptura at %s: %w\n\nMake sure Ruptura is running and --url is correct.", cfgURL, err)
		}

		if cfgOutput == "json" {
			return printJSON(health)
		}

		// parse self-metrics for ingest counts
		metricsRaw, metricsErr := c.Metrics(ctx())
		ingest := parseIngestMetrics(metricsRaw)

		statusDot := green("●")
		if health.Status != "ok" && health.Status != "healthy" {
			statusDot = yellow("●")
		}

		edition := health.Edition
		if edition == "" {
			edition = "community"
		}

		fmt.Println()
		fmt.Printf("  %s\n\n", bold(cyan("Ruptura Server Health")))

		fmt.Printf("  %-22s %s %s\n", dim("status"), statusDot, bold(health.Status))
		fmt.Printf("  %-22s %s\n", dim("version"), health.Version)
		fmt.Printf("  %-22s %s\n", dim("edition"), cyan(edition))
		fmt.Printf("  %-22s %s\n", dim("uptime"), uptimeStr(health.UptimeSeconds))
		fmt.Printf("  %-22s %s\n", dim("url"), cfgURL)
		if health.RuptureDetection != "" {
			fmt.Printf("  %-22s %s\n", dim("detection"), health.RuptureDetection)
		}
		fmt.Printf("  %-22s %s\n", dim("ctl version"), CTLVersion)

		// Warn if ctl major version is behind the server
		if health.Version != "" && CTLVersion != "" && health.Version != CTLVersion {
			fmt.Printf("\n  %s  ruptura-ctl v%s is behind the server v%s\n",
				yellow("⚠"), CTLVersion, health.Version)
			fmt.Printf("  %s  Update with: %s\n",
				dim("→"),
				cyan("go install github.com/benfradjselim/ruptura/cmd/ruptura-ctl@latest"),
			)
		}

		fmt.Println()
		ingestHeader := bold("Ingestion Statistics")
		if metricsErr != nil {
			ingestHeader += "  " + dim("(metrics endpoint unavailable)")
		}
		fmt.Printf("  %s\n\n", ingestHeader)

		// metrics
		fmt.Printf("  %s\n", dim("metrics (prometheus remote_write)"))
		fmt.Printf("    %-18s %s\n", dim("received"), fmtNum(ingest["metrics"]))

		fmt.Println()
		fmt.Printf("  %s\n", dim("logs (OTLP :4317)"))
		fmt.Printf("    %-18s %s\n", dim("received"), fmtNum(ingest["otlp"]))

		fmt.Println()
		fmt.Printf("  %s\n", dim("traces (OTLP :4317)"))
		fmt.Printf("    %-18s %s\n", dim("received"), fmtNum(ingest["grpc"]))

		totalAll := ingest["metrics"] + ingest["otlp"] + ingest["grpc"]
		fmt.Println()
		fmt.Printf("  %-22s %s\n\n", bold("total samples"), bold(fmtNum(totalAll)))

		// workload summary
		snaps, snapsErr := c.Snapshots(ctx())
		if snapsErr != nil {
			fmt.Printf("  %s\n\n", dim("(workload data unavailable: "+snapsErr.Error()+")"))
			return nil
		}
		calibrating := 0
		active := 0
		for _, s := range snaps {
			if s.CalibrationProgress < 100 {
				calibrating++
			} else {
				active++
			}
		}
		fmt.Printf("  %s\n\n", bold("Workload States"))
		fmt.Printf("  %-22s %s\n", dim("total"), fmt.Sprintf("%d", len(snaps)))
		fmt.Printf("  %-22s %s\n", dim("calibrating"), yellow(fmt.Sprintf("%d", calibrating)))
		fmt.Printf("  %-22s %s\n\n", dim("active"), green(fmt.Sprintf("%d", active)))

		return nil
	},
}

// parseIngestMetrics parses rpt_ingest_samples_total lines from /api/v2/metrics.
func parseIngestMetrics(raw string) map[string]int64 {
	result := map[string]int64{"metrics": 0, "otlp": 0, "grpc": 0}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "rpt_ingest_samples_total") {
			continue
		}
		// rpt_ingest_samples_total{source="prometheus"} 12847
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		switch {
		case strings.Contains(parts[0], `"prometheus"`):
			result["metrics"] += val
		case strings.Contains(parts[0], `"otlp"`):
			result["otlp"] += val
		case strings.Contains(parts[0], `"grpc"`):
			result["grpc"] += val
		}
	}
	return result
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
