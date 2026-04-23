package models

import (
	"testing"
)

func TestLoginRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		{"valid", LoginRequest{"user", "pass"}, false},
		{"empty username", LoginRequest{"", "pass"}, true},
		{"empty password", LoginRequest{"user", ""}, true},
		{"all empty", LoginRequest{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIKeyCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     APIKeyCreateRequest
		wantErr bool
	}{
		{"valid", APIKeyCreateRequest{"key", "admin", ""}, false},
		{"empty name", APIKeyCreateRequest{"", "admin", ""}, true},
		{"empty role", APIKeyCreateRequest{"key", "", ""}, true},
		{"all empty", APIKeyCreateRequest{"", "", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultQuota(t *testing.T) {
	quota := DefaultQuota()
	if quota.MaxDashboards != 10 {
		t.Errorf("expected 10, got %d", quota.MaxDashboards)
	}
}

func TestDefaultFatigueConfig(t *testing.T) {
	config := DefaultFatigueConfig()
	if config.RThreshold != 0.3 {
		t.Errorf("expected 0.3, got %f", config.RThreshold)
	}
}

func TestStructInitialization(t *testing.T) {
	_ = Metric{Name: "cpu", Value: 0.5}
	_ = MetricBatch{AgentID: "a1"}
	_ = KPI{Name: "stress", Value: 0.1}
	_ = NotificationChannel{ID: "n1", Name: "slack"}
	_ = Org{ID: "o1", Name: "default"}
	_ = Alert{ID: "a1", Severity: SeverityInfo}
	_ = Dashboard{ID: "d1", Name: "main"}
	_ = SLO{ID: "s1", Target: 99.9}
	_ = DataSource{ID: "ds1", Name: "prom"}
}
