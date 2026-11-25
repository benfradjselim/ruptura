package main

import (
	"context"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
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
		var err error
		switch card.Kind {
		case sources.CardPredicted:
			err = pollPredictedCard(ctx, source, card, objects, now)
		case sources.CardDetected:
			err = pollDetectedCard(ctx, source, card, objects, now)
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
		objects.ObserveTrend(card.ID, card.Level, target, failuremodes.Mode(card.FailureMode), card.Horizon, card.Unit, now, series.Value, limit)
	}
	return nil
}

func pollDetectedCard(ctx context.Context, source vectorSource, card sources.Card, objects *failuremodes.Objects, now time.Time) error {
	values, err := source.QueryVector(ctx, card.Value)
	if err != nil {
		return err
	}
	for _, series := range values {
		target, ok := cardTarget(card, series.Labels)
		mode, known := cardMode(card, series.Labels)
		if !ok || !known {
			continue
		}
		objects.ObserveDetected(card.ID, card.Level, target, mode, card.RaiseAfter, card.ClearAfter, now, series.Value)
	}
	return nil
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
		info[card.ID] = api.CardInfo{Action: card.Action, Severity: card.Severity}
	}
	return info
}
