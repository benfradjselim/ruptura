package api

import (
	"testing"

	"github.com/benfradjselim/ruptura/internal/normalize"
)

func TestPrometheusCoverage(t *testing.T) {
	tests := []struct {
		name        string
		coverage    []normalize.Coverage
		wantSeries  int64
		wantSignals int64
	}{
		{"nothing collected", nil, 0, 0},
		{"counts series and signals with data", []normalize.Coverage{
			{Role: "memory.usage", Targets: 120, Available: true},
			{Role: "cpu.throttled", Targets: 80, Available: true},
			{Role: "disk.limit", Targets: 0, Available: false},
		}, 200, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			series, signals := prometheusCoverage(tt.coverage)
			if series != tt.wantSeries || signals != tt.wantSignals {
				t.Fatalf("got %d series %d signals, want %d and %d", series, signals, tt.wantSeries, tt.wantSignals)
			}
		})
	}
}
