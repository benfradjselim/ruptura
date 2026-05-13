package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	"github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
)

type LogSink interface {
	IngestLine(service string, line []byte, ts time.Time)
}

// LogStoreSink persists structured log entries to durable storage.
// storage.Store satisfies this interface via its SaveLog method.
type LogStoreSink interface {
	SaveLog(service string, entry interface{}, ts time.Time) error
}

type SpanSink interface {
	IngestSpan(span models.Span) error
}

// TraceRSink receives per-workload rupture indices derived from trace error rates.
type TraceRSink interface {
	SetTraceR(key string, r float64, ts time.Time)
}

type SentimentSink interface {
	UpdateSentiment(service string, positive, negative int)
}

type Ingestor interface {
	StartHTTP(addr string) error
	StartGRPC(addr string) error
	StartDogStatsD(addr string) error
	Stop(ctx context.Context) error
}

type Engine struct {
	pipeline    metrics.MetricPipeline
	logs        LogSink
	logStore    LogStoreSink
	spans       SpanSink
	sentiment   SentimentSink
	traceR      TraceRSink
	ingestHook  func(source string) // optional; called once per ingested item

	activeSeries sync.Map
	seriesCount  int32

	metricsCount int64
	logsCount    int64
	tracesCount  int64

	httpServer  *http.Server
	udpConn     *net.UDPConn
	grpcServer  *grpc.Server
	grpcSamples chan *GRPCMetricPoint
}

func New(pipeline metrics.MetricPipeline, logs LogSink, spans SpanSink, sentiment SentimentSink, traceR TraceRSink) *Engine {
	return &Engine{
		pipeline:    pipeline,
		logs:        logs,
		spans:       spans,
		sentiment:   sentiment,
		traceR:      traceR,
		grpcSamples: make(chan *GRPCMetricPoint, 1024),
	}
}

// SetLogStore wires a durable storage backend for OTLP log persistence.
func (e *Engine) SetLogStore(s LogStoreSink) { e.logStore = s }

// SetIngestHook registers a callback invoked once per ingested log or trace item.
// Used to forward counts to the telemetry registry for Prometheus export.
func (e *Engine) SetIngestHook(fn func(source string)) { e.ingestHook = fn }

// rateLimiter is a simple token-bucket middleware for the ingest HTTP server.
// Capacity and refill rate are configurable via RUPTURA_INGEST_RPS env variable.
type rateLimiter struct {
	tokens   float64
	capacity float64
	refillPS float64 // tokens added per second
	last     time.Time
	mu       sync.Mutex
}

func newRateLimiter() *rateLimiter {
	rps := 1000.0 // default: 1000 req/s
	if v := os.Getenv("RUPTURA_INGEST_RPS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			rps = n
		}
	}
	return &rateLimiter{tokens: rps, capacity: rps, refillPS: rps, last: time.Now()}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(rl.last).Seconds()
	rl.last = now
	rl.tokens = min(rl.capacity, rl.tokens+rl.refillPS*elapsed)
	if rl.tokens < 1 {
		return false
	}
	rl.tokens--
	return true
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func rateLimitMiddleware(rl *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (e *Engine) StartHTTP(addr string) error {
	mux := http.NewServeMux()
	RegisterHandlers(mux, e)
	rl := newRateLimiter()
	e.httpServer = &http.Server{Addr: addr, Handler: rateLimitMiddleware(rl, mux)}
	go func() {
		if err := e.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.Default.Error("HTTP ingest failed", "error", err)
		}
	}()
	return nil
}

func (e *Engine) StartDogStatsD(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	e.udpConn = conn
	go func() {
		buf := make([]byte, 65535)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				e.parseDogStatsDLine(line)
			}
		}
	}()
	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	if e.httpServer != nil {
		e.httpServer.Shutdown(ctx)
	}
	if e.udpConn != nil {
		e.udpConn.Close()
	}
	if e.grpcServer != nil {
		e.grpcServer.GracefulStop()
	}
	return nil
}

func RegisterHandlers(mux *http.ServeMux, e *Engine) {
	mux.HandleFunc("/api/v2/write", e.handleRemoteWrite)
	mux.HandleFunc("/otlp/v1/metrics", e.handleOTLPMetrics)
	mux.HandleFunc("/otlp/v1/logs", e.handleOTLPLogs)
	mux.HandleFunc("/otlp/v1/traces", e.handleOTLPTraces)
}

