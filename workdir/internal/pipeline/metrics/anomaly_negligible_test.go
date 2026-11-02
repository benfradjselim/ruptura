package metrics

import "testing"

func TestNegligibleDeviation_ByMetric(t *testing.T) {
	cases := []struct {
		name            string
		metric          string
		value, expected float64
		want            bool
	}{
		{"small network change", "k8s.pod.network.io", 202, 277, true},
		{"network doubled", "k8s.pod.network.io", 600, 290, true},
		{"periodic burst below 3x", "k8s.pod.network.io", 7273, 3565, true},
		{"network tripled", "k8s.pod.network.io", 900, 290, false},
		{"network dropped to a third", "k8s.pod.network.io", 90, 290, false},
		{"network stopped", "k8s.pod.network.io", 0, 290, false},
		{"tiny cpu in cores", "k8s.pod.cpu.utilization", 0.03, 0.005, true},
		{"real cpu jump in cores", "k8s.pod.cpu.utilization", 0.4, 0.1, false},
		{"large but not doubled cpu", "k8s.pod.cpu.utilization", 1.3, 1.0, true},
		{"cpu time rate doubled", "k8s.pod.cpu.time", 0.3, 0.1, false},
		{"tiny cpu time rate", "k8s.pod.cpu.time", 0.04, 0.003, true},
		{"cpu limit share after restart", "k8s.pod.cpu_limit_utilization", 0.0228, 0.001, true},
		{"cpu limit share up 7 points", "k8s.pod.cpu_limit_utilization", 0.067, 0.0277, true},
		{"cpu limit saturated", "k8s.pod.cpu_limit_utilization", 0.99, 0.03, false},
		{"cpu limit share up but not doubled", "k8s.pod.cpu_limit_utilization", 0.75, 0.50, true},
		{"container cpu limit saturated", "k8s.container.cpu_limit_utilization", 0.9, 0.2, false},
		{"limit ratio small", "k8s.pod.memory_limit_utilization", 0.41, 0.40, true},
		{"limit ratio real", "k8s.pod.memory_limit_utilization", 0.60, 0.40, false},
		{"unknown metric keeps old floor", "custom", 130, 100, false},
	}
	for _, c := range cases {
		if got := negligibleDeviation(c.metric, c.value, c.expected); got != c.want {
			t.Errorf("%s: negligible = %v, want %v", c.name, got, c.want)
		}
	}
}
