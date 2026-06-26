// Package infra provides the dual-axis object identity, signal types, and
// collector interface for the v8.0 Infrastructure & Cluster Intelligence Layer.
//
// Every watched Kubernetes object is identified on two orthogonal axes:
//   - Axis A (topology): Scope + Namespace — "where it lives"
//   - Axis B (domain):   Group            — "what kind of problem it represents"
//
// Signals aggregate up from individual objects → group-in-namespace →
// namespace snapshot → cluster, and the Cross-Group Propagation Model (CGPM)
// computes how a problem in one Object-Group spreads to others.
package infra

import (
	"context"
	"strings"
	"time"
)

// Topological scope constants (Axis A).
const (
	ScopeCluster   = "cluster"
	ScopeNamespace = "namespace"
	ScopeWorkload  = "workload"
	ScopePod       = "pod"
)

// Object-Group constants (Axis B).
const (
	GroupWorkload     = "grp.workload"
	GroupNetwork      = "grp.network"
	GroupStorage      = "grp.storage"
	GroupControlPlane = "grp.controlplane"
	GroupAdmission    = "grp.admission"
	GroupOperators    = "grp.operators"
	GroupTenancy      = "grp.tenancy"
)

// Severity levels for InfraSignal.
const (
	SeverityStable    = "stable"
	SeverityElevated  = "elevated"
	SeverityWarning   = "warning"
	SeverityCritical  = "critical"
	SeverityEmergency = "emergency"
)

// ObjectID identifies a watched object on both grouping axes simultaneously.
// Axis A (topology) is encoded by Scope + Namespace; Axis B (domain) by Group.
// Cluster-scoped objects use Namespace="".
type ObjectID struct {
	// Group is the Axis B domain identifier (e.g. GroupNetwork, GroupStorage).
	Group string
	// Scope is the Axis A topological level: ScopeCluster | ScopeNamespace | ScopeWorkload | ScopePod.
	Scope string
	// Namespace is the owning namespace; empty for cluster-scoped objects.
	Namespace string
	// Kind is the Kubernetes object kind (e.g. "Route", "Node", "PersistentVolumeClaim").
	Kind string
	// Name is the object name.
	Name string
}

// Key returns the canonical pipe-delimited string key for this object.
// Examples:
//
//	"grp.network|namespace|openshift-console|Route|console"
//	"grp.controlplane|cluster||ClusterOperator|console"
func (o ObjectID) Key() string {
	return strings.Join([]string{o.Group, o.Scope, o.Namespace, o.Kind, o.Name}, "|")
}

// InfraSignal is one normalized [0,1] signal emitted by a single watched object.
// Value=0 means healthy; Value=1 means maximum severity. The ObjectID carries
// both axes so signals can be rolled up by namespace (Axis A) or by group (Axis B).
type InfraSignal struct {
	// Object is the dual-axis identity of the source object.
	Object ObjectID
	// Signal is the named sub-signal within the object's group (e.g. "nodeStress", "pvcStall").
	Signal string
	// Value is the normalized severity in [0,1].
	Value float64
	// Severity is the human-readable tier: stable|elevated|warning|critical|emergency.
	Severity string
	// Message is an optional human-readable description of the current condition.
	Message string
	// Timestamp is when this signal was last computed.
	Timestamp time.Time
}

// GroupSnapshot aggregates one Object-Group within one namespace (or cluster-wide
// when Namespace=""). It is the rung-2 aggregate in the signal ladder.
//
// Health = 1 - max(member signals)  — max preserves severity, avoids dilution.
// Spread = mean(member signals)     — context: localized vs widespread failure.
// GNI    = Group Noise Index        — noisy behavior as a standalone signal.
// Agitated signals a GNI spike while Health is still green: pre-rupture warning.
type GroupSnapshot struct {
	// Group is the Axis B domain identifier.
	Group string
	// Namespace is the aggregation scope; "" for cluster-scoped groups.
	Namespace string
	// Health is 1 - max(object signals) in [0,1]. 1.0 = all objects healthy.
	Health float64
	// Spread is mean(object signals) in [0,1]. Distinguishes localized from widespread faults.
	Spread float64
	// GNI is the Group Noise Index in [0,1]: 0.5*StateChurn + 0.5*EventBurst.
	GNI float64
	// Agitated is true when GNI is elevated while Health remains green (pre-rupture).
	Agitated bool
	// ObjectCount is the number of member objects contributing to this snapshot.
	ObjectCount int
	// Timestamp is when this snapshot was last computed.
	Timestamp time.Time
}

// NamespaceSnapshot is the primary integration point between the infra collector
// and the workload analyzer. It summarizes all group health signals for one
// namespace and carries the CGPM-computed PropPressure into the workload group.
//
// When the InfraRegistry is nil or has no data for a namespace, a healthy default
// is returned: NetworkHealth=1.0, all other fields zero.
type NamespaceSnapshot struct {
	// Namespace identifies the Kubernetes namespace.
	Namespace string
	// InfraStress = max over groups of (1 - GroupHealth). Feeds the extended HealthScore penalty.
	InfraStress float64
	// NetworkHealth = GroupHealth(grp.network). 1.0 when group is absent or healthy.
	NetworkHealth float64
	// StorageRisk = 1 - GroupHealth(grp.storage). 0.0 when group is absent or healthy.
	StorageRisk float64
	// AdmissionPressure = 1 - GroupHealth(grp.admission). 0.0 when group is absent or healthy.
	AdmissionPressure float64
	// PropPressure is the CGPM-computed propagated pressure into grp.workload for
	// this namespace. It folds into the extended contagion signal.
	PropPressure float64
	// Timestamp is when this snapshot was last computed.
	Timestamp time.Time
}

// HealthyNamespaceSnapshot returns the default NamespaceSnapshot used when the
// InfraRegistry is absent or has no data for the requested namespace.
// NetworkHealth defaults to 1.0 (fully healthy); all stress/risk fields are zero.
func HealthyNamespaceSnapshot(ns string) NamespaceSnapshot {
	return NamespaceSnapshot{
		Namespace:     ns,
		NetworkHealth: 1.0,
		Timestamp:     time.Now(),
	}
}

// InfraCollector is implemented by each domain-specific collector (node, networking,
// storage, admission, operator, tenancy). Each collector tags its signals with the
// correct ObjectID (resolved via ResolveGroup) and probes its API group once at
// startup to detect absent CRDs.
type InfraCollector interface {
	// Name returns the human-readable collector name (e.g. "node", "networking").
	Name() string
	// Probe performs a single LIST against the collector's API group.
	// Non-nil error means the API group is absent — the collector will be skipped.
	Probe(ctx context.Context) error
	// Start begins the watch loop. It blocks until ctx is cancelled.
	// Only called on collectors whose Probe succeeded.
	Start(ctx context.Context) error
	// Signals returns a concurrent-safe snapshot of all current object signals.
	Signals() []InfraSignal
}
