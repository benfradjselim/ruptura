package storage

import (
	"testing"
	"time"
)

func TestAudit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	entry := AuditEntry{
		Username: "user1",
		Action:   "create",
		Resource: "dashboard",
	}
	if err := store.AppendAuditEntry(entry); err != nil {
		t.Fatal(err)
	}

	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	logs, err := store.QueryAuditLog("user1", from, to, 10)
	if err != nil || len(logs) != 1 {
		t.Errorf("QueryAuditLog failed: %v, %v", logs, err)
	}
}
