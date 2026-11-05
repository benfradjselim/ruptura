package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

func TestHandleFailureSeries(t *testing.T) {
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tracker := failuremodes.NewTracker()
	tracker.Observe("shop/Deployment/api", "shop/api-a", at, 150, 200, 0)
	tests := []struct {
		name       string
		query      string
		modes      FailureEstimates
		wantStatus int
		wantPods   int
	}{
		{"series of a known workload", "?workload=shop/Deployment/api", tracker, http.StatusOK, 1},
		{"unknown workload is empty", "?workload=shop/Deployment/db", tracker, http.StatusOK, 0},
		{"no estimator is empty", "?workload=shop/Deployment/api", nil, http.StatusOK, 0},
		{"workload is required", "", tracker, http.StatusBadRequest, 0},
		{"workload too long", "?workload=" + strings.Repeat("a", maxWorkloadKeyLength+1), tracker, http.StatusBadRequest, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handlers{failureModes: tc.modes}
			rec := httptest.NewRecorder()
			h.handleFailureSeries(rec, httptest.NewRequest(http.MethodGet, "/api/v2/failures/series"+tc.query, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var got failureSeriesResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(got.Pods) != tc.wantPods {
				t.Fatalf("got %d pods, want %d", len(got.Pods), tc.wantPods)
			}
			if tc.wantPods == 1 && (got.Pods[0].Points[0].At != at.UnixMilli() || got.Pods[0].Points[0].Usage != 0.75) {
				t.Errorf("got point %+v", got.Pods[0].Points[0])
			}
		})
	}
}
