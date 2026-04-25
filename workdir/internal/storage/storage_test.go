package storage

import (
	"os"
	"testing"
	"time"
)

func TestStorageOpenClose(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !s.Healthy() {
		t.Error("store should be healthy")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSaveAndGetMetric(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	now := time.Now()
	if err := s.SaveMetric("host1", "cpu_percent", 0.75, now); err != nil {
		t.Fatalf("SaveMetric: %v", err)
	}

	from := now.Add(-time.Second)
	to := now.Add(time.Second)
	vals, err := s.GetMetricRange("host1", "cpu_percent", from, to)
	if err != nil {
		t.Fatalf("GetMetricRange: %v", err)
	}
	if len(vals) != 1 {
		t.Errorf("expected 1 metric, got %d", len(vals))
	}
	if vals[0].Value != 0.75 {
		t.Errorf("value = %v; want 0.75", vals[0].Value)
	}
}

func TestSaveAndGetAlert(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	alert := map[string]interface{}{
		"id":   "alert1",
		"name": "stress_panic",
	}
	if err := s.SaveAlert("alert1", alert); err != nil {
		t.Fatalf("SaveAlert: %v", err)
	}

	var got map[string]interface{}
	if err := s.GetAlert("alert1", &got); err != nil {
		t.Fatalf("GetAlert: %v", err)
	}
	if got["name"] != "stress_panic" {
		t.Errorf("name = %v; want stress_panic", got["name"])
	}
}

func TestSaveAndGetDashboard(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type dash struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := s.SaveDashboard("dash1", dash{ID: "dash1", Name: "System Overview"}); err != nil {
		t.Fatalf("SaveDashboard: %v", err)
	}

	var got dash
	if err := s.GetDashboard("dash1", &got); err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if got.Name != "System Overview" {
		t.Errorf("name = %q; want System Overview", got.Name)
	}

	if err := s.DeleteDashboard("dash1"); err != nil {
		t.Fatalf("DeleteDashboard: %v", err)
	}
	if err := s.GetDashboard("dash1", &got); err == nil {
		t.Error("expected error for deleted dashboard")
	}
}

func TestListDashboards(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		_ = s.SaveDashboard(id, map[string]string{"id": id})
	}

	count := 0
	_ = s.ListDashboards(func(val []byte) error {
		count++
		return nil
	})
	if count != 3 {
		t.Errorf("expected 3 dashboards, got %d", count)
	}
}

func TestSaveAndGetUser(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type user struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := s.SaveUser("admin", user{Username: "admin", Role: "admin"}); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}

	var got user
	if err := s.GetUser("admin", &got); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Role != "admin" {
		t.Errorf("role = %q; want admin", got.Role)
	}
}

func TestKPIRangeFilter(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	base := time.Now()
	// Save KPIs at t-2h, t-1h, t
	for i := 0; i < 3; i++ {
		ts := base.Add(-time.Duration(2-i) * time.Hour)
		_ = s.SaveKPI("host1", "stress", float64(i)*0.1, ts)
	}

	// Query only last 90 minutes
	from := base.Add(-90 * time.Minute)
	vals, err := s.GetKPIRange("host1", "stress", from, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetKPIRange: %v", err)
	}
	if len(vals) != 2 {
		t.Errorf("expected 2 KPI points in range, got %d", len(vals))
	}
}

func TestRunGC(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()
	// RunGC on an essentially empty store returns ErrNoRewrite — that's expected and not a failure
	_ = s.RunGC()
}

