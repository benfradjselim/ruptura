package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Instructions tells the assistant what this server offers.
const Instructions = `Ruptura names Kubernetes and OpenShift failures before and as they happen. This server is read-only: it reports what the Ruptura engine sees and never changes the cluster.

A failure is "predicted" (expected, with the time left in etaSeconds and an early-late range) or "detected" (happening now). Each one names its target, its failure mode, the evidence and a recommended action. Levels are pod, workload, namespace and node.

Start with list_failures; use within_minutes to ask what fails soon. Use failures_for to explain one workload, pod or node, and impact_of to see which services call it.

This is the community edition. Acting on failures (restart, scale, roll back), compound failures that combine several signals, and failures spreading across levels are part of Ruptura Autopilot.`

// Tool inputs and outputs. Field descriptions become the JSON schema the
// assistant reads.

// ListInput selects failures.
type ListInput struct {
	Namespace     string `json:"namespace,omitempty" jsonschema:"only failures in this namespace"`
	Level         string `json:"level,omitempty" jsonschema:"only this level: pod, workload, namespace or node"`
	Kind          string `json:"kind,omitempty" jsonschema:"only predicted (expected) or detected (happening now) failures"`
	WithinMinutes int    `json:"within_minutes,omitempty" jsonschema:"only predicted failures expected within this many minutes, plus every detected one"`
	Limit         int    `json:"limit,omitempty" jsonschema:"most failures to return, 50 by default, at most 200"`
}

// ListOutput is a page of failures.
type ListOutput struct {
	Matched  int       `json:"matched" jsonschema:"failures that match, before the limit"`
	Failures []Failure `json:"failures"`
}

// TargetInput names a workload, pod or node.
type TargetInput struct {
	Name string `json:"name" jsonschema:"workload, pod or node name, or namespace/name; matched as a substring, ignoring case"`
}

// TargetOutput lists the failures about one target.
type TargetOutput struct {
	Failures []Failure `json:"failures"`
}

// ImpactOutput lists who calls a service and whom it calls.
type ImpactOutput struct {
	Callers  []Edge    `json:"callers" jsonschema:"services that call it and would feel its failure"`
	Callees  []Edge    `json:"callees" jsonschema:"services it calls"`
	Failures []Failure `json:"failures" jsonschema:"failures Ruptura names on it now"`
}

// StatusOutput describes the engine and this server.
type StatusOutput struct {
	Edition string         `json:"edition"`
	Engine  map[string]any `json:"engine"`
}

// Empty is the input of tools that take none.
type Empty struct{}

// New returns an MCP server backed by engine.
func New(engine *Engine, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "ruptura", Title: "Ruptura", Version: version},
		&mcp.ServerOptions{Instructions: Instructions})
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_failures", Title: "List failures", Annotations: readOnly,
		Description: "Failures Ruptura names now, most urgent first, with time left, evidence and recommended action. Filter by namespace, level, kind or time left."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
			failures, err := engine.Failures(ctx)
			if err != nil {
				return nil, ListOutput{}, err
			}
			page, matched := Select(failures, Filter{Namespace: in.Namespace, Level: in.Level, Kind: in.Kind, WithinMinutes: in.WithinMinutes, Limit: in.Limit})
			return nil, ListOutput{Matched: matched, Failures: page}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "failures_for", Title: "Failures of one target", Annotations: readOnly,
		Description: "Every failure about one workload, pod or node, with its evidence and recommended action."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TargetInput) (*mcp.CallToolResult, TargetOutput, error) {
			failures, err := engine.Failures(ctx)
			if err != nil {
				return nil, TargetOutput{}, err
			}
			return nil, TargetOutput{Failures: About(failures, in.Name)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "impact_of", Title: "Impact of a service", Annotations: readOnly,
		Description: "Which services call a workload and which it calls, from the cluster configuration and from traces when available, with the failures named on it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TargetInput) (*mcp.CallToolResult, ImpactOutput, error) {
			edges, err := engine.Edges(ctx)
			if err != nil {
				return nil, ImpactOutput{}, err
			}
			failures, err := engine.Failures(ctx)
			if err != nil {
				return nil, ImpactOutput{}, err
			}
			callers, callees := Neighbours(edges, in.Name)
			return nil, ImpactOutput{Callers: callers, Callees: callees, Failures: About(failures, in.Name)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "coverage", Title: "Coverage", Annotations: readOnly,
		Description: "How many watched pods Ruptura can protect, and which ones it cannot (for example, no memory limit and no usual level yet)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, Coverage, error) {
			coverage, err := engine.Coverage(ctx)
			return nil, coverage, err
		})

	mcp.AddTool(server, &mcp.Tool{Name: "engine_status", Title: "Engine status", Annotations: readOnly,
		Description: "Version, edition and readiness of the Ruptura engine behind this server."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, StatusOutput, error) {
			health, err := engine.Health(ctx)
			if err != nil {
				return nil, StatusOutput{}, err
			}
			return nil, StatusOutput{Edition: "community, read-only", Engine: health}, nil
		})

	return server
}
