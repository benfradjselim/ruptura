package failuremodes

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// stateVersion is bumped when the saved layout changes; other versions are ignored.
const stateVersion = 1

// stateMaxAge is the oldest reading worth restoring: saturation reads 30 minutes,
// and release and divergence rates read less.
const stateMaxAge = saturationWindow

// stateMaxBytes bounds a decompressed state, so a corrupt record cannot exhaust memory.
const stateMaxBytes = 64 << 20

// State is the part of a Tracker that an engine restart would otherwise lose:
// the recent memory readings of each pod, the release state of each workload and
// the throttling timers. Failure history and outcomes are saved separately.
type State struct {
	Version  int             `json:"v"`
	At       int64           `json:"at"`
	Pods     []podSnapshot   `json:"pods,omitempty"`
	Releases []releaseRecord `json:"rel,omitempty"`
}

// sample is a reading as [unix seconds, value], the compact form of a point.
type sample [2]float64

type podSnapshot struct {
	Pod      string `json:"p"`
	Workload string `json:"w"`
	Restarts int    `json:"r"`
	Counted  bool   `json:"c,omitempty"`
	// Capacity is the capacity the points are shares of.
	Capacity float64  `json:"k,omitempty"`
	Points   []sample `json:"s,omitempty"`
	// History is the cycle guard's usage range as [unix seconds, low, high].
	History [][3]float64 `json:"h,omitempty"`
	// Throttling timers in unix seconds, zero when unset.
	CondRun   int64 `json:"cr,omitempty"`
	CondBelow int64 `json:"cb,omitempty"`
	CondLast  int64 `json:"cl,omitempty"`
	CondOn    bool  `json:"ca,omitempty"`
}

type releaseRecord struct {
	Workload string               `json:"w"`
	Pods     []releasePodSnapshot `json:"pods,omitempty"`
	Profiles map[string]sample    `json:"prof,omitempty"`
}

type releasePodSnapshot struct {
	Pod      string   `json:"p"`
	Version  string   `json:"v"`
	First    int64    `json:"f"`
	Capacity float64  `json:"c"`
	Samples  []sample `json:"s,omitempty"`
}

func toSamples(points []point, from time.Time) []sample {
	var out []sample
	for _, p := range points {
		if !p.at.Before(from) {
			out = append(out, sample{float64(p.at.UnixNano()) / 1e9, p.usage})
		}
	}
	return out
}

func fromSamples(samples []sample, from time.Time) []point {
	var out []point
	for _, s := range samples {
		at := unixTime(s[0])
		if !at.Before(from) {
			out = append(out, point{at: at, usage: s[1]})
		}
	}
	return out
}

func unixTime(seconds float64) time.Time {
	return time.Unix(0, int64(seconds*1e9)).UTC()
}

func unixSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnixSeconds(s int64) time.Time {
	if s == 0 {
		return time.Time{}
	}
	return time.Unix(s, 0).UTC()
}

// Snapshot returns the restorable state, keeping readings of the last
// stateMaxAge only.
func (t *Tracker) Snapshot(now time.Time) State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	from := now.Add(-stateMaxAge)
	state := State{Version: stateVersion, At: now.Unix()}
	for pod, s := range t.pods {
		snap := podSnapshot{Pod: pod, Workload: s.workload, Restarts: s.restarts, Counted: s.counted, Capacity: s.capacity}
		if s.detector != nil {
			snap.Points = toSamples(s.detector.points, from)
			for _, b := range s.detector.history {
				snap.History = append(snap.History, [3]float64{float64(b.at.Unix()), b.low, b.high})
			}
		}
		if c := s.condition; c != nil {
			snap.CondRun, snap.CondBelow, snap.CondLast, snap.CondOn = unixSeconds(c.runStart), unixSeconds(c.belowSince), unixSeconds(c.last), c.active
		}
		state.Pods = append(state.Pods, snap)
	}
	for workload, r := range t.releases {
		record := releaseRecord{Workload: workload, Profiles: map[string]sample{}}
		for pod, p := range r.pods {
			record.Pods = append(record.Pods, releasePodSnapshot{Pod: pod, Version: p.version, First: p.first.Unix(), Capacity: p.capacity, Samples: toSamples(p.samples, from)})
		}
		for version, profile := range r.profiles {
			record.Profiles[version] = sample{float64(profile.at.Unix()), profile.rate}
		}
		state.Releases = append(state.Releases, record)
	}
	return state
}

// Restore loads a state saved by Snapshot into an empty tracker. Readings older
// than stateMaxAge before now are dropped, so a long outage starts cold. A pod
// that restarted while the engine was down is recorded as a failure by its next
// reading, as it would have been. Estimates come back with the next readings.
func (t *Tracker) Restore(state State, now time.Time) {
	if state.Version != stateVersion {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	from := now.Add(-stateMaxAge)
	for _, snap := range state.Pods {
		s := t.podLocked(snap.Workload, snap.Pod)
		s.restarts, s.counted, s.capacity = snap.Restarts, snap.Counted, snap.Capacity
		s.seen = unixTime(float64(state.At))
		var history []bucket
		for _, h := range snap.History {
			if at := unixTime(h[0]); now.Sub(at) <= cycleHistory {
				history = append(history, bucket{at: at, low: h[1], high: h[2]})
			}
		}
		if points := fromSamples(snap.Points, from); len(points) > 0 || len(history) > 0 {
			s.detector = &Saturation{points: points, restarts: snap.Restarts, seen: true, history: history}
		}
		if snap.CondLast != 0 {
			s.condition = NewCondition()
			s.condition.runStart, s.condition.belowSince = fromUnixSeconds(snap.CondRun), fromUnixSeconds(snap.CondBelow)
			s.condition.last, s.condition.active = fromUnixSeconds(snap.CondLast), snap.CondOn
			s.condSeen = s.seen
		}
	}
	for _, record := range state.Releases {
		r := newReleaseState()
		for _, p := range record.Pods {
			samples := fromSamples(p.Samples, from)
			if len(samples) == 0 {
				continue
			}
			r.pods[p.Pod] = &releasePod{version: p.Version, first: fromUnixSeconds(p.First), capacity: p.Capacity, samples: samples}
			t.versions[p.Pod] = p.Version
		}
		for version, profile := range record.Profiles {
			r.profiles[version] = versionProfile{rate: profile[1], at: unixTime(profile[0])}
		}
		if len(r.pods) > 0 || len(r.profiles) > 0 {
			t.releases[record.Workload] = r
		}
	}
}

// EncodeState returns a state as gzip compressed JSON.
func EncodeState(state State) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(state); err != nil {
		return nil, fmt.Errorf("encode tracker state: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("compress tracker state: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeState reads a state written by EncodeState.
func DecodeState(data []byte) (State, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return State{}, fmt.Errorf("decompress tracker state: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, stateMaxBytes+1))
	if err != nil {
		return State{}, fmt.Errorf("decompress tracker state: %w", err)
	}
	if len(raw) > stateMaxBytes {
		return State{}, fmt.Errorf("tracker state larger than %d bytes", stateMaxBytes)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("decode tracker state: %w", err)
	}
	return state, nil
}