// extractWorkloadRef extracts a WorkloadRef from an OTLPResource by inspecting
// standard Kubernetes and OpenTelemetry semantic convention attributes.
func extractWorkloadRef(r models.OTLPResource) models.WorkloadRef {
	ns := r.GetAttr("k8s.namespace.name")
	node := models.FirstNonEmpty(r.GetAttr("k8s.node.name"), r.GetAttr("host.name"))
	name := models.FirstNonEmpty(
		r.GetAttr("k8s.deployment.name"),
		r.GetAttr("k8s.statefulset.name"),
		r.GetAttr("k8s.daemonset.name"),
		r.GetAttr("k8s.job.name"),
		r.GetAttr("service.name"),
		node, // final fallback: use node as identity (non-K8s)
	)
	kind := inferWorkloadKind(r)
	if ns == "" {
		ns = "default"
	}
	return models.WorkloadRef{Namespace: ns, Kind: kind, Name: name, Node: node}
}

// inferWorkloadKind returns the Kubernetes workload kind string from OTLP resource attributes.
func inferWorkloadKind(r models.OTLPResource) string {
	switch {
	case r.GetAttr("k8s.deployment.name") != "":
		return "Deployment"
	case r.GetAttr("k8s.statefulset.name") != "":
		return "StatefulSet"
	case r.GetAttr("k8s.daemonset.name") != "":
		return "DaemonSet"
	case r.GetAttr("k8s.job.name") != "":
		return "Job"
	default:
		return "host"
	}
}

