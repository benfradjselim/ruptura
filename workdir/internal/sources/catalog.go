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

// Catalog is the set of known signals.
type Catalog struct {
	Signals []Signal `yaml:"signals"`
}

// LoadCatalog parses the embedded catalog.yaml.
func LoadCatalog() (Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(catalogYAML, &c); err != nil {
		return Catalog{}, fmt.Errorf("sources: parse catalog: %w", err)
	}
	return c, nil
}
