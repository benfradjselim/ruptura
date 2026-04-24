package alerter

import (
	"fmt"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/utils"
)

// Rule defines an alert trigger condition
type Rule struct {
	Name      string
	Metric    string // KPI or metric name
	Threshold float64
	Severity  string
	Message   string
}

// defaultRules encodes the OHE alerting specification
var defaultRules = []Rule{
	{Name: "stress_nervous", Metric: "stress", Threshold: 0.3, Severity: models.SeverityInfo, Message: "System is nervous"},
	{Name: "stress_stressed", Metric: "stress", Threshold: 0.6, Severity: models.SeverityWarning, Message: "System is stressed"},
	{Name: "stress_panic", Metric: "stress", Threshold: 0.8, Severity: models.SeverityCritical, Message: "System in panic state"},
	{Name: "fatigue_tired", Metric: "fatigue", Threshold: 0.3, Severity: models.SeverityInfo, Message: "System is tired"},
	{Name: "fatigue_exhausted", Metric: "fatigue", Threshold: 0.6, Severity: models.SeverityWarning, Message: "System is exhausted"},
	{Name: "fatigue_burnout", Metric: "fatigue", Threshold: 0.8, Severity: models.SeverityCritical, Message: "Burnout imminent — schedule restart"},
	{Name: "pressure_rising", Metric: "pressure", Threshold: 0.6, Severity: models.SeverityWarning, Message: "Atmospheric pressure rising — storm approaching"},
	{Name: "pressure_storm", Metric: "pressure", Threshold: 0.7, Severity: models.SeverityCritical, Message: "Storm in ~2 hours — scale up recommended"},
	{Name: "humidity_humid", Metric: "humidity", Threshold: 0.1, Severity: models.SeverityInfo, Message: "Error humidity elevated"},
	{Name: "humidity_storm", Metric: "humidity", Threshold: 0.5, Severity: models.SeverityCritical, Message: "Error storm — activate circuit breaker"},
	{Name: "contagion_moderate", Metric: "contagion", Threshold: 0.3, Severity: models.SeverityWarning, Message: "Error contagion detected — monitor closely"},
	{Name: "contagion_epidemic", Metric: "contagion", Threshold: 0.6, Severity: models.SeverityCritical, Message: "Epidemic detected — isolate affected services"},
	{Name: "contagion_pandemic", Metric: "contagion", Threshold: 0.8, Severity: models.SeverityEmergency, Message: "Pandemic — global response required"},
	{Name: "cpu_high", Metric: "cpu_percent", Threshold: 0.85, Severity: models.SeverityWarning, Message: "CPU usage critical"},
	{Name: "memory_high", Metric: "memory_percent", Threshold: 0.90, Severity: models.SeverityWarning, Message: "Memory usage critical"},
	// ETF composed KPI rules
	{Name: "resilience_fragile", Metric: "resilience", Threshold: 0.3, Severity: models.SeverityWarning, Message: "System fragile — capacity to absorb disruption low"},
	{Name: "resilience_critical", Metric: "resilience", Threshold: 0.1, Severity: models.SeverityCritical, Message: "System critically fragile — any additional load may cascade"},
	{Name: "entropy_chaotic", Metric: "entropy", Threshold: 0.5, Severity: models.SeverityWarning, Message: "High entropy — system in chaotic state, metrics unstable"},
	{Name: "entropy_turbulent", Metric: "entropy", Threshold: 0.8, Severity: models.SeverityCritical, Message: "Turbulent entropy — extreme instability detected"},
	{Name: "velocity_volatile", Metric: "velocity", Threshold: 0.6, Severity: models.SeverityWarning, Message: "High velocity change — system state shifting rapidly"},
	{Name: "health_score_poor", Metric: "health_score", Threshold: 0.3, Severity: models.SeverityWarning, Message: "Health score degraded — system performance below baseline"},
	{Name: "health_score_critical", Metric: "health_score", Threshold: 0.15, Severity: models.SeverityCritical, Message: "Health score critical — immediate attention required"},
}

// Alerter evaluates KPI snapshots against rules and fires alerts
type Alerter struct {
	mu          sync.RWMutex
	rules       []Rule
	active      map[string]*models.Alert // key: alert ID (bounded by max 1/rule/host)
	ruleHostIdx map[string]string        // fireKey → active alert ID (O(1) resolve)
	fired       map[string]time.Time     // dedup: last fire time per rule+host
	ch          chan models.Alert
	dropped     int64 // count of alerts dropped due to full channel
}

// NewAlerter creates an alerter with default OHE rules
func NewAlerter(bufferSize int) *Alerter {
	return &Alerter{
		rules:       defaultRules,
		active:      make(map[string]*models.Alert),
		ruleHostIdx: make(map[string]string),
		fired:       make(map[string]time.Time),
		ch:          make(chan models.Alert, bufferSize),
	}
}

// Alerts returns the channel where new alerts are published
func (a *Alerter) Alerts() <-chan models.Alert {
	return a.ch
}

