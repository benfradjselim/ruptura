package mcpserver

import "strings"

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Filter selects failures. Zero values select everything.
type Filter struct {
	Namespace     string
	Level         string
	Kind          string
	WithinMinutes int
	Limit         int
}

// Select returns the failures that match f, in engine order, and how many
// matched before the limit.
func Select(failures []Failure, f Filter) ([]Failure, int) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	out := []Failure{}
	matched := 0
	for _, failure := range failures {
		if !f.match(failure) {
			continue
		}
		matched++
		if len(out) < limit {
			out = append(out, failure)
		}
	}
	return out, matched
}

func (f Filter) match(failure Failure) bool {
	if f.Namespace != "" && !inNamespace(failure, f.Namespace) {
		return false
	}
	if f.Level != "" && !strings.EqualFold(failure.Level, f.Level) {
		return false
	}
	if f.Kind != "" && !strings.EqualFold(failure.Kind, f.Kind) {
		return false
	}
	// A detected failure is already happening, so it is within any horizon.
	if f.WithinMinutes > 0 && strings.EqualFold(failure.Kind, "predicted") && failure.ETASeconds > f.WithinMinutes*60 {
		return false
	}
	return true
}

func inNamespace(failure Failure, namespace string) bool {
	prefix := strings.ToLower(namespace) + "/"
	return strings.HasPrefix(strings.ToLower(failure.Target), prefix) || strings.HasPrefix(strings.ToLower(failure.Workload), prefix)
}

// About returns the failures whose target, workload or pod contains name,
// ignoring case.
func About(failures []Failure, name string) []Failure {
	needle := strings.ToLower(strings.TrimSpace(name))
	out := []Failure{}
	if needle == "" {
		return out
	}
	for _, failure := range failures {
		for _, field := range []string{failure.Target, failure.Workload, shortName(failure.Workload), failure.Pod} {
			if field != "" && strings.Contains(strings.ToLower(field), needle) {
				out = append(out, failure)
				break
			}
		}
	}
	return out
}

// On returns the failures whose workload or target is name, matched like the
// nodes of the call graph (namespace/Kind/name), so impact_of reports the same
// object on both sides.
func On(failures []Failure, name string) []Failure {
	out := []Failure{}
	for _, failure := range failures {
		if sameNode(failure.Workload, name) || sameNode(failure.Target, name) {
			out = append(out, failure)
		}
	}
	return out
}

// Neighbours splits the edges that touch name into its callers and callees.
// A node matches when it equals name or ends with "/"+name, ignoring case.
func Neighbours(edges []Edge, name string) (callers, callees []Edge) {
	callers, callees = []Edge{}, []Edge{}
	for _, edge := range edges {
		if sameNode(edge.To, name) {
			callers = append(callers, edge)
		}
		if sameNode(edge.From, name) {
			callees = append(callees, edge)
		}
	}
	return callers, callees
}

func sameNode(node, name string) bool {
	node, name = strings.ToLower(node), strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	return node == name || strings.HasSuffix(node, "/"+name) || shortName(node) == name
}

// shortName turns namespace/Kind/name, as Ruptura writes workloads, into
// namespace/name, as people usually write them.
func shortName(workload string) string {
	parts := strings.Split(workload, "/")
	if len(parts) == 3 {
		return parts[0] + "/" + parts[2]
	}
	return workload
}
