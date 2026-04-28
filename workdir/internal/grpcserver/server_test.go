package grpcserver_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/benfradjselim/ruptura/internal/grpcserver"
	pb "github.com/benfradjselim/ruptura/internal/grpcserver/proto"
	"github.com/benfradjselim/ruptura/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "grpc-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// startTestServer starts a grpcserver on a random port and returns a connected
// client along with the address. The server is stopped when the test ends.
func startTestServer(t *testing.T) (*grpc.ClientConn, string) {
	t.Helper()
	store := openTestStore(t)

	srv, err := grpcserver.New(store, grpcserver.Config{})
	if err != nil {
		t.Fatalf("grpcserver.New: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(srv.GracefulStop)

	go func() {
		_ = srv.ServeListener(ctx, ln)
	}()

	// Give the server time to start
	time.Sleep(20 * time.Millisecond)

	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(), //nolint:staticcheck
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("ohe-json")),
	)
	if err != nil {
		t.Fatalf("grpc.Dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, addr
}

func callIngest(t *testing.T, conn *grpc.ClientConn, req *pb.IngestRequest) *pb.IngestResponse {
	t.Helper()
	var resp pb.IngestResponse
	err := conn.Invoke(context.Background(),
		"/ohe.v1.AgentService/Ingest",
		req, &resp,
		grpc.CallContentSubtype("ohe-json"),
	)
	if err != nil {
		t.Fatalf("Invoke Ingest: %v", err)
	}
	return &resp
}

func TestGRPCIngestMetrics(t *testing.T) {
	conn, _ := startTestServer(t)

	now := time.Now().UnixMilli()
	resp := callIngest(t, conn, &pb.IngestRequest{
		Metrics: []*pb.MetricSample{
			{Host: "web-01", Name: "cpu_percent", Value: 72.5, TimestampMs: now},
			{Host: "web-01", Name: "mem_percent", Value: 45.0, TimestampMs: now},
		},
	})
	if resp.MetricsWritten != 2 {
		t.Errorf("MetricsWritten = %d; want 2", resp.MetricsWritten)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestGRPCIngestLogs(t *testing.T) {
	conn, _ := startTestServer(t)

	now := time.Now().UnixMilli()
	resp := callIngest(t, conn, &pb.IngestRequest{
		Logs: []*pb.LogEntry{
			{Host: "api-01", Service: "api", Level: "error", Body: "connection refused", TimestampMs: now},
		},
	})
	if resp.LogsWritten != 1 {
		t.Errorf("LogsWritten = %d; want 1", resp.LogsWritten)
	}
}

func TestGRPCIngestMixed(t *testing.T) {
	conn, _ := startTestServer(t)

	now := time.Now().UnixMilli()
	resp := callIngest(t, conn, &pb.IngestRequest{
		Metrics: []*pb.MetricSample{
			{Host: "db-01", Name: "query_latency_ms", Value: 12.3, TimestampMs: now},
		},
		Logs: []*pb.LogEntry{
			{Host: "db-01", Service: "postgres", Level: "warn", Body: "slow query", TimestampMs: now},
		},
	})
	if resp.MetricsWritten != 1 || resp.LogsWritten != 1 {
		t.Errorf("metrics=%d logs=%d; want both 1", resp.MetricsWritten, resp.LogsWritten)
	}
}

func TestGRPCIngestSkipsEmptyName(t *testing.T) {
	conn, _ := startTestServer(t)

	resp := callIngest(t, conn, &pb.IngestRequest{
		Metrics: []*pb.MetricSample{
			{Host: "web-01", Name: "", Value: 1.0}, // missing name → skip
			{Host: "", Name: "cpu", Value: 1.0},    // missing host → skip
			{Host: "web-01", Name: "cpu", Value: 2.0}, // valid
		},
	})
	if resp.MetricsWritten != 1 {
		t.Errorf("MetricsWritten = %d; want 1 (only valid sample)", resp.MetricsWritten)
	}
}

func TestGRPCIngestZeroTimestamp(t *testing.T) {
	conn, _ := startTestServer(t)

	// TimestampMs=0 should default to server time (not error)
	resp := callIngest(t, conn, &pb.IngestRequest{
		Metrics: []*pb.MetricSample{
			{Host: "host", Name: "metric", Value: 1.0, TimestampMs: 0},
		},
	})
	if resp.MetricsWritten != 1 {
		t.Errorf("MetricsWritten = %d; want 1", resp.MetricsWritten)
	}
}
