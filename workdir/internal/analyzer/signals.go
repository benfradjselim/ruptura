package analyzer

import "sort"

// stateUnavailable is the KPI state of a signal whose inputs are absent.
const stateUnavailable = "unavailable"

// reading is a value that is either measured or absent. Zero is a measured value.
type reading struct {
	value     float64
	available bool
}

func measured(v float64) reading { return reading{value: v, available: true} }

// readMetric returns the first metric of names present in metrics.
func readMetric(metrics map[string]float64, names ...string) reading {
	for _, name := range names {
		if v, ok := metrics[name]; ok {
			return measured(v)
		}
	}
	return reading{}
}

// allAvailable reports whether every input was measured.
func allAvailable(inputs ...reading) bool {
	for _, in := range inputs {
		if !in.available {
			return false
		}
	}
	return true
}

type weightedInput struct {
	weight float64
	in     reading
}

// weightedMean averages the available inputs. Weights are rescaled over those
// inputs, so an absent input neither raises nor dilutes the result.
func weightedMean(inputs []weightedInput) reading {
	var sum, total float64
	for _, input := range inputs {
		if input.in.available && input.weight > 0 {
			sum += input.weight * input.in.value
			total += input.weight
		}
	}
	if total == 0 {
		return reading{}
	}
	return measured(sum / total)
}

// unavailableSignals lists the signals with absent inputs, sorted by name.
func unavailableSignals(signals map[string]reading) []string {
	var names []string
	for name, signal := range signals {
		if !signal.available {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
