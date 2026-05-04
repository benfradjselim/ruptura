package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benfradjselim/ruptura/internal/eventbus"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
	"gopkg.in/yaml.v3"
)

type ActionTier int

const (
	Tier1 ActionTier = 1 // Fully automated, C > 0.85
	Tier2 ActionTier = 2 // Suggested, C > 0.60
	Tier3 ActionTier = 3 // Human only
)

type RuptureEvent struct {
	ID         string
	Host       string
	Namespace  string // K8s namespace, empty for non-K8s hosts
	Kind       string // Deployment|StatefulSet|DaemonSet, empty for non-K8s hosts
	Metric     string
	R          float64
	Confidence float64
	Profile    string // "spike"|"fatigue"|"plateau"|"oscillation"
	Timestamp  time.Time
}

type ActionRecommendation struct {
	ID         string
	EventID    string
	Host       string
	Namespace  string     // K8s namespace for kubernetes provider
	Kind       string     // Deployment|StatefulSet|DaemonSet
	NodeName   string     // Node name for cordon actions
	ActionType string     // "scale"|"restart"|"cordon"|"alert"|"notify"|"page"|"custom"
	Tier       ActionTier
	Confidence float64
	R          float64 // rupture index at time of recommendation
	ScaleDelta int     // replica delta for "scale" action; defaults to +1 when zero
	Approved   bool
	Timestamp  time.Time
}

type ActionEngine interface {
	// Recommend evaluates a rupture event and returns recommended actions.
	Recommend(event RuptureEvent) ([]ActionRecommendation, error)
	// EmergencyStop halts all Tier-1 automated execution globally.
	EmergencyStop()
	// IsEmergencyStopped returns current emergency-stop state.
	IsEmergencyStopped() bool
}

type Rule struct {
	Name       string  `yaml:"name"`
	Profile    string  `yaml:"profile"`    // matches RuptureEvent.Profile; "" = any
	MinR       float64 `yaml:"min_r"`      // R threshold to activate rule
	ActionType string  `yaml:"action_type"`
}

const maxQueueSize = 256

type Engine struct {
	rules            []Rule
	emergencyStopped int32
	bus              eventbus.Bus

	queueMu  sync.RWMutex
	queue    []ActionRecommendation
	rejected map[string]bool
}

var defaultRules = []Rule{
	{Name: "default-spike", Profile: "spike", MinR: 3.0, ActionType: "alert"},
	{Name: "default-fatigue", Profile: "fatigue", MinR: 1.5, ActionType: "notify"},
	{Name: "default-any", Profile: "", MinR: 5.0, ActionType: "page"},
}

func New(rulesYAML []byte, bus eventbus.Bus) (*Engine, error) {
	e := &Engine{bus: bus, rejected: make(map[string]bool)}
	if len(rulesYAML) == 0 {
		e.rules = defaultRules
		return e, nil
	}
	if err := yaml.Unmarshal(rulesYAML, &e.rules); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}
	return e, nil
}

func (e *Engine) Recommend(event RuptureEvent) ([]ActionRecommendation, error) {
	if e.bus != nil {
		topic := fmt.Sprintf("ruptura.rupture.%s", event.Host)
		_ = e.bus.Publish(context.Background(), topic, "system", event)
	}

	var recs []ActionRecommendation

	tier := Tier3
	if event.Confidence > 0.85 {
		tier = Tier1
	} else if event.Confidence > 0.60 {
		tier = Tier2
	}

	for _, rule := range e.rules {
		if (rule.Profile == "" || rule.Profile == event.Profile) && event.R >= rule.MinR {
			recs = append(recs, ActionRecommendation{
				ID:         fmt.Sprintf("%s-%s", event.ID, rule.ActionType),
				EventID:    event.ID,
				Host:       event.Host,
				Namespace:  event.Namespace,
				Kind:       event.Kind,
				ActionType: rule.ActionType,
				Tier:       tier,
				Confidence: event.Confidence,
				R:          event.R,
				ScaleDelta: 1,
				Approved:   false,
				Timestamp:  time.Now(),
			})
		}
	}

	e.enqueue(recs)
	return recs, nil
}

func (e *Engine) EmergencyStop() {
	atomic.StoreInt32(&e.emergencyStopped, 1)
}

func (e *Engine) IsEmergencyStopped() bool {
	return atomic.LoadInt32(&e.emergencyStopped) == 1
}

// enqueue adds recommendations to the bounded queue (oldest evicted when full).
func (e *Engine) enqueue(recs []ActionRecommendation) {
	e.queueMu.Lock()
	e.queue = append(e.queue, recs...)
	if len(e.queue) > maxQueueSize {
		e.queue = e.queue[len(e.queue)-maxQueueSize:]
	}
	e.queueMu.Unlock()
}

// PendingActions returns all non-rejected actions in the queue.
func (e *Engine) PendingActions() []ActionRecommendation {
	e.queueMu.RLock()
	defer e.queueMu.RUnlock()
	out := make([]ActionRecommendation, 0, len(e.queue))
	for _, rec := range e.queue {
		if !e.rejected[rec.ID] {
			out = append(out, rec)
		}
	}
	return out
}

// Approve marks an action recommendation as approved.
func (e *Engine) Approve(id string) bool {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()
	for i := range e.queue {
		if e.queue[i].ID == id {
			e.queue[i].Approved = true
			return true
		}
	}
	return false
}

// Reject removes an action recommendation from the pending queue.
func (e *Engine) Reject(id string) {
	e.queueMu.Lock()
	e.rejected[id] = true
	e.queueMu.Unlock()
}

// RecommendFromAnomaly translates a critical anomaly event into a RuptureEvent
// and returns the recommended actions. Only processes SeverityCritical anomalies.
func (e *Engine) RecommendFromAnomaly(ev models.AnomalyEvent) ([]ActionRecommendation, error) {
	if ev.Severity != models.SeverityCritical {
		return nil, nil // only act on consensus anomalies
	}
	profile := "spike"
	if ev.Score <= 5.0 {
		profile = "plateau"
	}
	rupture := RuptureEvent{
		ID:         utils.GenerateID(8),
		Host:       ev.Host,
		Metric:     ev.Metric,
		R:          ev.Score,
		Confidence: 0.75, // anomaly consensus = moderate confidence
		Profile:    profile,
		Timestamp:  ev.Timestamp,
	}
	return e.Recommend(rupture)
}
