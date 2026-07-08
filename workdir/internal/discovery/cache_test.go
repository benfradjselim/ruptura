package discovery

import "testing"

func TestMetadataCache_SetGet(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("prod", "Deployment", "api", WorkloadMeta{
		Namespace: "prod", Kind: "Deployment", Name: "api",
		Image: "api:v1.2.3",
		Replicas: ReplicaStatus{Desired: 3, Ready: 3, Available: 3},
	})
	m, ok := c.Get("prod", "Deployment", "api")
	if !ok {
		t.Fatal("expected workload in cache")
	}
	if m.Image != "api:v1.2.3" {
		t.Errorf("wrong image: %q", m.Image)
	}
}

func TestMetadataCache_Delete(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("ns", "StatefulSet", "db", WorkloadMeta{Name: "db"})
	c.deleteWorkload("ns", "StatefulSet", "db")
	if _, ok := c.Get("ns", "StatefulSet", "db"); ok {
		t.Error("expected workload removed")
	}
}

func TestMetadataCache_PodAssociation_StatefulSet(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("ns", "StatefulSet", "db", WorkloadMeta{Namespace: "ns", Kind: "StatefulSet", Name: "db"})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "StatefulSet", ownerName: "db",
		info: PodInfo{Name: "db-0", Node: "node-1", Status: "Running", Restarts: 0},
	})

	m, ok := c.Get("ns", "StatefulSet", "db")
	if !ok {
		t.Fatal("expected workload")
	}
	if len(m.Pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(m.Pods))
	}
	if m.Pods[0].Name != "db-0" {
		t.Errorf("wrong pod name: %q", m.Pods[0].Name)
	}
}

func TestMetadataCache_PodAssociation_Deployment(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("ns", "Deployment", "api", WorkloadMeta{Namespace: "ns", Kind: "Deployment", Name: "api"})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "ReplicaSet", ownerName: "api-7d9f4b6c8",
		info: PodInfo{Name: "api-7d9f4b6c8-xk2jq", Node: "node-2", Status: "Running"},
	})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "ReplicaSet", ownerName: "other-7d9f4b6c8",
		info: PodInfo{Name: "other-7d9f4b6c8-xyz", Node: "node-3", Status: "Running"},
	})

	m, ok := c.Get("ns", "Deployment", "api")
	if !ok {
		t.Fatal("expected workload")
	}
	if len(m.Pods) != 1 {
		t.Fatalf("expected 1 pod (not 2), got %d: %+v", len(m.Pods), m.Pods)
	}
	if m.Pods[0].Name != "api-7d9f4b6c8-xk2jq" {
		t.Errorf("wrong pod: %q", m.Pods[0].Name)
	}
}

func TestMetadataCache_DeletePod(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("ns", "StatefulSet", "db", WorkloadMeta{Namespace: "ns", Kind: "StatefulSet", Name: "db"})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "StatefulSet", ownerName: "db",
		info: PodInfo{Name: "db-0"},
	})
	c.deletePod("ns", "db-0")

	m, _ := c.Get("ns", "StatefulSet", "db")
	if len(m.Pods) != 0 {
		t.Errorf("expected 0 pods after delete, got %d", len(m.Pods))
	}
}

