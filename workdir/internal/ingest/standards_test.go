package ingest

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

const metricsExport = "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export"

func gaugeExport(value float64) *metricspb.MetricsData {
	return &metricspb.MetricsData{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			{Key: "k8s.namespace.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "shop"}}},
			{Key: "k8s.deployment.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "api"}}},
		}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{
			Name: "queue_depth",
			Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{DataPoints: []*metricspb.NumberDataPoint{{
				TimeUnixNano: uint64(time.Now().UnixNano()),
				Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: value},
			}}}},
		}}}},
	}}}
}

func TestOTLPHTTP_StandardPathsAndResponses(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	mux := http.NewServeMux()
	RegisterHandlers(mux, e)
	pb, _ := proto.Marshal(gaugeExport(7))
	cases := []struct {
		name, path, contentType string
		body                    []byte
		code                    int
		respType                string
	}{
		{"protobuf", "/v1/metrics", "application/x-protobuf", pb, 200, "application/x-protobuf"},
		{"json", "/v1/metrics", "application/json", []byte(`{"resourceMetrics":[]}`), 200, "application/json"},
		{"logs", "/v1/logs", "application/json", []byte(`{"resourceLogs":[]}`), 200, "application/json"},
		{"traces", "/v1/traces", "application/json", []byte(`{"resourceSpans":[]}`), 200, "application/json"},
		{"old path is gone", "/otlp/v1/metrics", "application/json", []byte(`{}`), 404, ""},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.contentType)
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.code || (tc.respType != "" && rec.Header().Get("Content-Type") != tc.respType) {
			t.Errorf("%s: code %d type %q", tc.name, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	if len(mp.ingested) != 1 || mp.ingested[0].name != "queue_depth" || mp.ingested[0].value != 7 {
		t.Errorf("ingested = %+v", mp.ingested)
	}
}

func dialOTLP(t *testing.T, addr string, creds credentials.TransportCredentials) *grpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestOTLPGRPC_ExportOverTLSWithGzip(t *testing.T) {
	dir := t.TempDir()
	ca := newPair(t, "ca", nil)
	certFile, keyFile := newPair(t, "ruptura", ca).files(t, dir, "server")
	config, err := ServerTLS(certFile, keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	e.SetTLS(config)
	addr := freeAddr(t)
	if err := e.StartOTLPGRPC(addr); err != nil {
		t.Fatal(err)
	}
	defer e.otlpGRPC.Stop()
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	conn := dialOTLP(t, addr, credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}))
	if err := conn.Invoke(context.Background(), metricsExport, gaugeExport(3), &emptypb.Empty{}, grpc.UseCompressor(gzip.Name)); err != nil {
		t.Fatal(err)
	}
	if len(mp.ingested) != 1 || mp.ingested[0].value != 3 {
		t.Errorf("ingested = %+v", mp.ingested)
	}
	for _, service := range []string{"logs.v1.LogsService", "trace.v1.TraceService"} {
		err := conn.Invoke(context.Background(), "/opentelemetry.proto.collector."+service+"/Export", &metricspb.MetricsData{}, &emptypb.Empty{})
		if err != nil {
			t.Errorf("%s: %v", service, err)
		}
	}
}

func TestOTLPGRPC_RateLimit(t *testing.T) {
	t.Setenv("RUPTURA_INGEST_RPS", "1")
	e := New(&mockPipeline{}, nil, nil, nil, nil)
	addr := freeAddr(t)
	if err := e.StartOTLPGRPC(addr); err != nil {
		t.Fatal(err)
	}
	defer e.otlpGRPC.Stop()
	conn := dialOTLP(t, addr, insecure.NewCredentials())
	var limited bool
	for i := 0; i < 20 && !limited; i++ {
		err := conn.Invoke(context.Background(), metricsExport, gaugeExport(1), &emptypb.Empty{})
		limited = status.Code(err) == codes.ResourceExhausted
	}
	if !limited {
		t.Error("expected RESOURCE_EXHAUSTED over the rate")
	}
}

func writeRequest(name string, value float64, ms int64) []byte {
	label := func(k, v string) []byte {
		b := protowire.AppendTag(nil, 1, protowire.BytesType)
		b = protowire.AppendString(b, k)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		return protowire.AppendString(b, v)
	}
	sample := protowire.AppendTag(nil, 1, protowire.Fixed64Type)
	sample = protowire.AppendFixed64(sample, mathFloat64bits(value))
	sample = protowire.AppendTag(sample, 2, protowire.VarintType)
	sample = protowire.AppendVarint(sample, uint64(ms))
	var series []byte
	for _, l := range [][2]string{{"__name__", name}, {"instance", "node-1"}} {
		series = protowire.AppendTag(series, 1, protowire.BytesType)
		series = protowire.AppendBytes(series, label(l[0], l[1]))
	}
	series = protowire.AppendTag(series, 2, protowire.BytesType)
	series = protowire.AppendBytes(series, sample)
	req := protowire.AppendTag(nil, 1, protowire.BytesType)
	return protowire.AppendBytes(req, series)
}

func TestPrometheusRemoteWrite(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	mux := http.NewServeMux()
	RegisterHandlers(mux, e)
	cases := []struct {
		name string
		body []byte
		code int
	}{
		{"snappy protobuf", snappy.Encode(nil, writeRequest("up", 1.5, time.Now().UnixMilli())), http.StatusNoContent},
		{"not snappy", []byte("plain"), http.StatusBadRequest},
		{"malformed protobuf", snappy.Encode(nil, []byte{0x0a, 0xff}), http.StatusBadRequest},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewReader(tc.body))
		req.Header.Set("Content-Encoding", "snappy")
		req.Header.Set("Content-Type", "application/x-protobuf")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Errorf("%s: code %d, body %s", tc.name, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
	if len(mp.ingested) != 1 || mp.ingested[0].name != "up" || mp.ingested[0].value != 1.5 || mp.ingested[0].host != "node-1" {
		t.Errorf("ingested = %+v", mp.ingested)
	}
	if m, _, _ := e.IngestCounts(); m != 1 {
		t.Errorf("remote write samples must count as ingested metrics, got %d", m)
	}
}

func mathFloat64bits(v float64) uint64 { return math.Float64bits(v) }