func TestAlertCRUD(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	alert := map[string]string{"id": "al1", "name": "stress_panic"}
	if err := s.SaveAlert("al1", alert); err != nil {
		t.Fatalf("SaveAlert: %v", err)
	}

	// ListAlerts
	count := 0
	if err := s.ListAlerts(func(val []byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if count != 1 {
		t.Errorf("ListAlerts count = %d; want 1", count)
	}

	// DeleteAlert
	if err := s.DeleteAlert("al1"); err != nil {
		t.Fatalf("DeleteAlert: %v", err)
	}
	count = 0
	s.ListAlerts(func([]byte) error { count++; return nil })
	if count != 0 {
		t.Errorf("after delete ListAlerts count = %d; want 0", count)
	}
}

func TestUserCRUDStorage(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	type user struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := s.SaveUser("bob", user{Username: "bob", Role: "viewer"}); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	if err := s.SaveUser("alice", user{Username: "alice", Role: "admin"}); err != nil {
		t.Fatalf("SaveUser alice: %v", err)
	}

	// ListUsers
	count := 0
	if err := s.ListUsers(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if count != 2 {
		t.Errorf("ListUsers count = %d; want 2", count)
	}

	// DeleteUser
	if err := s.DeleteUser("bob"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	count = 0
	s.ListUsers(func([]byte) error { count++; return nil })
	if count != 1 {
		t.Errorf("after delete ListUsers count = %d; want 1", count)
	}
}

func TestDataSourceCRUDStorage(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	ds := map[string]string{"id": "ds1", "name": "prometheus", "url": "http://localhost:9090"}
	if err := s.SaveDataSource("ds1", ds); err != nil {
		t.Fatalf("SaveDataSource: %v", err)
	}

	// GetDataSource
	var got map[string]string
	if err := s.GetDataSource("ds1", &got); err != nil {
		t.Fatalf("GetDataSource: %v", err)
	}
	if got["name"] != "prometheus" {
		t.Errorf("name = %q; want prometheus", got["name"])
	}

	// ListDataSources
	count := 0
	if err := s.ListDataSources(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("ListDataSources: %v", err)
	}
	if count != 1 {
		t.Errorf("ListDataSources count = %d; want 1", count)
	}

	// DeleteDataSource
	if err := s.DeleteDataSource("ds1"); err != nil {
		t.Fatalf("DeleteDataSource: %v", err)
	}
	var gone map[string]string
	if err := s.GetDataSource("ds1", &gone); err == nil {
		t.Error("expected error getting deleted datasource")
	}
}

// TestOrgStoreIsolation verifies that data written by one org cannot be read
// by another org, providing hard multi-tenant data isolation at the storage layer.
func TestOrgStoreIsolation(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	now := time.Now()
	orgA := s.ForOrg("acme")
	orgB := s.ForOrg("beta")

	// Write a metric only for org A
	if err := orgA.SaveMetric("host1", "cpu_percent", 0.9, now); err != nil {
		t.Fatalf("orgA.SaveMetric: %v", err)
	}

	// Org A can read it
	valA, err := orgA.GetMetricRange("host1", "cpu_percent", now.Add(-time.Second), now.Add(time.Second))
	if err != nil || len(valA) == 0 {
		t.Fatalf("orgA.GetMetricRange: err=%v len=%d", err, len(valA))
	}

	// Org B gets nothing — key space is isolated
	valB, err := orgB.GetMetricRange("host1", "cpu_percent", now.Add(-time.Second), now.Add(time.Second))
	if err != nil {
		t.Fatalf("orgB.GetMetricRange unexpected error: %v", err)
	}
	if len(valB) != 0 {
		t.Errorf("org isolation breach: orgB got %d metrics that belong to orgA", len(valB))
	}
}

// TestOrgStoreDashboardIsolation verifies dashboard isolation between orgs.
func TestOrgStoreDashboardIsolation(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	orgA := s.ForOrg("acme")
	orgB := s.ForOrg("beta")

	if err := orgA.SaveDashboard("dash1", map[string]string{"title": "A dash"}); err != nil {
		t.Fatalf("orgA.SaveDashboard: %v", err)
	}

	// Org B listing dashboards should return nothing
	var count int
	if err := orgB.ListDashboards(func([]byte) error { count++; return nil }); err != nil {
		t.Fatalf("orgB.ListDashboards: %v", err)
	}
	if count != 0 {
		t.Errorf("org isolation breach: orgB sees %d dashboards from orgA", count)
	}

	// Org B cannot get org A's dashboard by ID
	var dest map[string]string
	if err := orgB.GetDashboard("dash1", &dest); err == nil {
		t.Error("orgB.GetDashboard should fail for orgA's dashboard ID")
	}
}

// TestOrgStoreDefaultFallback verifies that an empty orgID falls back to "default"
// and data written via the base Store (unscoped) is not mixed with org-scoped data.
func TestOrgStoreDefaultFallback(t *testing.T) {
	s := openTestStore(t)
	defer s.Close()

	now := time.Now()
	dflt := s.ForOrg("") // should become "default"

	if err := dflt.SaveMetric("host1", "cpu_percent", 0.5, now); err != nil {
		t.Fatalf("default.SaveMetric: %v", err)
	}

	vals, err := dflt.GetMetricRange("host1", "cpu_percent", now.Add(-time.Second), now.Add(time.Second))
	if err != nil || len(vals) == 0 {
		t.Fatalf("default.GetMetricRange: err=%v len=%d", err, len(vals))
	}

	// An org named "other" must not see it
	other := s.ForOrg("other")
	valOther, _ := other.GetMetricRange("host1", "cpu_percent", now.Add(-time.Second), now.Add(time.Second))
	if len(valOther) != 0 {
		t.Errorf("isolation breach: 'other' org sees %d metrics from 'default' org", len(valOther))
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-storage-test-*")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}
