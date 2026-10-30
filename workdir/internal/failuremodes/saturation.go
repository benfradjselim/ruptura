// Package failuremodes estimates time to failure for specific failure modes.
package failuremodes

import (
	"sort"
	"time"
)

// Mode names a failure mode.
type Mode string

// ModeSaturation is resource exhaustion against a known limit.
const ModeSaturation Mode = "saturation"

const (
	saturationWindow      = 10 * time.Minute
	saturationMinSamples  = 5
	saturationMaxSamples  = 120
	saturationMinAgreeing = 0.8
)

// Estimate is a time-to-failure prediction.
type Estimate struct {
	Mode       Mode
	ETF        time.Duration
	Confidence float64
	Usage      float64
}

type point struct {
	at    time.Time
	usage float64
}

// Saturation estimates memory exhaustion for a single pod. It is not safe for
// concurrent use.
type Saturation struct {
	points   []point
	restarts int
	seen     bool
}

// NewSaturation returns an empty estimator.
func NewSaturation() *Saturation { return &Saturation{} }

// Observe records one sample and returns an estimate when capacity is known and
// usage shows a significant upward trend. The window resets when restarts grow.
func (s *Saturation) Observe(at time.Time, usageBytes, limitBytes float64, restarts int) (Estimate, bool) {
	if s.seen && restarts > s.restarts {
		s.points = nil
	}
	s.restarts, s.seen = restarts, true
	if limitBytes <= 0 {
		return Estimate{}, false
	}
	usage := usageBytes / limitBytes
	s.points = append(s.points, point{at, usage})
	cut := 0
	for cut < len(s.points)-1 && at.Sub(s.points[cut].at) > saturationWindow {
		cut++
	}
	if over := len(s.points) - cut - saturationMaxSamples; over > 0 {
		cut += over
	}
	s.points = append([]point(nil), s.points[cut:]...)

	if len(s.points) < saturationMinSamples {
		return Estimate{}, false
	}
	slope, agree := theilSen(s.points)
	if slope <= 0 || agree < saturationMinAgreeing {
		return Estimate{}, false
	}
	remaining := 1 - usage
	if remaining < 0 {
		remaining = 0
	}
	etf := time.Duration(remaining / slope * float64(time.Second))
	return Estimate{Mode: ModeSaturation, ETF: etf, Confidence: agree, Usage: usage}, true
}

// theilSen returns the median pairwise slope in usage per second and the
// fraction of pairs whose slope has the same sign as the median.
func theilSen(points []point) (float64, float64) {
	var slopes []float64
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			dt := points[j].at.Sub(points[i].at).Seconds()
			if dt > 0 {
				slopes = append(slopes, (points[j].usage-points[i].usage)/dt)
			}
		}
	}
	if len(slopes) == 0 {
		return 0, 0
	}
	sort.Float64s(slopes)
	median := slopes[len(slopes)/2]
	if len(slopes)%2 == 0 {
		median = (slopes[len(slopes)/2-1] + slopes[len(slopes)/2]) / 2
	}
	positive := 0
	for _, v := range slopes {
		if v > 0 {
			positive++
		}
	}
	return median, float64(positive) / float64(len(slopes))
}
