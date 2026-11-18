package discovery

import (
	"encoding/json"
	"testing"
)

func TestProgressingReplicaSet(t *testing.T) {
	tests := []struct {
		message, want string
	}{
		{`ReplicaSet "web-5d4f8" has successfully progressed.`, "web-5d4f8"},
		{`ReplicaSet "web-6c7b9" is progressing.`, "web-6c7b9"},
		{`ReplicaSet "web-6c7b9" has timed out progressing.`, "web-6c7b9"},
		{"Deployment does not have minimum availability.", ""},
		{`ReplicaSet "" is progressing.`, ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := progressingReplicaSet(tt.message); got != tt.want {
			t.Errorf("progressingReplicaSet(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}

func TestToWorkloadMeta_CurrentReplicaSet(t *testing.T) {
	var w k8sWorkload
	raw := `{"metadata":{"name":"web","namespace":"shop"},"status":{"conditions":[
		{"type":"Available","message":"Deployment has minimum availability."},
		{"type":"Progressing","message":"ReplicaSet \"web-6c7b9\" is progressing."}]}}`
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	if got := toWorkloadMeta(w, "Deployment").ReplicaSet; got != "web-6c7b9" {
		t.Fatalf("ReplicaSet = %q, want web-6c7b9", got)
	}
}

func TestToCachedPod_Terminating(t *testing.T) {
	var p k8sPod
	raw := `{"metadata":{"name":"web-6c7b9-a","namespace":"shop","deletionTimestamp":"2026-11-18T10:00:00Z",
		"ownerReferences":[{"kind":"ReplicaSet","name":"web-6c7b9"}]}}`
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	cp, ok := toCachedPod(p)
	if !ok || !cp.terminating {
		t.Fatalf("terminating = %v (ok %v), want true", cp.terminating, ok)
	}
}

func TestMetadataCache_PodRetiring(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		owner       string
		ownerKind   string
		terminating bool
		want        bool
	}{
		{"pod of the current ReplicaSet", "web-new", "web-new", "ReplicaSet", false, false},
		{"pod of a superseded ReplicaSet", "web-new", "web-old", "ReplicaSet", false, true},
		{"current ReplicaSet unknown", "", "web-old", "ReplicaSet", false, false},
		{"terminating pod of the current ReplicaSet", "web-new", "web-new", "ReplicaSet", true, true},
		{"StatefulSet pod", "", "web", "StatefulSet", false, false},
		{"terminating StatefulSet pod", "", "web", "StatefulSet", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newMetadataCache()
			c.setWorkload("shop", "Deployment", "web", WorkloadMeta{Namespace: "shop", Kind: "Deployment", Name: "web", ReplicaSet: tt.current})
			c.upsertPod(cachedPod{namespace: "shop", ownerKind: tt.ownerKind, ownerName: tt.owner, terminating: tt.terminating, info: PodInfo{Name: "p"}})
			if got := c.PodRetiring("shop", "p"); got != tt.want {
				t.Fatalf("PodRetiring = %v, want %v", got, tt.want)
			}
		})
	}
	if newMetadataCache().PodRetiring("shop", "unknown") {
		t.Fatal("unknown pod reported retiring")
	}
}
