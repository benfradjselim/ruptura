package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

const (
	failureLevelNode    = "node"
	failureLevelCluster = "cluster"
)

// clusterWide reports whether a finding belongs to no namespace: a node, or a
// cluster component such as etcd, the API server or a cluster operator.
func clusterWide(level string) bool { return level == failureLevelNode || level == failureLevelCluster }

// ObjectFindings reports failures of objects read from catalog cards.
type ObjectFindings interface {
	Findings(now time.Time) []failuremodes.ObjectFinding
}

// CardInfo is what the API shows for a catalog card besides the finding.
type CardInfo struct {
	Action   string
	Severity string
	// Evidence states the reading behind a detected finding; empty keeps the default.
	Evidence string
}

// SetObjectFindings adds the findings of catalog cards to the failure list.
func (h *Handlers) SetObjectFindings(findings ObjectFindings, cards map[string]CardInfo) {
	h.objectFindings, h.cardInfo = findings, cards
}

// objectFailures returns the card findings this instance may show. Node and
// cluster findings belong to no namespace, so an instance limited to some
// namespaces does not show them.
func (h *Handlers) objectFailures(now time.Time) []*FailureResult {
	if h.objectFindings == nil {
		return nil
	}
	var out []*FailureResult
	for _, finding := range h.objectFindings.Findings(now) {
		if clusterWide(finding.Level) {
			if len(h.namespaceSelector) > 0 {
				continue
			}
		} else if !namespaceSelected(strings.SplitN(finding.Target, "/", 2)[0], h.namespaceSelector, h.namespaceLabels) {
			continue
		}
		out = append(out, objectResult(finding, h.cardInfo[finding.Card]))
	}
	return out
}

// objectResult turns a card finding into a failure result. A detected card
// takes its state from the card severity; a forecast is critical when close.
func objectResult(f failuremodes.ObjectFinding, info CardInfo) *FailureResult {
	result := &FailureResult{
		Level:             f.Level,
		Target:            f.Target,
		FailureMode:       string(f.Estimate.Mode),
		Kind:              failureKindDetected,
		State:             "degraded",
		RecommendedAction: info.Action,
		Evidence:          []string{objectEvidence(f, info)},
	}
	if !clusterWide(f.Level) {
		result.Workload = f.Target
	}
	if !f.Predicted {
		if info.Severity == "critical" {
			result.State = "critical"
		}
		return result
	}
	result.Kind = failureKindPredicted
	result.Confidence = f.Estimate.Confidence
	result.ETASeconds = int(f.Estimate.ETF.Seconds())
	result.ETAEarlySeconds = int(f.Estimate.ETFEarly.Seconds())
	result.ETALateSeconds = int(f.Estimate.ETFLate.Seconds())
	if f.Estimate.ETF <= criticalETF {
		result.State = "critical"
	}
	return result
}

// objectEvidence states the reading behind a finding.
func objectEvidence(f failuremodes.ObjectFinding, info CardInfo) string {
	if !f.Predicted {
		if info.Evidence != "" {
			return info.Evidence
		}
		return "Reported by kube-state-metrics at the last reading"
	}
	percent := 0.0
	if f.Capacity > 0 {
		percent = 100 * f.Value / f.Capacity
	}
	if f.Unit == "bytes" {
		return fmt.Sprintf("%d MiB used of %d MiB (%.0f%%) and growing", mebibytes(f.Value), mebibytes(f.Capacity), percent)
	}
	return fmt.Sprintf("%s used of %s (%.0f%%) and growing", plainNumber(f.Value), plainNumber(f.Capacity), percent)
}

func plainNumber(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}
