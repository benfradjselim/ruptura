package failuremodes

import (
	"sort"
	"time"
)

// pairSlopes returns the slope of every pair of points, sorted, in usage per
// second. Theil-Sen takes their median.
func pairSlopes(points []point) []float64 {
	var slopes []float64
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			if dt := points[j].at.Sub(points[i].at).Seconds(); dt > 0 {
				slopes = append(slopes, (points[j].usage-points[i].usage)/dt)
			}
		}
	}
	sort.Float64s(slopes)
	return slopes
}

// slopeBand returns the first and third quartiles of sorted slopes, the spread
// of growth rates the readings agree on.
func slopeBand(sorted []float64) (low, high float64) {
	if len(sorted) == 0 {
		return 0, 0
	}
	return sorted[len(sorted)/4], sorted[(3*len(sorted))/4]
}

// failureBand converts the room left before the limit and a slope band into the
// earliest and latest failure. A latest failure is unknown, zero, when the low
// slope does not grow.
func failureBand(remaining, low, high float64) (early, late time.Duration) {
	if remaining < 0 {
		remaining = 0
	}
	if high > 0 {
		early = time.Duration(remaining / high * float64(time.Second))
	}
	if low > 0 {
		late = time.Duration(remaining / low * float64(time.Second))
	}
	return early, late
}
