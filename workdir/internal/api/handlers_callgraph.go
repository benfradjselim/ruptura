package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/benfradjselim/ruptura/internal/callgraph"
)

// maxImpactNames bounds the callers named in one evidence line or annotation.
const maxImpactNames = 3

// CallGraphSource returns the current call graph.
type CallGraphSource interface {
	Graph() callgraph.Graph
}

// SetCallGraph sets where failures read the callers they can hit.
func (h *Handlers) SetCallGraph(source CallGraphSource) { h.callGraph = source }

func shortName(id string) string { return id[strings.LastIndex(id, "/")+1:] }

// ImpactText names the callers a failure can hit, in plain words.
func ImpactText(callers []string) string {
	if len(callers) == 0 {
		return ""
	}
	names := make([]string, 0, maxImpactNames)
	for _, c := range callers {
		if len(names) == maxImpactNames {
			break
		}
		names = append(names, shortName(c))
	}
	text := "Can hit its callers: " + strings.Join(names, ", ")
	if extra := len(callers) - len(names); extra > 0 {
		text += fmt.Sprintf(" and %d more", extra)
	}
	return text
}

type callGraphNode struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Namespace   string `json:"namespace"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Host        string `json:"host,omitempty"`
	State       string `json:"state"`
	FailureMode string `json:"failureMode,omitempty"`
	FailureKind string `json:"failureKind,omitempty"`
	ETASeconds  int    `json:"etaSeconds,omitempty"`
	Affects     int    `json:"affects,omitempty"`
}

type callGraphResponse struct {
	Nodes []callGraphNode  `json:"nodes"`
	Edges []callgraph.Edge `json:"edges"`
}

// callGraphView joins the graph with the current failures, so each node says
// whether it fails and how many callers it can hit.
func callGraphView(graph callgraph.Graph, failures []*FailureResult) callGraphResponse {
	byWorkload := map[string]*FailureResult{}
	for _, f := range failures {
		if f.Workload != "" {
			if prev, ok := byWorkload[f.Workload]; !ok || (prev.State != "critical" && f.State == "critical") {
				byWorkload[f.Workload] = f
			}
		}
	}
	hosts := map[string]string{}
	for _, entry := range graph.Entries {
		hosts[entry.ID()] = entry.Host
	}
	nodes := map[string]callGraphNode{}
	addNode := func(id string) {
		if _, ok := nodes[id]; ok {
			return
		}
		parts := strings.SplitN(id, "/", 3)
		if len(parts) != 3 {
			return
		}
		node := callGraphNode{ID: id, Type: "workload", Namespace: parts[0], Kind: parts[1], Name: parts[2], State: "ok"}
		if host, ok := hosts[id]; ok {
			node.Type, node.Host = "entry", host
		}
		if f, ok := byWorkload[id]; ok {
			node.State, node.FailureMode, node.FailureKind, node.ETASeconds = f.State, f.FailureMode, f.Kind, f.ETASeconds
			node.Affects = len(graph.Callers(id))
		}
		nodes[id] = node
	}
	for _, e := range graph.Edges {
		addNode(e.From)
		addNode(e.To)
	}
	response := callGraphResponse{Nodes: make([]callGraphNode, 0, len(nodes)), Edges: graph.Edges}
	for _, node := range nodes {
		response.Nodes = append(response.Nodes, node)
	}
	sort.Slice(response.Nodes, func(i, j int) bool { return response.Nodes[i].ID < response.Nodes[j].ID })
	if response.Edges == nil {
		response.Edges = []callgraph.Edge{}
	}
	return response
}

func (h *Handlers) handleCallGraph(w http.ResponseWriter, _ *http.Request) {
	var graph callgraph.Graph
	if h.callGraph != nil {
		graph = h.callGraph.Graph()
	}
	writeJSON(w, http.StatusOK, callGraphView(graph, h.Failures()))
}
