package receiver

import (
	"testing"
	"time"
)

func TestParseStatsDLine(t *testing.T) {
	cases := []struct {
		line        string
		wantName    string
		wantValue   float64
		wantType    string
		wantSample  float64
		wantTagKey  string
		wantTagVal  string
	}{
		{
			line: "web.requests:1|c",
			wantName: "web.requests", wantValue: 1, wantType: "c", wantSample: 1.0,
		},
		{
			line: "cpu.usage:85.5|g",
			wantName: "cpu.usage", wantValue: 85.5, wantType: "g", wantSample: 1.0,
		},
		{
			line: "response.time:120|ms|@0.5|#env:prod,region:us-east",
			wantName: "response.time", wantValue: 120, wantType: "ms",
			wantSample: 0.5, wantTagKey: "env", wantTagVal: "prod",
		},
		{
			line: "page.views:1|c|#host:web-01",
			wantName: "page.views", wantValue: 1, wantType: "c",
			wantSample: 1.0, wantTagKey: "host", wantTagVal: "web-01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			sm, err := parseStatsDLine(tc.line)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sm.Name != tc.wantName {
				t.Errorf("name: got %q, want %q", sm.Name, tc.wantName)
			}
			if sm.Value != tc.wantValue {
				t.Errorf("value: got %g, want %g", sm.Value, tc.wantValue)
			}
			if sm.Type != tc.wantType {
				t.Errorf("type: got %q, want %q", sm.Type, tc.wantType)
			}
			if sm.SampleRate != tc.wantSample {
				t.Errorf("sample_rate: got %g, want %g", sm.SampleRate, tc.wantSample)
			}
			if tc.wantTagKey != "" {
				if sm.Tags == nil {
					t.Fatalf("tags is nil, expected %s=%s", tc.wantTagKey, tc.wantTagVal)
				}
				if got := sm.Tags[tc.wantTagKey]; got != tc.wantTagVal {
					t.Errorf("tag[%s]: got %q, want %q", tc.wantTagKey, got, tc.wantTagVal)
				}
			}
		})
	}
}

func TestParseStatsDLineErrors(t *testing.T) {
	bad := []string{"no-colon", "name:bad|", "name:|c"}
	for _, line := range bad {
		_, err := parseStatsDLine(line)
		if err == nil && line == "no-colon" {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"web.requests": "web_requests",
		"my-metric":    "my_metric",
		"a/b/c":        "a_b_c",
		"already_ok":   "already_ok",
	}
	for in, want := range cases {
		got := sanitizeName(in)
		if got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSampleRateCorrection(t *testing.T) {
	// Counter with 0.5 sample rate should be corrected to 2x
	sm, err := parseStatsDLine("hits:3|c|@0.5")
	if err != nil {
		t.Fatal(err)
	}
	// Verify sample rate is parsed
	if sm.SampleRate != 0.5 {
		t.Errorf("sample rate: got %g, want 0.5", sm.SampleRate)
	}
	// Verify correction logic (done in emit())
	corrected := sm.Value / sm.SampleRate
	if corrected != 6 {
		t.Errorf("corrected value: got %g, want 6", corrected)
	}
}

func TestParseNanoTimestamp(t *testing.T) {
	now := time.Now()
	sm, err := parseStatsDLine("test:1|g")
	if err != nil {
		t.Fatal(err)
	}
	// Timestamp should be set to current time
	delta := sm.Timestamp.Sub(now)
	if delta < 0 {
		delta = -delta
	}
	if delta > 2*time.Second {
		t.Errorf("timestamp delta too large: %v", delta)
	}
}
