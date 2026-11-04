package failuremodes

import (
	"sort"
	"time"
)

// ModeReplicaDivergence is one replica growing toward its memory limit while
// the other replicas of the same version stay flat. The siblings make a longer
// forecast trustworthy and name the pod at fault.
const ModeReplicaDivergence Mode = "replica_divergence"

const (
	// divergenceMinReplicas is the smallest group where one pod can stand out.
	divergenceMinReplicas = 3
	// divergenceWindow is the span growth rates are read over.
	divergenceWindow = 5 * time.Minute
	// divergenceSustain is how long one pod must stand out before it is reported.
	divergenceSustain = 5 * time.Minute
	// divergenceFactor is how much faster than its siblings a pod must grow.
	divergenceFactor = 3.0
)

// divergence returns the most urgent pod growing far faster than the median of
// the other pods of its version. It reuses the per pod samples of the release
// state, so it needs the version of each pod.
func (s *releaseState) divergence(now time.Time) (Estimate, bool) {
	groups := map[string][]string{}
	for name, p := range s.pods {
		if now.Sub(p.first) >= releaseMinAge && p.capacity > 0 && len(p.samples) > 0 {
			groups[p.version] = append(groups[p.version], name)
		}
	}
	var best Estimate
	found := false
	for version, names := range groups {
		if len(names) < divergenceMinReplicas {
			continue
		}
		rates := map[string]float64{}
		for _, name := range names {
			if r, ok := rate(s.pods[name].samples, now.Add(-divergenceWindow)); ok {
				rates[name] = r
			}
		}
		for _, name := range names {
			p := s.pods[name]
			r, ok := rates[name]
			var others []float64
			for other, ro := range rates {
				if other != name {
					others = append(others, ro)
				}
			}
			if !ok || len(others) < divergenceMinReplicas-1 {
				p.divergentSince = time.Time{}
				continue
			}
			sort.Float64s(others)
			reference := others[len(others)/2]
			if reference < releaseFloor {
				reference = releaseFloor
			}
			last := p.samples[len(p.samples)-1].usage
			if r <= divergenceFactor*reference || last >= p.capacity {
				p.divergentSince = time.Time{}
				continue
			}
			if p.divergentSince.IsZero() {
				p.divergentSince = now
			}
			etf := time.Duration((p.capacity - last) / r * float64(time.Second))
			if now.Sub(p.divergentSince) < divergenceSustain || etf > releaseHorizon {
				continue
			}
			est := Estimate{Mode: ModeReplicaDivergence, Pod: name, ETF: etf, Confidence: 1, Usage: last / p.capacity,
				Version: version, Rate: r, PreviousRate: others[len(others)/2], Siblings: len(others)}
			if !found || est.ETF < best.ETF {
				best, found = est, true
			}
		}
	}
	return best, found
}
