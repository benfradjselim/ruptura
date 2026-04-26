package ingest

import (
	"context"
	"google.golang.org/grpc"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benfradjselim/kairo-core/internal/pipeline/metrics"
	"github.com/benfradjselim/kairo-core/pkg/logger"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

type LogSink interface {
	IngestLine(service string, line []byte, ts time.Time)
}

type SpanSink interface {
	IngestSpan(span models.Span) error
}

type Ingestor interface {
	StartHTTP(addr string) error
	StartGRPC(addr string) error
	StartDogStatsD(addr string) error
	Stop(ctx context.Context) error
}

type Engine struct {
	pipeline metrics.MetricPipeline
	logs     LogSink
	spans    SpanSink

	activeSeries sync.Map
	seriesCount  int32

	httpServer *http.Server
	udpConn    *net.UDPConn
	grpcServer  *grpc.Server
	grpcSamples chan *GRPCMetricPoint
}

func New(pipeline metrics.MetricPipeline, logs LogSink, spans SpanSink) *Engine {
	return &Engine{
		pipeline: pipeline,
		logs:     logs,
		spans:    spans,
		grpcSamples: make(chan *GRPCMetricPoint, 1024),
	}
}

func (e *Engine) StartHTTP(addr string) error {
	mux := http.NewServeMux()
	RegisterHandlers(mux, e)
	e.httpServer = &http.Server{Addr: addr, Handler: mux}
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
		var name, host string = "", "unknown"
		for _, lbl := range ts.Labels {
			if lbl.Name == "__name__" {
				name = lbl.Value
			} else if lbl.Name == "host" {
				host = lbl.Value
			}
		}
		if name == "" {
			continue
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
		fmt.Println("otlp metrics decode failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, rm := range req.ResourceMetrics {
		host := rm.Resource.GetAttr("host.name")
		if host == "" {
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
	if e.logs != nil {
		for _, rl := range req.ResourceLogs {
			service := rl.Resource.GetAttr("service.name")
			if service == "" {
				service = "unknown"
			}
			for _, sl := range rl.ScopeLogs {
				for _, lr := range sl.LogRecords {
					nanos, _ := strconv.ParseInt(lr.TimeUnixNano, 10, 64)
					e.logs.IngestLine(service, []byte(lr.Body.GetString()), time.Unix(0, nanos))
				}
			}
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
	if e.spans != nil {
		for _, rs := range req.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					s := models.Span{
						TraceID:   span.TraceID,
						SpanID:    span.SpanID,
						Operation: span.Name,
					}
					if span.Status.Code == 2 {
						s.Status = "error"
					} else if span.Status.Code == 1 {
						s.Status = "ok"
					} else {
						s.Status = "unset"
					}
					e.spans.IngestSpan(s)
				}
			}
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
