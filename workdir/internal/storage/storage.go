package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
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
	db            *badger.DB
	snapshotsMu   sync.RWMutex
	snapshots     map[string]models.KPISnapshot // host → latest snapshot (in-memory)
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
	return &Store{db: db, snapshots: make(map[string]models.KPISnapshot)}, nil
}

// StoreSnapshot saves the latest KPISnapshot indexed by host name AND WorkloadRef.Key().
// Both keys are stored so both /rupture/{host} and /rupture/{ns}/{workload} resolve correctly.
func (s *Store) StoreSnapshot(snap models.KPISnapshot) {
	s.snapshotsMu.Lock()
	if snap.Host != "" {
		s.snapshots[snap.Host] = snap
	}
	if key := snap.Workload.Key(); key != "" && key != snap.Host {
		s.snapshots[key] = snap
	}
	s.snapshotsMu.Unlock()
}

// LatestSnapshot returns the most recent KPISnapshot for a host name or WorkloadRef.Key().
func (s *Store) LatestSnapshot(key string) (models.KPISnapshot, bool) {
	s.snapshotsMu.RLock()
	snap, ok := s.snapshots[key]
	s.snapshotsMu.RUnlock()
	return snap, ok
}

// AllSnapshots returns copies of all stored KPISnapshots.
func (s *Store) AllSnapshots() []models.KPISnapshot {
	s.snapshotsMu.RLock()
	result := make([]models.KPISnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		result = append(result, snap)
	}
	s.snapshotsMu.RUnlock()
	return result
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
// Key schema: m:{host}:{metric}:{ts}
func tsKey(prefix string, ts time.Time) string {
	return fmt.Sprintf("%s%020d", prefix, ts.UnixNano())
}

// PutMetric stores a single metric value
func (s *Store) PutMetric(host, metric string, ts time.Time, value float64) error {
	prefix := fmt.Sprintf("m:%s:%s:", host, metric)
	key := tsKey(prefix, ts)
	return s.set(key, value, MetricsTTL)
}

// GetMetric retrieves a single metric value
func (s *Store) GetMetric(host, metric string, ts time.Time) (float64, error) {
	prefix := fmt.Sprintf("m:%s:%s:", host, metric)
	key := tsKey(prefix, ts)
	var v float64
	err := s.get(key, &v)
	return v, err
}

// ListMetrics retrieves metric values within [from, to]
func (s *Store) ListMetrics(host, metric string, from, to time.Time) ([]MetricSample, error) {
	prefix := fmt.Sprintf("m:%s:%s:", host, metric)
	return s.rangeQueryMetrics(prefix, from, to)
}

// SaveMetric is a wrapper for backward compatibility
func (s *Store) SaveMetric(host, name string, value float64, ts time.Time) error {
	return s.PutMetric(host, name, ts, value)
}

// MetricSample is a timestamp-value pair
type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// TimeValue is an alias for MetricSample for backward compatibility with retention engine
type TimeValue MetricSample

// GetMetricRange is a wrapper for backward compatibility
func (s *Store) GetMetricRange(host, metric string, from, to time.Time) ([]TimeValue, error) {
	samples, err := s.ListMetrics(host, metric, from, to)
	if err != nil {
		return nil, err
	}
	res := make([]TimeValue, len(samples))
	for i, s := range samples {
		res[i] = TimeValue(s)
	}
	return res, nil
}

// GetKPIRange is a wrapper for backward compatibility
func (s *Store) GetKPIRange(host, name string, from, to time.Time) ([]TimeValue, error) {
	samples, err := s.ListKPI(name, host, from, to)
	if err != nil {
		return nil, err
	}
	res := make([]TimeValue, len(samples))
	for i, s := range samples {
		res[i] = TimeValue{Timestamp: s.Timestamp, Value: s.Value}
	}
	return res, nil
}

// rangeQuery is a generic Seek-based range scan for backward compatibility
func (s *Store) rangeQuery(prefix string, from, to time.Time, _ time.Duration) ([]TimeValue, error) {
	// This is a bit of a hack, but should work for compatibility
	// We can use the existing ListMetrics/ListKPI logic
	// ... Actually, rangeQuery is called by getRollup.
	// I'll re-implement rangeQuery with the new schema.
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
			if string(k) > string(endKey) || !hasPrefix(k, pfxBytes) {
				break
			}
			keyStr := string(k)
			nanoStr := keyStr[len(prefix):]
			nanos, _ := strconv.ParseInt(nanoStr, 10, 64)
			ts := time.Unix(0, nanos)
			
			var v float64
			item.Value(func(val []byte) error {
				return json.Unmarshal(val, &v)
			})
			results = append(results, TimeValue{Timestamp: ts, Value: v})
		}
		return nil
	})
	return results, err
}

