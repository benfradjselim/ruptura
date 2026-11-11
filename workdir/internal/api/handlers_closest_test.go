package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

type closestEstimates struct {
	fixedEstimates
	pods []failuremodes.Watched
}

func (c closestEstimates) Closest(_ time.Time, n int) []failuremodes.Watched {
	if len(c.pods) > n {
		return c.pods[:n]
	}
	return c.pods
}

func TestHandleClosest(t *testing.T) {
	pods := []failuremodes.Watched{
		{Workload: "shop/Deployment/api", Pod: "shop/api-1", Usage: 0.91, TrendPerHour: 0.2, Readings: 60},
		{Workload: "infra/Deployment/db", Pod: "infra/db-1", Usage: 0.8, Readings: 60},
		{Workload: "shop/Deployment/web", Pod: "shop/web-1", Usage: 0.4, Readings: 60},
	}
	tests := []struct {
		name     string
		selector map[string]string
		want     []string
	}{
		{"every namespace", nil, []string{"shop/api-1", "infra/db-1", "shop/web-1"}},
		{"selected namespaces only", map[string]string{"ruptura.io/enabled": "true"}, []string{"shop/api-1", "shop/web-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handlers{}
			h.SetFailureModes(closestEstimates{pods: pods})
			h.SetNamespaceSelector(tt.selector, fakeNamespaces{"shop": {"ruptura.io/enabled": "true"}, "infra": {}})
			rec := httptest.NewRecorder()
			h.handleClosest(rec, httptest.NewRequest(http.MethodGet, "/api/v2/failures/closest", nil))
			var got []closestPod
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("code %d, err %v", rec.Code, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %+v, want pods %v", got, tt.want)
			}
			for i, pod := range tt.want {
				if got[i].Pod != pod {
					t.Fatalf("pod %d = %s, want %s", i, got[i].Pod, pod)
				}
			}
		})
	}
}

func TestHandleClosest_WithoutTrackerIsEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Handlers{}).handleClosest(rec, httptest.NewRequest(http.MethodGet, "/api/v2/failures/closest", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" && rec.Body.String() != "[]" {
		t.Fatalf("code %d body %q", rec.Code, rec.Body.String())
	}
}
