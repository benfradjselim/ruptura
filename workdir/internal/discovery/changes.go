package discovery

import "time"

// maxChanges bounds the workload changes kept in memory.
const maxChanges = 512

// Change is a real change of a workload specification, such as a new image.
type Change struct {
	Workload string    `json:"workload"`
	Image    string    `json:"image"`
	At       time.Time `json:"at"`
}

// recordChange stores a change when the generation or image of a known workload
// moved. Status updates, such as a pod going down, are not changes.
func (c *MetadataCache) recordChange(ns, kind, name, image string, generation int64, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous, known := c.workloads[workloadKey(ns, kind, name)]
	if !known || (previous.Image == image && previous.Generation == generation) {
		return
	}
	c.changes = append(c.changes, Change{Workload: ns + "/" + kind + "/" + name, Image: image, At: at})
	if len(c.changes) > maxChanges {
		c.changes = c.changes[len(c.changes)-maxChanges:]
	}
}

// Changes returns the changes since a time, oldest first.
func (c *MetadataCache) Changes(since time.Time) []Change {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []Change
	for _, change := range c.changes {
		if !change.At.Before(since) {
			out = append(out, change)
		}
	}
	return out
}

// RecentChanges returns workload changes since a time, oldest first.
func (inf *Informer) RecentChanges(since time.Time) []Change {
	if inf == nil || inf.cache == nil {
		return nil
	}
	return inf.cache.Changes(since)
}
