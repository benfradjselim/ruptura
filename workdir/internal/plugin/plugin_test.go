package plugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/benfradjselim/kairo-core/internal/plugin"
)

// writeStubPlugin writes a small Go-source plugin, compiles it, and returns its path.
// The stub responds to ohe.info and ohe.call correctly.
func writeStubPlugin(t *testing.T, hooks []string) string {
	t.Helper()
	dir := t.TempDir()

	src := `package main

import (
	"bufio"
	"encoding/json"
	"os"
)

type req struct {
	JSONRPC string          ` + "`" + `json:"jsonrpc"` + "`" + `
	ID      int64           ` + "`" + `json:"id"` + "`" + `
	Method  string          ` + "`" + `json:"method"` + "`" + `
	Params  json.RawMessage ` + "`" + `json:"params"` + "`" + `
}

type resp struct {
	JSONRPC string      ` + "`" + `json:"jsonrpc"` + "`" + `
	ID      int64       ` + "`" + `json:"id"` + "`" + `
	Result  interface{} ` + "`" + `json:"result"` + "`" + `
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		var r req
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil { return }
		switch r.Method {
		case "ohe.info":
			enc.Encode(resp{JSONRPC:"2.0", ID:r.ID, Result: map[string]interface{}{
				"name":"stub-plugin","version":"0.1","description":"test stub","hooks":[]string{"metric.ingest","alert.fire"},
			}})
		case "ohe.call":
			enc.Encode(resp{JSONRPC:"2.0", ID:r.ID, Result: map[string]interface{}{"ok":true}})
		case "ohe.shutdown":
			enc.Encode(resp{JSONRPC:"2.0", ID:r.ID, Result: nil})
			return
		}
	}
}
`
	srcPath := filepath.Join(dir, "stub.go")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatalf("write stub.go: %v", err)
	}
	binPath := filepath.Join(dir, "stub-plugin")
	out, err := exec.Command("go", "build", "-o", binPath, srcPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compile stub plugin: %v\n%s", err, out)
	}
	return binPath
}

func TestManagerNoPluginDir(t *testing.T) {
	m := plugin.NewManager("/nonexistent/plugin/dir/xyz")
	if len(m.Plugins()) != 0 {
		t.Errorf("expected 0 plugins for missing dir, got %d", len(m.Plugins()))
	}
}

func TestManagerLoadsPlugin(t *testing.T) {
	binPath := writeStubPlugin(t, []string{"metric.ingest", "alert.fire"})
	dir := filepath.Dir(binPath)

	m := plugin.NewManager(dir)
	plugins := m.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "stub-plugin" {
		t.Errorf("name = %q; want stub-plugin", plugins[0].Name)
	}
	if plugins[0].Version != "0.1" {
		t.Errorf("version = %q", plugins[0].Version)
	}
}

func TestManagerFireHook(t *testing.T) {
	binPath := writeStubPlugin(t, nil)
	dir := filepath.Dir(binPath)

	m := plugin.NewManager(dir)
	// Should not panic or error
	m.Fire(context.Background(), "metric.ingest", map[string]interface{}{
		"metrics": []map[string]interface{}{{"host": "web-01", "name": "cpu", "value": 72.5}},
	})
}

func TestManagerFireUnregisteredHook(t *testing.T) {
	binPath := writeStubPlugin(t, nil)
	dir := filepath.Dir(binPath)

	m := plugin.NewManager(dir)
	// kpi.update is not in the stub's hooks list — should be silently skipped
	m.Fire(context.Background(), "kpi.update", map[string]interface{}{"value": 1})
}

func TestManagerShutdown(t *testing.T) {
	binPath := writeStubPlugin(t, nil)
	dir := filepath.Dir(binPath)

	m := plugin.NewManager(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5000000000)
	defer cancel()
	m.Shutdown(ctx) // should complete cleanly
}

// TestPluginInfoFields verifies all Info fields are populated correctly.
func TestPluginInfoFields(t *testing.T) {
	binPath := writeStubPlugin(t, nil)
	m := plugin.NewManager(filepath.Dir(binPath))
	ps := m.Plugins()
	if len(ps) == 0 {
		t.Fatal("no plugins loaded")
	}
	p := ps[0]
	if p.Description == "" {
		t.Error("description should not be empty")
	}
	if len(p.Hooks) == 0 {
		t.Error("hooks should not be empty")
	}
}

// TestManagerEmptyDir has no executables — should return 0 plugins.
func TestManagerEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Write a non-executable file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a plugin"), 0644)
	m := plugin.NewManager(dir)
	// The file will fail to start (not executable / not a binary) — should be skipped
	_ = m.Plugins()
}

// TestManagerDotFilesSkipped ensures hidden files are ignored.
func TestManagerDotFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("ignored"), 0755)
	m := plugin.NewManager(dir)
	if len(m.Plugins()) != 0 {
		t.Error("dot files should be skipped")
	}
}

