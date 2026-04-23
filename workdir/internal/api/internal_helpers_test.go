package api

// White-box tests for unexported helper functions that cannot be reached
// through the public HTTP surface (e.g. OTLP conversion helpers, SLO isGood).
// These live in package api (not api_test) to access unexported symbols.

import (
	"testing"

	"github.com/benfradjselim/ohe/pkg/models"
)

// --- otlpSeverityToLevel ---

func TestOtlpSeverityToLevel_ByText(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"ERROR", "error"},
		{"err", "error"},
		{"FATAL", "error"},
		{"fatal", "error"},
		{"CRITICAL", "error"},
		{"critical", "error"},
		{"WARN", "warn"},
		{"warn", "warn"},
		{"WARNING", "warn"},
		{"DEBUG", "debug"},
		{"debug", "debug"},
		{"TRACE", "debug"},
		{"trace", "debug"},
		{"INFO", "info"},
		{"info", "info"},
		{"", "info"},     // falls through to number path (number=0)
	}
	for _, tc := range cases {
		t.Run(tc.text+"_text", func(t *testing.T) {
			got := otlpSeverityToLevel(tc.text, 0)
			if got != tc.want {
				t.Errorf("otlpSeverityToLevel(%q, 0) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestOtlpSeverityToLevel_ByNumber(t *testing.T) {
	cases := []struct {
		number int
		want   string
	}{
		{0, "info"},   // default
		{5, "debug"},  // 1..8
		{10, "info"},  // 9..12
		{14, "warn"},  // 13..16
		{17, "error"}, // >=17
		{24, "error"}, // >=17
	}
	for _, tc := range cases {
		got := otlpSeverityToLevel("", tc.number)
		if got != tc.want {
			t.Errorf("otlpSeverityToLevel(\"\", %d) = %q, want %q", tc.number, got, tc.want)
		}
	}
}

// --- otlpDataPoints ---

func TestOtlpDataPoints_Gauge(t *testing.T) {
	asDouble := 42.5
	m := models.OTLPMetric{
		Name: "cpu",
		Gauge: &models.OTLPGauge{
			DataPoints: []models.OTLPNumberDataPoint{
				{AsDouble: &asDouble, TimeUnixNano: "1700000000000000000"},
			},
		},
	}
	dps := otlpDataPoints(m)
	if len(dps) != 1 {
		t.Fatalf("got %d points, want 1", len(dps))
	}
	if dps[0].value != 42.5 {
		t.Errorf("got value %v, want 42.5", dps[0].value)
	}
}

func TestOtlpDataPoints_Sum(t *testing.T) {
	asInt := int64(100)
	m := models.OTLPMetric{
		Name: "reqs",
		Sum: &models.OTLPSum{
			DataPoints: []models.OTLPNumberDataPoint{
				{AsInt: &asInt, TimeUnixNano: ""},
			},
		},
	}
	dps := otlpDataPoints(m)
	if len(dps) != 1 {
		t.Fatalf("got %d points, want 1", len(dps))
	}
	if dps[0].value != 100.0 {
		t.Errorf("got value %v, want 100", dps[0].value)
	}
	// zero TimeUnixNano should fallback to time.Now (not zero)
	if dps[0].ts.IsZero() {
		t.Error("expected non-zero timestamp fallback")
	}
}

func TestOtlpDataPoints_NilGaugeAndSum(t *testing.T) {
	m := models.OTLPMetric{Name: "empty"}
	dps := otlpDataPoints(m)
	if len(dps) != 0 {
		t.Errorf("got %d points, want 0", len(dps))
	}
}

func TestOtlpDataPoints_BothNilValue(t *testing.T) {
	// Neither AsDouble nor AsInt set — should emit value=0
	m := models.OTLPMetric{
		Name:  "noop",
		Gauge: &models.OTLPGauge{DataPoints: []models.OTLPNumberDataPoint{{TimeUnixNano: "0"}}},
	}
	dps := otlpDataPoints(m)
	if len(dps) != 1 {
		t.Fatalf("got %d points, want 1", len(dps))
	}
	if dps[0].value != 0 {
		t.Errorf("got value %v, want 0", dps[0].value)
	}
}

// --- isGood ---

func TestIsGood_GTE(t *testing.T) {
	if !isGood(99.9, "gte", 99.9) {
		t.Error("99.9 gte 99.9 should be good")
	}
	if !isGood(100.0, "gte", 99.9) {
		t.Error("100.0 gte 99.9 should be good")
	}
	if isGood(99.8, "gte", 99.9) {
		t.Error("99.8 gte 99.9 should NOT be good")
	}
}

func TestIsGood_LTE(t *testing.T) {
	if !isGood(0.5, "lte", 1.0) {
		t.Error("0.5 lte 1.0 should be good")
	}
	if !isGood(1.0, "lte", 1.0) {
		t.Error("1.0 lte 1.0 should be good")
	}
	if isGood(1.1, "lte", 1.0) {
		t.Error("1.1 lte 1.0 should NOT be good")
	}
}

func TestIsGood_DefaultIsLTE(t *testing.T) {
	// Unknown comparator falls through to lte
	if !isGood(0.0, "unknown", 1.0) {
		t.Error("0.0 with unknown comparator (lte) should be good")
	}
}

// --- otlpSanitize ---

func TestOtlpSanitize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"cpu.usage", "cpu_usage"},
		{"mem-used", "mem_used"},
		{"valid_name", "valid_name"},
		{"spaces here", "spaces_here"},
	}
	for _, tc := range cases {
		got := otlpSanitize(tc.in)
		if got != tc.want {
			t.Errorf("otlpSanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
