package cmd

import (
	"testing"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestKPIHealth(t *testing.T) {
	noColor = true
	defer func() { noColor = false }()
	cases := []struct {
		name string
		kpi  models.KPI
		want string
	}{
		{"unmeasured health is not a red zero", models.KPI{Value: 0, Unavailable: true}, "n/a"},
		{"measured zero stays zero", models.KPI{Value: 0}, "0"},
		{"measured health", models.KPI{Value: 87.4}, "87"},
	}
	for _, c := range cases {
		if got := kpiHealth(c.kpi); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
