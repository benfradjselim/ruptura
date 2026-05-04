package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
)

type Provider interface {
	Execute(ctx context.Context, a engine.ActionRecommendation) error
	Name() string
}

// WebhookProvider

type WebhookProvider struct {
	url    string
	client *http.Client
}

func NewWebhookProvider(url string, client *http.Client) *WebhookProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &WebhookProvider{url: url, client: client}
}

func (p *WebhookProvider) Execute(ctx context.Context, a engine.ActionRecommendation) error {
	if p.url == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]interface{}{
		"event_id":    a.EventID,
		"host":        a.Host,
		"action_type": a.ActionType,
		"tier":        int(a.Tier),
		"confidence":  a.Confidence,
		"timestamp":   a.Timestamp,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", p.url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func (p *WebhookProvider) Name() string { return "webhook" }

// AlertmanagerProvider

type AlertmanagerProvider struct {
	url    string
	client *http.Client
}

func NewAlertmanagerProvider(url string, client *http.Client) *AlertmanagerProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &AlertmanagerProvider{url: url, client: client}
}

func (p *AlertmanagerProvider) Execute(ctx context.Context, a engine.ActionRecommendation) error {
	if p.url == "" {
		return nil
	}
	payload := []map[string]interface{}{
		{
			"labels": map[string]string{
				"alertname":   "RupturaRupture",
				"host":        a.Host,
				"action_type": a.ActionType,
			},
			"annotations": map[string]string{
				"confidence": fmt.Sprintf("%f", a.Confidence),
			},
			"generatorURL": "",
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.url+"/api/v2/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func (p *AlertmanagerProvider) Name() string { return "alertmanager" }

// PagerDutyProvider

type PagerDutyProvider struct {
	integrationKey string
	baseURL        string
	client         *http.Client
}

func NewPagerDutyProvider(integrationKey string, client *http.Client) *PagerDutyProvider {
	return NewPagerDutyProviderWithURL(integrationKey, "https://events.pagerduty.com/v2/enqueue", client)
}

func NewPagerDutyProviderWithURL(integrationKey, baseURL string, client *http.Client) *PagerDutyProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &PagerDutyProvider{integrationKey: integrationKey, baseURL: baseURL, client: client}
}

func (p *PagerDutyProvider) Execute(ctx context.Context, a engine.ActionRecommendation) error {
	if p.integrationKey == "" {
		return nil
	}
	payload := map[string]interface{}{
		"routing_key":  p.integrationKey,
		"event_action": "trigger",
		"payload": map[string]string{
			"summary":  fmt.Sprintf("Ruptura: %s FusedR=%.2f confidence=%.2f", a.Host, a.R, a.Confidence),
			"severity": "critical",
			"source":   a.Host,
		},
	}
	// ActionRecommendation doesn't have R value.
	// Oh, I see "Ruptura rupture: <host> R=<R>". I'll use Confidence for R, maybe.
	// Actually instruction says RuptureEvent has R, but ActionRecommendation does NOT have R.
	// I'll just put Confidence as R for now, as that's the only value I have that resembles R.
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func (p *PagerDutyProvider) Name() string { return "pagerduty" }
