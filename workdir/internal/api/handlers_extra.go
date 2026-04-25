package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/kairo-core/internal/alerter"
	"github.com/benfradjselim/kairo-core/internal/storage"
	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/utils"
	"github.com/gorilla/mux"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// ---------------------------------------------------------------------------
// Prometheus exposition endpoint — GET /metrics
// Exposes all KPIs and raw metrics in Prometheus text format 0.0.4.
// This makes OHE a first-class Prometheus scrape target so Grafana, Thanos,
// VictoriaMetrics and any Prometheus-compatible tool can consume OHE data.
// ---------------------------------------------------------------------------

// PrometheusMetricsHandler GET /metrics
// No auth required by default (scrape targets typically don't use auth).
func (h *Handlers) PrometheusMetricsHandler(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	var buf strings.Builder

	// --- OHE KPIs ---
	snap, ok := h.analyzer.Snapshot(host)
	if ok {
		writePrometheusKPI(&buf, "ohe_stress", snap.Stress.Value, snap.Stress.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_fatigue", snap.Fatigue.Value, snap.Fatigue.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_mood", snap.Mood.Value, snap.Mood.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_pressure", snap.Pressure.Value, snap.Pressure.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_humidity", snap.Humidity.Value, snap.Humidity.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_contagion", snap.Contagion.Value, snap.Contagion.State, host, snap.Timestamp)
		// ETF composed KPIs
		writePrometheusKPI(&buf, "ohe_resilience", snap.Resilience.Value, snap.Resilience.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_entropy", snap.Entropy.Value, snap.Entropy.State, host, snap.Timestamp)
		writePrometheusKPI(&buf, "ohe_velocity", snap.Velocity.Value, snap.Velocity.State, host, snap.Timestamp)
		// HealthScore is already 0-100, normalise back to [0,1] for Prometheus convention
		writePrometheusKPI(&buf, "ohe_health_score", snap.HealthScore.Value/100.0, snap.HealthScore.State, host, snap.Timestamp)
	}

	// --- Raw system metrics ---
	rawMetrics := []string{
		"cpu_percent", "memory_percent", "disk_percent",
		"net_rx_bps", "net_tx_bps", "load_avg_1", "load_avg_5",
		"load_avg_15", "uptime_seconds", "processes",
		"error_rate", "timeout_rate", "request_rate",
	}
	now := time.Now()
	for _, name := range rawMetrics {
		if val, ok := h.processor.GetNormalized(host, name); ok {
			promName := "ohe_metric_" + strings.ReplaceAll(name, ".", "_")
			fmt.Fprintf(&buf, "# TYPE %s gauge\n", promName)
			fmt.Fprintf(&buf, "%s{host=%q} %g %d\n", promName, host, val, now.UnixMilli())
		}
	}

	// --- Active alerts count by severity ---
	alerts := h.alerter.GetActive()
	severityCounts := map[string]int{"info": 0, "warning": 0, "critical": 0, "emergency": 0}
	for _, al := range alerts {
		if al.Host == host || host == h.hostname {
			severityCounts[al.Severity]++
		}
	}
	for sev, cnt := range severityCounts {
		fmt.Fprintf(&buf, "# TYPE ohe_alerts_active gauge\n")
		fmt.Fprintf(&buf, "ohe_alerts_active{host=%q,severity=%q} %d %d\n", host, sev, cnt, now.UnixMilli())
	}

	// --- OHE engine uptime ---
	fmt.Fprintf(&buf, "# TYPE ohe_uptime_seconds counter\n")
	fmt.Fprintf(&buf, "ohe_uptime_seconds{host=%q} %g %d\n", host, time.Since(h.startTime).Seconds(), now.UnixMilli())

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, buf.String())
}

func writePrometheusKPI(buf *strings.Builder, name string, val float64, state, host string, ts time.Time) {
	fmt.Fprintf(buf, "# HELP %s OHE holistic KPI — state: %s\n", name, state)
	fmt.Fprintf(buf, "# TYPE %s gauge\n", name)
	fmt.Fprintf(buf, "%s{host=%q,state=%q} %g %d\n", name, host, state, val, ts.UnixMilli())
}

// ---------------------------------------------------------------------------
// Fleet Overview — GET /api/v1/fleet
// Returns aggregated health summary for ALL known hosts.
// ---------------------------------------------------------------------------

// FleetHandler GET /api/v1/fleet
func (h *Handlers) FleetHandler(w http.ResponseWriter, r *http.Request) {
	hosts := h.analyzer.AllHosts()
	sort.Strings(hosts)

	allAlerts := h.alerter.GetActive()
	// count alerts per host
	alertsByHost := make(map[string]int, len(hosts))
	for _, al := range allAlerts {
		alertsByHost[al.Host]++
	}

	summaries := make([]models.HostSummary, 0, len(hosts))
	healthy, degraded, critical := 0, 0, 0

	for _, hst := range hosts {
		snap, ok := h.analyzer.Snapshot(hst)
		if !ok {
			continue
		}
		hs := models.HostSummary{
			Host:         hst,
			HealthScore:  snap.HealthScore.Value,
			Stress:       snap.Stress.Value,
			Fatigue:      snap.Fatigue.Value,
			Contagion:    snap.Contagion.Value,
			ActiveAlerts: alertsByHost[hst],
			LastSeen:     snap.Timestamp,
		}

		switch snap.HealthScore.State {
		case "excellent", "good":
			hs.State = "healthy"
			healthy++
		case "fair":
			hs.State = "degraded"
			degraded++
		default:
			hs.State = "critical"
			critical++
		}
		summaries = append(summaries, hs)
	}

	respondSuccess(w, models.FleetStatus{
		Timestamp:     time.Now(),
		TotalHosts:    len(hosts),
		HealthyHosts:  healthy,
		DegradedHosts: degraded,
		CriticalHosts: critical,
		Hosts:         summaries,
	})
}

// ---------------------------------------------------------------------------
// Notification Channels — webhook/Slack/PagerDuty delivery for alerts
// ---------------------------------------------------------------------------

// NotificationChannelListHandler GET /api/v1/notifications
func (h *Handlers) NotificationChannelListHandler(w http.ResponseWriter, r *http.Request) {
	var channels []*models.NotificationChannel
	err := h.store.ListNotificationChannels(func(val []byte) error {
		var ch models.NotificationChannel
		if err := json.Unmarshal(val, &ch); err != nil {
			return nil
		}
		channels = append(channels, &ch)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, channels)
}

// NotificationChannelCreateHandler POST /api/v1/notifications
func (h *Handlers) NotificationChannelCreateHandler(w http.ResponseWriter, r *http.Request) {
	var ch models.NotificationChannel
	if err := decodeBody(r, &ch); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if ch.URL != "" {
		if err := validateDataSourceURL(ch.URL); err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_URL", "webhook URL is not allowed: "+err.Error())
			return
		}
	}
	ch.ID = utils.GenerateID(8)
	ch.Enabled = true
	if err := h.store.SaveNotificationChannel(ch.ID, ch); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": ch, "timestamp": time.Now().UTC(),
	})
}

