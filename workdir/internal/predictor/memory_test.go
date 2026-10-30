package predictor

import (
	"runtime"
	"testing"
)

func TestSeriesEnsemble_MemoryBudget(t *testing.T) {
	const series = 200
	const maxBytesPerSeries = 25 * 1024

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	kept := make([]*seriesEnsemble, series)
	for i := range kept {
		kept[i] = newSeriesEnsemble()
	}
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(kept)

	perSeries := (after.TotalAlloc - before.TotalAlloc) / series
	if perSeries > maxBytesPerSeries {
		t.Fatalf("allocated %d bytes per series, budget is %d", perSeries, maxBytesPerSeries)
	}
}
