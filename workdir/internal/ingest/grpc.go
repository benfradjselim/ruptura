package ingest

import (
	"context"
	"fmt"
	"io"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCMetricPoint is a metric sample received over gRPC.
type GRPCMetricPoint struct {
	Name          string
	Host          string
	Value         float64
	TimestampUnix int64
}

// GRPCPushResult summarises a completed streaming session.
type GRPCPushResult struct {
	Accepted int64
	Rejected int64
}

// MetricIngestServer is implemented by the gRPC server.
type MetricIngestServer interface {
	PushMetrics(stream MetricIngest_PushMetricsServer) error
}

// MetricIngest_PushMetricsServer is the streaming server half.
type MetricIngest_PushMetricsServer interface {
	Recv() (*GRPCMetricPoint, error)
	SendAndClose(*GRPCPushResult) error
	grpc.ServerStream
}

// MetricIngestServiceDesc registers the hand-written service (no protoc).
var MetricIngestServiceDesc = grpc.ServiceDesc{
	ServiceName: "ruptura.v1.MetricIngest",
	HandlerType: (*MetricIngestServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "PushMetrics",
			Handler:       _pushMetricsHandler,
			ClientStreams: true,
		},
	},
}

func _pushMetricsHandler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MetricIngestServer).PushMetrics(&metricIngestPushServer{stream})
}

type metricIngestPushServer struct{ grpc.ServerStream }

func (s *metricIngestPushServer) Recv() (*GRPCMetricPoint, error) {
	m := new(GRPCMetricPoint)
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *metricIngestPushServer) SendAndClose(r *GRPCPushResult) error {
	return s.ServerStream.SendMsg(r)
}

// grpcIngestServer implements MetricIngestServer.
type grpcIngestServer struct {
	samples chan *GRPCMetricPoint
}

func (s *grpcIngestServer) PushMetrics(stream MetricIngest_PushMetricsServer) error {
	var accepted, rejected int64
	for {
		pt, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&GRPCPushResult{Accepted: accepted, Rejected: rejected})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv: %v", err)
		}
		if pt.Name == "" || pt.Host == "" {
			rejected++
			continue
		}
		select {
		case s.samples <- pt:
			accepted++
		default:
			rejected++ // back-pressure: drop if channel full
		}
	}
}

// DrainGRPCSamples returns a read-only channel of received metric points.
// Callers can range over it to process inbound gRPC samples.
func (e *Engine) DrainGRPCSamples() <-chan *GRPCMetricPoint {
	return e.grpcSamples
}

// StartGRPC binds a real gRPC server on addr.
func (e *Engine) StartGRPC(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", addr, err)
	}
	srv := grpc.NewServer()
	srv.RegisterService(&MetricIngestServiceDesc, &grpcIngestServer{samples: e.grpcSamples})
	e.grpcServer = srv
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

// grpcClientStream lets tests act as a gRPC client without a real network.
type grpcClientStream struct {
	ctx    context.Context
	points []*GRPCMetricPoint
	idx    int
	result *GRPCPushResult
}

func newTestStream(ctx context.Context, pts []*GRPCMetricPoint) *grpcClientStream {
	return &grpcClientStream{ctx: ctx, points: pts}
}

func (s *grpcClientStream) Context() context.Context        { return s.ctx }
func (s *grpcClientStream) SendMsg(m interface{}) error     { s.result = m.(*GRPCPushResult); return nil }
func (s *grpcClientStream) Recv() (*GRPCMetricPoint, error) {
	m := new(GRPCMetricPoint)
	if err := s.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *grpcClientStream) SendAndClose(r *GRPCPushResult) error {
	return s.SendMsg(r)
}
func (s *grpcClientStream) RecvMsg(m interface{}) error {
	if s.idx >= len(s.points) {
		return io.EOF
	}
	*(m.(*GRPCMetricPoint)) = *s.points[s.idx]
	s.idx++
	return nil
}
func (s *grpcClientStream) SetHeader(metadata.MD) error  { return nil }
func (s *grpcClientStream) SendHeader(metadata.MD) error { return nil }
func (s *grpcClientStream) SetTrailer(metadata.MD)       {}
