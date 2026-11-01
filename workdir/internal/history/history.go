package history

import (
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

const maxPoints = 120 // 1h at 30s intervals

// Point is a single time-series snapshot for one workload.
type Point struct {
	TS                time.Time `json:"ts"`
	HealthScore       float64   `json:"health_score"`
	FusedRuptureIndex float64   `json:"fused_r"`
	Stress            float64   `json:"stress"`
	Fatigue           float64   `json:"fatigue"`
	Contagion         float64   `json:"contagion"`
	Pressure          float64   `json:"pressure"`
	Mood              float64   `json:"mood"`
	CalibrationPct    int       `json:"calibration_pct"`
	// UnavailableSignals lists the fields above whose inputs were absent.
	UnavailableSignals []string `json:"unavailable_signals,omitempty"`
}

func PointFromSnapshot(snap models.KPISnapshot) Point {
	return Point{
		TS:                 time.Now(),
		HealthScore:        snap.HealthScore.Value,
		FusedRuptureIndex:  snap.FusedRuptureIndex,
		Stress:             snap.Stress.Value,
		Fatigue:            snap.Fatigue.Value,
		Contagion:          snap.Contagion.Value,
		Pressure:           snap.Pressure.Value,
		Mood:               snap.Mood.Value,
		CalibrationPct:     snap.CalibrationProgress,
		UnavailableSignals: snap.UnavailableSignals,
	}
}

type Manager struct {
	mu       sync.RWMutex
	data     map[string][]Point
	lastPush map[string]time.Time
}

func New() *Manager {
	return &Manager{
		data:     make(map[string][]Point),
		lastPush: make(map[string]time.Time),
	}
}

// MaybePush pushes a point only if the last push for this key was > interval ago.
func (m *Manager) MaybePush(key string, snap models.KPISnapshot, now time.Time, interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.lastPush[key]; ok && now.Sub(last) < interval {
		return
	}
	m.lastPush[key] = now
	buf := m.data[key]
	buf = append(buf, PointFromSnapshot(snap))
	if len(buf) > maxPoints {
		buf = buf[len(buf)-maxPoints:]
	}
	m.data[key] = buf
}

// Get returns the time-series for a workload key.
func (m *Manager) Get(key string) []Point {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.data[key]
	out := make([]Point, len(src))
	copy(out, src)
	return out
}

// All returns all known time-series.
func (m *Manager) All() map[string][]Point {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]Point, len(m.data))
	for k, v := range m.data {
		cp := make([]Point, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
