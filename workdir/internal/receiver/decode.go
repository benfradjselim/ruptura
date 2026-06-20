package receiver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/benfradjselim/ruptura/pkg/models"
)

const maxBodyBytes = 32 << 20 // 32 MiB

func isProtobuf(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/x-protobuf")
}

func readBody(r *http.Request) ([]byte, error) {
	rc := r.Body
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip open: %w", err)
		}
		defer gr.Close()
		rc = gr
	}
	return io.ReadAll(io.LimitReader(rc, maxBodyBytes))
}

// DecodeMetricsRequest decodes an OTLP metrics request from protobuf or JSON.
func DecodeMetricsRequest(r *http.Request) (*models.OTLPMetricsRequest, error) {
	body, err := readBody(r)
	if err != nil {
		return nil, err
	}
	if isProtobuf(r) {
		var pb metricspb.MetricsData
		if err := proto.Unmarshal(body, &pb); err != nil {
			return nil, fmt.Errorf("protobuf metrics: %w", err)
		}
		return convertMetrics(&pb), nil
	}
	var req models.OTLPMetricsRequest
	return &req, json.Unmarshal(body, &req)
}

// DecodeLogsRequest decodes an OTLP logs request from protobuf or JSON.
func DecodeLogsRequest(r *http.Request) (*models.OTLPLogsRequest, error) {
	body, err := readBody(r)
	if err != nil {
		return nil, err
	}
	if isProtobuf(r) {
		var pb logspb.LogsData
		if err := proto.Unmarshal(body, &pb); err != nil {
			return nil, fmt.Errorf("protobuf logs: %w", err)
		}
		return convertLogs(&pb), nil
	}
	var req models.OTLPLogsRequest
	return &req, json.Unmarshal(body, &req)
}

// DecodeTracesRequest decodes an OTLP traces request from protobuf or JSON.
func DecodeTracesRequest(r *http.Request) (*models.OTLPTraceRequest, error) {
	body, err := readBody(r)
	if err != nil {
		return nil, err
	}
	if isProtobuf(r) {
		var pb tracepb.TracesData
		if err := proto.Unmarshal(body, &pb); err != nil {
			return nil, fmt.Errorf("protobuf traces: %w", err)
		}
		return convertTraces(&pb), nil
	}
	var req models.OTLPTraceRequest
	return &req, json.Unmarshal(body, &req)
}

// --- protobuf → models converters ---

func pbAttrs(attrs []*commonpb.KeyValue) []models.OTLPAttribute {
	out := make([]models.OTLPAttribute, 0, len(attrs))
	for _, kv := range attrs {
		a := models.OTLPAttribute{Key: kv.Key}
		if kv.Value != nil {
			switch v := kv.Value.Value.(type) {
			case *commonpb.AnyValue_StringValue:
				s := v.StringValue
				a.Value = models.OTLPAnyValue{StringValue: &s}
			case *commonpb.AnyValue_IntValue:
				s := fmt.Sprintf("%d", v.IntValue)
				a.Value = models.OTLPAnyValue{StringValue: &s}
			case *commonpb.AnyValue_DoubleValue:
				s := fmt.Sprintf("%g", v.DoubleValue)
				a.Value = models.OTLPAnyValue{StringValue: &s}
			case *commonpb.AnyValue_BoolValue:
				s := fmt.Sprintf("%v", v.BoolValue)
				a.Value = models.OTLPAnyValue{StringValue: &s}
			}
		}
		out = append(out, a)
	}
	return out
}

func pbResource(res *resourcepb.Resource) models.OTLPResource {
	if res == nil {
		return models.OTLPResource{}
	}
	return models.OTLPResource{Attributes: pbAttrs(res.Attributes)}
}

func nanoStr(ns uint64) string { return fmt.Sprintf("%d", ns) }