// rangeQueryMetrics is a Seek-based range scan for metric keys
func (s *Store) rangeQueryMetrics(prefix string, from, to time.Time) ([]MetricSample, error) {
	seekKey := []byte(tsKey(prefix, from))
	endKey := []byte(tsKey(prefix, to))
	pfxBytes := []byte(prefix)

	var results []MetricSample
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if string(k) > string(endKey) || !hasPrefix(k, pfxBytes) {
				break
			}
			
			keyStr := string(k)
			nanoStr := keyStr[len(prefix):]
			nanos, _ := strconv.ParseInt(nanoStr, 10, 64)
			ts := time.Unix(0, nanos)
			
			var v float64
			item.Value(func(val []byte) error {
				return json.Unmarshal(val, &v)
			})
			results = append(results, MetricSample{Timestamp: ts, Value: v})
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

// --- KPI storage ---
// Key schema: kpi:{name}:{host}:{ts}

func (s *Store) PutKPI(name, host string, ts time.Time, value float64) error {
	prefix := fmt.Sprintf("kpi:%s:%s:", name, host)
	key := tsKey(prefix, ts)
	return s.set(key, value, KPIsTTL)
}

func (s *Store) ListKPI(name, host string, from, to time.Time) ([]KPISample, error) {
	prefix := fmt.Sprintf("kpi:%s:%s:", name, host)
	return s.rangeQueryKPI(prefix, from, to)
}

type KPISample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

func (s *Store) rangeQueryKPI(prefix string, from, to time.Time) ([]KPISample, error) {
	seekKey := []byte(tsKey(prefix, from))
	endKey := []byte(tsKey(prefix, to))
	pfxBytes := []byte(prefix)

	var results []KPISample
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seekKey); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if string(k) > string(endKey) || !hasPrefix(k, pfxBytes) {
				break
			}
			keyStr := string(k)
			nanoStr := keyStr[len(prefix):]
			nanos, _ := strconv.ParseInt(nanoStr, 10, 64)
			ts := time.Unix(0, nanos)
			
			var v float64
			item.Value(func(val []byte) error {
				return json.Unmarshal(val, &v)
			})
			results = append(results, KPISample{Timestamp: ts, Value: v})
		}
		return nil
	})
	return results, err
}

// --- Rupture events ---
// Key schema: r:{id}

func (s *Store) PutRupture(id string, payload []byte) error {
	return s.set(fmt.Sprintf("r:%s", id), payload, MetricsTTL)
}

func (s *Store) GetRupture(id string) ([]byte, error) {
	var payload []byte
	err := s.get(fmt.Sprintf("r:%s", id), &payload)
	return payload, err
}

// Key schema: r:{host}:history:{ts}

func (s *Store) PutRuptureHistory(host string, ts time.Time, payload []byte) error {
	prefix := fmt.Sprintf("r:%s:history:", host)
	key := tsKey(prefix, ts)
	return s.set(key, payload, MetricsTTL)
}

func (s *Store) ListRuptureHistory(host string, from, to time.Time) ([][]byte, error) {
	prefix := fmt.Sprintf("r:%s:history:", host)
	seekKey := []byte(tsKey(prefix, from))
	endKey := []byte(tsKey(prefix, to))
	
	var results [][]byte
	err := s.listByPrefix(prefix, func(key, val []byte) error {
		if string(key) < string(seekKey) || string(key) > string(endKey) {
			return nil
		}
		results = append(results, val)
		return nil
	})
	return results, err
}

