package main

import (
	"context"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/normalize"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	inputPollInterval = time.Minute
	inputProfileEnv   = "RUPTURA_INPUT_PROFILE"

	replicasUnavailableRole = "replicas.unavailable"
	replicasDesiredRole     = "replicas.desired"
	replicasAvailableRole   = "replicas.available"
	rolloutStalledRole      = "rollout.stalled"
	containerOOMRole        = "container.oom"
)

var inputLevels = []string{"pod", "workload"}

// inputCollector keeps the latest frames and coverage of the catalog roles.
type inputCollector struct {
	mu       sync.RWMutex
	coverage []normalize.Coverage
	frames   map[string][]normalize.Frame
}

// Coverage returns the latest coverage of every role.
func (c *inputCollector) Coverage() []normalize.Coverage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]normalize.Coverage{}, c.coverage...)
}

func (c *inputCollector) collect(ctx context.Context, source *sources.Thanos, catalog sources.Catalog, profile string) {
	coverage := []normalize.Coverage{}
	frames := make(map[string][]normalize.Frame, len(inputLevels))
	for _, level := range inputLevels {
		roles := catalog.RolesFor(level, profile)
		names := make([]string, 0, len(roles))
		collected := make(map[string]map[string]float64, len(roles))
		for _, role := range roles {
			names = append(names, role.Name)
			values, err := source.CollectRole(ctx, role)
			if err != nil {
				logger.Default.Warn("inputs: collect failed", "role", role.Name, "err", err)
				continue
			}
			collected[role.Name] = values
		}
		frames[level] = normalize.Build(names, collected)
		coverage = append(coverage, normalize.CoverageOf(names, collected)...)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.coverage = coverage
	c.frames = frames
}

// replicaRoles maps each role to the workload kind it describes.
var replicaRoles = map[string]string{
	replicasUnavailableRole:            "Deployment",
	"replicas.unavailable.statefulset": "StatefulSet",
	"replicas.unavailable.daemonset":   "DaemonSet",
}

type replicaGap struct {
	workload string
	missing  int
}

// replicaGaps reads the unavailable replicas of each frame. A frame without a role
// is skipped, because an absent reading says nothing about the replicas.
func replicaGaps(frames []normalize.Frame) []replicaGap {
	var gaps []replicaGap
	for _, frame := range frames {
		parts := strings.SplitN(frame.Target, "/", 2)
		if len(parts) != 2 {
			continue
		}
		for role, kind := range replicaRoles {
			missing, ok := frame.Values[role]
			if role == replicasUnavailableRole {
				missing, ok = deploymentMissing(frame.Values)
			}
			if ok {
				gaps = append(gaps, replicaGap{parts[0] + "/" + kind + "/" + parts[1], int(missing)})
			}
		}
	}
	return gaps
}

// deploymentMissing counts desired replicas that are not available. Kubernetes
// counts unavailable replicas over all pods, so the surge pod of a stuck rollout
// reads as missing while every desired replica serves.
// Without both counts it falls back to the unavailable reading.
func deploymentMissing(values map[string]float64) (float64, bool) {
	desired, okDesired := values[replicasDesiredRole]
	available, okAvailable := values[replicasAvailableRole]
	if !okDesired || !okAvailable {
		unavailable, ok := values[replicasUnavailableRole]
		return unavailable, ok
	}
	return math.Max(0, desired-available), true
}

// observeReplicas feeds workload frames to the tracker.
func (c *inputCollector) observeReplicas(tracker *failuremodes.Tracker, now time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, gap := range replicaGaps(c.frames["workload"]) {
		tracker.ObserveReplicas(gap.workload, gap.missing, now)
	}
}

// observeWorkloads feeds rollout state and OOM kills to the tracker.
func (c *inputCollector) observeWorkloads(tracker *failuremodes.Tracker, inf *discovery.Informer, now time.Time) {
	c.observeReplicas(tracker, now)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, frame := range c.frames["workload"] {
		stalled, ok := frame.Values[rolloutStalledRole]
		parts := strings.SplitN(frame.Target, "/", 2)
		if ok && len(parts) == 2 {
			tracker.ObserveRollout(parts[0]+"/Deployment/"+parts[1], stalled > 0, now)
		}
	}
	for _, frame := range c.frames["pod"] {
		oom, ok := frame.Values[containerOOMRole]
		parts := strings.SplitN(frame.Target, "/", 2)
		if !ok || len(parts) != 2 {
			continue
		}
		if kind, name, known := inf.ResolvePodOwner(parts[0], parts[1]); known {
			tracker.ObserveOOM(parts[0]+"/"+kind+"/"+name, frame.Target, oom > 0, now)
		}
	}
}

// Protection reports pods with and without a memory limit.
func (c *inputCollector) Protection() normalize.Protection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return normalize.ProtectionOf(c.frames["pod"], "memory.usage", "memory.limit")
}

// inputProfile returns the configured profile, standard by default.
func inputProfile() string {
	if profile := os.Getenv(inputProfileEnv); profile != "" {
		return profile
	}
	return sources.ProfileStandard
}

// startInputs polls the catalog roles. It returns nil without a Thanos endpoint.
func startInputs(ctx context.Context, url string, tracker *failuremodes.Tracker, inf *discovery.Informer) *inputCollector {
	if url == "" {
		return nil
	}
	catalog, err := sources.LoadCatalog()
	if err != nil {
		logger.Default.Warn("inputs disabled: catalog", "err", err)
		return nil
	}
	source, err := sources.NewPrometheus(prometheusConfig(url))
	if err != nil {
		logger.Default.Warn("inputs disabled: thanos client", "err", err)
		return nil
	}
	collector := &inputCollector{}
	profile := inputProfile()
	go func() {
		ticker := time.NewTicker(inputPollInterval)
		defer ticker.Stop()
		for {
			collector.collect(ctx, source, catalog, profile)
			if tracker != nil {
				collector.observeWorkloads(tracker, inf, time.Now())
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return collector
}
