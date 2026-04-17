package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
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
			nanos, parseErr := strconv.ParseInt(nanoStr, 10, 64)
			if parseErr != nil {
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

// --- NotificationChannel storage ---
// Key schema: nc:{id}

// SaveNotificationChannel persists a notification channel
func (s *Store) SaveNotificationChannel(id string, data interface{}) error {
	return s.set(fmt.Sprintf("nc:%s", id), data, 0)
}

// GetNotificationChannel retrieves a notification channel
func (s *Store) GetNotificationChannel(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("nc:%s", id), dest)
}

// DeleteNotificationChannel removes a notification channel
func (s *Store) DeleteNotificationChannel(id string) error {
	return s.delete(fmt.Sprintf("nc:%s", id))
}

// ListNotificationChannels returns all notification channels
func (s *Store) ListNotificationChannels(dest func(val []byte) error) error {
	return s.listByPrefix("nc:", func(_, val []byte) error {
		return dest(val)
	})
}

// --- SLO storage ---
// Key schema: slo:{id}

func (s *Store) SaveSLO(id string, data interface{}) error {
	return s.set(fmt.Sprintf("slo:%s", id), data, 0)
}

func (s *Store) GetSLO(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("slo:%s", id), dest)
}

func (s *Store) DeleteSLO(id string) error {
	return s.delete(fmt.Sprintf("slo:%s", id))
}

func (s *Store) ListSLOs(dest func(val []byte) error) error {
	return s.listByPrefix("slo:", func(_, val []byte) error {
		return dest(val)
	})
}

// --- Org storage ---
// Key schema: org:{id}

// SaveOrg persists an org
func (s *Store) SaveOrg(id string, data interface{}) error {
	return s.set(fmt.Sprintf("org:%s", id), data, 0)
}

// GetOrg retrieves an org
func (s *Store) GetOrg(id string, dest interface{}) error {
	return s.get(fmt.Sprintf("org:%s", id), dest)
}

// DeleteOrg removes an org
func (s *Store) DeleteOrg(id string) error {
	return s.delete(fmt.Sprintf("org:%s", id))
}

