package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/alertmanager"
	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	alertInterval   = time.Minute
	alertTargetsEnv = "RUPTURA_ALERTMANAGER_URLS"
)

// startAlerts reads failures every minute, which also notes forecasts for the track
// record, and repeats them to each configured Alertmanager.
func startAlerts(ctx context.Context, handlers *api.Handlers) {
	var clients []*alertmanager.Client
	for _, url := range strings.Split(os.Getenv(alertTargetsEnv), ",") {
		if url = strings.TrimSpace(url); url != "" {
			clients = append(clients, alertmanager.NewClient(url, nil))
		}
	}
	go func() {
		ticker := time.NewTicker(alertInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendAlerts(ctx, clients, handlers.Failures())
			}
		}
	}()
}

func sendAlerts(ctx context.Context, clients []*alertmanager.Client, failures []*api.FailureResult) {
	alerts := make([]alertmanager.Alert, 0, len(failures))
	for _, failure := range failures {
		alerts = append(alerts, alertmanager.Alert{
			Name:     failure.FailureMode,
			Severity: failure.State,
			Level:    failure.Level,
			Target:   failure.Target,
			Kind:     failure.Kind,
			Summary:  failure.RecommendedAction,
		})
	}
	for _, client := range clients {
		if err := client.Send(ctx, alerts); err != nil {
			logger.Default.Warn("alert delivery failed", "err", err)
		}
	}
}
