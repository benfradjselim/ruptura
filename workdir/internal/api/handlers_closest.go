package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

// closestLimit is how many pods the closest list returns.
const closestLimit = 10

// ClosestSource lists the pods nearest their memory limit.
type ClosestSource interface {
	Closest(now time.Time, n int) []failuremodes.Watched
}

type closestPod struct {
	Workload string `json:"workload"`
	Pod      string `json:"pod"`
	// Usage is the working set as a share of the memory limit.
	Usage float64 `json:"usage"`
	// TrendPerHour is how much that share moves per hour.
	TrendPerHour float64 `json:"trendPerHour"`
	Readings     int     `json:"readings"`
}

// handleClosest returns the pods nearest their memory limit with their trend,
// so an empty forecast shows what the engine watches.
func (h *Handlers) handleClosest(w http.ResponseWriter, _ *http.Request) {
	out := []closestPod{}
	source, ok := h.failureModes.(ClosestSource)
	if !ok {
		writeJSON(w, http.StatusOK, out)
		return
	}
	// Read more than shown, so the namespace selector can still fill the list.
	for _, p := range source.Closest(time.Now(), closestLimit*5) {
		if len(out) == closestLimit {
			break
		}
		if !namespaceSelected(strings.SplitN(p.Workload, "/", 2)[0], h.namespaceSelector, h.namespaceLabels) {
			continue
		}
		out = append(out, closestPod{Workload: p.Workload, Pod: p.Pod, Usage: p.Usage, TrendPerHour: p.TrendPerHour, Readings: p.Readings})
	}
	writeJSON(w, http.StatusOK, out)
}
