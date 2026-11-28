package main

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/internal/callgraph"
	"github.com/benfradjselim/ruptura/internal/collector/infra/mcp"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/logburst"
	"github.com/benfradjselim/ruptura/internal/sources"
	"github.com/benfradjselim/ruptura/pkg/outage"
)

// cardsOutage reports Prometheus outages seen by the card poll once each.
var cardsOutage = outage.New("failure cards: Prometheus", warnf)

// vectorSource runs instant queries; *sources.Thanos implements it.
type vectorSource interface {
	QueryVector(ctx context.Context, q string) ([]sources.Series, error)
}

// pollCards reads every catalog card once and feeds the object tracker. A card
// whose query fails keeps its objects until they go stale.
func pollCards(ctx context.Context, source vectorSource, cards []sources.Card, objects *failuremodes.Objects, now time.Time) {
	failed := false
	for _, card := range cards {
		if card.Source != "" {
			continue
		}
		var err error
		switch card.Kind {
		case sources.CardPredicted:
			err = pollPredictedCard(ctx, source, card, objects, now)
		case sources.CardDetected:
			err = pollDetectedCard(ctx, source, card, objects, now)
		case sources.CardDeadline:
			err = pollDeadlineCard(ctx, source, card, objects, now)
		}
		if err != nil {
			cardsOutage.Failed("card "+card.ID, err)
			failed = true
			continue
		}
		objects.Polled(card.ID, now)
	}
	if !failed {
		cardsOutage.Succeeded()
	}
	objects.Prune(now.Add(-failureModePodTTL))
}

func pollPredictedCard(ctx context.Context, source vectorSource, card sources.Card, objects *failuremodes.Objects, now time.Time) error {
	usage, err := source.QueryVector(ctx, card.Usage)
	if err != nil {
		return err
	}
	capacity, err := source.QueryVector(ctx, card.Capacity)
	if err != nil {
		return err
	}
	sampled, err := sampleTimes(ctx, source, card)
	if err != nil {
		return err
	}
	capacities := make(map[string]float64, len(capacity))
	for _, series := range capacity {
		if target, ok := cardTarget(card, series.Labels); ok {
			capacities[target] = series.Value
		}
	}
	for _, series := range usage {
		target, ok := cardTarget(card, series.Labels)
		limit, known := capacities[target]
		if !ok || !known || limit <= 0 {
			continue
		}
		at, dated := sampleOf(card, sampled, target)
		if !dated {
			objects.Seen(card.ID, target, now)
			continue
		}
		objects.ObserveTrend(card.ID, card.Level, target, failuremodes.Mode(card.FailureMode), card.Horizon, card.Unit, now, at, series.Value, limit)
	}
	return nil
}

func pollDetectedCard(ctx context.Context, source vectorSource, card sources.Card, objects *failuremodes.Objects, now time.Time) error {
	values, err := source.QueryVector(ctx, card.Value)
	if err != nil {
		return err
	}
	sampled, err := sampleTimes(ctx, source, card)
	if err != nil {
		return err
	}
	for _, series := range values {
		target, ok := cardTarget(card, series.Labels)
		mode, known := cardMode(card, series.Labels)
		if !ok || !known {
			continue
		}
		at, dated := sampleOf(card, sampled, target)
		if !dated {
			objects.Seen(card.ID, target, now)
			continue
		}
		objects.ObserveDetected(card.ID, card.Level, target, mode, card.RaiseAfter, card.ClearAfter, now, at, series.Value)
	}
	return nil
}

func pollDeadlineCard(ctx context.Context, source vectorSource, card sources.Card, objects *failuremodes.Objects, now time.Time) error {
	values, err := source.QueryVector(ctx, card.Value)
	if err != nil {
		return err
	}
	sampled, err := sampleTimes(ctx, source, card)
	if err != nil {
		return err
	}
	for _, series := range values {
		target, ok := cardTarget(card, series.Labels)
		if !ok || math.IsNaN(series.Value) {
			continue
		}
		at, dated := sampleOf(card, sampled, target)
		if !dated {
			objects.Seen(card.ID, target, now)
			continue
		}
		objects.ObserveDeadline(card.ID, card.Level, target, failuremodes.Mode(card.FailureMode), card.Horizon, now, at, series.Value)
	}
	return nil
}

// objectReading is one object's value read outside Prometheus: 1 while the
// failure holds and 0 otherwise, or a raw value the card compares to Above.
type objectReading struct {
	Target string
	Value  float64
}

// objectSource returns the current readings of a Kubernetes API reader.
type objectSource func() []objectReading

