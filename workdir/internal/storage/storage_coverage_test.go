package storage

import (
	"testing"
	"time"
)

// --- NotificationChannel ---

func TestNotificationChannelCRUD(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type nc struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}

	if err := s.SaveNotificationChannel("nc1", nc{ID: "nc1", Type: "slack", URL: "https://hooks.slack.com/x"}); err != nil {
		t.Fatalf("SaveNotificationChannel: %v", err)
	}
	if err := s.SaveNotificationChannel("nc2", nc{ID: "nc2", Type: "webhook", URL: "https://example.com"}); err != nil {
		t.Fatalf("SaveNotificationChannel nc2: %v", err)
	}

	var got nc
	if err := s.GetNotificationChannel("nc1", &got); err != nil {
		t.Fatalf("GetNotificationChannel: %v", err)
	}
	if got.Type != "slack" {
		t.Errorf("type = %q; want slack", got.Type)
	}

	count := 0
	if err := s.ListNotificationChannels(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListNotificationChannels: %v", err)
	}
	if count != 2 {
		t.Errorf("ListNotificationChannels count = %d; want 2", count)
	}

	if err := s.DeleteNotificationChannel("nc1"); err != nil {
		t.Fatalf("DeleteNotificationChannel: %v", err)
	}
	count = 0
	_ = s.ListNotificationChannels(func([]byte) error { count++; return nil })
	if count != 1 {
		t.Errorf("after delete count = %d; want 1", count)
	}
}

// --- SLO ---

func TestSLOCRUD(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type slo struct {
		ID     string  `json:"id"`
		Target float64 `json:"target"`
	}

	if err := s.SaveSLO("slo1", slo{ID: "slo1", Target: 0.999}); err != nil {
		t.Fatalf("SaveSLO: %v", err)
	}
	if err := s.SaveSLO("slo2", slo{ID: "slo2", Target: 0.99}); err != nil {
		t.Fatalf("SaveSLO slo2: %v", err)
	}

	var got slo
	if err := s.GetSLO("slo1", &got); err != nil {
		t.Fatalf("GetSLO: %v", err)
	}
	if got.Target != 0.999 {
		t.Errorf("target = %v; want 0.999", got.Target)
	}

	count := 0
	if err := s.ListSLOs(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListSLOs: %v", err)
	}
	if count != 2 {
		t.Errorf("ListSLOs count = %d; want 2", count)
	}

	if err := s.DeleteSLO("slo2"); err != nil {
		t.Fatalf("DeleteSLO: %v", err)
	}
	count = 0
	_ = s.ListSLOs(func([]byte) error { count++; return nil })
	if count != 1 {
		t.Errorf("after delete count = %d; want 1", count)
	}

	// GetSLO on deleted item should error
	var gone slo
	if err := s.GetSLO("slo2", &gone); err == nil {
		t.Error("expected error getting deleted SLO")
	}
}

// --- Org ---

