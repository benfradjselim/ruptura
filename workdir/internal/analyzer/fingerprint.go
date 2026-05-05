package analyzer

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

const (
	fingerprintRuptureThreshold = 3.0  // FusedR must exceed this to record a fingerprint
	fingerprintMatchThreshold   = 0.85 // cosine similarity required for a pattern match
	fingerprintDebounce         = time.Hour
)

// fingerprintEngine records KPI signal vectors at confirmed ruptures and matches
// incoming snapshots against historical patterns via cosine similarity.
type fingerprintEngine struct {
	mu           sync.RWMutex
	fingerprints []models.RuptureFingerprint
	lastRecorded map[string]time.Time // workload key → last fingerprint time
	seq          int
}

func newFingerprintEngine() *fingerprintEngine {
	return &fingerprintEngine{lastRecorded: make(map[string]time.Time)}
}

// signalVector builds the 11-dimensional vector from a KPISnapshot and FusedR.
// All components are normalized to [0,1]; higher values always mean worse health.
func signalVector(snap models.KPISnapshot, fusedR float64) [11]float64 {
	return [11]float64{
		snap.Stress.Value,
		snap.Fatigue.Value,
		1 - snap.Mood.Value,       // inverted: bad = high
		snap.Pressure.Value,
		snap.Humidity.Value,
		snap.Contagion.Value,
		1 - snap.Resilience.Value, // inverted: bad = high
		snap.Entropy.Value,
		snap.Velocity.Value,
		snap.Throughput.Value,
		math.Min(fusedR/10.0, 1.0),
	}
}

func cosineSimilarity(a, b [11]float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA < 1e-12 || normB < 1e-12 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func workloadKey(snap models.KPISnapshot) string {
	if k := snap.Workload.Key(); k != "" {
		return k
	}
	return snap.Host
}

// maybeRecord stores a fingerprint when FusedR exceeds the rupture threshold and
// the per-workload debounce window has elapsed. Returns the fingerprint and true on success.
func (fe *fingerprintEngine) maybeRecord(snap models.KPISnapshot, fusedR float64) (models.RuptureFingerprint, bool) {
	if fusedR < fingerprintRuptureThreshold {
		return models.RuptureFingerprint{}, false
	}
	key := workloadKey(snap)

	fe.mu.Lock()
	defer fe.mu.Unlock()
	if last, ok := fe.lastRecorded[key]; ok && time.Since(last) < fingerprintDebounce {
		return models.RuptureFingerprint{}, false
	}

	fe.seq++
	fp := models.RuptureFingerprint{
		ID:          fmt.Sprintf("fp-%s-%d", key, fe.seq),
		WorkloadKey: key,
		CapturedAt:  time.Now(),
		Vector:      signalVector(snap, fusedR),
		FusedR:      fusedR,
	}
	fe.fingerprints = append(fe.fingerprints, fp)
	fe.lastRecorded[key] = fp.CapturedAt
	return fp, true
}

// match returns the best cosine-similarity match for the given snapshot against all
// stored fingerprints for the same workload. Returns nil when no match exceeds 0.85.
func (fe *fingerprintEngine) match(snap models.KPISnapshot, fusedR float64) *models.PatternMatch {
	key := workloadKey(snap)
	vec := signalVector(snap, fusedR)

	fe.mu.RLock()
	defer fe.mu.RUnlock()

	var best float64
	var bestFP *models.RuptureFingerprint
	for i := range fe.fingerprints {
		fp := &fe.fingerprints[i]
		if fp.WorkloadKey != key {
			continue
		}
		if sim := cosineSimilarity(vec, fp.Vector); sim > best {
			best = sim
			bestFP = fp
		}
	}
	if bestFP == nil || best < fingerprintMatchThreshold {
		return nil
	}
	return &models.PatternMatch{
		Similarity:       math.Round(best*1000) / 1000,
		MatchedRuptureID: bestFP.ID,
		MatchedAt:        bestFP.CapturedAt,
		Resolution:       bestFP.Resolution,
	}
}

// setResolution annotates a stored fingerprint with the resolution note.
func (fe *fingerprintEngine) setResolution(id, resolution string) bool {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	for i := range fe.fingerprints {
		if fe.fingerprints[i].ID == id {
			fe.fingerprints[i].Resolution = resolution
			return true
		}
	}
	return false
}

// all returns a copy of all stored fingerprints (for inspection / persistence).
func (fe *fingerprintEngine) all() []models.RuptureFingerprint {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	out := make([]models.RuptureFingerprint, len(fe.fingerprints))
	copy(out, fe.fingerprints)
	return out
}
