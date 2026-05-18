package k8smetrics

import (
	"testing"
)

func TestWorkloadKey(t *testing.T) {
	tests := []struct {
		ns, pod string
		want    string
	}{
		{"ruptura-lab", "payment-service-7d6b9f4c8-xk2pq", "ruptura-lab/Deployment/payment-service"},
		{"default", "api-server-abc12-xyz", "default/Deployment/api-server"},
		{"", "payment-service-abc-xyz", ""},
		{"ns", "", ""},
		{"ns", "simple", "ns/Deployment/simple"},
	}
	for _, tc := range tests {
		got := workloadKey(tc.ns, tc.pod)
		if got != tc.want {
			t.Errorf("workloadKey(%q, %q) = %q, want %q", tc.ns, tc.pod, got, tc.want)
		}
	}
}

func TestParseCPU(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"250m", 250},
		{"1", 1000},
		{"2", 2000},
		{"100m", 100},
		{"0", 0},
	}
	for _, tc := range tests {
		got := parseCPU(tc.in)
		if got != tc.want {
			t.Errorf("parseCPU(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseMem(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"128Mi", 128 * 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"512Ki", 512 * 1024},
		{"1000000", 1000000},
	}
	for _, tc := range tests {
		got := parseMem(tc.in)
		if got != tc.want {
			t.Errorf("parseMem(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