func TestOrgCRUD(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type org struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := s.SaveOrg("org1", org{ID: "org1", Name: "Acme"}); err != nil {
		t.Fatalf("SaveOrg: %v", err)
	}
	if err := s.SaveOrg("org2", org{ID: "org2", Name: "Beta"}); err != nil {
		t.Fatalf("SaveOrg org2: %v", err)
	}

	var got org
	if err := s.GetOrg("org1", &got); err != nil {
		t.Fatalf("GetOrg: %v", err)
	}
	if got.Name != "Acme" {
		t.Errorf("name = %q; want Acme", got.Name)
	}

	count := 0
	if err := s.ListOrgs(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListOrgs: %v", err)
	}
	if count != 2 {
		t.Errorf("ListOrgs count = %d; want 2", count)
	}

	ids, err := s.ListOrgIDs()
	if err != nil {
		t.Fatalf("ListOrgIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("ListOrgIDs len = %d; want 2", len(ids))
	}

	if err := s.DeleteOrg("org2"); err != nil {
		t.Fatalf("DeleteOrg: %v", err)
	}
	count = 0
	_ = s.ListOrgs(func([]byte) error { count++; return nil })
	if count != 1 {
		t.Errorf("after delete count = %d; want 1", count)
	}
}

// --- sanitizeKeySegment ---

func TestSanitizeKeySegment(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"with:colon", "with_colon"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"a:b/c\\d", "a_b_c_d"},
		{"", ""},
	}
	for _, tc := range cases {
		got := sanitizeKeySegment(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeKeySegment(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// --- Logs ---

func TestSaveAndQueryLogs(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type logEntry struct {
		Level   string `json:"level"`
		Message string `json:"msg"`
	}

	now := time.Now()
	for i := 0; i < 5; i++ {
		ts := now.Add(time.Duration(i) * time.Second)
		if err := s.SaveLog("myservice", logEntry{Level: "info", Message: "ok"}, ts); err != nil {
			t.Fatalf("SaveLog[%d]: %v", i, err)
		}
	}

	// QueryLogs with service filter
	logs, err := s.QueryLogs("myservice", now.Add(-time.Second), now.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected log entries, got 0")
	}

	// QueryAllLogs (no service filter)
	all, err := s.QueryAllLogs(now.Add(-time.Second), now.Add(time.Minute), 0)
	if err != nil {
		t.Fatalf("QueryAllLogs: %v", err)
	}
	if len(all) == 0 {
		t.Error("QueryAllLogs: expected entries, got 0")
	}

	// QueryLogs with limit
	limited, err := s.QueryLogs("myservice", now.Add(-time.Second), now.Add(time.Minute), 2)
	if err != nil {
		t.Fatalf("QueryLogs limited: %v", err)
	}
	if len(limited) > 2 {
		t.Errorf("limit=2 but got %d entries", len(limited))
	}
}

func TestQueryLogs_EmptyService(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type logEntry struct {
		Msg string `json:"msg"`
	}
	now := time.Now()
	_ = s.SaveLog("svcA", logEntry{"from A"}, now)
	_ = s.SaveLog("svcB", logEntry{"from B"}, now.Add(time.Second))

	all, err := s.QueryLogs("", now.Add(-time.Second), now.Add(2*time.Second), 0)
	if err != nil {
		t.Fatalf("QueryLogs empty service: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("expected ≥2 entries across services, got %d", len(all))
	}
}

// --- Spans / Traces ---

func TestSaveAndQuerySpans(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type span struct {
		TraceID    string  `json:"trace_id"`
		SpanID     string  `json:"span_id"`
		Service    string  `json:"service"`
		Name       string  `json:"name"`
		StartMs    int64   `json:"start_time_ms"`
		DurationMs float64 `json:"duration_ms"`
	}

	traceID := "trace-abc-123"
	sp1 := span{TraceID: traceID, SpanID: "span-1", Service: "api", Name: "GET /health", StartMs: 1000, DurationMs: 5}
	sp2 := span{TraceID: traceID, SpanID: "span-2", Service: "db", Name: "SELECT", StartMs: 1002, DurationMs: 3}

	if err := s.SaveSpan(sp1, traceID, "span-1"); err != nil {
		t.Fatalf("SaveSpan span-1: %v", err)
	}
	if err := s.SaveSpan(sp2, traceID, "span-2"); err != nil {
		t.Fatalf("SaveSpan span-2: %v", err)
	}

	spans, err := s.QuerySpansByTrace(traceID)
	if err != nil {
		t.Fatalf("QuerySpansByTrace: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(spans))
	}

	// QueryTraceList
	headers, err := s.QueryTraceList("", 10)
	if err != nil {
		t.Fatalf("QueryTraceList: %v", err)
	}
	if len(headers) == 0 {
		t.Error("expected trace headers, got 0")
	}
}

func TestQueryTraceList_ServiceFilter(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type span struct {
		TraceID    string  `json:"trace_id"`
		SpanID     string  `json:"span_id"`
		Service    string  `json:"service"`
		Name       string  `json:"name"`
		StartMs    int64   `json:"start_time_ms"`
		DurationMs float64 `json:"duration_ms"`
	}

	_ = s.SaveSpan(span{TraceID: "t1", SpanID: "s1", Service: "frontend", Name: "op", StartMs: 100, DurationMs: 10}, "t1", "s1")
	_ = s.SaveSpan(span{TraceID: "t2", SpanID: "s2", Service: "backend", Name: "op", StartMs: 200, DurationMs: 5}, "t2", "s2")

	headers, err := s.QueryTraceList("frontend", 10)
	if err != nil {
		t.Fatalf("QueryTraceList with filter: %v", err)
	}
	for _, h := range headers {
		if h.TraceID == "t2" {
			t.Errorf("unexpected trace t2 from backend service in frontend filter results")
		}
	}
}

// --- GetKPIRangeTiered ---

func TestGetKPIRangeTiered_ShortWindow(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	now := time.Now()
	_ = s.SaveKPI("h1", "stress", 0.5, now.Add(-1*time.Hour))

	pts, err := s.GetKPIRangeTiered("h1", "stress", now.Add(-2*time.Hour), now)
	if err != nil {
		t.Fatalf("GetKPIRangeTiered short: %v", err)
	}
	if len(pts) == 0 {
		t.Error("expected KPI points in short window, got 0")
	}
}

func TestGetKPIRangeTiered_MediumWindow(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	// Query a 2-day window (>6h, ≤7d) — hits kr5: rollup tier (may be empty if not compacted)
	now := time.Now()
	_, err := s.GetKPIRangeTiered("h1", "stress", now.Add(-48*time.Hour), now)
	if err != nil {
		t.Fatalf("GetKPIRangeTiered medium: %v", err)
	}
}

func TestGetKPIRangeTiered_LongWindow(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	// Query a 30-day window (>7d) — hits kr1h: rollup tier
	now := time.Now()
	_, err := s.GetKPIRangeTiered("h1", "stress", now.Add(-30*24*time.Hour), now)
	if err != nil {
		t.Fatalf("GetKPIRangeTiered long: %v", err)
	}
}

// --- RevokeToken / IsTokenRevoked ---

func TestRevokeAndCheckToken(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	if s.IsTokenRevoked("jti-999") {
		t.Error("token should not be revoked before revocation")
	}

	if err := s.RevokeToken("jti-999", time.Hour); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}

	if !s.IsTokenRevoked("jti-999") {
		t.Error("token should be revoked after RevokeToken")
	}

	// Edge cases: empty JTI and zero TTL should be no-ops
	if err := s.RevokeToken("", time.Hour); err != nil {
		t.Fatalf("RevokeToken empty jti: %v", err)
	}
	if err := s.RevokeToken("jti-zero", 0); err != nil {
		t.Fatalf("RevokeToken zero ttl: %v", err)
	}
}
