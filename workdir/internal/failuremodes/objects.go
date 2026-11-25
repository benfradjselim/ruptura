package failuremodes

import (
	"sort"
	"sync"
	"time"
)

const (
	// objectStale is how long an object may go without a reading before its
	// finding is no longer reported.
	objectStale = 3 * time.Minute
	// staleScrapes is how many source intervals a sample may age before its
	// source counts as stopped, even while Prometheus still serves it.
	staleScrapes = 2
	// gapScrapes is how many source intervals may pass between two samples of
	// a detected card before its pending run restarts: one missed scrape is
	// tolerated.
	gapScrapes = 3
	// stallMinRises is how many rises a trend needs before a pause can end it.
	stallMinRises = 3
)

// ObjectFinding is a failure of an object other than a pod, read from a
// catalog card: a node, a namespace quota, an autoscaler, a disruption budget.
type ObjectFinding struct {
	Card      string
	Level     string
	Target    string
	Predicted bool
	Estimate  Estimate
	// Value and Capacity are the last raw readings: usage and its capacity for
	// a predicted card, the query value for a detected one.
	Value    float64
	Capacity float64
	Unit     string
}

// sampling follows when one object was read and which source sample it saw.
type sampling struct {
	last time.Time // poll time of the last reading
	at   time.Time // time of the reading fed to the detector
	// dated is true for a card whose readings carry their sample time;
	// interval is then the gap between its last two distinct samples.
	dated    bool
	interval time.Duration
}

// next records a reading and returns the time to feed the detector, and
// whether the reading is new. A sample read twice is not new.
func (s *sampling) next(polled, sampled time.Time) (time.Time, bool) {
	s.last = polled
	if sampled.IsZero() {
		s.at = polled
		return polled, true
	}
	s.dated = true
	if !sampled.After(s.at) {
		return sampled, false
	}
	if !s.at.IsZero() {
		s.interval = sampled.Sub(s.at)
	}
	s.at = sampled
	return sampled, true
}

// stopped reports whether a dated object's newest sample is older than its
// source should allow, which happens when the source stops being scraped
// while Prometheus still serves its last sample.
func (s *sampling) stopped(now time.Time) bool {
	if !s.dated {
		return false
	}
	limit := objectStale
	if l := staleScrapes * s.interval; l > limit {
		limit = l
	}
	return now.Sub(s.at) > limit
}

type objectTrend struct {
	sampling
	level, unit string
	mode        Mode
	horizon     time.Duration
	detector    *Saturation
	estimate    Estimate
	active      bool
	value, cap  float64
}

type objectCondition struct {
	sampling
	level    string
	detector *Condition
	estimate Estimate
	active   bool
	value    float64
}

// Objects follows catalog cards per object. It is safe for concurrent use.
type Objects struct {
	mu         sync.Mutex
	trends     map[string]*objectTrend
	conditions map[string]*objectCondition
	// polled is the time of the last successful query of each card. An object
	// missing from that poll is gone: a deleted Job, a removed node.
	polled map[string]time.Time
	// intervals is the latest sample interval seen on each card, so that an
	// object first seen at one sample can be dated from the source's rhythm.
	intervals map[string]time.Duration
}

// NewObjects returns an empty object tracker.
func NewObjects() *Objects {
	return &Objects{trends: map[string]*objectTrend{}, conditions: map[string]*objectCondition{},
		polled: map[string]time.Time{}, intervals: map[string]time.Duration{}}
}

func objectKey(card, target string) string { return card + "\x00" + target }

func splitObjectKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// ObserveTrend records usage against capacity for a predicted card, read at
// polled from a source sample taken at sampled, or zero for a card whose
// readings carry no sample time. The saturation detector decides whether the
// trend is real; a forecast further than horizon is kept but not reported,
// and a trend that pauses for longer than any pause it showed while rising is
// withdrawn.
func (o *Objects) ObserveTrend(card, level, target string, mode Mode, horizon time.Duration, unit string, polled, sampled time.Time, usage, capacity float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	key := objectKey(card, target)
	state, ok := o.trends[key]
	if !ok {
		state = &objectTrend{level: level, unit: unit, mode: mode, horizon: horizon, detector: NewSaturation()}
		o.trends[key] = state
	}
	at, fresh := state.next(polled, sampled)
	if !fresh {
		return
	}
	o.noteInterval(card, state.interval)
	state.value, state.cap = usage, capacity
	est, active := state.detector.Observe(at, usage, capacity, 0)
	if active && usage < capacity && stalled(state.detector.points) {
		est, active = Estimate{}, false
	}
	est.Mode = mode
	state.estimate, state.active = est, active
}