func (e *Engine) handleRemoteWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Timeseries []struct {
			Labels  []struct{ Name, Value string }
			Samples []struct {
				Value     float64
				Timestamp int64
			}
		} `json:"timeseries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, ts := range req.Timeseries {
		var name string
		host := "unknown"
		workload := models.WorkloadRef{}
		for _, lbl := range ts.Labels {
			switch lbl.Name {
			case "__name__":
				name = lbl.Value
			case "host", "instance":
				host = lbl.Value
			case "namespace":
				workload.Namespace = lbl.Value
			case "deployment":
				workload.Name = lbl.Value
				workload.Kind = "Deployment"
			}
		}
		if name == "" {
			continue
		}
		if workload.IsEmpty() {
			workload = models.WorkloadRefFromHost(host)
		}

		if e.checkCardinality(host, name) {
			for _, s := range ts.Samples {
				e.pipeline.Ingest(host, name, s.Value, time.UnixMilli(s.Timestamp))
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (e *Engine) handleOTLPMetrics(w http.ResponseWriter, r *http.Request) {
	var req models.OTLPMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Default.Error("otlp metrics decode failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, rm := range req.ResourceMetrics {
		ref := extractWorkloadRef(rm.Resource)
		var host string
		if !ref.IsEmpty() {
			host = ref.Key() // prefer workload identity (namespace/kind/name) over node name
		} else if ref.Node != "" {
			host = ref.Node
		} else {
			host = "unknown"
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				var name string
				var value float64
				var ts time.Time
				if m.Gauge != nil {
					name = m.Name
					for _, dp := range m.Gauge.DataPoints {
						if dp.AsDouble != nil {
							value = *dp.AsDouble
						}
						nanos, _ := strconv.ParseInt(dp.TimeUnixNano, 10, 64)
						ts = time.Unix(0, nanos)
						if e.checkCardinality(host, name) {
							e.pipeline.Ingest(host, name, value, ts)
							atomic.AddInt64(&e.metricsCount, 1)
						}
					}
				} else if m.Sum != nil {
					name = m.Name
					for _, dp := range m.Sum.DataPoints {
						if dp.AsInt != nil {
							value = float64(*dp.AsInt)
						} else if dp.AsDouble != nil {
							value = *dp.AsDouble
						}
						nanos, _ := strconv.ParseInt(dp.TimeUnixNano, 10, 64)
						ts = time.Unix(0, nanos)
						if e.checkCardinality(host, name) {
							e.pipeline.Ingest(host, name, value, ts)
							atomic.AddInt64(&e.metricsCount, 1)
						}
					}
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (e *Engine) handleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	var req models.OTLPLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, rl := range req.ResourceLogs {
		ref := extractWorkloadRef(rl.Resource)
		service := rl.Resource.GetAttr("service.name")
		if service == "" {
			service = ref.Name
		}
		if service == "" {
			service = "unknown"
		}
		var pos, neg int
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				nanos, _ := strconv.ParseInt(lr.TimeUnixNano, 10, 64)
				ts := time.Unix(0, nanos)
				if ts.IsZero() {
					ts = time.Now()
				}
				body := lr.Body.GetString()

				if e.logs != nil {
					e.logs.IngestLine(service, []byte(body), ts)
				}

				if e.logStore != nil {
					entry := map[string]interface{}{
						"service":    service,
						"body":       body,
						"severity":   lr.SeverityText,
						"timestamp":  ts.UnixNano(),
						"workload":   ref.Key(),
						"namespace":  ref.Namespace,
						"kind":       ref.Kind,
						"workload_name": ref.Name,
					}
					_ = e.logStore.SaveLog(service, entry, ts)
				}

				lower := strings.ToLower(body)
				if strings.Contains(lower, "error") || strings.Contains(lower, "warn") {
					neg++
				} else {
					pos++
				}
				atomic.AddInt64(&e.logsCount, 1)
				if e.ingestHook != nil {
					e.ingestHook("logs")
				}
			}
		}
		if e.sentiment != nil && (pos > 0 || neg > 0) {
			e.sentiment.UpdateSentiment(service, pos, neg)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (e *Engine) handleOTLPTraces(w http.ResponseWriter, r *http.Request) {
	var req models.OTLPTraceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	now := time.Now()
	for _, rs := range req.ResourceSpans {
		ref := extractWorkloadRef(rs.Resource)
		var total, errors int
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				total++
				s := models.Span{
					TraceID:   span.TraceID,
					SpanID:    span.SpanID,
					Operation: span.Name,
				}
				if span.Status.Code == 2 {
					s.Status = "error"
					errors++
				} else if span.Status.Code == 1 {
					s.Status = "ok"
				} else {
					s.Status = "unset"
				}
				if e.spans != nil {
					e.spans.IngestSpan(s)
				}
				atomic.AddInt64(&e.tracesCount, 1)
				if e.ingestHook != nil {
					e.ingestHook("traces")
				}
			}
		}
		// Derive traceR from span error rate: 100% error rate → R≈5, 20% → R≈1.
		if e.traceR != nil && total > 0 {
			errRate := float64(errors) / float64(total)
			traceR := errRate * 5.0
			e.traceR.SetTraceR(ref.Key(), traceR, now)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (e *Engine) parseDogStatsDLine(line string) {
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return
	}
	
	head := strings.Split(parts[0], ":")
	if len(head) < 2 {
		return
	}
	name := head[0]
	value := 0.0
	fmt.Sscanf(head[1], "%f", &value)
	
	host := "unknown"
	for _, p := range parts {
		if strings.HasPrefix(p, "#") {
			tags := strings.Split(p[1:], ",")
			for _, tag := range tags {
				kv := strings.Split(tag, ":")
				if len(kv) == 2 && kv[0] == "host" {
					host = kv[1]
				}
			}
		}
	}
	
	if e.checkCardinality(host, name) {
		e.pipeline.Ingest(host, name, value, time.Now())
	}
}

func (e *Engine) SendDogStatsDPacket(data []byte) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		e.parseDogStatsDLine(line)
	}
}

func (e *Engine) checkCardinality(host, name string) bool {
	key := host + ":" + name
	if _, ok := e.activeSeries.Load(key); ok {
		return true
	}
	
	count := atomic.LoadInt32(&e.seriesCount)
	if count >= 50000 {
		logger.Default.Warn("Cardinality limit reached, rejecting series", "key", key)
		return false
	}
	
	e.activeSeries.Store(key, true)
	atomic.AddInt32(&e.seriesCount, 1)
	return true
}

// IngestCounts returns the total number of metrics, logs, and traces ingested since startup.
func (e *Engine) IngestCounts() (metrics, logs, traces int64) {
	return atomic.LoadInt64(&e.metricsCount), atomic.LoadInt64(&e.logsCount), atomic.LoadInt64(&e.tracesCount)
}
