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
	// conditionRisk is the risk of a condition that is already happening.
	conditionRisk = 5.0
	maxRisk       = 10.0

	// restartWindow is the period in which restarts count as active instability.
	restartWindow = 30 * time.Minute

	// recentCrashLoopRestarts is the restart count in restartWindow that marks a
	// crash loop. Kubernetes backoff limits a looping pod to about one restart
	// every five minutes.
	recentCrashLoopRestarts = 3
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
	if h.failureModes.RecentRestarts(workload, time.Now(), restartWindow) >= recentCrashLoopRestarts {
		return crashLoopRestartThreshold
	}
	return 0
}

// baseRisk returns the risk before failure modes apply. A tracked workload
// takes its risk from failure modes only, because the raw rupture ratio is
// meaningless on flat series.
func (h *Handlers) baseRisk(fused float64) float64 {
	if h.failureModes != nil {
		return 0
	}
	return fused
}

// crashLoopRisk is the risk reported for a workload in a detected crash loop.
const crashLoopRisk = 9.0

// applyCrashLoop raises the risk of a workload in a detected crash loop to
// crashLoopRisk so a critical state never reports a zero rupture index. It
// never lowers a higher risk.
func applyCrashLoop(state string, risk float64, restarts int) (string, float64) {
	if restarts < crashLoopRestartThreshold {
		return state, risk
	}
	return state, math.Max(risk, crashLoopRisk)
}

// applyEstimate raises a workload's state and risk from its time to failure.
// It never lowers a state or a risk that is already higher.
func applyEstimate(state string, risk float64, est failuremodes.Estimate) (string, float64) {
	if est.Mode == failuremodes.ModeCPUThrottling {
		if state != "critical" {
			state = "degraded"
		}
		return state, math.Max(risk, conditionRisk)
	}
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
	if est.Mode == failuremodes.ModeCPUThrottling {
		return fmt.Sprintf("%s is CPU throttled %d percent of the time.", name, int(math.Round(est.Usage*100)))
	}
	return fmt.Sprintf("%s is projected to reach its memory limit in %d minutes.", name, int(math.Round(est.ETF.Minutes())))
}

func estimateAction(est failuremodes.Estimate) string {
	if est.Mode == failuremodes.ModeCPUThrottling {
		return "Raise the CPU limit or reduce the workload's CPU demand"
	}
	return "Review the memory limit or restart the pod before it is killed"
}
