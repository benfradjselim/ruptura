package infra

import "math"

// Cross-Group Propagation Model (CGPM) constants.
const (
	// kappaNoiseAmp is the noise amplification coefficient κ in the propagation formula.
	// A source group with GNI=1 amplifies its outgoing pressure by 1+κ = 1.5.
	kappaNoiseAmp = 0.5

	// thetaBlast is the minimum PropPressure for a downstream group to be counted
	// in the blast radius of a source group.
	thetaBlast = 0.2
)

// edge is a directed causal dependency between two Object-Groups with weight ω ∈ (0,1].
// The graph is acyclic by construction — all paths flow toward grp.workload.
type edge struct {
	from, to string
	omega    float64
}

// canonicalEdges is the fixed CGPM propagation graph (design doc §4.1).
// Each edge encodes a real causal dependency: a failure in the source group
// propagates into the target group with the given weight.
var canonicalEdges = []edge{
	{GroupControlPlane, GroupWorkload, 1.0}, // node/CO failure breaks hosted pods
	{GroupControlPlane, GroupNetwork, 0.9},  // network-operator CO failure breaks routing
	{GroupNetwork, GroupWorkload, 0.9},      // endpoint/route failure makes service unreachable
	{GroupStorage, GroupWorkload, 0.8},      // PVC stall blocks pod start
	{GroupAdmission, GroupWorkload, 0.7},    // admission denial blocks pod creation
	{GroupAdmission, GroupNetwork, 0.6},     // admission blocks route/service creation
	{GroupOperators, GroupStorage, 0.6},     // CSI operator failure breaks provisioning
	{GroupOperators, GroupNetwork, 0.6},     // network operator (AKO) failure breaks routes
	{GroupOperators, GroupWorkload, 0.5},    // operator-managed workload reconcile loop
}

// topoOrder is the fixed topological processing order for the propagation DAG.
// Each group appears strictly after all of its upstream dependencies, ensuring
// that PropPressure inputs for a target are fully resolved before it is processed.
//
//	Sources (in-degree 0): controlplane, operators, admission, tenancy
//	Intermediates:         storage, network
//	Sink:                  workload
var topoOrder = []string{
	GroupControlPlane,
	GroupOperators,
	GroupAdmission,
	GroupTenancy,
	GroupStorage,
	GroupNetwork,
	GroupWorkload,
}

// BlastInfo describes the downstream reach of one source group's activation.
type BlastInfo struct {
	// GroupsReached is the count of downstream groups with PropPressure >= thetaBlast.
	GroupsReached int
	// Downstream maps each reached downstream group to the pressure it received.
	Downstream map[string]float64
}

// ComputePropPressure runs the CGPM propagation over the DAG for one namespace context.
//
// activation maps each group to its current activation A(g) = 1 - GroupHealth ∈ [0,1].
// gni maps each group to its Group Noise Index ∈ [0,1].
//
// For each target group t (processed in topological order):
//
//	PropPressure(t) = clamp( max over upstream edges g→t of [
//	    effectiveA(g) · ω(g→t) · (1 + κ · GNI(g))
//	], 0, 1)
//
// effectiveA(g) = max(activation(g), PropPressure(g)) so that pressure propagates
// through intermediate hops (multi-hop attenuation by the product of edge weights).
//
// Groups absent from activation are treated as fully healthy (activation=0).
func ComputePropPressure(activation, gni map[string]float64) map[string]float64 {
	// effective holds max(own activation, received PropPressure) for each group.
	// Initialised from activation; updated as each group is processed.
	effective := make(map[string]float64, len(topoOrder))
	for g, a := range activation {
		effective[g] = a
	}

	result := make(map[string]float64, len(topoOrder))

	for _, tgt := range topoOrder {
		var maxP float64
		for _, e := range canonicalEdges {
			if e.to != tgt {
				continue
			}
			srcA := effective[e.from]
			srcGNI := gni[e.from]
			amp := 1.0 + kappaNoiseAmp*srcGNI
			p := srcA * e.omega * amp
			if p > maxP {
				maxP = p
			}
		}
		if maxP > 1.0 {
			maxP = 1.0
		}
		result[tgt] = maxP
		// Propagate into effective so downstream hops see the full received pressure.
		if maxP > effective[tgt] {
			effective[tgt] = maxP
		}
	}
	return result
}

// ComputeBlastRadius computes, for each source group, the set of downstream groups
// whose PropPressure (from that source alone) exceeds thetaBlast.
// It runs ComputePropPressure with each source in isolation.
// Only source groups with activation >= thetaBlast are evaluated.
func ComputeBlastRadius(activation, gni map[string]float64) map[string]BlastInfo {
	result := make(map[string]BlastInfo)
	for _, src := range topoOrder {
		a := activation[src]
		if a < thetaBlast {
			continue // source not activated enough to radiate
		}
		// Isolate: propagate only this source's activation.
		pp := ComputePropPressure(map[string]float64{src: a}, gni)
		info := BlastInfo{Downstream: make(map[string]float64)}
		for g, p := range pp {
			if g == src || p < thetaBlast {
				continue
			}
			info.GroupsReached++
			info.Downstream[g] = math.Round(p*1000) / 1000
		}
		result[src] = info
	}
	return result
}
