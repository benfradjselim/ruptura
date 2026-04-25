package storage

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func openQuotaTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-quota-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCheckDashboardQuota(t *testing.T) {
	s := openQuotaTestStore(t)
	os := s.ForOrg("quota-org")

	// Unlimited quota — always passes
	if err := os.CheckDashboardQuota(0); err != nil {
		t.Errorf("unlimited quota should not error: %v", err)
	}

	// Add 2 dashboards
	type dash struct{ Name string }
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("d%d", i)
		if err := os.SaveDashboard(id, dash{Name: id}); err != nil {
			t.Fatalf("SaveDashboard: %v", err)
		}
	}

	// Quota of 3 — should still pass
	if err := os.CheckDashboardQuota(3); err != nil {
		t.Errorf("quota 3 with 2 dashboards should pass: %v", err)
	}

	// Quota of 2 — should fail
	if err := os.CheckDashboardQuota(2); err == nil {
		t.Error("quota 2 with 2 dashboards should fail")
	}
}

func TestCheckDataSourceQuota(t *testing.T) {
	s := openQuotaTestStore(t)
	os := s.ForOrg("ds-org")

	type ds struct{ Name string }
	_ = os.SaveDataSource("ds1", ds{"one"})
	_ = os.SaveDataSource("ds2", ds{"two"})

	if err := os.CheckDataSourceQuota(0); err != nil {
		t.Errorf("unlimited quota: %v", err)
	}
	if err := os.CheckDataSourceQuota(5); err != nil {
		t.Errorf("quota 5 with 2 should pass: %v", err)
	}
	if err := os.CheckDataSourceQuota(1); err == nil {
		t.Error("quota 1 with 2 should fail")
	}
}

func TestCheckAPIKeyQuota(t *testing.T) {
	s := openQuotaTestStore(t)
	os := s.ForOrg("key-org")

	type key struct{ ID string }
	_ = os.SaveAPIKey("k1", key{"k1"})

	if err := os.CheckAPIKeyQuota(0); err != nil {
		t.Errorf("unlimited quota: %v", err)
	}
	if err := os.CheckAPIKeyQuota(2); err != nil {
		t.Errorf("quota 2 with 1 should pass: %v", err)
	}
	if err := os.CheckAPIKeyQuota(1); err == nil {
		t.Error("quota 1 with 1 should fail")
	}
}

func TestCheckAlertRuleQuota(t *testing.T) {
	s := openQuotaTestStore(t)
	os := s.ForOrg("ar-org")

	if err := os.CheckAlertRuleQuota(0); err != nil {
		t.Errorf("unlimited quota: %v", err)
	}
	if err := os.CheckAlertRuleQuota(10); err != nil {
		t.Errorf("quota 10 with 0 should pass: %v", err)
	}
}

func TestCheckSLOQuota(t *testing.T) {
	s := openQuotaTestStore(t)
	os := s.ForOrg("slo-org")

	type slo struct{ ID string }
	_ = os.SaveSLO("slo1", slo{"slo1"})
	_ = os.SaveSLO("slo2", slo{"slo2"})
	_ = os.SaveSLO("slo3", slo{"slo3"})

	if err := os.CheckSLOQuota(5); err != nil {
		t.Errorf("quota 5 with 3 should pass: %v", err)
	}
	if err := os.CheckSLOQuota(3); err == nil {
		t.Error("quota 3 with 3 should fail")
	}
}

func TestAuditLog(t *testing.T) {
	s := openQuotaTestStore(t)

	entry := AuditEntry{
		Timestamp:  time.Now().UTC(),
		OrgID:      "test-org",
		Username:   "alice",
		Action:     "create",
		Resource:   "dashboard",
		ResourceID: "d1",
		Details:    "My Dashboard",
		IPAddress:  "1.2.3.4",
	}
	if err := s.AppendAuditEntry(entry); err != nil {
		t.Fatalf("AppendAuditEntry: %v", err)
	}

	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)
	entries, err := s.QueryAuditLog("test-org", "", from, to, 10)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Username != "alice" {
		t.Errorf("username: got %q, want alice", entries[0].Username)
	}
}

func TestAuditLogFilterByUsername(t *testing.T) {
	s := openQuotaTestStore(t)

	for _, user := range []string{"alice", "bob", "alice"} {
		_ = s.AppendAuditEntry(AuditEntry{
			Timestamp: time.Now().UTC(),
			OrgID:     "org",
			Username:  user,
			Action:    "read",
			Resource:  "dashboard",
		})
	}

	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)
	entries, err := s.QueryAuditLog("org", "alice", from, to, 10)
	if err != nil {
		t.Fatalf("QueryAuditLog: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 alice entries, got %d", len(entries))
	}
}

func TestTokenRevocation(t *testing.T) {
	s := openQuotaTestStore(t)

	jti := "test-jti-1234"
	if s.IsTokenRevoked(jti) {
		t.Error("token should not be revoked before RevokeToken")
	}

	if err := s.RevokeToken(jti, time.Minute); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}

	if !s.IsTokenRevoked(jti) {
		t.Error("token should be revoked after RevokeToken")
	}
}

func TestListOrgIDs(t *testing.T) {
	s := openQuotaTestStore(t)

	type org struct{ ID string }
	_ = s.SaveOrg("org1", org{"org1"})
	_ = s.SaveOrg("org2", org{"org2"})

	ids, err := s.ListOrgIDs()
	if err != nil {
		t.Fatalf("ListOrgIDs: %v", err)
	}
	if len(ids) < 2 {
		t.Errorf("expected ≥2 org IDs, got %d: %v", len(ids), ids)
	}
}

func TestOrgIsolationMetrics(t *testing.T) {
	s := openQuotaTestStore(t)
	orgA := s.ForOrg("orgA")
	orgB := s.ForOrg("orgB")

	now := time.Now()
	_ = orgA.SaveMetric("host1", "cpu", 80.0, now)
	_ = orgB.SaveMetric("host1", "cpu", 50.0, now)

	from := now.Add(-time.Minute)
	to := now.Add(time.Minute)
	aMetrics, err := orgA.GetMetricRange("host1", "cpu", from, to)
	if err != nil {
		t.Fatalf("GetMetricRange orgA: %v", err)
	}
	bMetrics, err := orgB.GetMetricRange("host1", "cpu", from, to)
	if err != nil {
		t.Fatalf("GetMetricRange orgB: %v", err)
	}

	if len(aMetrics) == 0 || len(bMetrics) == 0 {
		t.Fatal("expected metrics in both orgs")
	}
	if aMetrics[0].Value == bMetrics[0].Value {
		t.Errorf("org isolation failed: both orgs returned same value %g", aMetrics[0].Value)
	}
}
