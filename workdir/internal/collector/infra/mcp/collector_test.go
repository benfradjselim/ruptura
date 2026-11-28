package mcp

import (
	"encoding/json"
	"testing"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

func pool(t *testing.T, raw string) *mcpItem {
	t.Helper()
	var item mcpItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatal(err)
	}
	return &item
}

func TestPools(t *testing.T) {
	c := &Collector{signals: map[string]infra.InfraSignal{}, pools: map[string]Pool{}}
	c.upsert(pool(t, `{"metadata":{"name":"master"},"status":{"machineCount":3,"conditions":[{"type":"Degraded","status":"False"},{"type":"Updated","status":"True"}]}}`))
	c.upsert(pool(t, `{"metadata":{"name":"worker"},"status":{"machineCount":3,"conditions":[{"type":"Degraded","status":"True"}]}}`))
	c.upsert(pool(t, `{"metadata":{"name":"infra"},"status":{"machineCount":2,"degradedMachineCount":1}}`))
	got := map[string]bool{}
	for _, p := range c.Pools() {
		got[p.Name] = p.Degraded
	}
	want := map[string]bool{"master": false, "worker": true, "infra": true}
	if len(got) != len(want) {
		t.Fatalf("pools = %v", got)
	}
	for name, degraded := range want {
		if got[name] != degraded {
			t.Fatalf("pool %s degraded = %v, want %v", name, got[name], degraded)
		}
	}
	c.remove("worker")
	if len(c.Pools()) != 2 {
		t.Fatalf("after delete pools = %v", c.Pools())
	}
}
