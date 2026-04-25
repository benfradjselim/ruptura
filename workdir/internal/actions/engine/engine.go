package engine

import (
	"fmt"
	"sync/atomic"
	"time"

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
	ActionType string // "scale"|"restart"|"cordon"|"alert"|"notify"|"page"|"custom"
	Tier       ActionTier
	Confidence float64
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

type Engine struct {
	rules           []Rule
	emergencyStopped int32
}

var defaultRules = []Rule{
	{Name: "default-spike", Profile: "spike", MinR: 3.0, ActionType: "alert"},
	{Name: "default-fatigue", Profile: "fatigue", MinR: 1.5, ActionType: "notify"},
	{Name: "default-any", Profile: "", MinR: 5.0, ActionType: "page"},
}

func New(rulesYAML []byte) (*Engine, error) {
	e := &Engine{}
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
				ActionType: rule.ActionType,
				Tier:       tier,
				Confidence: event.Confidence,
				Approved:   false,
				Timestamp:  time.Now(),
			})
		}
	}

	return recs, nil
}

func (e *Engine) EmergencyStop() {
	atomic.StoreInt32(&e.emergencyStopped, 1)
}

func (e *Engine) IsEmergencyStopped() bool {
	return atomic.LoadInt32(&e.emergencyStopped) == 1
}
