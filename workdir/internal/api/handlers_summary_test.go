package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestBuildWorkloadSummary_HeadlineGeneration(t *testing.T) {
	ref := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "payment-api"}

	tests := []struct {
		name           string
		forecast       *models.HealthForecast
		etaMinutes     int
		pending        *engine.ActionRecommendation
		wantWarmingUp  bool
		wantHeadlineHas []string
		wantTTFSeconds int64
	}{
		{
			name:           "warming-up: no forecast yet",
			forecast:       nil,
			etaMinutes:     150, // 2.5h
			wantWarmingUp:  true,
			wantHeadlineHas: []string{"Learning", "payment-api", "baseline", "~2h"},
		},
		{
			name:           "warming-up: sub-hour ETA still renders at least ~1h",
			forecast:       nil,
			etaMinutes:     10,
			wantWarmingUp:  true,
			wantHeadlineHas: []string{"~1h"},
		},
		{
			name: "stable: healthy, no critical ETA",
			forecast: &models.HealthForecast{
				Trend:              "stable",
				In15Min:            0.9,
				In30Min:            0.9,
				CriticalETAMinutes: 0,
				ConfidenceWindow:   60,
			},
			wantWarmingUp:  false,
			wantHeadlineHas: []string{"payment-api", "healthy", "stable"},
		},
		{
			name: "stable: improving trend",
			forecast: &models.HealthForecast{
				Trend:              "improving",
				CriticalETAMinutes: 0,
				ConfidenceWindow:   60,
			},
			wantWarmingUp:  false,
			wantHeadlineHas: []string{"payment-api", "healthy", "improving"},
		},
		{
			name: "breaching: predicted to cross threshold, no pending action",
			forecast: &models.HealthForecast{
				Trend:              "degrading",
				CriticalETAMinutes: 42,
				ConfidenceWindow:   54, // 90% of 60
			},
			wantWarmingUp:  false,
			wantHeadlineHas: []string{"payment-api", "predicted to breach", "42 minutes", "90% confidence", "Recommended action"},
			wantTTFSeconds: 42 * 60,
		},
		{
			name: "breaching: with a queued scale action",
			forecast: &models.HealthForecast{
				Trend:              "degrading",
				CriticalETAMinutes: 5,
				ConfidenceWindow:   30,
			},
			pending: &engine.ActionRecommendation{
				Host:       ref.Key(),
				ActionType: "scale",
				ScaleDelta: 2,
			},
			wantWarmingUp:  false,
			wantHeadlineHas: []string{"Scale out by 2 replica"},
			wantTTFSeconds: 5 * 60,
		},
		{
			name: "breaching: over an hour renders in hours",
			forecast: &models.HealthForecast{
				Trend:              "degrading",
				CriticalETAMinutes: 130,
				ConfidenceWindow:   60,
			},
			wantWarmingUp:  false,
			wantHeadlineHas: []string{"2 hours"},
			wantTTFSeconds: 130 * 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWorkloadSummary(ref, tt.forecast, tt.etaMinutes, tt.pending)

			if got.WarmingUp != tt.wantWarmingUp {
				t.Errorf("WarmingUp = %v, want %v", got.WarmingUp, tt.wantWarmingUp)
			}
			for _, substr := range tt.wantHeadlineHas {
				if !strings.Contains(got.Headline, substr) {
					t.Errorf("Headline = %q, want substring %q", got.Headline, substr)
				}
			}
			if tt.wantTTFSeconds != 0 && got.TTFSeconds != tt.wantTTFSeconds {
				t.Errorf("TTFSeconds = %d, want %d", got.TTFSeconds, tt.wantTTFSeconds)
			}
			if got.RecommendedAction == "" {
				t.Error("RecommendedAction must never be empty")
			}
		})
	}
}

func TestNoDataSummary_NeverRawJSON(t *testing.T) {
	ref := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "ghost-service"}
	got := noDataSummary(ref)

	if got.WarmingUp {
		t.Error("no-data case must not be reported as warming_up — it is a distinct case")
	}
	if !strings.Contains(got.Headline, "ghost-service") {
		t.Errorf("Headline = %q, want workload name present", got.Headline)
	}
	if strings.HasPrefix(strings.TrimSpace(got.Headline), "{") {
		t.Errorf("Headline looks like raw JSON, not a sentence: %q", got.Headline)
	}
}

func TestHandleWorkloadSummary_NoDataCase_HTTP(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	h := &Handlers{store: store}
	r := mux.NewRouter()
	r.HandleFunc("/api/v2/workloads/{namespace}/{kind}/{name}/summary", h.handleWorkloadSummary).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/api/v2/workloads/prod/Deployment/unknown-svc/summary", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no data received yet") {
		t.Errorf("body = %s, want no-data headline", rec.Body.String())
	}
}

func TestHandleWorkloadSummary_WarmingUpCase_HTTP(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	a := analyzer.NewAnalyzer()
	ref := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "fresh-svc"}
	// A single Update() call is enough to create a snapshot but far short of
	// the analyzer's calibration bar — the summary must report warming_up,
	// not crash or fall through to the no-data case.
	snap := a.Update(ref, map[string]float64{"cpu_percent": 0.2})
	store.StoreSnapshot(snap)

	h := &Handlers{store: store, analyzer: a}
	r := mux.NewRouter()
	r.HandleFunc("/api/v2/workloads/{namespace}/{kind}/{name}/summary", h.handleWorkloadSummary).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/api/v2/workloads/prod/Deployment/fresh-svc/summary", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"warming_up":true`) {
		t.Errorf("body = %s, want warming_up:true", body)
	}
	if !strings.Contains(body, "Learning") {
		t.Errorf("body = %s, want the warming-up headline text", body)
	}
}

func TestRecommendedActionText_AllActionTypes(t *testing.T) {
	tests := []struct {
		actionType string
		scaleDelta int
		wantHas    string
	}{
		{"scale", 3, "Scale out by 3"},
		{"scale", -2, "Scale in by 2"},
		{"scale", 0, "Scale out by 1"},
		{"restart", 0, "Restart"},
		{"cordon", 0, "Cordon"},
		{"page", 0, "Page on-call"},
		{"alert", 0, "Alert on-call"},
		{"notify", 0, "Alert on-call"},
		{"custom", 0, "Actions tab"},
	}
	for _, tt := range tests {
		t.Run(tt.actionType, func(t *testing.T) {
			rec := &engine.ActionRecommendation{ActionType: tt.actionType, ScaleDelta: tt.scaleDelta}
			got := recommendedActionText(rec)
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("recommendedActionText(%q, delta=%d) = %q, want substring %q", tt.actionType, tt.scaleDelta, got, tt.wantHas)
			}
		})
	}
	if got := recommendedActionText(nil); !strings.Contains(got, "Monitor") {
		t.Errorf("recommendedActionText(nil) = %q, want a monitor fallback", got)
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{5, "5 minutes"},
		{59, "59 minutes"},
		{60, "1 hours"},
		{130, "2 hours"},
		{1439, "23 hours"},
		{1440, "1 days"},
		{2880, "2 days"},
	}
	for _, tt := range tests {
		if got := formatETA(tt.minutes); got != tt.want {
			t.Errorf("formatETA(%d) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}
