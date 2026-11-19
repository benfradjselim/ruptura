package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/outage"
)

const (
	failureModePollInterval = 30 * time.Second
	failureModePodTTL       = 10 * time.Minute
	// failureModeSaveInterval is how often the tracker state is saved, so a
	// restart loses at most this much history.
	failureModeSaveInterval = time.Minute
)

// startFailureModes polls the configured Thanos endpoint and feeds the tracker.
// It returns nil when no endpoint is configured, and reports every failure.
func startFailureModes(ctx context.Context, url string, inf *discovery.Informer, store *storage.Store, limits *ceilings) *failuremodes.Tracker {
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
	source, err := sources.NewPrometheus(prometheusConfig(url))
	if err != nil {
		logger.Default.Warn("failure modes disabled: thanos client", "err", err)
		return nil
	}
	tracker := failuremodes.NewTracker()
	restoreFailureHistory(tracker, store)
	restoreTrackerState(tracker, store, time.Now())
	startCeilings(ctx, limits, source, inf, os.Getenv("RUPTURA_INFERRED_CEILINGS") == "false")
	go func() {
		ticker := time.NewTicker(failureModePollInterval)
		defer ticker.Stop()
		var saved time.Time
		for {
			pollFailureModes(ctx, source, catalog, inf, tracker, limits)
			if now := time.Now(); now.Sub(saved) >= failureModeSaveInterval {
				saveTrackerState(tracker, store, now)
				saved = now
			}
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

// newCeilings returns the ceilings with the namespace defaults in raw. An
// unreadable value is reported and ignored, so inference still applies.
func newCeilings(raw string) *ceilings {
	defaults, err := parseDefaultLimits(raw)
	if err != nil {
		logger.Default.Warn("failure modes: default memory limits ignored", "err", err)
		defaults = nil
	}
	return &ceilings{defaults: defaults}
}

// failureModesOutage reports Prometheus outages seen by the failure mode poll
// once each.
var failureModesOutage = outage.New("failure modes: Prometheus", warnf)

// warnf writes a formatted warning.
func warnf(format string, args ...interface{}) {
	logger.Default.Warn(fmt.Sprintf(format, args...))
}

func pollFailureModes(ctx context.Context, source *sources.Thanos, catalog sources.Catalog, inf *discovery.Informer, tracker *failuremodes.Tracker, limits *ceilings) {
	now := time.Now()
	failed := false
	for _, signal := range catalog.Signals {
		readings, err := source.Collect(ctx, signal)
		if err != nil {
			failureModesOutage.Failed("signal "+signal.Name, err)
			failed = true
			continue
		}
		for _, reading := range readings {
			kind, name, ok := inf.ResolvePodOwner(reading.Namespace, reading.Pod)
			if !ok {
				continue
			}
			workload := reading.Namespace + "/" + kind + "/" + name
			pod := reading.Namespace + "/" + reading.Pod
			tracker.ObserveRetiring(workload, pod, inf.PodRetiring(reading.Namespace, reading.Pod))
			if signal.Mode == "condition" {
				tracker.ObserveCondition(workload, pod, now, reading.Usage)
				continue
			}
			if version, ok := inf.PodReplicaSet(reading.Namespace, reading.Pod); ok {
				tracker.ObserveVersion(pod, version)
			}
			if reading.RestartsOnly {
				tracker.ObserveRestarts(workload, pod, now, reading.Restarts)
				continue
			}
			if reading.Capacity <= 0 && signal.Mode == "saturation" {
				if ceiling, from, ok := limits.lookup(reading.Namespace, workload); ok {
					tracker.ObserveUnlimited(workload, pod, now, reading.Usage, ceiling, from, reading.Restarts)
					continue
				}
			}
			tracker.Observe(workload, pod, now, reading.Usage, reading.Capacity, reading.Restarts)
		}
	}
	if !failed {
		failureModesOutage.Succeeded()
	}
	observeWarningEvents(ctx, inf, tracker)
	tracker.Prune(now.Add(-failureModePodTTL))
}

func observeWarningEvents(ctx context.Context, inf *discovery.Informer, tracker *failuremodes.Tracker) {
	events, err := inf.ListWarningEvents(ctx)
	if err != nil {
		logger.Default.Warn("failure modes: list events failed", "err", err)
		return
	}
	for _, event := range events {
		kind, name, ok := inf.ResolvePodOwner(event.Namespace, event.Pod)
		if !ok {
			continue
		}
		workload := event.Namespace + "/" + kind + "/" + name
		tracker.ObserveEvent(workload, event.Namespace+"/"+event.Pod, event.Reason, event.Message, event.At)
	}
}

// saveTrackerState writes the tracker's recent readings to storage. A nil
// tracker, as when failure modes are disabled, saves nothing.
func saveTrackerState(tracker *failuremodes.Tracker, store *storage.Store, now time.Time) {
	if tracker == nil {
		return
	}
	data, err := failuremodes.EncodeState(tracker.Snapshot(now))
	if err == nil {
		err = store.PutTrackerState(data)
	}
	if err != nil {
		logger.Default.Warn("failure modes: state not saved", "err", err)
	}
}

// restoreTrackerState reloads the readings saved before a restart, so forecasts
// resume without a new warm-up. A missing or unreadable state starts cold.
func restoreTrackerState(tracker *failuremodes.Tracker, store *storage.Store, now time.Time) {
	data, err := store.GetTrackerState()
	if err != nil {
		logger.Default.Warn("failure modes: state not restored", "err", err)
		return
	}
	if len(data) == 0 {
		return
	}
	state, err := failuremodes.DecodeState(data)
	if err != nil {
		logger.Default.Warn("failure modes: state not restored", "err", err)
		return
	}
	tracker.Restore(state, now)
	logger.Default.Info("failure modes: state restored", "pods", len(state.Pods), "saved_at", time.Unix(state.At, 0).UTC())
}

// restoreFailureHistory reloads saved failures and saves each new one, so a restart
// keeps the rhythm of a workload.
func restoreFailureHistory(tracker *failuremodes.Tracker, store *storage.Store) {
	saved, err := store.ListFailures()
	if err != nil {
		logger.Default.Warn("failure modes: history not restored", "err", err)
	}
	for workload, times := range saved {
		tracker.RestoreFailures(workload, times)
	}
	outcomes, err := store.ListOutcomes()
	if err != nil {
		logger.Default.Warn("failure modes: track record not restored", "err", err)
	}
	for _, outcome := range outcomes {
		tracker.RestoreOutcome(outcome.At, outcome.Lead)
	}
	tracker.SetOutcomeRecorder(func(at time.Time, lead time.Duration) {
		if err := store.PutOutcome(at, lead); err != nil {
			logger.Default.Warn("failure modes: track record not saved", "err", err)
		}
	})
	tracker.SetFailureRecorder(func(workload string, at time.Time) {
		if err := store.PutFailure(workload, at); err != nil {
			logger.Default.Warn("failure modes: history not saved", "err", err)
		}
	})
}
