// Package plugin implements OHE's subprocess-based plugin system.
//
// # Architecture
//
// Plugins are standalone executables placed in the plugins/ directory (or
// OHE_PLUGIN_DIR). They communicate with OHE over stdin/stdout using a
// newline-delimited JSON-RPC 2.0 protocol:
//
//	→  {"jsonrpc":"2.0","id":1,"method":"ohe.info","params":{}}
//	←  {"jsonrpc":"2.0","id":1,"result":{"name":"my-plugin","version":"1.0"}}
//
// OHE calls three lifecycle methods:
//
//	ohe.info    → {name, version, description, hooks:[]}
//	ohe.call    → (hook string, payload map) → result map
//	ohe.shutdown → {}
//
// # Hooks
//
// Plugins declare which hooks they handle in ohe.info. Currently supported:
//
//	metric.ingest   — called after each ingest batch; payload: {metrics:[...]}
//	alert.fire      — called when an alert fires; payload: {alert:{...}}
//	alert.resolve   — called when an alert resolves; payload: {alert:{...}}
//	kpi.update      — called when a KPI is computed; payload: {host, name, value}
package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	pluginTimeout    = 5 * time.Second
	maxResponseBytes = 1 << 20 // 1 MiB per response
)

// Info describes a loaded plugin.
type Info struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Hooks       []string `json:"hooks"`
}

// Plugin wraps a running plugin subprocess.
type Plugin struct {
	Info   Info
	cmd    *exec.Cmd
	enc    *json.Encoder
	dec    *json.Decoder
	mu     sync.Mutex
	nextID int64
	log    *logger.Logger
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Manager loads and manages all plugins in a directory.
type Manager struct {
	plugins []*Plugin
	log     *logger.Logger
}

// NewManager creates a Manager that loads plugins from dir.
// Plugins that fail to start or fail ohe.info are skipped with a warning.
func NewManager(dir string) *Manager {
	m := &Manager{log: logger.New("plugin")}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			m.log.Warn("plugin dir not readable", "dir", dir, "err", err)
		}
		return m
	}

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		p, err := launch(path)
		if err != nil {
			m.log.Warn("plugin launch failed", "path", path, "err", err)
			continue
		}
		m.plugins = append(m.plugins, p)
		m.log.Info("plugin loaded", "name", p.Info.Name, "version", p.Info.Version, "hooks", strings.Join(p.Info.Hooks, ","))
	}
	return m
}

// Fire calls all plugins that registered hook, passing payload.
// Errors from individual plugins are logged but do not stop other plugins.
func (m *Manager) Fire(ctx context.Context, hook string, payload interface{}) {
	for _, p := range m.plugins {
		if !p.hasHook(hook) {
			continue
		}
		if err := p.call(ctx, hook, payload); err != nil {
			m.log.Warn("plugin hook error", "plugin", p.Info.Name, "hook", hook, "err", err)
		}
	}
}

// Shutdown sends ohe.shutdown to every plugin and waits for processes to exit.
func (m *Manager) Shutdown(ctx context.Context) {
	for _, p := range m.plugins {
		_ = p.call(ctx, "ohe.shutdown", nil)
		_ = p.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = p.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
			_ = p.cmd.Process.Kill()
		}
	}
}

// Plugins returns the loaded plugins (read-only).
func (m *Manager) Plugins() []Info {
	out := make([]Info, len(m.plugins))
	for i, p := range m.plugins {
		out[i] = p.Info
	}
	return out
}

// launch starts the plugin executable, calls ohe.info, and returns a Plugin.
func launch(path string) (*Plugin, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", path, err)
	}

	p := &Plugin{
		cmd: cmd,
		enc: json.NewEncoder(stdin),
		dec: json.NewDecoder(bufio.NewReaderSize(stdout, maxResponseBytes)),
		log: logger.New("plugin"),
	}

	// Handshake: call ohe.info within 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), pluginTimeout)
	defer cancel()

	var info Info
	if err := p.invoke(ctx, "ohe.info", nil, &info); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("ohe.info: %w", err)
	}
	p.Info = info
	return p, nil
}

func (p *Plugin) hasHook(hook string) bool {
	for _, h := range p.Info.Hooks {
		if h == hook {
			return true
		}
	}
	return false
}

// call fires a hook method (used by Manager.Fire) — result is discarded.
func (p *Plugin) call(ctx context.Context, method string, params interface{}) error {
	return p.invoke(ctx, "ohe.call", map[string]interface{}{"hook": method, "payload": params}, nil)
}

// invoke sends a JSON-RPC request and decodes the result into out (may be nil).
func (p *Plugin) invoke(ctx context.Context, method string, params interface{}, out interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	id := p.nextID
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}

	done := make(chan error, 1)
	go func() {
		if err := p.enc.Encode(req); err != nil {
			done <- fmt.Errorf("encode: %w", err)
			return
		}
		var resp rpcResponse
		if err := p.dec.Decode(&resp); err != nil {
			done <- fmt.Errorf("decode: %w", err)
			return
		}
		if resp.Error != nil {
			done <- fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			return
		}
		if out != nil && resp.Result != nil {
			done <- json.Unmarshal(resp.Result, out)
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
