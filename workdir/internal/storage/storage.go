package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// TTL constants aligned with spec
const (
	MetricsTTL     = 7 * 24 * time.Hour  // 7 days
	LogsTTL        = 30 * 24 * time.Hour // 30 days
	PredictionsTTL = 30 * 24 * time.Hour
	AlertsTTL      = 90 * 24 * time.Hour
	KPIsTTL        = 7 * 24 * time.Hour
)

// Store wraps Badger with typed key helpers
type Store struct {
	db *badger.DB
}

// Open opens (or creates) the Badger database at path
func Open(path string) (*Store, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil). // silence Badger internal logs
		WithNumVersionsToKeep(1).
		WithCompactL0OnClose(true)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}
	return &Store{db: db}, nil
}

// Close shuts the database
func (s *Store) Close() error {
	return s.db.Close()
}

// RunGC runs value log garbage collection
func (s *Store) RunGC() error {
	return s.db.RunValueLogGC(0.5)
}

// --- Generic set/get helpers ---

func (s *Store) set(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), data)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return txn.SetEntry(entry)
	})
}

func (s *Store) get(key string, dest interface{}) error {
	return s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, dest)
		})
	})
}

func (s *Store) delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// listByPrefix returns all values under a given key prefix
func (s *Store) listByPrefix(prefix string, dest func(key, val []byte) error) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		pfx := []byte(prefix)
		for it.Seek(pfx); it.ValidForPrefix(pfx); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)
			err := item.Value(func(val []byte) error {
				return dest(key, val)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Metric storage ---
// Key schema: m:{host}:{metric_name}:{20-digit-zero-padded-unix-ns}
// Zero-padding makes timestamps lexicographically sortable, enabling O(log N) Seek.

func tsKey(prefix string, ts time.Time) string {
	return fmt.Sprintf("%s%020d", prefix, ts.UnixNano())
}

// SaveMetric stores a single metric value
func (s *Store) SaveMetric(host, name string, value float64, ts time.Time) error {
	prefix := fmt.Sprintf("m:%s:%s:", host, name)
	key := tsKey(prefix, ts)
	return s.set(key, value, MetricsTTL)
}

// GetMetricRange retrieves metric values within [from, to] using Badger Seek
func (s *Store) GetMetricRange(host, name string, from, to time.Time) ([]TimeValue, error) {
	prefix := fmt.Sprintf("m:%s:%s:", host, name)
	return s.rangeQuery(prefix, from, to, MetricsTTL)
}

// TimeValue is a timestamp-value pair
type TimeValue struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// --- KPI storage ---
// Key schema: k:{host}:{kpi_name}:{20-digit-zero-padded-unix-ns}

// SaveKPI stores a KPI value
func (s *Store) SaveKPI(host, name string, value float64, ts time.Time) error {
	prefix := fmt.Sprintf("k:%s:%s:", host, name)
	key := tsKey(prefix, ts)
	return s.set(key, value, KPIsTTL)
}

// GetKPIRange retrieves KPI values within [from, to] using Badger Seek
func (s *Store) GetKPIRange(host, name string, from, to time.Time) ([]TimeValue, error) {
	prefix := fmt.Sprintf("k:%s:%s:", host, name)
	return s.rangeQuery(prefix, from, to, KPIsTTL)
}

// rangeQuery is a generic Seek-based range scan for time-series keys
func (s *Store) rangeQuery(prefix string, from, to time.Time, _ time.Duration) ([]TimeValue, error) {
	seekKey := []byte(tsKey(prefix, from))
	endKey := []byte(tsKey(prefix, to))
	pfxBytes := []byte(prefix)

	var results []TimeValue
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			// Stop when we exceed the end key or leave the prefix
			if string(k) > string(endKey) || !hasPrefix(k, pfxBytes) {
				break
			}
			// Parse timestamp from last 20 chars of key
			keyStr := string(k)
			if len(keyStr) < len(prefix)+20 {
				continue
			}
			nanoStr := keyStr[len(prefix):]
			var nanos int64
			if _, err := fmt.Sscan(nanoStr, &nanos); err != nil {
				continue
			}
			ts := time.Unix(0, nanos)
			var v float64
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &v)
			})
			if err != nil {
				continue
			}
			results = append(results, TimeValue{Timestamp: ts, Value: v})
		}
		return nil
	})
	return results, err
}