// NotificationChannelGetHandler GET /api/v1/notifications/{id}
func (h *Handlers) NotificationChannelGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ch models.NotificationChannel
	if err := h.store.GetNotificationChannel(id, &ch); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "notification channel not found")
		return
	}
	respondSuccess(w, ch)
}

// NotificationChannelUpdateHandler PUT /api/v1/notifications/{id}
func (h *Handlers) NotificationChannelUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ch models.NotificationChannel
	if err := decodeBody(r, &ch); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if ch.URL != "" {
		if err := validateDataSourceURL(ch.URL); err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_URL", "webhook URL is not allowed: "+err.Error())
			return
		}
	}
	ch.ID = id
	if err := h.store.SaveNotificationChannel(id, ch); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, ch)
}

// NotificationChannelDeleteHandler DELETE /api/v1/notifications/{id}
func (h *Handlers) NotificationChannelDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.store.DeleteNotificationChannel(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "notification channel not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// NotificationChannelTestHandler POST /api/v1/notifications/{id}/test
// Sends a test payload to the configured webhook URL.
func (h *Handlers) NotificationChannelTestHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ch models.NotificationChannel
	if err := h.store.GetNotificationChannel(id, &ch); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "notification channel not found")
		return
	}
	if err := validateDataSourceURL(ch.URL); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_URL", "webhook URL is not allowed")
		return
	}

	payload := map[string]interface{}{
		"ohe_event": "test",
		"message":   "OHE notification channel test — if you receive this, routing is working",
		"timestamp": time.Now().UTC(),
	}
	if err := fireWebhook(ch, payload); err != nil {
		respondSuccess(w, map[string]interface{}{"status": "error", "message": "webhook delivery failed"})
		return
	}
	respondSuccess(w, map[string]string{"status": "ok"})
}

