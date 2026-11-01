package predictor

import (
	"math"
	"testing"
)

func TestARIMA_ForecastFollowsTrendBeyondTwentySteps(t *testing.T) {
	cases := []struct {
		name  string
		noise func(int) float64
	}{
		{"clean slope", func(int) float64 { return 0 }},
		{"noisy slope", func(i int) float64 { return 0.002 * math.Sin(float64(i)*1.7) }},
	}
	for _, c := range cases {
		a := newARIMA()
		y := 0.0
		for i := 0; i < 120; i++ {
			y = 0.1 + 0.01*float64(i) + c.noise(i)
			a.Update(y)
		}
		want := y + 0.01*60
		if got := a.Forecast(60); math.Abs(got-want) > 0.1 {
			t.Errorf("%s: forecast = %v, want near %v", c.name, got, want)
		}
	}
}

func TestARIMA_FlatSeriesStaysFlat(t *testing.T) {
	a := newARIMA()
	for i := 0; i < 120; i++ {
		a.Update(0.5)
	}
	if got := a.Forecast(60); math.Abs(got-0.5) > 1e-6 {
		t.Errorf("forecast = %v, want 0.5", got)
	}
}