// --- Actions ---
// Key schema: ac:{id}

func (s *Store) PutAction(id string, payload []byte) error {
	return s.set(fmt.Sprintf("ac:%s", id), payload, MetricsTTL)
}

func (s *Store) GetAction(id string) ([]byte, error) {
	var payload []byte
	err := s.get(fmt.Sprintf("ac:%s", id), &payload)
	return payload, err
}

func (s *Store) ListActions() ([][]byte, error) {
	var results [][]byte
	err := s.listByPrefix("ac:", func(_, val []byte) error {
		results = append(results, val)
		return nil
	})
	return results, err
}

// --- Context entries ---
// Key schema: ctx:{id}

func (s *Store) PutContext(id string, payload []byte) error {
	return s.set(fmt.Sprintf("ctx:%s", id), payload, 0)
}

func (s *Store) GetContext(id string) ([]byte, error) {
	var payload []byte
	err := s.get(fmt.Sprintf("ctx:%s", id), &payload)
	return payload, err
}

func (s *Store) DeleteContext(id string) error {
	return s.delete(fmt.Sprintf("ctx:%s", id))
}

func (s *Store) ListContexts() ([][]byte, error) {
	var results [][]byte
	err := s.listByPrefix("ctx:", func(_, val []byte) error {
		results = append(results, val)
		return nil
	})
	return results, err
}

// --- Suppressions ---
// Key schema: sup:{id}

func (s *Store) PutSuppression(id string, payload []byte) error {
	return s.set(fmt.Sprintf("sup:%s", id), payload, 0)
}

func (s *Store) DeleteSuppression(id string) error {
	return s.delete(fmt.Sprintf("sup:%s", id))
}

func (s *Store) ListSuppressions() ([][]byte, error) {
	var results [][]byte
	err := s.listByPrefix("sup:", func(_, val []byte) error {
		results = append(results, val)
		return nil
	})
	return results, err
}

// --- JWT Token Revocation (blocklist) ---
// Key schema: rev:{jti}   — value is empty, TTL = remaining token lifetime.
// Auth middleware checks this list before accepting a JWT.

// RevokeToken adds a token JTI to the blocklist with the given TTL.
// TTL should equal the token's remaining lifetime so Badger GCs the entry automatically.
func (s *Store) RevokeToken(jti string, ttl time.Duration) error {
	if jti == "" || ttl <= 0 {
		return nil
	}
	return s.set(fmt.Sprintf("rev:%s", jti), struct{}{}, ttl)
}

// IsTokenRevoked returns true if the JTI is in the blocklist.
func (s *Store) IsTokenRevoked(jti string) bool {
	var dummy struct{}
	err := s.get(fmt.Sprintf("rev:%s", jti), &dummy)
	return err == nil // key exists → revoked
}

// GetStore returns the store itself, for StorageBackend compatibility
func (s *Store) GetStore() *Store { return s }

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
		prefix = "l:"
		startKey = tsKey("l::", from)
		endKey = "l:~"
	} else {
		svc := sanitizeKeySegment(service)
		prefix = fmt.Sprintf("l:%s:", svc)
		startKey = tsKey(prefix, from)
		endKey = tsKey(prefix, to)
	}
	return s.queryLogsRaw(prefix, startKey, endKey, service != "", from, to, limit)
}

// queryLogsRaw is the shared implementation used by both Store.QueryLogs and
// OrgStore.QueryLogs. Callers supply fully-formed prefix/start/end keys.
func (s *Store) queryLogsRaw(prefix, startKey, endKey string, filterTime bool, _, _ time.Time, limit int) ([]json.RawMessage, error) {
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
			if filterTime && (key < startKey || key >= endKey) {
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
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].StartTimeMs > headers[j].StartTimeMs
	})
	if limit > 0 && len(headers) > limit {
		headers = headers[:limit]
	}
	return headers, nil
}
