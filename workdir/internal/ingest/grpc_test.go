package ingest

import (
	"context"
	"net"
	"testing"
)

func TestStartGRPC_BindsPort(t *testing.T) {
	e := &Engine{grpcSamples: make(chan *GRPCMetricPoint, 10)}
	addr := "localhost:0"
	if err := e.StartGRPC(addr); err != nil {
		t.Fatalf("failed to start grpc: %v", err)
	}
	defer e.Stop(context.Background())
}

func TestStartGRPC_PortInUse(t *testing.T) {
	addr := "localhost:0"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	e := &Engine{grpcSamples: make(chan *GRPCMetricPoint, 10)}
	if err := e.StartGRPC(ln.Addr().String()); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPushMetrics_AllValid(t *testing.T) {
	samples := make(chan *GRPCMetricPoint, 10)
	srv := &grpcIngestServer{samples: samples}
	stream := newTestStream(context.Background(), []*GRPCMetricPoint{
		{Name: "a", Host: "h1"},
		{Name: "b", Host: "h1"},
		{Name: "c", Host: "h2"},
	})
	if err := srv.PushMetrics(stream); err != nil {
		t.Fatal(err)
	}
	if stream.result.Accepted != 3 || stream.result.Rejected != 0 {
		t.Errorf("expected 3 accepted, 0 rejected, got %+v", stream.result)
	}
}

func TestPushMetrics_InvalidPoints(t *testing.T) {
	samples := make(chan *GRPCMetricPoint, 10)
	srv := &grpcIngestServer{samples: samples}
	stream := newTestStream(context.Background(), []*GRPCMetricPoint{
		{Name: "", Host: "h1"},
		{Name: "b", Host: ""},
	})
	if err := srv.PushMetrics(stream); err != nil {
		t.Fatal(err)
	}
	if stream.result.Accepted != 0 || stream.result.Rejected != 2 {
		t.Errorf("expected 0 accepted, 2 rejected, got %+v", stream.result)
	}
}

func TestPushMetrics_BackPressure(t *testing.T) {
	samples := make(chan *GRPCMetricPoint, 1) // capacity 1
	srv := &grpcIngestServer{samples: samples}
	stream := newTestStream(context.Background(), []*GRPCMetricPoint{
		{Name: "a", Host: "h1"},
		{Name: "b", Host: "h1"}, // Should be rejected due to backpressure
	})
	if err := srv.PushMetrics(stream); err != nil {
		t.Fatal(err)
	}
	if stream.result.Accepted != 1 || stream.result.Rejected != 1 {
		t.Errorf("expected 1 accepted, 1 rejected, got %+v", stream.result)
	}
}
