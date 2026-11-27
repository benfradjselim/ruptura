package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/alertmanager"
	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/internal/notify"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	alertInterval        = time.Minute
	alertTargetsEnv      = "RUPTURA_ALERTMANAGER_URLS"
	alertTokenTargetsEnv = "RUPTURA_ALERTMANAGER_TOKEN_URLS"
	alertCAFileEnv       = "RUPTURA_ALERTMANAGER_CA_FILE"
)

var serviceAccountToken = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func splitURLs(value string) []string {
	var urls []string
	for _, url := range strings.Split(value, ",") {
		if url = strings.TrimSpace(url); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

// alertClients builds one client per target. Only targets listed in
// RUPTURA_ALERTMANAGER_TOKEN_URLS receive the service account token.
func alertClients() []*alertmanager.Client {
	var httpClient *http.Client
	if caFile := os.Getenv(alertCAFileEnv); caFile != "" {
		client, err := alertmanager.HTTPClientWithCA(caFile)
		if err != nil {
			logger.Default.Warn("alertmanager CA not loaded, using system roots", "err", err)
		} else {
			httpClient = client
		}
	}
	var clients []*alertmanager.Client
	for _, url := range splitURLs(os.Getenv(alertTargetsEnv)) {
		clients = append(clients, alertmanager.NewClient(url, httpClient))
	}
	for _, url := range splitURLs(os.Getenv(alertTokenTargetsEnv)) {
		clients = append(clients, alertmanager.NewClient(url, httpClient, alertmanager.WithTokenFile(serviceAccountToken)))
	}
	return clients
}

// startAlerts reads failures every minute, which also notes forecasts for the track
// record, and repeats them to each configured Alertmanager.
func startAlerts(ctx context.Context, handlers *api.Handlers) {
	clients := alertClients()
	notifier := notify.New(notify.Config{
		SlackWebhookURL:   os.Getenv("RUPTURA_SLACK_URL"),
		GenericWebhookURL: os.Getenv("RUPTURA_WEBHOOK_URL"),
	})
	notified := map[string]bool{}
	go func() {
		ticker := time.NewTicker(alertInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				failures := handlers.Failures()
				sendAlerts(ctx, clients, failures)
				if notifier.Configured() {
					notified = notifyNew(ctx, notifier, notified, failures, time.Now())
				}
			}
		}
	}()
}

func sendAlerts(ctx context.Context, clients []*alertmanager.Client, failures []*api.FailureResult) {
	alerts := make([]alertmanager.Alert, 0, len(failures))
	for _, failure := range failures {
		description, timeLeft, window := api.AlertDetails(failure)
		alerts = append(alerts, alertmanager.Alert{
			Name:        failure.FailureMode,
			Severity:    failure.State,
			Level:       failure.Level,
			Target:      failure.Target,
			Kind:        failure.Kind,
			Summary:     failure.RecommendedAction,
			Impact:      api.ImpactText(failure.Affects, nil),
			Description: description,
			TimeLeft:    timeLeft,
			Window:      window,
		})
	}
	for _, client := range clients {
		if err := client.Send(ctx, alerts); err != nil {
			logger.Default.Warn("alert delivery failed", "err", err)
		}
	}
}

// eventSender sends one notification; *notify.Notifier implements it.
type eventSender interface {
	Notify(ctx context.Context, ev notify.Event) error
}

// notifyNew sends a webhook or Slack notification for each failure not
// notified yet, and returns the failures notified so far: a failure that
// clears and comes back is notified again.
func notifyNew(ctx context.Context, sender eventSender, notified map[string]bool, failures []*api.FailureResult, now time.Time) map[string]bool {
	current := make(map[string]bool, len(failures))
	for _, failure := range failures {
		key := failure.Target + "|" + failure.FailureMode
		current[key] = true
		if notified[key] {
			continue
		}
		description, timeLeft, _ := api.AlertDetails(failure)
		reason := description
		if timeLeft != "" {
			reason += " Time left: " + timeLeft + "."
		}
		reason += " " + failure.RecommendedAction
		tier := notify.Tier2
		if failure.State == "critical" {
			tier = notify.Tier1
		}
		ev := notify.Event{ID: key, Host: failure.Target, ActionType: failure.FailureMode, Tier: tier, Reason: reason, Timestamp: now}
		if err := sender.Notify(ctx, ev); err != nil {
			logger.Default.Warn("notify failed", "err", err)
			continue
		}
	}
	return current
}
