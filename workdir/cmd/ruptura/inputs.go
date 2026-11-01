package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/normalize"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	inputPollInterval = time.Minute
	inputProfileEnv   = "RUPTURA_INPUT_PROFILE"
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

// inputProfile returns the configured profile, standard by default.
func inputProfile() string {
	if profile := os.Getenv(inputProfileEnv); profile != "" {
		return profile
	}
	return sources.ProfileStandard
}

// startInputs polls the catalog roles. It returns nil without a Thanos endpoint.
func startInputs(ctx context.Context, url string) *inputCollector {
	if url == "" {
		return nil
	}
	catalog, err := sources.LoadCatalog()
	if err != nil {
		logger.Default.Warn("inputs disabled: catalog", "err", err)
		return nil
	}
	source, err := sources.NewInClusterThanos(url)
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
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return collector
}
