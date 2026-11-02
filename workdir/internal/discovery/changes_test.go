package discovery

import (
	"testing"
	"time"
)

func TestMetadataCache_RecordsRealChanges(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	c := newMetadataCache()
	put := func(image string, generation int64, ready int, at time.Time) {
		c.recordChange("n", "Deployment", "w", image, generation, at)
		c.setWorkload("n", "Deployment", "w", WorkloadMeta{Image: image, Generation: generation, Replicas: ReplicaStatus{Ready: ready}})
	}
	put("app:1", 1, 1, now)
	put("app:1", 1, 0, now.Add(time.Minute))
	put("app:2", 2, 1, now.Add(2*time.Minute))
	put("app:2", 3, 1, now.Add(3*time.Minute))

	got := c.Changes(now.Add(-time.Hour))
	if len(got) != 2 {
		t.Fatalf("changes = %+v, want the image and spec changes only", got)
	}
	if got[0].Workload != "n/Deployment/w" || got[0].Image != "app:2" || !got[0].At.Equal(now.Add(2*time.Minute)) {
		t.Errorf("first change = %+v", got[0])
	}
	if len(c.Changes(now.Add(150*time.Second))) != 1 {
		t.Error("changes before the window must be left out")
	}
}
