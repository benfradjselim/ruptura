package api

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"xd", 0, true},
		{"bad", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseDuration(%q) = %v; want %v", tc.input, got, tc.want)
			}
		})
	}
}
