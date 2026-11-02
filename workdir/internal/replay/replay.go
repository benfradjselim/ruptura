// Package replay scores failure detectors against recorded cluster telemetry.
// It runs without a cluster so a detector change can be measured in seconds.
package replay

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"
)

// Sample is one pod observation from a recording.
type Sample struct {
	At           time.Time
	Workload     string
	Pod          string
	MemoryBytes  float64
	CPUNanoCores float64
	LimitBytes   float64
	Restarts     int
	LastReason   string
}

// Detector reports whether a pod is warned about at a given sample.
type Detector interface {
	Observe(s Sample) bool
}

// Result is the score of one detector on one recording.
type Result struct {
	Failures    int
	Eligible    int
	Predicted   int
	LeadMinutes []float64
	FalseAlarms int
}

// Options controls scoring.
type Options struct {
	Target    string
	Controls  []string
	MinLead   time.Duration
	Warmup    time.Duration
	Eligibles time.Duration
}

// Load reads a recording and returns its samples ordered by time.
func Load(path string) ([]Sample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recording: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	var samples []Sample
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		sample, err := parseRow(row)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].At.Before(samples[j].At) })
	return samples, nil
}

func parseRow(row []string) (Sample, error) {
	if len(row) != 8 {
		return Sample{}, fmt.Errorf("expected 8 columns, got %d", len(row))
	}
	unix, err := strconv.ParseInt(row[0], 10, 64)
	if err != nil {
		return Sample{}, fmt.Errorf("parse time: %w", err)
	}
	values := make([]float64, 3)
	for i, column := range []int{3, 4, 5} {
		if values[i], err = strconv.ParseFloat(row[column], 64); err != nil {
			return Sample{}, fmt.Errorf("parse column %d: %w", column, err)
		}
	}
	restarts, err := strconv.Atoi(row[6])
	if err != nil {
		return Sample{}, fmt.Errorf("parse restarts: %w", err)
	}
	return Sample{
		At:           time.Unix(unix, 0).UTC(),
		Workload:     row[1],
		Pod:          row[2],
		MemoryBytes:  values[0],
		CPUNanoCores: values[1],
		LimitBytes:   values[2],
		Restarts:     restarts,
		LastReason:   row[7],
	}, nil
}

// Score runs a detector over samples. A failure is eligible when the pod was
// observed for at least Options.Eligibles before it, so a recording that starts
// mid-cycle cannot count against or for a detector.
func Score(samples []Sample, detector Detector, opts Options) Result {
	var result Result
	var cycleStart, firstWarning time.Time
	previous := -1

	for _, sample := range samples {
		warned := detector.Observe(sample)
		if isControl(sample.Workload, opts.Controls) && warned {
			result.FalseAlarms++
		}
		if sample.Workload != opts.Target {
			continue
		}
		if cycleStart.IsZero() {
			cycleStart = sample.At
		}
		if previous >= 0 && sample.Restarts > previous {
			result.Failures++
			if sample.At.Sub(cycleStart) >= opts.Eligibles {
				result.Eligible++
				if !firstWarning.IsZero() {
					lead := sample.At.Sub(firstWarning)
					result.LeadMinutes = append(result.LeadMinutes, lead.Minutes())
					if lead >= opts.MinLead {
						result.Predicted++
					}
				}
			}
			cycleStart = sample.At
			firstWarning = time.Time{}
		} else if warned && firstWarning.IsZero() && sample.At.Sub(cycleStart) >= opts.Warmup {
			firstWarning = sample.At
		}
		previous = sample.Restarts
	}
	return result
}

func isControl(workload string, controls []string) bool {
	for _, control := range controls {
		if workload == control {
			return true
		}
	}
	return false
}
