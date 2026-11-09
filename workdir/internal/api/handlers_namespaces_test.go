package api

import "testing"

type fakeNamespaces map[string]map[string]string

func (f fakeNamespaces) Of(ns string) (map[string]string, bool) { l, ok := f[ns]; return l, ok }

func TestSelectNamespaces(t *testing.T) {
	results := []*FailureResult{
		{Workload: "shop/Deployment/api"},
		{Workload: "batch/Deployment/job"},
		{Workload: "new/Deployment/web"},
	}
	known := fakeNamespaces{"shop": {"ruptura.io/enabled": "true"}, "batch": {}}
	tests := []struct {
		name     string
		selector map[string]string
		source   NamespaceLabels
		want     []string
	}{
		{"no selector keeps all", nil, known, []string{"shop/Deployment/api", "batch/Deployment/job", "new/Deployment/web"}},
		{"opted-in namespace only", map[string]string{"ruptura.io/enabled": "true"}, known, []string{"shop/Deployment/api"}},
		{"labels unavailable keeps none", map[string]string{"ruptura.io/enabled": "true"}, nil, []string{}},
	}
	for _, tc := range tests {
		got := selectNamespaces(results, tc.selector, tc.source)
		names := []string{}
		for _, r := range got {
			names = append(names, r.Workload)
		}
		if len(names) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, names, tc.want)
			continue
		}
		for i := range names {
			if names[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, names, tc.want)
			}
		}
	}
}
