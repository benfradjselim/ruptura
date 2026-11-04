package ingest

import (
	"context"
	"fmt"
	"net"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // OTLP exporters compress with gzip by default
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/benfradjselim/ruptura/internal/receiver"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const maxOTLPMessage = 32 << 20

// An Export*ServiceRequest has the same wire format as the matching *Data
// message (field 1, repeated resource entries), and an empty message is a valid
// Export*ServiceResponse, so the official services are served from these
// types without the collector packages and their gateway dependency.
func otlpService(name string, newMessage func() proto.Message, ingest func(*Engine, proto.Message)) grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: name,
		HandlerType: (*interface{})(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Export",
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				in := newMessage()
				if err := dec(in); err != nil {
					return nil, err
				}
				handle := func(_ context.Context, req interface{}) (interface{}, error) {
					ingest(srv.(*Engine), req.(proto.Message))
					return &emptypb.Empty{}, nil
				}
				if interceptor == nil {
					return handle(ctx, in)
				}
				return interceptor(ctx, in, &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + name + "/Export"}, handle)
			},
		}},
		Metadata: "opentelemetry/proto/collector",
	}
}

var otlpServices = []grpc.ServiceDesc{
	otlpService("opentelemetry.proto.collector.metrics.v1.MetricsService",
		func() proto.Message { return &metricspb.MetricsData{} },
		func(e *Engine, m proto.Message) {
			e.ingestMetrics(*receiver.MetricsFromProto(m.(*metricspb.MetricsData).ResourceMetrics))
		}),
	otlpService("opentelemetry.proto.collector.logs.v1.LogsService",
		func() proto.Message { return &logspb.LogsData{} },
		func(e *Engine, m proto.Message) { e.ingestLogs(*receiver.LogsFromProto(m.(*logspb.LogsData).ResourceLogs)) }),
	otlpService("opentelemetry.proto.collector.trace.v1.TraceService",
		func() proto.Message { return &tracepb.TracesData{} },
		func(e *Engine, m proto.Message) {
			e.ingestTraces(*receiver.TracesFromProto(m.(*tracepb.TracesData).ResourceSpans))
		}),
}

// StartOTLPGRPC serves the OTLP gRPC services on addr, over TLS when SetTLS
// was called. Over the request rate it answers RESOURCE_EXHAUSTED, which OTLP
// exporters retry with backoff.
func (e *Engine) StartOTLPGRPC(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("otlp grpc listen %s: %w", addr, err)
	}
	limiter := newRateLimiter()
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxOTLPMessage),
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (interface{}, error) {
			if !limiter.allow() {
				return nil, status.Error(codes.ResourceExhausted, "ingest rate limit exceeded")
			}
			return next(ctx, req)
		}),
	}
	if e.httpTLS != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(e.httpTLS)))
	}
	srv := grpc.NewServer(opts...)
	for i := range otlpServices {
		srv.RegisterService(&otlpServices[i], e)
	}
	e.otlpGRPC = srv
	go func() {
		if err := srv.Serve(ln); err != nil {
			logger.Default.Error("OTLP gRPC ingest failed", "error", err)
		}
	}()
	return nil
}
