package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalog_Roles(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		level        string
		profile      string
		wantRole     string
		wantIncluded bool
	}{
		{"minimal has memory", "pod", ProfileMinimal, "memory.usage", true},
		{"minimal leaves out network", "pod", ProfileMinimal, "network.receive", false},
		{"standard has network", "pod", ProfileStandard, "network.receive", true},
		{"standard keeps minimal roles", "pod", ProfileStandard, "memory.usage", true},
		{"level filters roles", "workload", ProfileStandard, "memory.usage", false},
		{"workload has replicas", "workload", ProfileMinimal, "replicas.desired", true},
		{"unknown profile selects nothing", "pod", "other", "memory.usage", false},
	}
	for _, tc := range cases {
		found := false
		for _, role := range c.RolesFor(tc.level, tc.profile) {
			found = found || role.Name == tc.wantRole
		}
		if found != tc.wantIncluded {
			t.Errorf("%s: found = %v, want %v", tc.name, found, tc.wantIncluded)
		}
	}
}

func TestThanos_CollectRole_UsesTargetLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"metric":{"namespace":"n","deployment":"d"},"value":[1,"3"]}]}}`))
	}))
	defer srv.Close()

	got, err := NewThanos(srv.URL, "", nil).CollectRole(context.Background(), Role{Target: "deployment", Query: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if got["n/d"] != 3 {
		t.Errorf("values = %v, want n/d = 3", got)
	}
}
