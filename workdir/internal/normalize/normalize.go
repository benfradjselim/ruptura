// Package normalize turns collected roles into per target frames. A role with no
// value is listed as missing, so absent data is never read as zero.
package normalize

import "sort"

// Frame holds the measured roles of one target.
type Frame struct {
	Target  string
	Values  map[string]float64
	Missing []string
}

// Coverage reports how many targets a role reached.
type Coverage struct {
	Role      string `json:"role"`
	Targets   int    `json:"targets"`
	Available bool   `json:"available"`
}

// Build returns one frame per target found in collected, sorted by target.
func Build(roles []string, collected map[string]map[string]float64) []Frame {
	values := make(map[string]map[string]float64)
	for role, byTarget := range collected {
		for target, value := range byTarget {
			if values[target] == nil {
				values[target] = make(map[string]float64)
			}
			values[target][role] = value
		}
	}
	sorted := append([]string(nil), roles...)
	sort.Strings(sorted)
	frames := make([]Frame, 0, len(values))
	for target, measured := range values {
		frame := Frame{Target: target, Values: measured}
		for _, role := range sorted {
			if _, ok := measured[role]; !ok {
				frame.Missing = append(frame.Missing, role)
			}
		}
		frames = append(frames, frame)
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].Target < frames[j].Target })
	return frames
}

// CoverageOf reports each role in order, including roles that returned nothing.
func CoverageOf(roles []string, collected map[string]map[string]float64) []Coverage {
	out := make([]Coverage, 0, len(roles))
	for _, role := range roles {
		targets := len(collected[role])
		out = append(out, Coverage{Role: role, Targets: targets, Available: targets > 0})
	}
	return out
}

// Protection says how many targets have a limit to predict against.
type Protection struct {
	Watched     int      `json:"watched"`
	Protected   int      `json:"protected"`
	Unprotected []string `json:"unprotected"`
}

// maxListedTargets keeps the report readable on large clusters.
const maxListedTargets = 20

// ProtectionOf reports targets that have a usage reading but no limit.
func ProtectionOf(frames []Frame, usageRole, limitRole string) Protection {
	report := Protection{Unprotected: []string{}}
	for _, frame := range frames {
		if _, ok := frame.Values[usageRole]; !ok {
			continue
		}
		report.Watched++
		if _, ok := frame.Values[limitRole]; ok {
			report.Protected++
		} else if len(report.Unprotected) < maxListedTargets {
			report.Unprotected = append(report.Unprotected, frame.Target)
		}
	}
	return report
}
