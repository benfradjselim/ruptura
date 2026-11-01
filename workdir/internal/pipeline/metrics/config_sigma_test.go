package metrics

import "testing"

func TestSensitivitySigma(t *testing.T) {
	cases := []struct {
		name string
		want float64
	}{
		{"conservative", 4},
		{"balanced", 3},
		{"aggressive", 2.5},
		{"", 3},
		{"unknown", 3},
	}
	for _, c := range cases {
		if got := SensitivitySigma(c.name); got != c.want {
			t.Errorf("%q: sigma = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEngineConfig_AnomalySigmaChangesDetection(t *testing.T) {
	feed := func(sigma float64) int {
		e := newAnomalyEngineWithSigma(sigma)
		return observeShift(e)
	}
	if strict, loose := feed(6), feed(2.5); strict >= loose {
		t.Errorf("alerts strict = %d, loose = %d, want fewer when strict", strict, loose)
	}
}
