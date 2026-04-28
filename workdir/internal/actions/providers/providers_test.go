package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benfradjselim/kairo-core/internal/actions/engine"
)

func TestWebhookProvider_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := NewWebhookProvider(ts.URL, nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestWebhookProvider_serverError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	p := NewWebhookProvider(ts.URL, nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestWebhookProvider_emptyURL(t *testing.T) {
	p := NewWebhookProvider("", nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestAlertmanagerProvider_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := NewAlertmanagerProvider(ts.URL, nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestAlertmanagerProvider_emptyURL(t *testing.T) {
	p := NewAlertmanagerProvider("", nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestPagerDutyProvider_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	p := NewPagerDutyProviderWithURL("key", ts.URL, nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestPagerDutyProvider_emptyKey(t *testing.T) {
	p := NewPagerDutyProvider("", nil)
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestKubernetesProvider_alwaysNil(t *testing.T) {
	p := NewKubernetesProvider()
	err := p.Execute(context.Background(), engine.ActionRecommendation{Tier: engine.Tier1})
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}
