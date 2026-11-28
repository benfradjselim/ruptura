// Package logburst counts error log records per workload and reports a burst
// against the workload's own usual error rate.
package logburst

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// window is the history kept per workload, one bucket a minute.
	window = 60
	// recent is how many of the last complete minutes form the current rate.
	recent = 5
	// minHistory is how many minutes a workload needs before it is read.
	minHistory = 30
	// minPerMinute is the current error rate below which no burst is read:
	// a quiet service going from 0 to 3 errors a minute is noise, not a burst.
	minPerMinute = 10
	// errorSeverity is the first OTLP severity number of the ERROR range.
	errorSeverity = 17
)

// IsError reports whether a record is at ERROR level or above: by its OTLP
// severity number when set, by its severity text otherwise.
func IsError(number int, text string) bool {
	if number > 0 {
		return number >= errorSeverity
	}
	switch strings.ToUpper(text) {
	case "ERROR", "FATAL", "CRITICAL", "ALERT", "EMERGENCY", "PANIC":
		return true
	}
	return false
}

type series struct {
	first   time.Time
	buckets map[int64]int // minute number -> errors
}

// Counter counts error records per workload. It is safe for concurrent use.
type Counter struct {
	mu        sync.Mutex
	workloads map[string]*series
}

// New returns an empty counter.
func New() *Counter { return &Counter{workloads: map[string]*series{}} }

// Add counts one error record of workload at time at.
func (c *Counter) Add(workload string, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.workloads[workload]
	if !ok {
		s = &series{first: at, buckets: map[int64]int{}}
		c.workloads[workload] = s
	}
	s.buckets[minute(at)]++
}

func minute(at time.Time) int64 { return at.Unix() / 60 }

// Reading is the error rate of the last minutes as a multiple of the usual
// rate: the median errors a minute over the rest of the window, at least 1.
// Ratio is 0 while the current rate is under the burst floor.
type Reading struct {
	Workload string
	Ratio    float64
}

// Readings returns one reading per workload with enough history at now, and
// forgets workloads with no error in the window.
func (c *Counter) Readings(now time.Time) []Reading {
	c.mu.Lock()
	defer c.mu.Unlock()
	last := minute(now) - 1 // the last complete minute
	var out []Reading
	for name, s := range c.workloads {
		for m := range s.buckets {
			if m <= last-window {
				delete(s.buckets, m)
			}
		}
		if len(s.buckets) == 0 {
			delete(c.workloads, name)
			continue
		}
		if minute(now)-minute(s.first) < minHistory {
			continue
		}
		current := 0
		for m := last - recent + 1; m <= last; m++ {
			current += s.buckets[m]
		}
		rate := float64(current) / recent
		ratio := 0.0
		if rate >= minPerMinute {
			ratio = rate / usual(s, last)
		}
		out = append(out, Reading{Workload: name, Ratio: ratio})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Workload < out[j].Workload })
	return out
}

// usual is the median errors a minute before the recent minutes, at least 1.
func usual(s *series, last int64) float64 {
	var counts []int
	for m := last - window + 1; m <= last-recent; m++ {
		if m >= minute(s.first) {
			counts = append(counts, s.buckets[m])
		}
	}
	sort.Ints(counts)
	median := float64(counts[len(counts)/2])
	if median < 1 {
		return 1
	}
	return median
}

// Total returns the error records of workload kept in the window.
func (c *Counter) Total(workload string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	if s, ok := c.workloads[workload]; ok {
		for _, n := range s.buckets {
			total += n
		}
	}
	return total
}
