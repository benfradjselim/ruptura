package engine

import (
	"testing"
)

func TestRecommend_tier1(t *testing.T) {
	e, _ := New(nil)
	event := RuptureEvent{Confidence: 0.90}
	recs, _ := e.Recommend(event)
	if len(recs) > 0 && recs[0].Tier != Tier1 {
		t.Errorf("Expected Tier1, got %v", recs[0].Tier)
	}
}

func TestRecommend_tier2(t *testing.T) {
	e, _ := New(nil)
	event := RuptureEvent{Confidence: 0.70}
	recs, _ := e.Recommend(event)
	if len(recs) > 0 && recs[0].Tier != Tier2 {
		t.Errorf("Expected Tier2, got %v", recs[0].Tier)
	}
}

func TestRecommend_tier3(t *testing.T) {
	e, _ := New(nil)
	event := RuptureEvent{Confidence: 0.50}
	recs, _ := e.Recommend(event)
	if len(recs) > 0 && recs[0].Tier != Tier3 {
		t.Errorf("Expected Tier3, got %v", recs[0].Tier)
	}
}

func TestRecommend_defaultRules(t *testing.T) {
	e, _ := New(nil)
	event := RuptureEvent{Profile: "spike", R: 5.0} // Matches default-spike (R>=3) and default-any (R>=5)
	recs, _ := e.Recommend(event)

	foundPage := false
	for _, r := range recs {
		if r.ActionType == "page" {
			foundPage = true
		}
	}
	if !foundPage {
		t.Error("Expected page action, not found")
	}
}

func TestRecommend_profileFilter(t *testing.T) {
	e, _ := New(nil)
	event := RuptureEvent{Profile: "fatigue", R: 2.0} // Matches default-fatigue (R>=1.5)
	recs, _ := e.Recommend(event)

	for _, r := range recs {
		if r.ActionType == "alert" { // alert is only for spike
			t.Errorf("Unexpected action type: %s", r.ActionType)
		}
	}
}

func TestEmergencyStop_preventsAutoExec(t *testing.T) {
	e, _ := New(nil)
	e.EmergencyStop()
	if !e.IsEmergencyStopped() {
		t.Error("EmergencyStop failed")
	}
}

func TestLoadRules_validYAML(t *testing.T) {
	yaml := []byte(`
- name: custom
  profile: spike
  min_r: 10.0
  action_type: custom
`)
	e, err := New(yaml)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}
	if len(e.rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(e.rules))
	}
}

func TestLoadRules_emptyYAML(t *testing.T) {
	e, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}
	if len(e.rules) != len(defaultRules) {
		t.Errorf("Expected %d rules, got %d", len(defaultRules), len(e.rules))
	}
}
