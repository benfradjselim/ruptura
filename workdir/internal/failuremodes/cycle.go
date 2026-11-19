package failuremodes

import "time"

const (
	// cycleBucket is the resolution of the usage history the cycle guard reads.
	cycleBucket = 5 * time.Minute
	// cycleHistory is how far back the guard looks for earlier cycles.
	cycleHistory = 6 * time.Hour
	// cycleMinFalls is how many times usage must have come back down on its
	// own before a rise counts as one more cycle.
	cycleMinFalls = 2
	// cycleMinFall is the drop, as a share of the limit, that counts as a fall.
	cycleMinFall = 0.1
	// cycleMargin, as a share of the limit, is how far above the earlier peaks
	// usage may go and still count as the same cycle, so that noise at the top
	// of a cycle is not a new high.
	cycleMargin = 0.05
)

// bucket is the lowest and highest usage seen during one cycleBucket.
type bucket struct {
	at        time.Time
	low, high float64
}

// record adds a reading to the history and drops buckets older than cycleHistory.
func record(history []bucket, at time.Time, usage float64) []bucket {
	start := at.Truncate(cycleBucket)
	if n := len(history); n > 0 && history[n-1].at.Equal(start) {
		last := &history[n-1]
		if usage < last.low {
			last.low = usage
		}
		if usage > last.high {
			last.high = usage
		}
	} else {
		history = append(history, bucket{at: start, low: usage, high: usage})
	}
	cut := 0
	for cut < len(history) && at.Sub(history[cut].at) > cycleHistory {
		cut++
	}
	return history[cut:]
}

// withinCycle reports whether a rise is one more turn of a load that repeats:
// before the forecast window, usage came back down by itself at least
// cycleMinFalls times, and the current usage is still below the earlier peaks
// plus cycleMargin.
// The history resets at every restart, so a leak that ends in an OOM kill never
// counts as a cycle, and a rise past the earlier peaks is forecast as usual.
func withinCycle(history []bucket, windowStart time.Time, usage float64) bool {
	falls, peak, envelope := 0, -1.0, -1.0
	for _, b := range history {
		if !b.at.Add(cycleBucket).Before(windowStart) {
			break
		}
		if peak >= 0 && b.low <= peak-cycleMinFall {
			falls++
			peak = b.high
		} else if b.high > peak {
			peak = b.high
		}
		if b.high > envelope {
			envelope = b.high
		}
	}
	return falls >= cycleMinFalls && usage <= envelope+cycleMargin
}
