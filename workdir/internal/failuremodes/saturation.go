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
	Mode Mode
	Pod  string
	ETF  time.Duration
	// ETFEarly and ETFLate bound ETF with the quartiles of the growth rates
	// the readings show. Zero means unknown.
	ETFEarly   time.Duration
	ETFLate    time.Duration
	Confidence float64
	Usage      float64
	// Reason is the Kubernetes event reason behind a detected failure, or
	// OOMReason on a recurrent failure explained by OOM kills.
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
	// Ceiling, in bytes, and CeilingSource name the usual level a pod with no
	// memory limit is compared with.
	Ceiling       float64
	CeilingSource string
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
	// history is the coarse usage range of the pod since its last restart,
	// read by the cycle guard.
	history []bucket
}

// NewSaturation returns an empty estimator.
func NewSaturation() *Saturation { return &Saturation{} }

// Observe records one sample and returns an estimate when capacity is known and
// usage shows a significant upward trend. The window resets when restarts grow.
func (s *Saturation) Observe(at time.Time, usageBytes, limitBytes float64, restarts int) (Estimate, bool) {
	if s.seen && restarts > s.restarts {
		s.points, s.history = nil, nil
	}
	s.restarts, s.seen = restarts, true
	if limitBytes <= 0 {
		return Estimate{}, false
	}
	usage := usageBytes / limitBytes
	s.history = record(s.history, at, usage)
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
	slopes := pairSlopes(s.points)
	slope, agree := theilSen(slopes)
	if slope <= 0 || agree < saturationMinAgreeing {
		return Estimate{}, false
	}
	if !sustained(s.points, slope) || decelerating(s.points, slope) || withinCycle(s.history, s.points[0].at, usage) {
		return Estimate{}, false
	}
	remaining := 1 - usage
	if remaining < 0 {
		remaining = 0
	}
	etf := time.Duration(remaining / slope * float64(time.Second))
	low, high := slopeBand(slopes)
	early, late := failureBand(remaining, low, high)
	return Estimate{Mode: ModeSaturation, ETF: etf, ETFEarly: early, ETFLate: late, Confidence: agree, Usage: usage}, true
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

// minTailRise is the share of the expected rise the last third of the window
// must still show.
const minTailRise = 0.5

// decelerating reports whether the recent samples, the last third of the window
// by time and at least saturationMinSamples points, grow at less than
// minTailRise of the full-window slope. A heavy release that warms up and then
// stabilises is concave: its early climb dominates the window slope while the
// recent samples are flat. A leak keeps its pace. The median of pairwise
// slopes keeps one outlier from deciding.
func decelerating(points []point, slope float64) bool {
	last := points[len(points)-1].at
	start := last.Add(-last.Sub(points[0].at) / 3)
	from := sort.Search(len(points), func(i int) bool { return !points[i].at.Before(start) })
	if max := len(points) - saturationMinSamples; from > max {
		from = max
	}
	recent := points[from:]
	median, _ := theilSen(pairSlopes(recent))
	span := recent[len(recent)-1].at.Sub(recent[0].at).Seconds()
	// Either measure clears a leak: the median ignores an outlier, the net
	// rise accepts a leak that grows in steps.
	return median < slope*minTailRise && netRise(recent) < slope*span*minTailRise
}

// rescale multiplies the stored usage shares by factor, so that history read
// against one capacity stays comparable after the capacity changes.
func (s *Saturation) rescale(factor float64) {
	for i := range s.points {
		s.points[i].usage *= factor
	}
	for i := range s.history {
		s.history[i].low *= factor
		s.history[i].high *= factor
	}
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

// theilSen returns the median of sorted pairwise slopes, in usage per second,
// and the fraction of changing pairs that rise. Flat pairs are ignored, because
// a leak that grows in steps has long plateaus.
func theilSen(slopes []float64) (float64, float64) {
	if len(slopes) == 0 {
		return 0, 0
	}
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