// fireWebhook sends a JSON payload to a notification channel.
// Supports generic webhook, Slack (incoming webhook), and PagerDuty event API v2.
func fireWebhook(ch models.NotificationChannel, payload interface{}) error {
	var body []byte
	var err error

	switch ch.Type {
	case "slack":
		// Slack incoming webhook expects {"text": "..."} or blocks
		msg := fmt.Sprintf("[OHE Alert] %v", payload)
		slackPayload := map[string]interface{}{
			"text": msg,
		}
		body, err = json.Marshal(slackPayload)
	case "pagerduty":
		// PagerDuty Events API v2
		alertData, _ := json.Marshal(payload)
		pdPayload := map[string]interface{}{
			"routing_key":  ch.Headers["routing_key"],
			"event_action": "trigger",
			"payload": map[string]interface{}{
				"summary":   fmt.Sprintf("[OHE] %v", payload),
				"source":    "ohe",
				"severity":  "critical",
				"custom_details": json.RawMessage(alertData),
			},
		}
		body, err = json.Marshal(pdPayload)
	default:
		body, err = json.Marshal(payload)
	}
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, ch.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OHE/4.0.0")
	for k, v := range ch.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver webhook: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// DispatchAlertToChannels fans an alert out to all enabled notification channels.
// Called by the alerter goroutine in the orchestrator.
func (h *Handlers) DispatchAlertToChannels(alert models.Alert) {
	var channels []*models.NotificationChannel
	_ = h.store.ListNotificationChannels(func(val []byte) error {
		var ch models.NotificationChannel
		if json.Unmarshal(val, &ch) == nil && ch.Enabled {
			channels = append(channels, &ch)
		}
		return nil
	})

	payload := map[string]interface{}{
		"ohe_event":   "alert",
		"alert_id":    alert.ID,
		"name":        alert.Name,
		"description": alert.Description,
		"severity":    alert.Severity,
		"host":        alert.Host,
		"metric":      alert.Metric,
		"value":       alert.Value,
		"threshold":   alert.Threshold,
		"timestamp":   alert.CreatedAt,
	}

	for _, ch := range channels {
		// Severity filter
		if len(ch.Severities) > 0 {
			matched := false
			for _, s := range ch.Severities {
				if s == alert.Severity {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		go func(c models.NotificationChannel) {
			if err := fireWebhook(c, payload); err != nil {
				logger.Default.Error("notify channel error", "name", c.Name, "id", c.ID, "err", err)
			}
		}(*ch)
	}
}

// ---------------------------------------------------------------------------
// KPI Multi-host — GET /api/v1/kpis/multi
// Returns KPI snapshots for multiple hosts in one call.
// ---------------------------------------------------------------------------

// KPIMultiHandler GET /api/v1/kpis/multi?host=h1&host=h2
func (h *Handlers) KPIMultiHandler(w http.ResponseWriter, r *http.Request) {
	hostList := r.URL.Query()["host"]
	if len(hostList) == 0 {
		hostList = h.analyzer.AllHosts()
	}

	result := make(map[string]interface{}, len(hostList))
	for _, hst := range hostList {
		if snap, ok := h.analyzer.Snapshot(hst); ok {
			result[hst] = snap
		}
	}
	respondSuccess(w, result)
}

// ---------------------------------------------------------------------------
// Alert Rules CRUD — GET/POST/DELETE /api/v1/alert-rules
// Allows operators to create custom alert rules at runtime without restart.
// ---------------------------------------------------------------------------

// AlertRuleListHandler GET /api/v1/alert-rules
func (h *Handlers) AlertRuleListHandler(w http.ResponseWriter, r *http.Request) {
	respondSuccess(w, h.alerter.GetRules())
}

// AlertRuleCreateHandler POST /api/v1/alert-rules
func (h *Handlers) AlertRuleCreateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.orgStore(r).CheckAlertRuleQuota(h.orgQuota(r).MaxAlertRules); err != nil {
		respondError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", err.Error())
		return
	}
	var rule struct {
		Name      string  `json:"name"`
		Metric    string  `json:"metric"`
		Threshold float64 `json:"threshold"`
		Severity  string  `json:"severity"`
		Message   string  `json:"message"`
	}
	if err := decodeBody(r, &rule); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if rule.Name == "" || rule.Metric == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "name and metric are required")
		return
	}
	h.alerter.AddRule(alerter.Rule{
		Name:      rule.Name,
		Metric:    rule.Metric,
		Threshold: rule.Threshold,
		Severity:  rule.Severity,
		Message:   rule.Message,
	})
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": rule, "timestamp": time.Now().UTC(),
	})
}

