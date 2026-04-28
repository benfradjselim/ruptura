// Package grpcserver implements the OHE agent gRPC ingest service.
package grpcserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/benfradjselim/ruptura/internal/grpcserver/proto"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// StorageBackend is the subset of *storage.Store used by the gRPC server.
type StorageBackend interface {
	GetStore() *storage.Store
}

// Server is the OHE gRPC agent ingest server.
type Server struct {
	store StorageBackend
	grpc  *grpc.Server
	log   *logger.Logger
}

// Config holds options for the gRPC server.
type Config struct {
	ListenAddr string
	TLSCert    string
	TLSKey     string
	DefaultOrg string
}

// New creates a gRPC server that stores ingested data via store.
func New(store StorageBackend, cfg Config) (*Server, error) {
	var opts []grpc.ServerOption
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("grpcserver: TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	gs := grpc.NewServer(opts...)
	s := &Server{
		store: store,
		grpc:  gs,
		log:   logger.New("grpc"),
	}

	gs.RegisterService(&grpc.ServiceDesc{
		ServiceName: "ohe.v1.AgentService",
		HandlerType: (*agentServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Ingest",
				Handler:    ingestHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "ohe.proto",
	}, s)

	return s, nil
}

type agentServiceServer interface {
	Ingest(context.Context, *pb.IngestRequest) (*pb.IngestResponse, error)
}

func ingestHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(pb.IngestRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(agentServiceServer).Ingest(ctx, req)
}

// Ingest implements agentServiceServer — the actual RPC handler.
func (s *Server) Ingest(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error) {
	os := s.store.GetStore()
	resp := &pb.IngestResponse{}

	for _, m := range req.Metrics {
		if m.Name == "" || m.Host == "" {
			continue
		}
		t := time.UnixMilli(m.TimestampMs).UTC()
		if m.TimestampMs == 0 {
			t = time.Now().UTC()
		}
		if err := os.SaveMetric(m.Host, m.Name, m.Value, t); err != nil {
			s.log.Error("grpc ingest metric", "err", err, "host", m.Host, "name", m.Name)
			resp.Error = err.Error()
			continue
		}
		resp.MetricsWritten++
	}

	for _, l := range req.Logs {
		t := time.UnixMilli(l.TimestampMs).UTC()
		if l.TimestampMs == 0 {
			t = time.Now().UTC()
		}
		entry := map[string]interface{}{
			"host":    l.Host,
			"service": l.Service,
			"level":   l.Level,
			"body":    l.Body,
		}
		if err := os.SaveLog(l.Service, entry, t); err != nil {
			s.log.Error("grpc ingest log", "err", err)
			resp.Error = err.Error()
			continue
		}
		resp.LogsWritten++
	}

	return resp, nil
}

// Serve starts accepting connections on addr. It blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpcserver: listen %s: %w", addr, err)
	}
	return s.ServeListener(ctx, ln)
}

// ServeListener accepts connections on the provided listener. Blocks until ctx
// is cancelled, then performs a graceful shutdown.
func (s *Server) ServeListener(ctx context.Context, ln net.Listener) error {
	s.log.Info("grpc server listening", "addr", ln.Addr().String())
	errCh := make(chan error, 1)
	go func() { errCh <- s.grpc.Serve(ln) }()

	select {
	case <-ctx.Done():
		s.grpc.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) GracefulStop() {
	s.grpc.GracefulStop()
}
