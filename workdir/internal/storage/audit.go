package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v3"
)

// AuditEntry records a single write operation for compliance and security tracing.
// Stored under global key audit:{zero-padded-unix-ns}:{id} (not org-prefixed so
// admins can see cross-org activity — access is guarded by RequireRole(admin)).
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`    // e.g. "create", "update", "delete", "login", "logout"
	Resource  string    `json:"resource"`  // e.g. "dashboard", "datasource", "api_key", "user"
	ResourceID string   `json:"resource_id,omitempty"`
	Details   string    `json:"details,omitempty"` // short human-readable description
	IPAddress string    `json:"ip,omitempty"`
}

// AppendAuditEntry writes an immutable audit entry.
// Entries are keyed by timestamp so they are naturally ordered for range scans.
// TTL is 2 years — audit logs must be retained for compliance.
func (s *Store) AppendAuditEntry(entry AuditEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	key := fmt.Sprintf("audit:%020d:%s", entry.Timestamp.UnixNano(), entry.ID)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	const auditTTL = 2 * 365 * 24 * time.Hour
	return s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), data).WithTTL(auditTTL)
		return txn.SetEntry(e)
	})
}

// QueryAuditLog returns audit entries in [from, to], newest first, up to limit.
// Filter by username (empty = all).
func (s *Store) QueryAuditLog(username string, from, to time.Time, limit int) ([]AuditEntry, error) {
	prefix := "audit:"
	seekKey := fmt.Sprintf("audit:%020d", from.UnixNano())
	endKey := fmt.Sprintf("audit:%020d", to.UnixNano())

	var results []AuditEntry
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(seekKey)); it.Valid(); it.Next() {
			item := it.Item()
			k := string(item.Key())
			if !hasPrefix(item.Key(), []byte(prefix)) || k > endKey {
				break
			}
			err := item.Value(func(val []byte) error {
				var e AuditEntry
				if err := json.Unmarshal(val, &e); err != nil {
					return nil
				}
				if username != "" && e.Username != username {
					return nil
				}
				results = append(results, e)
				return nil
			})
			if err != nil {
				return err
			}
			if limit > 0 && len(results) >= limit {
				break
			}
		}
		return nil
	})
	// Reverse: newest first
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, err
}