// pollSourceCards feeds the detected cards whose value comes from a
// Kubernetes API reader instead of Prometheus. A card whose reader is absent,
// such as machineconfigpools outside OpenShift, reports nothing.
func pollSourceCards(cards []sources.Card, readers map[string]objectSource, objects *failuremodes.Objects, now time.Time) {
	for _, card := range cards {
		read, ok := readers[card.Source]
		if card.Source == "" || !ok {
			continue
		}
		for _, r := range read() {
			value := r.Value
			if card.Above > 0 {
				value = 0
				if r.Value > card.Above {
					value = 1
				}
			}
			objects.ObserveDetected(card.ID, card.Level, r.Target, failuremodes.Mode(card.FailureMode), card.RaiseAfter, card.ClearAfter, now, time.Time{}, value)
		}
		objects.Polled(card.ID, now)
	}
}

// sampleOf returns the sample time of target, zero for a card without a
// Sampled query, and false when the card has one but it missed this target: that
// reading is not dated by the poll, which would be later than its sample.
func sampleOf(card sources.Card, sampled map[string]time.Time, target string) (time.Time, bool) {
	if card.Sampled == "" {
		return time.Time{}, true
	}
	at, ok := sampled[target]
	return at, ok
}

// sampleTimes reads the card's Sampled query: the time of the source sample
// behind each target. A card without one is dated by the poll.
func sampleTimes(ctx context.Context, source vectorSource, card sources.Card) (map[string]time.Time, error) {
	if card.Sampled == "" {
		return nil, nil
	}
	series, err := source.QueryVector(ctx, card.Sampled)
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(series))
	for _, s := range series {
		if target, ok := cardTarget(card, s.Labels); ok && s.Value > 0 {
			out[target] = time.Unix(0, int64(s.Value*float64(time.Second))).UTC()
		}
	}
	return out, nil
}

// cardTarget joins the card's target labels with "/". A series missing one of
// them cannot be named and is skipped.
func cardTarget(card sources.Card, labels map[string]string) (string, bool) {
	parts := make([]string, 0, len(card.Target))
	for _, name := range card.Target {
		value := labels[name]
		if value == "" {
			return "", false
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "/"), true
}

// cardMode names the failure of one series: the card's mode, or the mode that
// its ModeLabel value maps to.
func cardMode(card sources.Card, labels map[string]string) (failuremodes.Mode, bool) {
	if card.ModeLabel == "" {
		return failuremodes.Mode(card.FailureMode), true
	}
	mode, ok := card.Modes[labels[card.ModeLabel]]
	return failuremodes.Mode(mode), ok
}

// cardInfo returns the action and severity of every catalog card, for the API.
func cardInfo() map[string]api.CardInfo {
	catalog, err := sources.LoadCatalog()
	if err != nil {
		return nil
	}
	info := make(map[string]api.CardInfo, len(catalog.Cards))
	for _, card := range catalog.Cards {
		info[card.ID] = api.CardInfo{Action: card.Action, Severity: card.Severity, Evidence: card.Evidence}
	}
	return info
}

// poolReadings reads the MachineConfigPools of an OpenShift cluster: 1 for a
// degraded pool.
func poolReadings(c *mcp.Collector) objectSource {
	return func() []objectReading {
		pools := c.Pools()
		out := make([]objectReading, 0, len(pools))
		for _, p := range pools {
			value := 0.0
			if p.Degraded {
				value = 1
			}
			out = append(out, objectReading{Target: p.Name, Value: value})
		}
		return out
	}
}

// minEdgeCallsPerMinute is the traffic below which an edge's error ratio is
// too noisy to read: one failed call in two would read 50%.
const minEdgeCallsPerMinute = 1

// edgeReadings reads the measured edges of the call graph: the share of
// failed calls from one workload to another.
func edgeReadings(graph func() callgraph.Graph) objectSource {
	return func() []objectReading {
		var out []objectReading
		for _, e := range graph().Edges {
			if e.ErrorRatio == nil || e.CallsPerMinute < minEdgeCallsPerMinute {
				continue
			}
			out = append(out, objectReading{Target: e.From + " -> " + e.To, Value: *e.ErrorRatio})
		}
		return out
	}
}

// logReadings reads the error log rate of each workload as a multiple of its
// usual rate.
func logReadings(c *logburst.Counter) objectSource {
	return func() []objectReading {
		bursts := c.Readings(time.Now())
		out := make([]objectReading, 0, len(bursts))
		for _, b := range bursts {
			out = append(out, objectReading{Target: b.Workload, Value: b.Ratio})
		}
		return out
	}
}
