// Package sources collects workload signals from Prometheus-compatible APIs.
package sources

import (
	_ "embed"
	"fmt"

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

// Catalog is the set of known signals and roles.
type Catalog struct {
	Signals []Signal `yaml:"signals"`
	Roles   []Role   `yaml:"roles"`
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

// LoadCatalog parses the embedded catalog.yaml.
func LoadCatalog() (Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(catalogYAML, &c); err != nil {
		return Catalog{}, fmt.Errorf("sources: parse catalog: %w", err)
	}
	return c, nil
}
