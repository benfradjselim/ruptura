package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	// ceilingRefresh is how often the 7-day ceilings of workloads with no
	// memory limit are recomputed.
	ceilingRefresh = time.Hour
	// ceilingMargin is added to the 7-day 99th percentile.
	ceilingMargin = 1.2
	// ceilingMinHistory is the history a workload needs before its usual
	// level is trusted.
	ceilingMinHistory = 24 * time.Hour

	ceilingSourceDefault  = "namespace default"
	ceilingSourceInferred = "7-day P99 + 20%"

	// unlimitedMemory is the working set of pods that have no memory limit.
	unlimitedMemory = `sum by (namespace,pod) (container_memory_working_set_bytes{container!="",container!="POD"}) unless on (namespace,pod) kube_pod_container_resource_limits{resource="memory"}`
)

var (
	ceilingP99Query   = `quantile_over_time(0.99, (` + unlimitedMemory + `)[7d:5m])`
	ceilingStartQuery = `min_over_time(timestamp(` + unlimitedMemory + `)[7d:5m])`
)

// ceilings gives the usual memory level of workloads with no memory limit: a
// default per namespace from the operator, else the 7-day 99th percentile of
// the workload's pods plus 20 percent. It is safe for concurrent use.
type ceilings struct {
	defaults map[string]float64

	mu       sync.RWMutex
	inferred map[string]float64 // key: namespace/kind/name
}

// lookup returns the ceiling of a workload in namespace ns and its source, or
// ok=false when none is known.
func (c *ceilings) lookup(ns, workload string) (float64, string, bool) {
	if v, ok := c.defaults[ns]; ok {
		return v, ceilingSourceDefault, true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.inferred[workload]; ok {
		return v, ceilingSourceInferred, true
	}
	return 0, "", false
}

// collector runs one PromQL query and returns its readings.
type collector interface {
	Collect(ctx context.Context, sig sources.Signal) ([]sources.Reading, error)
}

// podResolver maps a pod name, live or gone, to its workload.
type podResolver interface {
	WorkloadForPodName(ns, podName string) (kind, name string, ok bool)
}

// refresh recomputes the inferred ceilings. On a query error the previous
// ceilings are kept, so a slow Prometheus does not silence forecasts.
func (c *ceilings) refresh(ctx context.Context, src collector, pods podResolver, now time.Time) error {
	p99, err := src.Collect(ctx, sources.Signal{Usage: ceilingP99Query})
	if err != nil {
		return fmt.Errorf("7-day percentile: %w", err)
	}
	starts, err := src.Collect(ctx, sources.Signal{Usage: ceilingStartQuery})
	if err != nil {
		return fmt.Errorf("history start: %w", err)
	}
	c.setInferred(inferCeilings(p99, starts, pods, now))
	return nil
}

func (c *ceilings) setInferred(inferred map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inferred = inferred
}

// inferCeilings takes, per workload, the highest 99th percentile of its pods
// plus the margin, for workloads whose oldest pod sample is at least
// ceilingMinHistory old.
func inferCeilings(p99, starts []sources.Reading, pods podResolver, now time.Time) map[string]float64 {
	workloadOf := func(r sources.Reading) (string, bool) {
		kind, name, ok := pods.WorkloadForPodName(r.Namespace, r.Pod)
		return r.Namespace + "/" + kind + "/" + name, ok
	}
	oldest := map[string]float64{}
	for _, r := range starts {
		if w, ok := workloadOf(r); ok && r.Usage > 0 {
			if v, seen := oldest[w]; !seen || r.Usage < v {
				oldest[w] = r.Usage
			}
		}
	}
	out := map[string]float64{}
	for _, r := range p99 {
		w, ok := workloadOf(r)
		start, known := oldest[w]
		if !ok || !known || r.Usage <= 0 || now.Sub(time.Unix(int64(start), 0)) < ceilingMinHistory {
			continue
		}
		if v := r.Usage * ceilingMargin; v > out[w] {
			out[w] = v
		}
	}
	return out
}

// startCeilings refreshes the inferred ceilings now and every ceilingRefresh
// until ctx ends. Inference is off when disabled is true.
func startCeilings(ctx context.Context, c *ceilings, src collector, pods podResolver, disabled bool) {
	if disabled {
		logger.Default.Info("failure modes: inferred memory ceilings disabled")
		return
	}
	go func() {
		ticker := time.NewTicker(ceilingRefresh)
		defer ticker.Stop()
		for {
			if err := c.refresh(ctx, src, pods, time.Now()); err != nil {
				logger.Default.Warn("failure modes: memory ceilings not refreshed", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// parseDefaultLimits reads "namespace=quantity,namespace=quantity", as the
// operator writes it from spec.analysis.defaultMemoryLimits.
func parseDefaultLimits(raw string) (map[string]float64, error) {
	out := map[string]float64{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		pair := strings.SplitN(entry, "=", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) == "" {
			return nil, fmt.Errorf("default memory limit %q: want namespace=quantity", entry)
		}
		v, err := parseQuantity(strings.TrimSpace(pair[1]))
		if err != nil {
			return nil, fmt.Errorf("default memory limit %q: %w", entry, err)
		}
		out[strings.TrimSpace(pair[0])] = v
	}
	return out, nil
}

// parseQuantity reads a Kubernetes memory quantity such as 512Mi, 1G or 1048576.
func parseQuantity(q string) (float64, error) {
	units := []struct {
		suffix string
		factor float64
	}{
		{"Ki", 1 << 10}, {"Mi", 1 << 20}, {"Gi", 1 << 30}, {"Ti", 1 << 40},
		{"k", 1e3}, {"K", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12},
	}
	factor := 1.0
	for _, u := range units {
		if strings.HasSuffix(q, u.suffix) {
			q, factor = strings.TrimSuffix(q, u.suffix), u.factor
			break
		}
	}
	v, err := strconv.ParseFloat(q, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid quantity")
	}
	return v * factor, nil
}
