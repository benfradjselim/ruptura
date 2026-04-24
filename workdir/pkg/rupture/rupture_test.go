package rupture_test

import (
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/rupture"
)

func TestIndex_stable(t *testing.T) {
	r := rupture.Index(0.5, 1.0)
	if r != 0.5 {
		t.Fatalf("want 0.5, got %f", r)
	}
}

func TestIndex_epsilon(t *testing.T) {
	r := rupture.Index(1.0, 0.0)
	want := 1.0 / 1e-6
	if r != want {
		t.Fatalf("want %f, got %f", want, r)
	}
}

func TestIndex_emergency(t *testing.T) {
	r := rupture.Index(10.0, 1.0)
	if r < 5.0 {
		t.Fatalf("expected emergency level, got %f", r)
	}
}

func TestIndex_negativeSigns(t *testing.T) {
	r := rupture.Index(-2.0, -1.0)
	if r != 2.0 {
		t.Fatalf("want 2.0, got %f", r)
	}
}

func TestTTF_basic(t *testing.T) {
	d := rupture.TTF(50.0, 100.0, 10.0)
	want := 5 * time.Second
	if d != want {
		t.Fatalf("want %v, got %v", want, d)
	}
}

func TestTTF_clamped(t *testing.T) {
	d := rupture.TTF(0, 1e9, 1.0)
	if d != 3600*time.Second {
		t.Fatalf("expected clamp to 3600s, got %v", d)
	}
}

func TestTTF_nonPositiveBurst(t *testing.T) {
	if d := rupture.TTF(50.0, 100.0, 0.0); d != 3600*time.Second {
		t.Fatalf("want 3600s for zero burst, got %v", d)
	}
	if d := rupture.TTF(50.0, 100.0, -1.0); d != 3600*time.Second {
		t.Fatalf("want 3600s for negative burst, got %v", d)
	}
}

func TestTTF_zeroFloor(t *testing.T) {
	d := rupture.TTF(110.0, 100.0, 5.0)
	if d != 0 {
		t.Fatalf("want 0, got %v", d)
	}
}

func TestClassify_allTiers(t *testing.T) {
	cases := []struct {
		r    float64
		want string
	}{
		{0.5, "Stable"},
		{1.0, "Elevated"},
		{1.2, "Elevated"},
		{1.5, "Warning"},
		{2.9, "Warning"},
		{3.0, "Critical"},
		{4.9, "Critical"},
		{5.0, "Emergency"},
		{99.0, "Emergency"},
	}
	for _, tc := range cases {
		got := rupture.Classify(tc.r)
		if got != tc.want {
			t.Errorf("Classify(%f): want %q, got %q", tc.r, tc.want, got)
		}
	}
}
