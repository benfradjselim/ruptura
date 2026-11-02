package normalize

import (
	"reflect"
	"testing"
)

func TestBuild_MissingIsExplicit(t *testing.T) {
	collected := map[string]map[string]float64{
		"memory.usage": {"n/a": 10, "n/b": 0},
		"memory.limit": {"n/a": 100},
		"disk.limit":   {},
	}
	frames := Build([]string{"memory.usage", "memory.limit", "disk.limit"}, collected)
	if len(frames) != 2 {
		t.Fatalf("frames = %+v", frames)
	}
	a, b := frames[0], frames[1]
	if a.Target != "n/a" || a.Values["memory.limit"] != 100 || !reflect.DeepEqual(a.Missing, []string{"disk.limit"}) {
		t.Errorf("a = %+v", a)
	}
	if v, ok := b.Values["memory.usage"]; !ok || v != 0 {
		t.Errorf("a measured zero was lost: %+v", b)
	}
	if !reflect.DeepEqual(b.Missing, []string{"disk.limit", "memory.limit"}) {
		t.Errorf("b missing = %v", b.Missing)
	}
}

func TestCoverageOf(t *testing.T) {
	collected := map[string]map[string]float64{"a": {"n/x": 1, "n/y": 2}, "b": {}}
	got := CoverageOf([]string{"a", "b", "c"}, collected)
	want := []Coverage{
		{Role: "a", Targets: 2, Available: true},
		{Role: "b", Targets: 0, Available: false},
		{Role: "c", Targets: 0, Available: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coverage = %+v, want %+v", got, want)
	}
}

func TestProtectionOf(t *testing.T) {
	frames := []Frame{
		{Target: "n/a", Values: map[string]float64{"use": 1, "lim": 2}},
		{Target: "n/b", Values: map[string]float64{"use": 1}},
		{Target: "n/c", Values: map[string]float64{"lim": 2}},
	}
	got := ProtectionOf(frames, "use", "lim")
	if got.Watched != 2 || got.Protected != 1 || !reflect.DeepEqual(got.Unprotected, []string{"n/b"}) {
		t.Errorf("report = %+v", got)
	}
}
