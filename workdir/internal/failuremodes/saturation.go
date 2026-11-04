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
	saturationWindow      = 30 * time.Minute
	saturationMinSamples  = 5
	saturationMaxSamples  = 120
	saturationMinAgreeing = 0.8
	// saturationMinSpan is the history required before any estimate, so that a
	// single ramp of a sawtooth is not mistaken for a trend.
	saturationMinSpan = 10 * time.Minute
)

// Estimate is a time-to-failure prediction.
type Estimate struct {
	Mode       Mode
	Pod        string
	ETF        time.Duration
	Confidence float64
	Usage      float64
	// Reason is the Kubernetes event reason behind a detected failure.
	Reason string
	// Interval and Failures describe the rhythm of a recurrent failure.
	Interval time.Duration
	Failures int
	// Version and PreviousVersion name the releases a regression compares,
	// and Rate and PreviousRate their memory growth in bytes per second.
	Version         string
	PreviousVersion string
	Rate            float64
	PreviousRate    float64
	// Siblings counts the other replicas a diverging pod is compared with.
	Siblings int
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

	if len(s.points) < saturationMinSamples || at.Sub(s.points[0].at) < saturationMinSpan {
		return Estimate{}, false
	}
	slope, agree := theilSen(s.points)
	if slope <= 0 || agree < saturationMinAgreeing {
		return Estimate{}, false
	}
	if !sustained(s.points, slope) {
		return Estimate{}, false
	}
	remaining := 1 - usage
	if remaining < 0 {
		remaining = 0
	}
	etf := time.Duration(remaining / slope * float64(time.Second))
	return Estimate{Mode: ModeSaturation, ETF: etf, Confidence: agree, Usage: usage}, true
}

// minHalfRise is the share of the expected rise each half of the window must show.
const minHalfRise = 0.2

// sustained reports whether both halves of the window, split by time, rose by at
// least minHalfRise of what the full-window slope predicts. Net rise accepts a leak
// that grows in steps, while a level shift or a spike fails because one half is flat.
func sustained(points []point, slope float64) bool {
	mid := points[0].at.Add(points[len(points)-1].at.Sub(points[0].at) / 2)
	split := sort.Search(len(points), func(i int) bool { return points[i].at.After(mid) })
	if split < 2 || len(points)-split < 2 {
		return false
	}
	for _, half := range [][]point{points[:split], points[split:]} {
		span := half[len(half)-1].at.Sub(half[0].at).Seconds()
		if netRise(half) < slope*span*minHalfRise {
			return false
		}
	}
	return true
}

// netRise compares the average of the last and first samples of a range, so that
// one noisy sample does not decide the result.
func netRise(points []point) float64 {
	edge := len(points) / 4
	if edge < 1 {
		edge = 1
	}
	return average(points[len(points)-edge:]) - average(points[:edge])
}

func average(points []point) float64 {
	sum := 0.0
	for _, p := range points {
		sum += p.usage
	}
	return sum / float64(len(points))
}

// theilSen returns the median pairwise slope in usage per second and the
// fraction of changing pairs that rise. Flat pairs are ignored, because a leak
// that grows in steps has long plateaus.
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
	positive, changing := 0, 0
	for _, v := range slopes {
		if v != 0 {
			changing++
		}
		if v > 0 {
			positive++
		}
	}
	if changing == 0 {
		return median, 0
	}
	return median, float64(positive) / float64(changing)
}
