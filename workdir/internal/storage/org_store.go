package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// OrgStore wraps Store with an organisation-scoped key prefix.
// All tenant resources (metrics, KPIs, alerts, dashboards, datasources,
// notification channels, SLOs, logs, spans) are stored under the namespace
//
//	o:{orgID}:{original_key}
//
// This provides hard data isolation between organisations at the storage layer:
// a query for org "acme" can never return data belonging to org "beta" because
// Badger's prefix scan is bounded to the orgID namespace.
//
// Global resources (users, orgs) are managed via the base Store directly.
type OrgStore struct {
	*Store
	orgID string
}

// ForOrg returns an OrgStore scoped to orgID. Empty orgID falls back to "default"
// so legacy code that doesn't supply an org still works correctly.
func (s *Store) ForOrg(orgID string) *OrgStore {
	if orgID == "" {
		orgID = "default"
	}
	return &OrgStore{Store: s, orgID: orgID}
}

// ok returns the org-scoped version of key.
func (o *OrgStore) ok(key string) string {
	return fmt.Sprintf("o:%s:%s", o.orgID, key)
}

// op returns the org-scoped version of prefix.
func (o *OrgStore) op(prefix string) string {
	return fmt.Sprintf("o:%s:%s", o.orgID, prefix)
}

// --- Metrics ---

func (o *OrgStore) SaveMetric(host, name string, value float64, ts time.Time) error {
	prefix := o.op(fmt.Sprintf("m:%s:%s:", host, name))
	key := tsKey(prefix, ts)
	return o.set(key, value, MetricsTTL)
}

func (o *OrgStore) GetMetricRange(host, name string, from, to time.Time) ([]TimeValue, error) {
	prefix := o.op(fmt.Sprintf("m:%s:%s:", host, name))
	return o.rangeQuery(prefix, from, to, MetricsTTL)
}

// --- KPIs ---

func (o *OrgStore) SaveKPI(host, name string, value float64, ts time.Time) error {
	prefix := o.op(fmt.Sprintf("k:%s:%s:", host, name))
	key := tsKey(prefix, ts)
	return o.set(key, value, KPIsTTL)
}

func (o *OrgStore) GetKPIRange(host, name string, from, to time.Time) ([]TimeValue, error) {
	prefix := o.op(fmt.Sprintf("k:%s:%s:", host, name))
	return o.rangeQuery(prefix, from, to, KPIsTTL)
}

// --- Alerts ---

func (o *OrgStore) SaveAlert(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("a:%s", id)), data, AlertsTTL)
}

func (o *OrgStore) GetAlert(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("a:%s", id)), dest)
}

func (o *OrgStore) DeleteAlert(id string) error {
	return o.delete(o.ok(fmt.Sprintf("a:%s", id)))
}

func (o *OrgStore) ListAlerts(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("a:"), func(_, val []byte) error {
		return dest(val)
	})
}

// --- Dashboards ---

func (o *OrgStore) SaveDashboard(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("d:%s", id)), data, 0)
}

func (o *OrgStore) GetDashboard(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("d:%s", id)), dest)
}

func (o *OrgStore) DeleteDashboard(id string) error {
	return o.delete(o.ok(fmt.Sprintf("d:%s", id)))
}

func (o *OrgStore) ListDashboards(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("d:"), func(_, val []byte) error {
		return dest(val)
	})
}

// --- DataSources ---

func (o *OrgStore) SaveDataSource(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("ds:%s", id)), data, 0)
}

func (o *OrgStore) GetDataSource(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("ds:%s", id)), dest)
}

func (o *OrgStore) DeleteDataSource(id string) error {
	return o.delete(o.ok(fmt.Sprintf("ds:%s", id)))
}

func (o *OrgStore) ListDataSources(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("ds:"), func(_, val []byte) error {
		return dest(val)
	})
}

// --- Notification Channels ---

func (o *OrgStore) SaveNotificationChannel(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("nc:%s", id)), data, 0)
}

func (o *OrgStore) GetNotificationChannel(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("nc:%s", id)), dest)
}

func (o *OrgStore) DeleteNotificationChannel(id string) error {
	return o.delete(o.ok(fmt.Sprintf("nc:%s", id)))
}

func (o *OrgStore) ListNotificationChannels(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("nc:"), func(_, val []byte) error {
		return dest(val)
	})
}

// --- SLOs ---

func (o *OrgStore) SaveSLO(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("slo:%s", id)), data, 0)
}

func (o *OrgStore) GetSLO(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("slo:%s", id)), dest)
}

func (o *OrgStore) DeleteSLO(id string) error {
	return o.delete(o.ok(fmt.Sprintf("slo:%s", id)))
}

func (o *OrgStore) ListSLOs(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("slo:"), func(_, val []byte) error {
		return dest(val)
	})
}

// --- API Keys ---
// Key schema: o:{orgID}:ak:{key_id}
// The "ak:" namespace is separate from other resources so ListAPIKeys can
// enumerate only key records without touching metrics or dashboards.

func (o *OrgStore) SaveAPIKey(id string, data interface{}) error {
	return o.set(o.ok(fmt.Sprintf("ak:%s", id)), data, 0)
}

func (o *OrgStore) GetAPIKey(id string, dest interface{}) error {
	return o.get(o.ok(fmt.Sprintf("ak:%s", id)), dest)
}