func hasPrefix(key, prefix []byte) bool {
	if len(key) < len(prefix) {
		return false
	}
	for i, b := range prefix {
		if key[i] != b {
			return false
		}
	}
	return true
}

// --- Alert storage ---
// Key schema: a:{alert_id}

// SaveAlert persists an alert
func (s *Store) SaveAlert(id string, data interface{}) error {
	key := fmt.Sprintf("a:%s", id)
	return s.set(key, data, AlertsTTL)
}

// GetAlert retrieves an alert by ID
func (s *Store) GetAlert(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("a:%s", id), dest)
}

// DeleteAlert removes an alert
func (s *Store) DeleteAlert(id string) error {
	return s.delete(fmt.Sprintf("a:%s", id))
}

// ListAlerts returns all stored alerts
func (s *Store) ListAlerts(dest func(val []byte) error) error {
	return s.listByPrefix("a:", func(_, val []byte) error {
		return dest(val)
	})
}

// --- Dashboard storage ---
// Key schema: d:{dashboard_id}

// SaveDashboard persists a dashboard
func (s *Store) SaveDashboard(id string, data interface{}) error {
	key := fmt.Sprintf("d:%s", id)
	return s.set(key, data, 0) // no TTL for dashboards
}

// GetDashboard retrieves a dashboard by ID
func (s *Store) GetDashboard(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("d:%s", id), dest)
}

// DeleteDashboard removes a dashboard
func (s *Store) DeleteDashboard(id string) error {
	return s.delete(fmt.Sprintf("d:%s", id))
}

// ListDashboards returns all stored dashboards
func (s *Store) ListDashboards(dest func(val []byte) error) error {
	return s.listByPrefix("d:", func(_, val []byte) error {
		return dest(val)
	})
}

// --- User storage ---
// Key schema: u:{username}

// SaveUser persists a user
func (s *Store) SaveUser(username string, data interface{}) error {
	key := fmt.Sprintf("u:%s", username)
	return s.set(key, data, 0)
}

// GetUser retrieves a user by username
func (s *Store) GetUser(username string, dest interface{}) error {
	return s.get(fmt.Sprintf("u:%s", username), dest)
}

// DeleteUser removes a user
func (s *Store) DeleteUser(username string) error {
	return s.delete(fmt.Sprintf("u:%s", username))
}

// ListUsers returns all stored users
func (s *Store) ListUsers(dest func(val []byte) error) error {
	return s.listByPrefix("u:", func(_, val []byte) error {
		return dest(val)
	})
}

// --- DataSource storage ---

// SaveDataSource persists a data source
func (s *Store) SaveDataSource(id string, data interface{}) error {
	return s.set(fmt.Sprintf("ds:%s", id), data, 0)
}

// GetDataSource retrieves a data source
func (s *Store) GetDataSource(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("ds:%s", id), dest)
}

// DeleteDataSource removes a data source
func (s *Store) DeleteDataSource(id string) error {
	return s.delete(fmt.Sprintf("ds:%s", id))
}

// ListDataSources returns all data sources
func (s *Store) ListDataSources(dest func(val []byte) error) error {
	return s.listByPrefix("ds:", func(_, val []byte) error {
		return dest(val)
	})
}

// Healthy returns true if the database is responsive
func (s *Store) Healthy() bool {
	err := s.db.View(func(txn *badger.Txn) error {
		return nil
	})
	return err == nil
}