func convertMetrics(pb *metricspb.MetricsData) *models.OTLPMetricsRequest {
	req := &models.OTLPMetricsRequest{}
	for _, rm := range pb.ResourceMetrics {
		mrm := models.OTLPResourceMetrics{Resource: pbResource(rm.Resource)}
		for _, sm := range rm.ScopeMetrics {
			msm := models.OTLPScopeMetrics{}
			for _, m := range sm.Metrics {
				mm := models.OTLPMetric{Name: m.Name}
				switch t := m.Data.(type) {
				case *metricspb.Metric_Gauge:
					mm.Gauge = &models.OTLPGauge{}
					for _, dp := range t.Gauge.DataPoints {
						ndp := pbNumberDP(dp)
						mm.Gauge.DataPoints = append(mm.Gauge.DataPoints, ndp)
					}
				case *metricspb.Metric_Sum:
					mm.Sum = &models.OTLPSum{}
					for _, dp := range t.Sum.DataPoints {
						ndp := pbNumberDP(dp)
						mm.Sum.DataPoints = append(mm.Sum.DataPoints, ndp)
					}
				}
				msm.Metrics = append(msm.Metrics, mm)
			}
			mrm.ScopeMetrics = append(mrm.ScopeMetrics, msm)
		}
		req.ResourceMetrics = append(req.ResourceMetrics, mrm)
	}
	return req
}

func pbNumberDP(dp *metricspb.NumberDataPoint) models.OTLPNumberDataPoint {
	ndp := models.OTLPNumberDataPoint{
		TimeUnixNano: nanoStr(dp.TimeUnixNano),
		Attributes:   pbAttrs(dp.Attributes),
	}
	switch v := dp.Value.(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		f := v.AsDouble
		ndp.AsDouble = &f
	case *metricspb.NumberDataPoint_AsInt:
		i := models.OTLPInt64(v.AsInt)
		ndp.AsInt = &i
	}
	return ndp
}

func convertLogs(pb *logspb.LogsData) *models.OTLPLogsRequest {
	req := &models.OTLPLogsRequest{}
	for _, rl := range pb.ResourceLogs {
		mrl := models.OTLPResourceLogs{Resource: pbResource(rl.Resource)}
		for _, sl := range rl.ScopeLogs {
			msl := models.OTLPScopeLogs{}
			for _, lr := range sl.LogRecords {
				body := ""
				if lr.Body != nil {
					if sv, ok := lr.Body.Value.(*commonpb.AnyValue_StringValue); ok {
						body = sv.StringValue
					}
				}
				rec := models.OTLPLogRecord{
					TimeUnixNano:   nanoStr(lr.TimeUnixNano),
					SeverityText:   lr.SeverityText,
					SeverityNumber: int(lr.SeverityNumber),
					Body:           models.OTLPAnyValue{StringValue: &body},
					Attributes:     pbAttrs(lr.Attributes),
					TraceID:        fmt.Sprintf("%x", lr.TraceId),
					SpanID:         fmt.Sprintf("%x", lr.SpanId),
				}
				msl.LogRecords = append(msl.LogRecords, rec)
			}
			mrl.ScopeLogs = append(mrl.ScopeLogs, msl)
		}
		req.ResourceLogs = append(req.ResourceLogs, mrl)
	}
	return req
}

func convertTraces(pb *tracepb.TracesData) *models.OTLPTraceRequest {
	req := &models.OTLPTraceRequest{}
	for _, rs := range pb.ResourceSpans {
		mrs := models.OTLPResourceSpans{Resource: pbResource(rs.Resource)}
		for _, ss := range rs.ScopeSpans {
			mss := models.OTLPScopeSpans{}
			for _, s := range ss.Spans {
				code := models.OTLPStatusCode(0)
				if s.Status != nil {
					code = models.OTLPStatusCode(s.Status.Code)
				}
				span := models.OTLPSpan{
					TraceID:           fmt.Sprintf("%x", s.TraceId),
					SpanID:            fmt.Sprintf("%x", s.SpanId),
					ParentSpanID:      fmt.Sprintf("%x", s.ParentSpanId),
					Name:              s.Name,
					StartTimeUnixNano: nanoStr(s.StartTimeUnixNano),
					EndTimeUnixNano:   nanoStr(s.EndTimeUnixNano),
					Attributes:        pbAttrs(s.Attributes),
					Status:            models.OTLPSpanStatus{Code: code},
				}
				mss.Spans = append(mss.Spans, span)
			}
			mrs.ScopeSpans = append(mrs.ScopeSpans, mss)
		}
		req.ResourceSpans = append(req.ResourceSpans, mrs)
	}
	return req
}
