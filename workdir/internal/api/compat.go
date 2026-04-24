package api

// compat.go — Drop-in compatibility endpoints for Grafana Cloud (Loki),
// Datadog, and Elasticsearch/ELK.
//
// Endpoints:
//   Loki:          POST /loki/api/v1/push
//                  GET  /loki/api/v1/query_range
//                  GET  /loki/api/v1/labels
//                  GET  /loki/api/v1/label/{name}/values
//
//   Elasticsearch: POST /_bulk
//                  GET  /_cat/indices
//                  GET  /_search
//                  POST /{index}/_search
//                  GET  /               (cluster info handshake)
//
//   Datadog:       POST /api/v1/series  (metrics)
//                  POST /api/v2/logs    (logs)

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/gorilla/mux"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// ============================================================
// LOKI COMPATIBILITY
// ============================================================

const compatMaxBodyBytes = 32 << 20 // 32 MB

// LokiPushHandler handles POST /loki/api/v1/push
func (h *Handlers) LokiPushHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, compatMaxBodyBytes)
	var req models.LokiPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	count := 0
	for _, stream := range req.Streams {
		service := stream.Stream["service"]
		if service == "" {
			service = stream.Stream["app"]
		}
		if service == "" {
			service = stream.Stream["job"]
		}
		if service == "" {
			service = "unknown"
		}
		level := stream.Stream["level"]
		if level == "" {
			level = stream.Stream["severity"]
		}
		if level == "" {
			level = "info"
		}
		host := stream.Stream["host"]
		if host == "" {
			host = h.hostname
		}

		for _, val := range stream.Values {
			if len(val) < 2 {
				continue
			}
			ns, err := strconv.ParseInt(val[0], 10, 64)
			if err != nil {
				continue
			}
			ts := time.Unix(0, ns)
			entry := models.LogEntry{
				Timestamp: ts,
				Level:     level,
				Message:   val[1],
				Service:   service,
				Host:      host,
				Labels:    stream.Stream,
				Source:    "loki",
			}
			if err := h.store.SaveLog(service, entry, ts); err != nil {
				logger.Default.ErrorCtx(r.Context(), "loki push save error", "err", err)
			}
			count++
		}
	}

	logger.Default.InfoCtx(r.Context(), "loki push ingested", "count", count)
	w.WriteHeader(http.StatusNoContent) // Loki expects 204
}

// LokiQueryRangeHandler handles GET /loki/api/v1/query_range
func (h *Handlers) LokiQueryRangeHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	service := extractLabelFromSelector(q.Get("query"))

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()

	if s := q.Get("start"); s != "" {
		if ns, err := strconv.ParseInt(s, 10, 64); err == nil {
			from = time.Unix(0, ns)
		}
	}
	if s := q.Get("end"); s != "" {
		if ns, err := strconv.ParseInt(s, 10, 64); err == nil {
			to = time.Unix(0, ns)
		}
	}

	limit := 100
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	entries, err := h.store.QueryLogs(service, from, to, limit)
	if err != nil {
		logger.Default.ErrorCtx(r.Context(), "loki query error", "err", err)
		entries = nil
	}

	var values [][]string
	for _, raw := range entries {
		var entry models.LogEntry
		if json.Unmarshal(raw, &entry) == nil {
			values = append(values, []string{
				strconv.FormatInt(entry.Timestamp.UnixNano(), 10),
				entry.Message,
			})
		}
	}

	stream := map[string]interface{}{
		"stream": map[string]string{"service": service},
		"values": values,
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "streams",
			"result":     []interface{}{stream},
		},
	})
}

// LokiLabelsHandler handles GET /loki/api/v1/labels
func (h *Handlers) LokiLabelsHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   []string{"service", "host", "level", "app", "job"},
	})
}

// LokiLabelValuesHandler handles GET /loki/api/v1/label/{name}/values
func (h *Handlers) LokiLabelValuesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	labelName := vars["name"]

	var values []string
	switch labelName {
	case "service", "app", "job":
		values = append(values, h.hostname)
	case "level", "severity":
		values = []string{"debug", "info", "warn", "error"}
	case "host":
		values = append(values, h.hostname)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   values,
	})
}

// extractLabelFromSelector pulls the service/app value from a LogQL selector
func extractLabelFromSelector(selector string) string {
	for _, key := range []string{"service", "app", "job"} {
		prefix := key + `="`
		if idx := strings.Index(selector, prefix); idx >= 0 {
			rest := selector[idx+len(prefix):]
			if end := strings.Index(rest, `"`); end >= 0 {
				return rest[:end]
			}
		}
	}
	return ""
}