// TestMetadataCache_ResolvePodOwner is FBL-V2's core cache-level regression
// test: forward lookup (pod name -> owner), the direction podsForWorkload
// never provided, which is what lets the ingest path resolve a pod's real
// treatment unit instead of registering the pod itself as a "host" workload.
func TestMetadataCache_ResolvePodOwner(t *testing.T) {
	c := newMetadataCache()
	c.setWorkload("ns", "Deployment", "web-stable", WorkloadMeta{Namespace: "ns", Kind: "Deployment", Name: "web-stable"})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "ReplicaSet", ownerName: "web-stable-58cdf6849d",
		info: PodInfo{Name: "web-stable-58cdf6849d-n5l2d", Node: "node-1"},
	})
	c.setWorkload("ns", "StatefulSet", "db", WorkloadMeta{Namespace: "ns", Kind: "StatefulSet", Name: "db"})
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "StatefulSet", ownerName: "db",
		info: PodInfo{Name: "db-0"},
	})

	tests := []struct {
		name     string
		ns, pod  string
		wantKind string
		wantName string
		wantOK   bool
	}{
		{"pod owned by a Deployment via ReplicaSet", "ns", "web-stable-58cdf6849d-n5l2d", "Deployment", "web-stable", true},
		{"pod directly owned by a StatefulSet", "ns", "db-0", "StatefulSet", "db", true},
		{"unknown pod", "ns", "never-seen-pod-abc123", "", "", false},
		{"known pod name, wrong namespace", "other-ns", "web-stable-58cdf6849d-n5l2d", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotName, gotOK := c.ResolvePodOwner(tt.ns, tt.pod)
			if gotKind != tt.wantKind || gotName != tt.wantName || gotOK != tt.wantOK {
				t.Errorf("ResolvePodOwner(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.ns, tt.pod, gotKind, gotName, gotOK, tt.wantKind, tt.wantName, tt.wantOK)
			}
		})
	}
}

// TestMetadataCache_ResolvePodOwner_ReplicaSetOwnerNotYetKnown covers the
// transient case: the pod's ReplicaSet is known, but the owning Deployment
// hasn't been listed by the informer yet (race at startup, or a Deployment
// scaled to zero and never observed). Must resolve as unknown, not guess.
func TestMetadataCache_ResolvePodOwner_ReplicaSetOwnerNotYetKnown(t *testing.T) {
	c := newMetadataCache()
	c.upsertPod(cachedPod{
		namespace: "ns", ownerKind: "ReplicaSet", ownerName: "web-stable-58cdf6849d",
		info: PodInfo{Name: "web-stable-58cdf6849d-n5l2d"},
	})
	if _, _, ok := c.ResolvePodOwner("ns", "web-stable-58cdf6849d-n5l2d"); ok {
		t.Error("expected ok=false when the owning Deployment hasn't been listed yet")
	}
}

func TestIsRSOwnedBy(t *testing.T) {
	cases := []struct {
		rs, dep string
		want    bool
	}{
		{"api-7d9f4b6c8", "api", true},
		{"payment-api-7d9f4b6c8", "payment-api", true},
		{"other-7d9f4b6c8", "api", false},
		{"api", "api", false},     // no suffix
		{"api-", "api", false},    // empty suffix
	}
	for _, c := range cases {
		if got := isRSOwnedBy(c.rs, c.dep); got != c.want {
			t.Errorf("isRSOwnedBy(%q, %q) = %v, want %v", c.rs, c.dep, got, c.want)
		}
	}
}

func TestToWorkloadMeta(t *testing.T) {
	w := k8sWorkload{}
	w.Metadata.Name = "api"
	w.Metadata.Namespace = "prod"
	w.Metadata.Labels = map[string]string{"app": "api"}
	w.Metadata.CreationTimestamp = "2026-05-01T00:00:00Z"
	w.Spec.Replicas = 3
	w.Status.ReadyReplicas = 2
	w.Status.AvailableReplicas = 2
	w.Spec.Template.Spec.Containers = []struct {
		Image     string `json:"image"`
		Resources struct {
			Requests struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"requests"`
			Limits struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"limits"`
		} `json:"resources"`
	}{{Image: "api:v2.1.0"}}
	w.Spec.Template.Spec.Containers[0].Resources.Requests.CPU = "100m"
	w.Spec.Template.Spec.Containers[0].Resources.Limits.Memory = "512Mi"

	m := toWorkloadMeta(w, "Deployment")
	if m.Image != "api:v2.1.0" {
		t.Errorf("wrong image: %q", m.Image)
	}
	if m.Replicas.Desired != 3 {
		t.Errorf("wrong desired: %d", m.Replicas.Desired)
	}
	if m.Resources.Requests.CPU != "100m" {
		t.Errorf("wrong cpu request: %q", m.Resources.Requests.CPU)
	}
	if m.Resources.Limits.Memory != "512Mi" {
		t.Errorf("wrong memory limit: %q", m.Resources.Limits.Memory)
	}
}