func (o *OrgStore) DeleteAPIKey(id string) error {
	return o.delete(o.ok(fmt.Sprintf("ak:%s", id)))
}

func (o *OrgStore) ListAPIKeys(dest func(val []byte) error) error {
	return o.listByPrefix(o.op("ak:"), func(_, val []byte) error {
		return dest(val)
	})
}

// LookupAPIKeyByPrefix scans all API keys in the org and returns the first one
// whose Prefix field matches keyPrefix. Used during auth to avoid a full scan
// when the prefix is included in the Authorization header.
func (o *OrgStore) LookupAPIKeyByPrefix(keyPrefix string, dest interface{}) error {
	return o.listByPrefix(o.op("ak:"), func(_, val []byte) error {
		// Unmarshal just enough to check prefix
		var partial struct {
			Prefix string `json:"prefix"`
		}
		if err := jsonUnmarshal(val, &partial); err != nil {
			return nil // skip corrupt records
		}
		if partial.Prefix == keyPrefix {
			return jsonUnmarshal(val, dest)
		}
		return nil
	})
}

// jsonUnmarshal is a local alias so we don't need to import encoding/json twice.
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// --- Quota enforcement ---

// countByPrefix counts the number of records under an org-scoped prefix.
func (o *OrgStore) countByPrefix(prefix string) int {
	n := 0
	_ = o.listByPrefix(o.op(prefix), func(_, _ []byte) error {
		n++
		return nil
	})
	return n
}

// CheckDashboardQuota returns an error if the org has reached its dashboard limit.
func (o *OrgStore) CheckDashboardQuota(max int) error {
	if max <= 0 {
		return nil
	}
	if o.countByPrefix("d:") >= max {
		return fmt.Errorf("quota exceeded: org allows at most %d dashboards", max)
	}
	return nil
}

// CheckDataSourceQuota returns an error if the org has reached its data source limit.
func (o *OrgStore) CheckDataSourceQuota(max int) error {
	if max <= 0 {
		return nil
	}
	if o.countByPrefix("ds:") >= max {
		return fmt.Errorf("quota exceeded: org allows at most %d data sources", max)
	}
	return nil
}

// CheckAPIKeyQuota returns an error if the org has reached its API key limit.
func (o *OrgStore) CheckAPIKeyQuota(max int) error {
	if max <= 0 {
		return nil
	}
	if o.countByPrefix("ak:") >= max {
		return fmt.Errorf("quota exceeded: org allows at most %d API keys", max)
	}
	return nil
}

// CheckAlertRuleQuota returns an error if the org has reached its alert rule limit.
func (o *OrgStore) CheckAlertRuleQuota(max int) error {
	if max <= 0 {
		return nil
	}
	if o.countByPrefix("ar:") >= max {
		return fmt.Errorf("quota exceeded: org allows at most %d alert rules", max)
	}
	return nil
}

// CheckSLOQuota returns an error if the org has reached its SLO limit.
func (o *OrgStore) CheckSLOQuota(max int) error {
	if max <= 0 {
		return nil
	}
	if o.countByPrefix("slo:") >= max {
		return fmt.Errorf("quota exceeded: org allows at most %d SLOs", max)
	}
	return nil
}

// --- Logs ---

func (o *OrgStore) SaveLog(service string, entry interface{}, ts time.Time) error {
	prefix := o.op(fmt.Sprintf("l:%s:", sanitizeKeySegment(service)))
	key := tsKey(prefix, ts)
	return o.set(key, entry, LogsTTL)
}

func (o *OrgStore) QueryLogs(service string, from, to time.Time, limit int) ([]json.RawMessage, error) {
	// Delegate to base Store QueryLogs but with org-scoped prefix.
	// We re-implement the prefix logic here to keep isolation tight.
	var prefix, startKey, endKey string
	orgBase := o.op("l:")
	if service == "" {
		prefix = orgBase
		startKey = tsKey(orgBase+":", from)
		endKey = o.op("l:~")
	} else {
		svc := sanitizeKeySegment(service)
		prefix = o.op(fmt.Sprintf("l:%s:", svc))
		startKey = tsKey(prefix, from)
		endKey = tsKey(prefix, to)
	}
	return o.queryLogsRaw(prefix, startKey, endKey, service != "", from, to, limit)
}

func (o *OrgStore) QueryAllLogs(from, to time.Time, limit int) ([]json.RawMessage, error) {
	return o.QueryLogs("", from, to, limit)
}

// --- Spans/Traces ---

func (o *OrgStore) SaveSpan(span interface{}, traceID, spanID string) error {
	key := o.ok(fmt.Sprintf("sp:%s:%s", sanitizeKeySegment(traceID), sanitizeKeySegment(spanID)))
	return o.set(key, span, LogsTTL)
}

func (o *OrgStore) QuerySpansByTrace(traceID string) ([]json.RawMessage, error) {
	prefix := o.op(fmt.Sprintf("sp:%s:", sanitizeKeySegment(traceID)))
	var results []json.RawMessage
	err := o.listByPrefix(prefix, func(_, val []byte) error {
		cp := make([]byte, len(val))
		copy(cp, val)
		results = append(results, json.RawMessage(cp))
		return nil
	})
	return results, err
}