// Evaluate checks KPI values against all rules and fires new alerts.
// Uses an O(1) secondary index for resolution to avoid O(n) scans.
func (a *Alerter) Evaluate(host string, kpis map[string]float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Evict stale fired entries (> 5 min old) to bound map growth
	for key, ts := range a.fired {
		if now.Sub(ts) > 5*time.Minute {
			delete(a.fired, key)
		}
	}

	for _, rule := range a.rules {
		val, ok := kpis[rule.Metric]
		if !ok {
			continue
		}
		fireKey := rule.Name + ":" + host

		if val >= rule.Threshold {
			// Dedup: only fire once per minute per rule+host
			if last, fired := a.fired[fireKey]; fired && now.Sub(last) < time.Minute {
				continue
			}

			id := utils.GenerateID(8)
			alert := models.Alert{
				ID:          id,
				Name:        rule.Name,
				Description: rule.Message,
				Severity:    rule.Severity,
				Status:      models.StatusActive,
				Host:        host,
				Metric:      rule.Metric,
				Value:       val,
				Threshold:   rule.Threshold,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			a.active[id] = &alert
			a.ruleHostIdx[fireKey] = id
			a.fired[fireKey] = now

			select {
			case a.ch <- alert:
			default:
				// channel full; count dropped alerts
				a.dropped++
			}
		} else {
			// Resolve the active alert for this rule+host using the O(1) index
			if existingID, ok := a.ruleHostIdx[fireKey]; ok {
				if al, ok := a.active[existingID]; ok && al.Status == models.StatusActive {
					t := now
					al.Status = models.StatusResolved
					al.ResolvedAt = &t
					al.UpdatedAt = now
					// Remove from active map — resolved alerts are not kept
					delete(a.active, existingID)
				}
				delete(a.ruleHostIdx, fireKey)
				delete(a.fired, fireKey)
			}
		}
	}
}

// FireRupture emits an ExponentialFailure alert for a CA-ILR rupture event (v5.0).
// Deduplicates: fires at most once per minute per host:metric pair.
func (a *Alerter) FireRupture(ev models.RuptureEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	fireKey := "exponential_failure:" + ev.Host + ":" + ev.Metric
	now := time.Now()
	if last, fired := a.fired[fireKey]; fired && now.Sub(last) < time.Minute {
		return
	}

	id := utils.GenerateID(8)
	alert := models.Alert{
		ID:          id,
		Name:        "exponential_failure",
		Description: fmt.Sprintf("Exponential acceleration on %s (R=%.2f) — crash imminent before saturation threshold", ev.Metric, ev.RuptureIndex),
		Severity:    models.SeverityCritical,
		Status:      models.StatusActive,
		Host:        ev.Host,
		Metric:      ev.Metric,
		Value:       ev.RuptureIndex,
		Threshold:   3.0,
		Prediction:  "exponential_failure",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	a.active[id] = &alert
	a.ruleHostIdx[fireKey] = id
	a.fired[fireKey] = now

	select {
	case a.ch <- alert:
	default:
		a.dropped++
	}
}

// GetActive returns copies of all currently active alerts (safe for concurrent use)
func (a *Alerter) GetActive() []*models.Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*models.Alert, 0, len(a.active))
	for _, al := range a.active {
		if al.Status == models.StatusActive {
			cp := *al
			result = append(result, &cp)
		}
	}
	return result
}

// DroppedCount returns the number of alerts dropped due to a full channel buffer
func (a *Alerter) DroppedCount() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dropped
}

// GetAll returns all alerts (active + resolved)
func (a *Alerter) GetAll() []*models.Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*models.Alert, 0, len(a.active))
	for _, al := range a.active {
		copy := *al
		result = append(result, &copy)
	}
	return result
}

// GetByID returns an alert by ID
func (a *Alerter) GetByID(id string) (*models.Alert, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	al, ok := a.active[id]
	if !ok {
		return nil, false
	}
	copy := *al
	return &copy, true
}

// Acknowledge marks an alert as acknowledged
func (a *Alerter) Acknowledge(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	al, ok := a.active[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	al.Status = models.StatusAcknowledged
	al.UpdatedAt = time.Now()
	return nil
}

// Silence marks an alert as silenced
func (a *Alerter) Silence(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	al, ok := a.active[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	al.Status = models.StatusSilenced
	al.UpdatedAt = time.Now()
	return nil
}

// Delete removes an alert
func (a *Alerter) Delete(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.active[id]; !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	delete(a.active, id)
	return nil
}

// AddRule adds a custom alert rule
func (a *Alerter) AddRule(rule Rule) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rules = append(a.rules, rule)
}

// GetRules returns a copy of all rules (safe for concurrent use)
func (a *Alerter) GetRules() []Rule {
	a.mu.RLock()
	defer a.mu.RUnlock()
	cp := make([]Rule, len(a.rules))
	copy(cp, a.rules)
	return cp
}

// UpdateRule replaces the first rule whose Name matches; returns false if not found.
func (a *Alerter) UpdateRule(name string, updated Rule) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, r := range a.rules {
		if r.Name == name {
			a.rules[i] = updated
			return true
		}
	}
	return false
}

// DeleteRule removes the first rule whose Name matches; returns false if not found.
func (a *Alerter) DeleteRule(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, r := range a.rules {
		if r.Name == name {
			a.rules = append(a.rules[:i], a.rules[i+1:]...)
			return true
		}
	}
	return false
}
