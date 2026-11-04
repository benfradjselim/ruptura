package failuremodes

import (
	"sort"
	"time"
)

// ModeReleaseRegression is a new version that grows toward its memory limit far
// faster than the version it replaced. The comparison makes a longer forecast
// trustworthy than a trend alone.
const ModeReleaseRegression Mode = "release_regression"

const (
	// releaseMinAge skips the start of a pod, where warm-up dominates.
	releaseMinAge = 6 * time.Minute
	// releaseWindow is the span the current growth rate is read over.
	releaseWindow = 3 * time.Minute
	// releaseSustain is how long the regression must hold before it is reported.
	releaseSustain = 5 * time.Minute
	// releaseFactor is how much faster than the previous version a pod must grow.
	releaseFactor = 3.0
	// releaseFloor, in bytes per second (0.25 MiB per minute), keeps a flat
	// previous version from making any growth look like a regression.
	releaseFloor = 0.25 * 1024 * 1024 / 60
	// releaseHorizon is the longest time to failure reported.
	releaseHorizon = 2 * time.Hour
	// releaseSteady is how long a version must run before it is a reference.
	releaseSteady = 15 * time.Minute
	// releaseMaxSamples bounds memory per pod: two hours at 30 seconds.
	releaseMaxSamples = 240
)

type releasePod struct {
	version  string
	first    time.Time
	samples  []point
	capacity float64
	since    time.Time
}

type versionProfile struct {
	rate float64
	at   time.Time
}

type releaseState struct {
	pods     map[string]*releasePod
	profiles map[string]versionProfile
}

func newReleaseState() *releaseState {
	return &releaseState{pods: map[string]*releasePod{}, profiles: map[string]versionProfile{}}
}

// rate is the Theil-Sen slope, in bytes per second, of the samples after from.
func rate(samples []point, from time.Time) (float64, bool) {
	var pts []point
	for _, p := range samples {
		if !p.at.Before(from) {
			pts = append(pts, p)
		}
	}
	if len(pts) < 5 {
		return 0, false
	}
	var slopes []float64
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if dt := pts[j].at.Sub(pts[i].at).Seconds(); dt > 0 {
				slopes = append(slopes, (pts[j].usage-pts[i].usage)/dt)
			}
		}
	}
	if len(slopes) == 0 {
		return 0, false
	}
	sort.Float64s(slopes)
	return slopes[len(slopes)/2], true
}

func (s *releaseState) observe(pod, version string, at time.Time, usage, capacity float64) {
	if version == "" {
		return
	}
	p, ok := s.pods[pod]
	if !ok || p.version != version {
		p = &releasePod{version: version, first: at}
		s.pods[pod] = p
	}
	p.samples = append(p.samples, point{at: at, usage: usage})
	if len(p.samples) > releaseMaxSamples {
		p.samples = p.samples[len(p.samples)-releaseMaxSamples:]
	}
	p.capacity = capacity
}

// refreshProfiles stores the steady growth rate of each version whose pods have
// run long enough, so it outlives the pods after a rollout.
func (s *releaseState) refreshProfiles(now time.Time) {
	rates := map[string][]float64{}
	for _, p := range s.pods {
		if now.Sub(p.first) < releaseSteady {
			continue
		}
		if r, ok := rate(p.samples, now.Add(-releaseSteady+releaseMinAge)); ok {
			rates[p.version] = append(rates[p.version], r)
		}
	}
	for version, rs := range rates {
		sort.Float64s(rs)
		s.profiles[version] = versionProfile{rate: rs[len(rs)/2], at: now}
	}
}

// estimate returns the most urgent regression of the newest version against
// the version seen before it.
func (s *releaseState) estimate(now time.Time) (Estimate, bool) {
	s.refreshProfiles(now)
	newest, newestStart := "", time.Time{}
	for _, p := range s.pods {
		if p.first.After(newestStart) {
			newest, newestStart = p.version, p.first
		}
	}
	var previous string
	var previousAt time.Time
	for version, profile := range s.profiles {
		if version != newest && profile.at.After(previousAt) {
			previous, previousAt = version, profile.at
		}
	}
	if newest == "" || previous == "" {
		return Estimate{}, false
	}
	reference := s.profiles[previous].rate
	if reference < releaseFloor {
		reference = releaseFloor
	}
	var best Estimate
	found := false
	for name, p := range s.pods {
		if p.version != newest || now.Sub(p.first) < releaseMinAge || p.capacity <= 0 || len(p.samples) == 0 {
			continue
		}
		r, ok := rate(p.samples, now.Add(-releaseWindow))
		last := p.samples[len(p.samples)-1].usage
		regressing := ok && r > releaseFactor*reference && last < p.capacity
		if !regressing {
			p.since = time.Time{}
			continue
		}
		if p.since.IsZero() {
			p.since = now
		}
		etf := time.Duration((p.capacity - last) / r * float64(time.Second))
		if now.Sub(p.since) < releaseSustain || etf > releaseHorizon {
			continue
		}
		est := Estimate{Mode: ModeReleaseRegression, Pod: name, ETF: etf, Confidence: 1, Usage: last / p.capacity,
			Version: newest, PreviousVersion: previous, Rate: r, PreviousRate: s.profiles[previous].rate}
		if !found || est.ETF < best.ETF {
			best, found = est, true
		}
	}
	return best, found
}

// releaseProfileTTL bounds how long a version stays a reference after its
// pods are gone.
const releaseProfileTTL = 24 * time.Hour

func (s *releaseState) prune(cutoff time.Time) {
	for name, p := range s.pods {
		if len(p.samples) == 0 || p.samples[len(p.samples)-1].at.Before(cutoff) {
			delete(s.pods, name)
		}
	}
	for version, profile := range s.profiles {
		if profile.at.Before(cutoff.Add(-releaseProfileTTL)) {
			delete(s.profiles, version)
		}
	}
}