// ============================================================
// ELASTICSEARCH COMPATIBILITY
// ============================================================

// ESBulkHandler handles POST /_bulk
func (h *Handlers) ESBulkHandler(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r, 32<<20)
	if err != nil {
		http.Error(w, "read error: "+err.Error(), http.StatusBadRequest)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	items := []map[string]interface{}{}
	ingested := 0

	for i := 0; i+1 < len(lines); i += 2 {
		var action models.ESBulkAction
		if err := json.Unmarshal([]byte(lines[i]), &action); err != nil {
			continue
		}

		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i+1]), &doc); err != nil {
			continue
		}

		entry := esDocToLogEntry(doc, h.hostname)
		service := entry.Service
		if service == "" {
			if action.Index != nil && action.Index.Index != "" {
				service = action.Index.Index
			} else {
				service = "unknown"
			}
			entry.Service = service
		}

		if err := h.store.SaveLog(service, entry, entry.Timestamp); err != nil {
			logger.Default.ErrorCtx(r.Context(), "es bulk save error", "err", err)
		}
		ingested++

		items = append(items, map[string]interface{}{
			"index": map[string]interface{}{
				"_index": service,
				"_id":    fmt.Sprintf("%d", entry.Timestamp.UnixNano()),
				"result": "created",
				"status": 201,
			},
		})
	}

	logger.Default.InfoCtx(r.Context(), "es bulk ingested", "count", ingested)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"took":   1,
		"errors": false,
		"items":  items,
	})
}

// ESCatIndicesHandler handles GET /_cat/indices
func (h *Handlers) ESCatIndicesHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	indices := []map[string]interface{}{
		{"index": "ohe-metrics", "docs.count": "0", "store.size": "0", "status": "green"},
		{"index": "ohe-logs", "docs.count": "0", "store.size": "0", "status": "green"},
		{"index": "ohe-alerts", "docs.count": "0", "store.size": "0", "status": "green"},
		{"index": "ohe-spans", "docs.count": "0", "store.size": "0", "status": "green"},
	}

	if format == "json" || r.Header.Get("Accept") == "application/json" {
		respondJSON(w, http.StatusOK, indices)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "green open ohe-metrics 1 0")
	fmt.Fprintln(w, "green open ohe-logs    1 0")
	fmt.Fprintln(w, "green open ohe-alerts  1 0")
	fmt.Fprintln(w, "green open ohe-spans   1 0")
}

// ESSearchHandler handles GET|POST /_search and POST /{index}/_search
func (h *Handlers) ESSearchHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	index := vars["index"]
	if index == "" {
		index = "ohe-logs"
	}

	var queryBody map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&queryBody)

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()

	entries, _ := h.store.QueryLogs("", from, to, 100)

	hits := make([]map[string]interface{}, 0, len(entries))
	for i, raw := range entries {
		var entry models.LogEntry
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		hits = append(hits, map[string]interface{}{
			"_index":  index,
			"_id":     strconv.Itoa(i),
			"_score":  1.0,
			"_source": entry,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"took":      1,
		"timed_out": false,
		"_shards":   map[string]interface{}{"total": 1, "successful": 1, "failed": 0},
		"hits": map[string]interface{}{
			"total":     map[string]interface{}{"value": len(hits), "relation": "eq"},
			"max_score": 1.0,
			"hits":      hits,
		},
	})
}

// ESInfoHandler handles GET / — cluster info handshake for ES clients
func (h *Handlers) ESInfoHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":         "ohe-node",
		"cluster_name": "ohe",
		"cluster_uuid": "ohe-" + h.hostname,
		"version": map[string]interface{}{
			"number":                             "8.12.0",
			"build_flavor":                       "default",
			"minimum_wire_compatibility_version": "7.17.0",
			"minimum_index_compatibility_version": "7.0.0",
		},
		"tagline": "OHE — Observability Holistic Engine",
	})
}

// ============================================================
// DATADOG COMPATIBILITY
// ============================================================

