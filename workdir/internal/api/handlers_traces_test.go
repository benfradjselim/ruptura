package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/gorilla/mux"
)

func TestTraces_ListAndGet(t *testing.T) {
	store, done := newTestStore(t)
	defer done()
	span := models.Span{TraceID: "t1", SpanID: "s1", Service: "checkout", Operation: "pay", StartTime: time.Now(), Status: "ok"}
	if err := store.SaveSpan(span, span.TraceID, span.SpanID); err != nil {
		t.Fatal(err)
	}
	h := New(store, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "")

	rec := httptest.NewRecorder()
	h.handleTraceSearch(rec, httptest.NewRequest(http.MethodGet, "/api/v2/traces?service=checkout", nil))
	var list struct {
		Traces []json.RawMessage `json:"traces"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Traces) != 1 {
		t.Fatalf("list = %s, %v", rec.Body.String(), err)
	}

	rec = httptest.NewRecorder()
	h.handleTraceGet(rec, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/api/v2/traces/t1", nil), map[string]string{"id": "t1"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("get = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.handleTraceGet(rec, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/api/v2/traces/none", nil), map[string]string{"id": "none"}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing trace = %d, want 404", rec.Code)
	}
}
