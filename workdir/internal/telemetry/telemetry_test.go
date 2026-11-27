package telemetry

import (
	"strings"
	"testing"
)

func TestRegistryRender(t *testing.T) {
	reg := NewRegistry("8.7.12")
	reg.IncIngestTotal("traces")
	reg.IncIngestTotal("traces")
	reg.IncIngestTotal("logs")
	rendered := reg.Render()
	for _, want := range []string{
		`rpt_ingest_samples_total{source="logs"} 1`,
		`rpt_ingest_samples_total{source="traces"} 2`,
		`rpt_version_info{version="8.7.12"} 1`,
		"rpt_uptime_seconds",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("render is missing %q:\n%s", want, rendered)
		}
	}
}