// DDMetricsHandler handles POST /api/v1/series (Datadog agent metrics)
func (h *Handlers) DDMetricsHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, compatMaxBodyBytes)
	var payload struct {
		Series []struct {
			Metric string      `json:"metric"`
			Points [][]float64 `json:"points"`
			Host   string      `json:"host"`
			Tags   []string    `json:"tags"`
		} `json:"series"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	for _, s := range payload.Series {
		host := s.Host
		if host == "" {
			host = h.hostname
		}
		for _, point := range s.Points {
			if len(point) < 2 {
				continue
			}
			ts := time.Unix(int64(point[0]), 0)
			if err := h.store.SaveMetric(host, sanitizeMetricName(s.Metric), point[1], ts); err != nil {
				logger.Default.ErrorCtx(r.Context(), "dd metrics save error", "err", err)
			}
		}
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{"status": "ok"})
}

// DDLogsHandler handles POST /api/v2/logs (Datadog log pipeline)
func (h *Handlers) DDLogsHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, compatMaxBodyBytes)
	var logs []struct {
		Message string `json:"message"`
		Service string `json:"service"`
		Host    string `json:"hostname"`
		Status  string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&logs); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	for _, l := range logs {
		host := l.Host
		if host == "" {
			host = h.hostname
		}
		service := l.Service
		if service == "" {
			service = host
		}
		entry := models.LogEntry{
			Timestamp: time.Now(),
			Level:     normalizeDDStatus(l.Status),
			Message:   l.Message,
			Service:   service,
			Host:      host,
			Source:    "datadog",
		}
		if err := h.store.SaveLog(service, entry, entry.Timestamp); err != nil {
			logger.Default.ErrorCtx(r.Context(), "dd logs save error", "err", err)
		}
	}

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, `{"status":"ok"}`)
}

// ============================================================
// TOPOLOGY & APM
// ============================================================

// TopologyHandler handles GET /api/v1/topology
func (h *Handlers) TopologyHandler(w http.ResponseWriter, r *http.Request) {
	graph := h.topology.Graph()
	respondSuccess(w, graph)
}

// LogQueryHandler handles GET /api/v1/logs
func (h *Handlers) LogQueryHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	service := q.Get("service")

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()

	if s := q.Get("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = t
		}
	}
	if s := q.Get("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = t
		}
	}

	limit := 200
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}

	entries, err := h.store.QueryLogs(service, from, to, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}

	var results []models.LogEntry
	for _, raw := range entries {
		var entry models.LogEntry
		if json.Unmarshal(raw, &entry) == nil {
			results = append(results, entry)
		}
	}

	respondSuccess(w, results)
}

// TraceQueryHandler handles GET /api/v1/traces/{traceID}
func (h *Handlers) TraceQueryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	traceID := vars["traceID"]

	spans, err := h.store.QuerySpansByTrace(traceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}

	var results []models.Span
	for _, raw := range spans {
		var span models.Span
		if json.Unmarshal(raw, &span) == nil {
			results = append(results, span)
		}
	}

	respondSuccess(w, results)
}

// --- helpers ---

func esDocToLogEntry(doc map[string]interface{}, defaultHost string) models.LogEntry {
	entry := models.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Source:    "elasticsearch",
		Host:      defaultHost,
	}

	if v, ok := doc["message"].(string); ok {
		entry.Message = v
	} else if v, ok := doc["@message"].(string); ok {
		entry.Message = v
	}

	if v, ok := doc["@timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			entry.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			entry.Timestamp = t
		}
	}

	for _, k := range []string{"level", "severity", "log.level", "log_level"} {
		if v, ok := doc[k].(string); ok && v != "" {
			entry.Level = strings.ToLower(v)
			break
		}
	}

	for _, k := range []string{"service", "service.name", "fields.service"} {
		if v, ok := doc[k].(string); ok && v != "" {
			entry.Service = v
			break
		}
	}

	for _, k := range []string{"host", "hostname", "host.name"} {
		if v, ok := doc[k].(string); ok && v != "" {
			entry.Host = v
			break
		}
	}

	return entry
}

func normalizeDDStatus(status string) string {
	switch strings.ToLower(status) {
	case "error", "err", "critical", "emerg", "alert":
		return "error"
	case "warn", "warning":
		return "warn"
	case "debug", "trace":
		return "debug"
	default:
		return "info"
	}
}

func sanitizeMetricName(name string) string {
	return strings.NewReplacer(".", "_", "-", "_", " ", "_").Replace(name)
}

// readBody reads the full request body up to maxBytes.
// w must be the live ResponseWriter so MaxBytesReader can send 413 if exceeded.
func readBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
