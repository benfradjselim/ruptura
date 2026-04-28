package storage

// Retention + downsampling engine
//
// Tier layout:
//   raw   m:{host}:{metric}:{ts_ns}    TTL 7 days    MetricsTTL
//   5m    r5:{host}:{metric}:{ts_ns}   TTL 35 days   Rollup5mTTL
//   1h    r1h:{host}:{metric}:{ts_ns}  TTL 400 days  Rollup1hTTL
//
// A background goroutine calls Compact() periodically.
// Compact scans raw points older than CompactRawAfter and writes averages
// into the 5-min bucket, then does the same for 5-min → 1h.

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

const (
	Rollup5mTTL  = 35 * 24 * time.Hour
	Rollup1hTTL  = 400 * 24 * time.Hour

	CompactRawAfter = 2 * time.Hour   // compact raw data older than 2h into 5m buckets
	Compact5mAfter  = 12 * time.Hour  // compact 5m data older than 12h into 1h buckets
)

// GetMetricRangeTiered queries the best-fit tier for the requested window.
// <6h → raw, 6h–7d → 5m rollups, >7d → 1h rollups
func (s *Store) GetMetricRangeTiered(host, name string, from, to time.Time) ([]TimeValue, error) {
	span := to.Sub(from)
	switch {
	case span <= 6*time.Hour:
		return s.GetMetricRange(host, name, from, to)
	case span <= 7*24*time.Hour:
		return s.getRollup("r5:", host, name, from, to)
	default:
		return s.getRollup("r1h:", host, name, from, to)
	}
}

// GetKPIRangeTiered mirrors GetMetricRangeTiered for KPI series.
func (s *Store) GetKPIRangeTiered(host, name string, from, to time.Time) ([]TimeValue, error) {
	span := to.Sub(from)
	switch {
	case span <= 6*time.Hour:
		return s.GetKPIRange(host, name, from, to)
	case span <= 7*24*time.Hour:
		return s.getRollup("kr5:", host, name, from, to)
	default:
		return s.getRollup("kr1h:", host, name, from, to)
	}
}

func (s *Store) getRollup(tierPrefix, host, name string, from, to time.Time) ([]TimeValue, error) {
	prefix := fmt.Sprintf("%s%s:%s:", tierPrefix, host, name)
	return s.rangeQuery(prefix, from, to, 0)
}

// Compact runs one pass of raw→5m and 5m→1h downsampling.
// Safe to call concurrently — each bucket is written atomically.
func (s *Store) Compact() {
	now := time.Now()
	cutRaw := now.Add(-CompactRawAfter)
	cut5m  := now.Add(-Compact5mAfter)

	// raw → 5m (metrics)
	if err := s.compactTier("m:", "r5:", 5*time.Minute, cutRaw, Rollup5mTTL); err != nil {
		logger.Default.Error("compact raw->5m metrics", "err", err)
	}
	// raw → 5m (KPIs)
	if err := s.compactTier("k:", "kr5:", 5*time.Minute, cutRaw, Rollup5mTTL); err != nil {
		logger.Default.Error("compact raw->5m kpis", "err", err)
	}
	// 5m → 1h (metrics)
	if err := s.compactTier("r5:", "r1h:", time.Hour, cut5m, Rollup1hTTL); err != nil {
		logger.Default.Error("compact 5m->1h metrics", "err", err)
	}
	// 5m → 1h (KPIs)
	if err := s.compactTier("kr5:", "kr1h:", time.Hour, cut5m, Rollup1hTTL); err != nil {
		logger.Default.Error("compact 5m->1h kpis", "err", err)
	}
}

// compactTier reads all source keys older than cutoff, groups them into buckets
// of bucketSize, writes avg to destPrefix, and deletes source keys.
func (s *Store) compactTier(srcPrefix, destPrefix string, bucketSize time.Duration, cutoff time.Time, destTTL time.Duration) error {
	// Collect source key/value pairs older than cutoff
	type kv struct {
		key string
		ts  time.Time
		val float64
	}
	var points []kv

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 500
		it := txn.NewIterator(opts)
		defer it.Close()

		pfx := []byte(srcPrefix)
		for it.Seek(pfx); it.ValidForPrefix(pfx); it.Next() {
			item := it.Item()
			keyBytes := item.KeyCopy(nil)
			keyStr := string(keyBytes)

			// Extract timestamp from last 20 chars
			if len(keyStr) < len(srcPrefix)+20 {
				continue
			}
			nanoStr := keyStr[len(keyStr)-20:]
			var nanos int64
			if _, err := fmt.Sscanf(nanoStr, "%d", &nanos); err != nil {
				continue
			}
			ts := time.Unix(0, nanos)
			if !ts.Before(cutoff) {
				continue // too recent, skip
			}

			var v float64
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &v)
			}); err != nil {
				continue
			}
			points = append(points, kv{key: keyStr, ts: ts, val: v})
		}
		return nil
	})
	if err != nil || len(points) == 0 {
		return err
	}

	// Group by (prefix_without_ts + bucket_floor)
	type bucketKey struct {
		seriesPrefix string // e.g. "m:host:cpu:"
		floor        time.Time
	}
	buckets := map[bucketKey][]float64{}
	keysByBucket := map[bucketKey][]string{}

	for _, p := range points {
		// series prefix = everything except last 20 char timestamp
		seriesPrefix := p.key[:len(p.key)-20]
		floor := p.ts.Truncate(bucketSize)
		bk := bucketKey{seriesPrefix: seriesPrefix, floor: floor}
		buckets[bk] = append(buckets[bk], p.val)
		keysByBucket[bk] = append(keysByBucket[bk], p.key)
	}

	// Write rollups and delete source keys
	for bk, vals := range buckets {
		avg := mean(vals)

		// Build dest key: replace src prefix with destPrefix
		// seriesPrefix is e.g. "m:host:cpu:" — strip srcPrefix, prepend destPrefix
		innerSeries := bk.seriesPrefix[len(srcPrefix):]
		destKey := fmt.Sprintf("%s%s%020d", destPrefix, innerSeries, bk.floor.UnixNano())

		// Write rollup
		if err := s.set(destKey, avg, destTTL); err != nil {
			logger.Default.Error("write rollup", "key", destKey, "err", err)
			continue
		}

		// Delete source keys
		if err := s.db.Update(func(txn *badger.Txn) error {
			for _, k := range keysByBucket[bk] {
				if err := txn.Delete([]byte(k)); err != nil && err != badger.ErrKeyNotFound {
					return err
				}
			}
			return nil
		}); err != nil {
			logger.Default.Error("delete source keys", "key", destKey, "err", err)
		}
	}
	return nil
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return math.NaN()
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// RetentionStats returns counts of keys per tier for observability.
func (s *Store) RetentionStats() map[string]int64 {
	prefixes := []string{"m:", "k:", "r5:", "kr5:", "r1h:", "kr1h:"}
	stats := make(map[string]int64, len(prefixes))
	for _, pfx := range prefixes {
		var count int64
		_ = s.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			it := txn.NewIterator(opts)
			defer it.Close()
			p := []byte(pfx)
			for it.Seek(p); it.ValidForPrefix(p); it.Next() {
				count++
			}
			return nil
		})
		stats[pfx] = count
	}
	return stats
}
