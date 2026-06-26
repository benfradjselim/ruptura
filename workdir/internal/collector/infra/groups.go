package infra

// kindRegistry is the static mapping from a Kubernetes Kind to its Object-Group (Axis B)
// and topological scope (Axis A). This is the single authoritative registry — adding
// a new watched kind means one line here; nothing else in the propagation math changes.
//
// OpenShift Project is treated as Namespace (same Axis A node, extra annotations as labels).
// Never add a "project" scope; key everything by namespace name.
var kindRegistry = map[string]struct{ Group, Scope string }{
	// Workload group — the existing v7 grain.
	"Deployment":  {GroupWorkload, ScopeWorkload},
	"StatefulSet": {GroupWorkload, ScopeWorkload},
	"DaemonSet":   {GroupWorkload, ScopeWorkload},
	"Pod":         {GroupWorkload, ScopePod},

	// Networking group — namespace-scoped connectivity objects.
	"Route":         {GroupNetwork, ScopeNamespace}, // OpenShift Route (probed; skipped on vanilla k8s)
	"Service":       {GroupNetwork, ScopeNamespace},
	"Endpoints":     {GroupNetwork, ScopeNamespace},
	"NetworkPolicy": {GroupNetwork, ScopeNamespace},
	"Ingress":       {GroupNetwork, ScopeNamespace},

	// Storage group — mixed namespace/cluster scope.
	"PersistentVolumeClaim": {GroupStorage, ScopeNamespace},
	"PersistentVolume":      {GroupStorage, ScopeCluster},
	"StorageClass":          {GroupStorage, ScopeCluster},

	// Control-Plane group — cluster-scoped infrastructure objects.
	"Node":               {GroupControlPlane, ScopeCluster},
	"ClusterOperator":    {GroupControlPlane, ScopeCluster}, // OpenShift CO (probed)
	"MachineConfigPool":  {GroupControlPlane, ScopeCluster}, // OpenShift MCP (probed)

	// Admission group — policy enforcement objects.
	"PolicyReport":                    {GroupAdmission, ScopeNamespace}, // Kyverno / wgpolicyk8s.io (probed)
	"ValidatingWebhookConfiguration":  {GroupAdmission, ScopeCluster},

	// Operators group — OLM lifecycle objects.
	"Subscription":            {GroupOperators, ScopeNamespace}, // OLM (probed)
	"ClusterServiceVersion":   {GroupOperators, ScopeNamespace}, // OLM (probed)
	"CustomResourceDefinition": {GroupOperators, ScopeCluster},
	"InstallPlan":             {GroupOperators, ScopeNamespace}, // OLM (probed)

	// Tenancy group — resource boundary objects.
	"ResourceQuota": {GroupTenancy, ScopeNamespace},
	"LimitRange":    {GroupTenancy, ScopeNamespace},
	"Namespace":     {GroupTenancy, ScopeNamespace},
}

// ResolveGroup returns the Object-Group (Axis B) and topological scope (Axis A)
// for the given Kubernetes Kind. ok is false when the Kind is not in the registry.
// Callers should tag InfraSignals using the returned group and scope.
func ResolveGroup(kind string) (group, scope string, ok bool) {
	entry, ok := kindRegistry[kind]
	return entry.Group, entry.Scope, ok
}

// AllGroups returns the set of distinct Object-Group IDs defined in the registry.
// Useful for initializing per-group data structures without hard-coding the list.
func AllGroups() []string {
	seen := make(map[string]struct{})
	for _, entry := range kindRegistry {
		seen[entry.Group] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	return out
}

// KindsForGroup returns all Kubernetes Kinds that belong to the given Object-Group.
// Returns nil when the group is unrecognized.
func KindsForGroup(group string) []string {
	var out []string
	for kind, entry := range kindRegistry {
		if entry.Group == group {
			out = append(out, kind)
		}
	}
	return out
}
