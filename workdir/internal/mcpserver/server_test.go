package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fakeEngine(t *testing.T) *httptest.Server {
	t.Helper()
	routes := map[string]any{
		"/api/v2/failures":  sample,
		"/api/v2/callgraph": map[string]any{"edges": []Edge{{From: "shop/frontend", To: "shop/api", Source: "traces", CallsPerMinute: 84}}},
		"/api/v2/coverage":  Coverage{Watched: 10, Protected: 8, Uncovered: 2, Unprotected: []string{"shop/legacy"}},
		"/api/v2/health":    map[string]any{"status": "ready", "version": "8.7.11", "edition": "community"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func connect(t *testing.T, engine *Engine) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverSide, clientSide := mcp.NewInMemoryTransports()
	if _, err := New(engine, "test").Connect(ctx, serverSide, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil).Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, into any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if into != nil && !res.IsError {
		raw, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("%s structured content: %v", name, err)
		}
	}
	return res
}

func TestToolsAreReadOnlyAndCommunityOnly(t *testing.T) {
	session := connect(t, NewEngine(fakeEngine(t).URL, "test-key"))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s is not marked read-only", tool.Name)
		}
	}
	sort.Strings(names)
	want := []string{"coverage", "engine_status", "failures_for", "impact_of", "list_failures"}
	if len(names) != len(want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tools = %v, want %v", names, want)
		}
	}
}

func TestListFailuresTool(t *testing.T) {
	session := connect(t, NewEngine(fakeEngine(t).URL, "test-key"))
	var out ListOutput
	call(t, session, "list_failures", map[string]any{"namespace": "shop", "within_minutes": 15}, &out)
	if out.Matched != 3 || len(out.Failures) != 3 || out.Failures[0].FailureMode != "saturation" {
		t.Fatalf("list_failures = %+v", out)
	}
}

func TestImpactTool(t *testing.T) {
	session := connect(t, NewEngine(fakeEngine(t).URL, "test-key"))
	var out ImpactOutput
	call(t, session, "impact_of", map[string]any{"name": "api"}, &out)
	if len(out.Callers) != 1 || out.Callers[0].CallsPerMinute != 84 || len(out.Callees) != 0 || len(out.Failures) != 2 {
		t.Fatalf("impact_of = %+v", out)
	}
}

func TestCoverageAndStatusTools(t *testing.T) {
	session := connect(t, NewEngine(fakeEngine(t).URL, "test-key"))
	var coverage Coverage
	call(t, session, "coverage", nil, &coverage)
	if coverage.Protected != 8 || len(coverage.Unprotected) != 1 {
		t.Fatalf("coverage = %+v", coverage)
	}
	var status StatusOutput
	call(t, session, "engine_status", nil, &status)
	if status.Engine["version"] != "8.7.11" || status.Edition == "" {
		t.Fatalf("engine_status = %+v", status)
	}
}

func TestEngineErrorIsAToolError(t *testing.T) {
	session := connect(t, NewEngine(fakeEngine(t).URL, "wrong-key"))
	res := call(t, session, "list_failures", nil, nil)
	if !res.IsError {
		t.Fatal("an engine refusal was not reported as a tool error")
	}
}

func TestEngineTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("["))
		chunk := make([]byte, 1<<20)
		for i := range chunk {
			chunk[i] = ' '
		}
		for i := 0; i < 17; i++ {
			_, _ = w.Write(chunk)
		}
	}))
	defer srv.Close()
	_, err := NewEngine(srv.URL, "k").Failures(context.Background())
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("err = %v, want a size error", err)
	}
}
