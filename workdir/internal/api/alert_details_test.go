package api

import "testing"

func TestFormatMinutes(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{30, "less than 1 min"}, {660, "11 min"}, {689, "11 min"}, {3600, "1 h"}, {4020, "1 h 7 min"},
	}
	for _, tc := range tests {
		if got := formatMinutes(tc.seconds); got != tc.want {
			t.Errorf("formatMinutes(%d) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestAlertDetails(t *testing.T) {
	tests := []struct {
		name                   string
		failure                FailureResult
		desc, timeLeft, window string
	}{
		{"predicted with range",
			FailureResult{FailureMode: "saturation", Kind: failureKindPredicted, ETASeconds: 840, ETAEarlySeconds: 660, ETALateSeconds: 1140, Evidence: []string{"Memory at 71% of its limit", "Can hit its callers: frontend"}},
			"Predicted saturation in 14 min, window 11 min to 19 min. Memory at 71% of its limit. Can hit its callers: frontend.", "14 min", "11 min to 19 min"},
		{"predicted without range",
			FailureResult{FailureMode: "recurrent_failure", Kind: failureKindPredicted, ETASeconds: 360, Evidence: []string{"8 failures, about every 11 min"}},
			"Predicted recurrent failure in 6 min. 8 failures, about every 11 min.", "6 min", ""},
		{"detected",
			FailureResult{FailureMode: "crash_loop", Kind: failureKindDetected, Evidence: []string{"Repeated restarts in the last 30 minutes"}},
			"Repeated restarts in the last 30 minutes.", "", ""},
	}
	for _, tc := range tests {
		desc, left, window := AlertDetails(&tc.failure)
		if desc != tc.desc || left != tc.timeLeft || window != tc.window {
			t.Errorf("%s: got %q %q %q", tc.name, desc, left, window)
		}
	}
}