// ListOrgs returns all orgs
func (s *Store) ListOrgs(dest func(val []byte) error) error {
	return s.listByPrefix("org:", func(_, val []byte) error {
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

// --- Log storage ---
// Key: l:{service}:{20-digit-zero-padded-unix-ns}

// sanitizeKeySegment removes Badger key namespace separators from user-supplied input.
// Prevents key injection attacks where a crafted service name or trace ID could
// escape its intended key prefix (e.g. "l::" or "sp:" embedded in a service name).
func sanitizeKeySegment(s string) string {
	return strings.NewReplacer(":", "_", "/", "_", "\\", "_").Replace(s)
}

// SaveLog persists a structured log entry
func (s *Store) SaveLog(service string, entry interface{}, ts time.Time) error {
	prefix := fmt.Sprintf("l:%s:", sanitizeKeySegment(service))
	key := tsKey(prefix, ts)
	return s.set(key, entry, LogsTTL)
}

// QueryLogs returns log entries for a service in the given time range (newest first, up to limit).
// When service is empty, all log namespaces are scanned using the common "l:" prefix.
func (s *Store) QueryLogs(service string, from, to time.Time, limit int) ([]json.RawMessage, error) {
	var prefix, startKey, endKey string
	if service == "" {
		// Scan all services under the "l:" namespace
		prefix = "l:"
		startKey = tsKey("l::", from) // "l::" sorts before any "l:{service}:"
		endKey = "l:~"               // "~" (0x7E) is the highest printable ASCII, terminates scan
	} else {
		svc := sanitizeKeySegment(service)
		prefix = fmt.Sprintf("l:%s:", svc)
		startKey = tsKey(prefix, from)
		endKey = tsKey(prefix, to)
	}

	var results []json.RawMessage
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			if !strings.HasPrefix(key, prefix) {
				break
			}
			if service != "" && (key < startKey || key >= endKey) {
				if key >= endKey {
					break
				}
				continue
			}
			err := item.Value(func(val []byte) error {
				cp := make([]byte, len(val))
				copy(cp, val)
				results = append(results, json.RawMessage(cp))
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
	// Reverse so newest entries are first
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
	return results, err
}

// QueryAllLogs returns logs across all services in time range
func (s *Store) QueryAllLogs(from, to time.Time, limit int) ([]json.RawMessage, error) {
	return s.QueryLogs("", from, to, limit)
}

// --- Span/Trace storage ---
// Key: sp:{trace_id}:{span_id}

// SaveSpan persists a trace span
func (s *Store) SaveSpan(span interface{}, traceID, spanID string) error {
	key := fmt.Sprintf("sp:%s:%s", sanitizeKeySegment(traceID), sanitizeKeySegment(spanID))
	return s.set(key, span, LogsTTL) // reuse 30d TTL
}

// QuerySpansByTrace returns all spans for a trace ID
func (s *Store) QuerySpansByTrace(traceID string) ([]json.RawMessage, error) {
	prefix := fmt.Sprintf("sp:%s:", sanitizeKeySegment(traceID))
	var results []json.RawMessage
	err := s.listByPrefix(prefix, func(_, val []byte) error {
		cp := make([]byte, len(val))
		copy(cp, val)
		results = append(results, json.RawMessage(cp))
		return nil
	})
	return results, err
}

// TraceHeader is a lightweight trace summary for list views.
type TraceHeader struct {
	TraceID     string  `json:"traceId"`
	RootService string  `json:"rootService"`
	RootOp      string  `json:"rootOp"`
	StartTimeMs int64   `json:"startTimeMs"`
	DurationMs  float64 `json:"durationMs"`
	SpanCount   int     `json:"spanCount"`
	HasError    bool    `json:"hasError"`
}

// QueryTraceList returns lightweight trace summaries, newest first, up to limit.
// It scans the "sp:" prefix and groups spans by traceID.
func (s *Store) QueryTraceList(service string, limit int) ([]TraceHeader, error) {
	type spanMin struct {
		TraceID   string  `json:"trace_id"`
		SpanID    string  `json:"span_id"`
		ParentID  string  `json:"parent_id"`
		Service   string  `json:"service"`
		Operation string  `json:"name"`
		StartMs   int64   `json:"start_time_ms"`
		DurationMs float64 `json:"duration_ms"`
		Status    string  `json:"status"`
	}

	byTrace := make(map[string][]spanMin)
	err := s.listByPrefix("sp:", func(_, val []byte) error {
		var sp spanMin
		if err := json.Unmarshal(val, &sp); err == nil {
			byTrace[sp.TraceID] = append(byTrace[sp.TraceID], sp)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var headers []TraceHeader
	for tid, spans := range byTrace {
		if service != "" {
			found := false
			for _, sp := range spans {
				if strings.EqualFold(sp.Service, service) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Find root span (no parent or earliest start)
		var root spanMin
		var minStart int64 = 1<<62
		hasError := false
		for _, sp := range spans {
			if sp.StartMs < minStart {
				minStart = sp.StartMs
				root = sp
			}
			if sp.Status == "error" || sp.Status == "ERROR" {
				hasError = true
			}
		}
		var totalDur float64
		for _, sp := range spans {
			if sp.StartMs+int64(sp.DurationMs) > minStart+int64(totalDur) {
				totalDur = float64(sp.StartMs+int64(sp.DurationMs)) - float64(minStart)
			}
		}
		headers = append(headers, TraceHeader{
			TraceID:     tid,
			RootService: root.Service,
			RootOp:      root.Operation,
			StartTimeMs: minStart,
			DurationMs:  totalDur,
			SpanCount:   len(spans),
			HasError:    hasError,
		})
	}

	// Sort newest first
	for i := 0; i < len(headers)-1; i++ {
		for j := i + 1; j < len(headers); j++ {
			if headers[j].StartTimeMs > headers[i].StartTimeMs {
				headers[i], headers[j] = headers[j], headers[i]
			}
		}
	}
	if limit > 0 && len(headers) > limit {
		headers = headers[:limit]
	}
	return headers, nil
}
