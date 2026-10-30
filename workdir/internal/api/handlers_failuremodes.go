package api

import (
	"fmt"
	"math"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

const (
	degradedETF = 30 * time.Minute
	criticalETF = 10 * time.Minute
	riskHorizon = time.Hour
	maxRisk     = 10.0

	// restartWindow is the period in which restarts count as active instability.
	restartWindow = 30 * time.Minute
)

// FailureEstimates answers the current time to failure of a workload.
type FailureEstimates interface {
	Worst(workload string, now time.Time) (failuremodes.Estimate, bool)
	RecentRestarts(workload string, now time.Time, window time.Duration) int
}

// SetFailureModes wires the failure-mode estimates into the API.
func (h *Handlers) SetFailureModes(f FailureEstimates) { h.failureModes = f }

func (h *Handlers) workloadEstimate(workload string) (failuremodes.Estimate, bool) {
	if h.failureModes == nil {
		return failuremodes.Estimate{}, false
	}
	return h.failureModes.Worst(workload, time.Now())
}

// stateRestarts returns the restart count used for state classification. Recent
// restarts replace the lifetime count when tracking is active, so a workload
// that recovered does not stay critical forever.
func (h *Handlers) stateRestarts(workload string, lifetime int) int {
	if h.failureModes == nil {
		return lifetime
	}
	return h.failureModes.RecentRestarts(workload, time.Now(), restartWindow)
}

// applyEstimate raises a workload's state and risk from its time to failure.
// It never lowers a state or a risk that is already higher.
func applyEstimate(state string, risk float64, est failuremodes.Estimate) (string, float64) {
	if est.ETF > riskHorizon {
		return state, risk
	}
	risk = math.Max(risk, maxRisk*(1-est.ETF.Seconds()/riskHorizon.Seconds()))
	switch {
	case est.ETF <= criticalETF:
		return "critical", risk
	case est.ETF <= degradedETF && state != "critical":
		return "degraded", risk
	}
	return state, risk
}

func estimateHeadline(name string, est failuremodes.Estimate) string {
	return fmt.Sprintf("%s is projected to reach its memory limit in %d minutes.", name, int(math.Round(est.ETF.Minutes())))
}
