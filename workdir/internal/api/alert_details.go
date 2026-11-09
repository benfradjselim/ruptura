package api

import (
	"fmt"
	"strings"
)

// formatMinutes renders a duration in seconds as the console shows it.
func formatMinutes(seconds int) string {
	if seconds < 60 {
		return "less than 1 min"
	}
	minutes := (seconds + 30) / 60
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%d h", minutes/60)
	}
	return fmt.Sprintf("%d h %d min", minutes/60, minutes%60)
}

// AlertDetails returns what an alert says beyond its labels: a description
// with the evidence and the expected time, the time left and the failure
// window. Detected failures have no time left.
func AlertDetails(f *FailureResult) (description, timeLeft, window string) {
	var parts []string
	if f.Kind == failureKindPredicted && f.ETASeconds > 0 {
		timeLeft = formatMinutes(f.ETASeconds)
		sentence := fmt.Sprintf("Predicted %s in %s", strings.ReplaceAll(f.FailureMode, "_", " "), timeLeft)
		if f.ETAEarlySeconds > 0 && f.ETALateSeconds > 0 {
			window = formatMinutes(f.ETAEarlySeconds) + " to " + formatMinutes(f.ETALateSeconds)
			sentence += ", window " + window
		}
		parts = append(parts, sentence+".")
	}
	for _, fact := range f.Evidence {
		parts = append(parts, fact+".")
	}
	return strings.Join(parts, " "), timeLeft, window
}