// ObserveDetected records the value of a detected card's query, 1 while the
// failure holds and 0 otherwise, read at polled from a source sample taken at
// sampled, or zero for a card whose readings carry no sample time. A dated
// change is placed halfway between samples, and one missed scrape does not
// restart a pending run.
func (o *Objects) ObserveDetected(card, level, target string, mode Mode, raiseAfter, clearAfter time.Duration, polled, sampled time.Time, value float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	key := objectKey(card, target)
	state, ok := o.conditions[key]
	if !ok {
		state = &objectCondition{level: level, detector: NewConditionFor(mode, 1, raiseAfter, clearAfter)}
		if !sampled.IsZero() {
			var prior time.Time
			if interval := o.intervals[card]; interval > 0 {
				prior = sampled.Add(-interval)
			}
			state.detector = NewSampledCondition(mode, 1, raiseAfter, clearAfter, objectStale, prior)
		}
		o.conditions[key] = state
	}
	at, fresh := state.next(polled, sampled)
	state.value = value
	if !fresh {
		return
	}
	o.noteInterval(card, state.interval)
	if gap := gapScrapes * state.interval; state.dated && gap > objectStale {
		state.detector.maxGap = gap
	}
	state.estimate, state.active = state.detector.Observe(at, value)
}

// Seen keeps an object of a dated card alive when a poll returned its value
// but not its sample time, without feeding a reading.
func (o *Objects) Seen(card, target string, polled time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	key := objectKey(card, target)
	if state, ok := o.trends[key]; ok {
		state.last = polled
	}
	if state, ok := o.conditions[key]; ok {
		state.last = polled
	}
}

func (o *Objects) noteInterval(card string, interval time.Duration) {
	if interval > 0 {
		o.intervals[card] = interval
	}
}

// stalled reports whether usage has not risen for longer than the longest
// pause between rises in the window, plus one sample. A fill that stops is
// withdrawn within a few samples, while a leak that grows in steps keeps its
// forecast through pauses like the ones it already showed.
func stalled(points []point) bool {
	var rises []time.Time
	for i := 1; i < len(points); i++ {
		if points[i].usage > points[i-1].usage {
			rises = append(rises, points[i].at)
		}
	}
	if len(rises) < stallMinRises {
		return false
	}
	var longest time.Duration
	for i := 1; i < len(rises); i++ {
		if gap := rises[i].Sub(rises[i-1]); gap > longest {
			longest = gap
		}
	}
	last := points[len(points)-1].at
	spacing := last.Sub(points[0].at) / time.Duration(len(points)-1)
	return last.Sub(rises[len(rises)-1]) >= longest+spacing
}

// aged returns est counted down by the age of its sample at now.
func aged(est Estimate, age time.Duration) Estimate {
	if age < 0 {
		age = 0
	}
	down := func(d time.Duration) time.Duration {
		if d == 0 {
			return 0
		}
		if d -= age; d < 0 {
			return 0
		}
		return d
	}
	est.ETF, est.ETFEarly, est.ETFLate = down(est.ETF), down(est.ETFEarly), down(est.ETFLate)
	return est
}

// Polled records a successful query of card at the given time. Call it after
// the readings of that poll.
func (o *Objects) Polled(card string, at time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.polled[card] = at
}

// gone reports whether an object missed the last successful poll of its card,
// has had no reading for objectStale, or its source stopped.
func (o *Objects) gone(card string, s sampling, now time.Time) bool {
	if polled, ok := o.polled[card]; ok && s.last.Before(polled) {
		return true
	}
	return now.Sub(s.last) > objectStale || s.stopped(now)
}

// Findings returns the active findings with a recent reading, most urgent
// first. A forecast beyond its card's horizon is left out.
func (o *Objects) Findings(now time.Time) []ObjectFinding {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []ObjectFinding
	for key, state := range o.trends {
		card, target := splitObjectKey(key)
		est := aged(state.estimate, now.Sub(state.at))
		if !state.active || o.gone(card, state.sampling, now) || est.ETF > state.horizon {
			continue
		}
		out = append(out, ObjectFinding{Card: card, Level: state.level, Target: target, Predicted: true,
			Estimate: est, Value: state.value, Capacity: state.cap, Unit: state.unit})
	}
	for key, state := range o.conditions {
		card, target := splitObjectKey(key)
		if !state.active || o.gone(card, state.sampling, now) {
			continue
		}
		out = append(out, ObjectFinding{Card: card, Level: state.level, Target: target,
			Estimate: state.estimate, Value: state.value})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Predicted != out[j].Predicted {
			return !out[i].Predicted
		}
		if out[i].Estimate.ETF != out[j].Estimate.ETF {
			return out[i].Estimate.ETF < out[j].Estimate.ETF
		}
		return out[i].Target < out[j].Target
	})
	return out
}

// Prune forgets objects with no reading since before.
func (o *Objects) Prune(before time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for key, state := range o.trends {
		if state.last.Before(before) {
			delete(o.trends, key)
		}
	}
	for key, state := range o.conditions {
		if state.last.Before(before) {
			delete(o.conditions, key)
		}
	}
}
