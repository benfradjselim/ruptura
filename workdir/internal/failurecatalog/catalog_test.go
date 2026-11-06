package failurecatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

const catalogDir = "../../../failures"

type card struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Level   string `yaml:"level"`
	Symptom string `yaml:"symptom"`
	Impact  string `yaml:"impact"`
	Detect  struct {
		Inputs []string `yaml:"inputs"`
		Rule   string   `yaml:"rule"`
	} `yaml:"detect"`
	Predict struct {
		Possible  string `yaml:"possible"`
		Precursor string `yaml:"precursor"`
	} `yaml:"predict"`
	PropagatesTo []string `yaml:"propagates_to"`
	Action       string   `yaml:"action"`
	Sources      []string `yaml:"sources"`
	Ruptura      struct {
		Status string `yaml:"status"`
		Mode   string `yaml:"mode"`
	} `yaml:"ruptura"`
}

var (
	idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	levels    = set("container", "pod", "workload", "namespace", "node", "network", "storage", "control-plane", "cluster", "application")
	possible  = set("yes", "partial", "no")
	statuses  = set("tested", "implemented", "planned", "not-covered")
	// engineModes are the failure modes the engine reports; crash_loop is set
	// by the API from restarts and back-off events.
	engineModes = set(string(failuremodes.ModeSaturation), string(failuremodes.ModeCPUThrottling), string(failuremodes.ModeStartupStuck),
		string(failuremodes.ModeReplicasUnavailable), string(failuremodes.ModeRecurrentFailure), string(failuremodes.ModeRolloutStuck),
		string(failuremodes.ModeReplicaDivergence), string(failuremodes.ModeReleaseRegression), string(failuremodes.ModeOOMKilled), "crash_loop")
)

func set(values ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	return m
}

func load(t *testing.T) []card {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(catalogDir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no catalog files in %s: %v", catalogDir, err)
	}
	var cards []card
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var found []card
		dec := yaml.NewDecoder(bytes.NewReader(raw))
		dec.KnownFields(true)
		if err := dec.Decode(&found); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		cards = append(cards, found...)
	}
	return cards
}

func TestCatalogCards(t *testing.T) {
	cards := load(t)
	seen := map[string]bool{}
	for _, c := range cards {
		if !idPattern.MatchString(c.ID) || seen[c.ID] {
			t.Errorf("card %q: id must be unique kebab-case", c.ID)
		}
		seen[c.ID] = true
		for field, value := range map[string]string{"title": c.Title, "symptom": c.Symptom, "impact": c.Impact, "detect.rule": c.Detect.Rule, "predict.precursor": c.Predict.Precursor, "action": c.Action} {
			if value == "" {
				t.Errorf("card %s: %s is empty", c.ID, field)
			}
		}
		if len(c.Detect.Inputs) == 0 || len(c.Sources) == 0 || len(c.PropagatesTo) == 0 {
			t.Errorf("card %s: detect.inputs, sources and propagates_to need at least one entry", c.ID)
		}
		if !levels[c.Level] || !possible[c.Predict.Possible] || !statuses[c.Ruptura.Status] {
			t.Errorf("card %s: level %q, predict.possible %q or status %q not allowed", c.ID, c.Level, c.Predict.Possible, c.Ruptura.Status)
		}
		if c.Ruptura.Mode != "" && !engineModes[c.Ruptura.Mode] {
			t.Errorf("card %s: mode %q is not a failure mode of the engine", c.ID, c.Ruptura.Mode)
		}
		if c.Ruptura.Status == "tested" && c.Ruptura.Mode == "" {
			t.Errorf("card %s: a tested card names its engine mode", c.ID)
		}
		if (c.Ruptura.Status == "planned" || c.Ruptura.Status == "not-covered") && c.Ruptura.Mode != "" {
			t.Errorf("card %s: status %s cannot name an engine mode", c.ID, c.Ruptura.Status)
		}
	}
}

func TestEveryEngineModeHasACard(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range load(t) {
		covered[c.Ruptura.Mode] = true
	}
	var missing []string
	for mode := range engineModes {
		if !covered[mode] {
			missing = append(missing, mode)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("engine modes without a catalog card: %v", missing)
	}
}
