package main

import (
	"context"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/sources"
)

// catalogCard returns a card of the embedded catalog by id.
func catalogCard(t *testing.T, id string) sources.Card {
	t.Helper()
	catalog, err := sources.LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	for _, card := range catalog.Cards {
		if card.ID == id {
			return card
		}
	}
	t.Fatalf("card %q not in the catalog", id)
	return sources.Card{}
}

// runCard polls one card for steps readings 30 s apart against a fake
// Prometheus that answers the card's own queries, and returns the findings.
func runCard(card sources.Card, steps int, usage func(i int) float64, capacity, value float64) []failuremodes.ObjectFinding {
	labels := map[string]string{}
	for _, name := range card.Target {
		labels[name] = "t-" + name
	}
	objects := failuremodes.NewObjects()
	start := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	var now time.Time
	for i := 0; i < steps; i++ {
		now = start.Add(time.Duration(i) * 30 * time.Second)
		source := fakeVectors{card.Sampled: {{Labels: labels, Value: float64(now.Unix())}}}
		if card.Kind == sources.CardPredicted {
			source[card.Usage] = []sources.Series{{Labels: labels, Value: usage(i)}}
			source[card.Capacity] = []sources.Series{{Labels: labels, Value: capacity}}
		} else {
			source[card.Value] = []sources.Series{{Labels: labels, Value: value}}
		}
		pollCards(context.Background(), source, []sources.Card{card}, objects, now)
	}
	return objects.Findings(now)
}

// TestCatalogCardsFire runs the coverage 67 cards, batch 1 (node), batch 2
// (cluster) and batch 3 (pod, workload, storage, network), on a failing and a
// healthy object.
func TestCatalogCardsFire(t *testing.T) {
	predicted := []string{"inode-exhaustion", "too-many-pods", "conntrack-table-full", "node-capacity-cpu", "node-capacity-memory",
		"etcd-database-quota", "persistent-volume-filling", "ephemeral-storage-eviction"}
	for _, id := range predicted {
		t.Run(id, func(t *testing.T) {
			card := catalogCard(t, id)
			if card.Kind != sources.CardPredicted {
				t.Fatalf("kind = %q", card.Kind)
			}
			// 50% to about 90% of capacity in 30 minutes: full well within the horizon.
			filling := runCard(card, 61, func(i int) float64 { return 500 + 6.5*float64(i) }, 1000, 0)
			if len(filling) != 1 || !filling[0].Predicted || filling[0].Estimate.Mode != failuremodes.Mode(card.FailureMode) {
				t.Fatalf("filling findings = %+v", filling)
			}
			if flat := runCard(card, 61, func(int) float64 { return 500 }, 1000, 0); len(flat) != 0 {
				t.Fatalf("flat findings = %+v", flat)
			}
		})
	}
	detected := []string{"clock-not-synchronised", "node-readiness-flapping", "tcp-retransmits",
		"webhook-failing", "apiserver-errors-latency", "etcd-slow-disk", "etcd-member-down", "cluster-operator-degraded",
		"cluster-operator-down", "operator-install-failed", "monitoring-target-down", "dns-resolution-failures",
		"liveness-probe-restarts", "readiness-probe-failing", "config-error-at-start", "init-container-failing",
		"service-without-endpoints", "route-without-backend", "pvc-pending", "persistent-volume-errors",
		"eviction-node-memory-pressure", "pod-stuck-terminating", "cronjob-not-running"}
	for _, id := range detected {
		t.Run(id, func(t *testing.T) {
			card := catalogCard(t, id)
			if card.Kind != sources.CardDetected || card.Evidence == "" {
				t.Fatalf("kind = %q evidence = %q", card.Kind, card.Evidence)
			}
			steps := int((card.RaiseAfter+2*time.Minute)/(30*time.Second)) + 1
			if early := runCard(card, int(card.RaiseAfter/(30*time.Second))-1, nil, 0, 1); len(early) != 0 {
				t.Fatalf("raised before %s: %+v", card.RaiseAfter, early)
			}
			holding := runCard(card, steps, nil, 0, 1)
			if len(holding) != 1 || holding[0].Predicted || holding[0].Estimate.Mode != failuremodes.Mode(card.FailureMode) {
				t.Fatalf("holding findings = %+v", holding)
			}
			if healthy := runCard(card, steps, nil, 0, 0); len(healthy) != 0 {
				t.Fatalf("healthy findings = %+v", healthy)
			}
		})
	}
}
