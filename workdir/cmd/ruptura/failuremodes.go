package main

import (
	"context"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	failureModePollInterval = 30 * time.Second
	failureModePodTTL       = 10 * time.Minute
)

// startFailureModes polls the configured Thanos endpoint and feeds the tracker.
// It returns nil when no endpoint is configured, and reports every failure.
func startFailureModes(ctx context.Context, url string, inf *discovery.Informer) *failuremodes.Tracker {
	if url == "" {
		return nil
	}
	if inf == nil {
		logger.Default.Warn("failure modes disabled: workload discovery is unavailable")
		return nil
	}
	catalog, err := sources.LoadCatalog()
	if err != nil {
		logger.Default.Warn("failure modes disabled: catalog", "err", err)
		return nil
	}
	source, err := sources.NewInClusterThanos(url)
	if err != nil {
		logger.Default.Warn("failure modes disabled: thanos client", "err", err)
		return nil
	}
	tracker := failuremodes.NewTracker()
	go func() {
		ticker := time.NewTicker(failureModePollInterval)
		defer ticker.Stop()
		for {
			pollFailureModes(ctx, source, catalog, inf, tracker)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	logger.Default.Info("failure modes active", "signals", len(catalog.Signals))
	return tracker
}

func pollFailureModes(ctx context.Context, source *sources.Thanos, catalog sources.Catalog, inf *discovery.Informer, tracker *failuremodes.Tracker) {
	now := time.Now()
	for _, signal := range catalog.Signals {
		readings, err := source.Collect(ctx, signal)
		if err != nil {
			logger.Default.Warn("failure modes: collect failed", "signal", signal.Name, "err", err)
			continue
		}
		for _, reading := range readings {
			kind, name, ok := inf.ResolvePodOwner(reading.Namespace, reading.Pod)
			if !ok {
				continue
			}
			workload := reading.Namespace + "/" + kind + "/" + name
			pod := reading.Namespace + "/" + reading.Pod
			if signal.Mode == "condition" {
				tracker.ObserveCondition(workload, pod, now, reading.Usage)
				continue
			}
			if reading.RestartsOnly {
				tracker.ObserveRestarts(workload, pod, now, reading.Restarts)
				continue
			}
			tracker.Observe(workload, pod, now, reading.Usage, reading.Capacity, reading.Restarts)
		}
	}
	tracker.Prune(now.Add(-failureModePodTTL))
}
