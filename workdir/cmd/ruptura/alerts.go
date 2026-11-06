package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/alertmanager"
	"github.com/benfradjselim/ruptura/internal/api"
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
			Impact:   api.ImpactText(failure.Affects),
		})
	}
	for _, client := range clients {
		if err := client.Send(ctx, alerts); err != nil {
			logger.Default.Warn("alert delivery failed", "err", err)
		}
	}
}
