// Package sources collects workload signals from Prometheus-compatible APIs.
package sources

import (
	_ "embed"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed catalog.yaml
var catalogYAML []byte

// Signal describes one signal and the PromQL queries that produce it.
type Signal struct {
	Name     string `yaml:"name"`
	Mode     string `yaml:"mode"`
	Usage    string `yaml:"usage"`
	Capacity string `yaml:"capacity"`
	Restarts string `yaml:"restarts"`
}

// Input profiles, from the narrowest to the widest.
const (
	ProfileMinimal  = "minimal"
	ProfileStandard = "standard"
)

// Role is one measure with a meaning, independent of the metric that provides it.
type Role struct {
	Name    string `yaml:"name"`
	Level   string `yaml:"level"`
	Profile string `yaml:"profile"`
	Target  string `yaml:"target"`
	Query   string `yaml:"query"`
}

// Card kinds.
const (
	CardPredicted = "predicted"
	CardDetected  = "detected"
	// CardDeadline is a failure at a known time: Value returns the seconds
	// left, such as a certificate's remaining validity.
	CardDeadline = "deadline"
)

// Card is one failure of the catalog that the engine reads as data: a trend
// toward a capacity (predicted), a query that returns 1 while the failure
// holds (detected), or the seconds left before a known time (deadline).
type Card struct {
	ID          string   `yaml:"id"`
	FailureMode string   `yaml:"failureMode"`
	Level       string   `yaml:"level"`
	Kind        string   `yaml:"kind"`
	Target      []string `yaml:"target"`
	Usage       string   `yaml:"usage"`
	Capacity    string   `yaml:"capacity"`
	Value       string   `yaml:"value"`
	// Sampled, when set, returns per target the Unix time of the source
	// sample behind Usage or Value, so that a source scraped minutes apart is
	// dated by its sample, not by the time the engine read it.
	Sampled string `yaml:"sampled"`
	// ModeLabel, when set, names a label whose value becomes the failure mode
	// through Modes, for a card that covers several related failures.
	ModeLabel  string            `yaml:"modeLabel"`
	Modes      map[string]string `yaml:"modes"`
	RaiseAfter time.Duration     `yaml:"raiseAfter"`
	ClearAfter time.Duration     `yaml:"clearAfter"`
	Horizon    time.Duration     `yaml:"horizon"`
	Severity   string            `yaml:"severity"`
	Profile    string            `yaml:"profile"`
	Action     string            `yaml:"action"`
	Unit       string            `yaml:"unit"`
	// Source, when set, names a Kubernetes API reader that gives a detected
	// card its value in place of a PromQL query, such as machineconfigpools.
	Source string `yaml:"source"`
	// Above, for a source card, turns the reader's raw value into the
	// failure: 1 above it, 0 otherwise. Zero keeps the reader's 0 or 1.
	Above float64 `yaml:"above"`
	// Evidence, when set, states the reading behind a detected finding.
	Evidence string `yaml:"evidence"`
}

// Catalog is the set of known signals, roles and failure cards.
type Catalog struct {
	Signals []Signal `yaml:"signals"`
	Roles   []Role   `yaml:"roles"`
	Cards   []Card   `yaml:"cards"`
}

// Validate reports the first card that the engine cannot run.
func (c Catalog) Validate() error {
	seen := map[string]bool{}
	for _, card := range c.Cards {
		if card.ID == "" || seen[card.ID] {
			return fmt.Errorf("sources: card %q: missing or duplicate id", card.ID)
		}
		seen[card.ID] = true
		if len(card.Target) == 0 || card.Level == "" || (card.FailureMode == "" && card.ModeLabel == "") {
			return fmt.Errorf("sources: card %q: level, target and failure mode are required", card.ID)
		}
		switch card.Kind {
		case CardPredicted:
			if card.Usage == "" || card.Capacity == "" || card.Horizon <= 0 {
				return fmt.Errorf("sources: card %q: predicted needs usage, capacity and horizon", card.ID)
			}
		case CardDetected:
			if (card.Value == "") == (card.Source == "") {
				return fmt.Errorf("sources: card %q: detected needs a value or a source", card.ID)
			}
		case CardDeadline:
			if card.Value == "" || card.Horizon <= 0 {
				return fmt.Errorf("sources: card %q: deadline needs value and horizon", card.ID)
			}
		default:
			return fmt.Errorf("sources: card %q: unknown kind %q", card.ID, card.Kind)
		}
	}
	return nil
}

// RolesFor returns the roles of a level included in a profile. A wider profile
// keeps the roles of the narrower ones.
func (c Catalog) RolesFor(level, profile string) []Role {
	rank := map[string]int{ProfileMinimal: 1, ProfileStandard: 2}
	selected := rank[profile]
	var out []Role
	for _, role := range c.Roles {
		if role.Level == level && rank[role.Profile] > 0 && rank[role.Profile] <= selected {
			out = append(out, role)
		}
	}
	return out
}

// CardsFor returns the cards whose minimum profile is within profile.
func (c Catalog) CardsFor(profile string) []Card {
	rank := map[string]int{ProfileMinimal: 1, ProfileStandard: 2}
	selected := rank[profile]
	var out []Card
	for _, card := range c.Cards {
		if rank[card.Profile] > 0 && rank[card.Profile] <= selected {
			out = append(out, card)
		}
	}
	return out
}

// LoadCatalog parses the embedded catalog.yaml.
func LoadCatalog() (Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(catalogYAML, &c); err != nil {
		return Catalog{}, fmt.Errorf("sources: parse catalog: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Catalog{}, err
	}
	return c, nil
}
