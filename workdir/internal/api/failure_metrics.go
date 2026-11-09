package api

import (
	"fmt"
	"sort"
	"strings"
)

// failureSeverity maps a failure state to the severity alert rules use.
func failureSeverity(state string) string {
	if state == "critical" {
		return "critical"
	}
	return "warning"
}

func labelValue(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}

// failureMetrics renders one series per failure in the Prometheus text format,
// so a PrometheusRule can raise each failure as a native cluster alert:
// ruptura_failure_eta_seconds is the time left (0 when detected), and the early
// and late series bound it when the engine measured a range.
func failureMetrics(failures []*FailureResult) string {
	type series struct{ name, help string }
	all := []series{
		{"ruptura_failure_eta_seconds", "Seconds until the failure Ruptura names; 0 when it is detected now."},
		{"ruptura_failure_eta_early_seconds", "Earliest expected failure, in seconds, from the spread of measured growth rates."},
		{"ruptura_failure_eta_late_seconds", "Latest expected failure, in seconds, from the spread of measured growth rates."},
	}
	sorted := append([]*FailureResult(nil), failures...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Workload+sorted[i].FailureMode < sorted[j].Workload+sorted[j].FailureMode
	})
	var b strings.Builder
	for i, s := range all {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", s.name, s.help, s.name)
		for _, f := range sorted {
			parts := strings.SplitN(f.Workload, "/", 3)
			if len(parts) != 3 {
				continue
			}
			value := []int{f.ETASeconds, f.ETAEarlySeconds, f.ETALateSeconds}[i]
			if i > 0 && value == 0 {
				continue
			}
			fmt.Fprintf(&b, "%s{namespace=\"%s\",workload_kind=\"%s\",workload=\"%s\",pod=\"%s\",failure_mode=\"%s\",failure_kind=\"%s\",severity=\"%s\",action=\"%s\"} %d\n",
				s.name, labelValue(parts[0]), labelValue(parts[1]), labelValue(parts[2]), labelValue(podName(f.Pod)), labelValue(f.FailureMode),
				labelValue(f.Kind), failureSeverity(f.State), labelValue(f.RecommendedAction), value)
		}
	}
	return b.String()
}