// AlertRuleUpdateHandler PUT /api/v1/alert-rules/{name}
func (h *Handlers) AlertRuleUpdateHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	var rule struct {
		Name      string  `json:"name"`
		Metric    string  `json:"metric"`
		Threshold float64 `json:"threshold"`
		Severity  string  `json:"severity"`
		Message   string  `json:"message"`
	}
	if err := decodeBody(r, &rule); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if rule.Name == "" {
		rule.Name = name
	}
	if !h.alerter.UpdateRule(name, alerter.Rule{
		Name:      rule.Name,
		Metric:    rule.Metric,
		Threshold: rule.Threshold,
		Severity:  rule.Severity,
		Message:   rule.Message,
	}) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "rule not found: "+name)
		return
	}
	respondSuccess(w, rule)
}

// AlertRuleDeleteHandler DELETE /api/v1/alert-rules/{name}
func (h *Handlers) AlertRuleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if !h.alerter.DeleteRule(name) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "rule not found: "+name)
		return
	}
	respondSuccess(w, map[string]string{"deleted": name})
}

// TraceSearchHandler GET /api/v1/traces — search traces by service and time range
func (h *Handlers) TraceSearchHandler(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	headers, err := h.store.QueryTraceList(service, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if headers == nil {
		headers = []storage.TraceHeader{}
	}
	respondSuccess(w, map[string]interface{}{"traces": headers, "total": len(headers)})
}

// LogStreamHandler GET /api/v1/logs/stream — Server-Sent Events tail of recent logs
func (h *Handlers) LogStreamHandler(w http.ResponseWriter, r *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	service := r.URL.Query().Get("service")
	severity := r.URL.Query().Get("severity")
	q := r.URL.Query().Get("q")

	// Send a heartbeat immediately so the client knows the stream is live
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastSeen int64
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			from := time.Unix(0, lastSeen)
			if lastSeen == 0 {
				from = time.Now().Add(-10 * time.Second)
			}
			to := time.Now()
			raw, err := h.store.QueryLogs(service, from, to, 100)
			if err != nil || len(raw) == 0 {
				// send keepalive comment
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
				continue
			}
			for _, entry := range raw {
				// Apply severity and query filters client-side (fast enough at tail rates)
				s := string(entry)
				if severity != "" && !strings.Contains(s, `"`+severity+`"`) {
					continue
				}
				if q != "" && !strings.Contains(strings.ToLower(s), strings.ToLower(q)) {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", s)
			}
			lastSeen = to.UnixNano()
			flusher.Flush()
		}
	}
}
