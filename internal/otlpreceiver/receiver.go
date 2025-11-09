package otlpreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

// SpanReceiver is the interface for storing received spans.
// Implementations should be thread-safe as Export may be called concurrently.
type SpanReceiver interface {
	ReceiveSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error
}

// Config holds configuration for the OTLP receiver.
type Config struct {
	Host string // e.g., "127.0.0.1"
	Port int    // 0 for ephemeral port assignment
}

// Server is the OTLP gRPC server that receives trace data.
type Server struct {
	listener     net.Listener
	grpcServer   *grpc.Server
	spanReceiver SpanReceiver
	stopOnce     sync.Once
	stopChan     chan struct{}
	stopDone     chan struct{}
}

// NewServer creates a new OTLP gRPC server.
// The server will bind to the configured host and port (use port 0 for ephemeral).
// Received spans are passed to the SpanReceiver implementation.
func NewServer(cfg Config, receiver SpanReceiver) (*Server, error) {
	if receiver == nil {
		return nil, fmt.Errorf("span receiver cannot be nil")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	server := &Server{
		listener:     listener,
		grpcServer:   grpcServer,
		spanReceiver: receiver,
		stopChan:     make(chan struct{}),
		stopDone:     make(chan struct{}, 1),
	}

	// Register the trace service
	traceService := &traceServiceImpl{
		receiver: receiver,
	}
	collectortrace.RegisterTraceServiceServer(grpcServer, traceService)

	return server, nil
}

// Start begins serving OTLP requests. This method blocks until Stop is called.
// It should typically be run in a goroutine.
func (s *Server) Start(ctx context.Context) error {
	// Handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			s.Stop()
		case <-s.stopChan:
			// Stop was called directly
		}
	}()

	err := s.grpcServer.Serve(s.listener)
	s.stopDone <- struct{}{}
	return err
}

// Stop initiates graceful shutdown of the server.
// Safe to call multiple times.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.grpcServer.GracefulStop()
		close(s.stopChan)
	})
}

// StopWait stops the server and waits for shutdown to complete.
func (s *Server) StopWait() {
	s.Stop()
	<-s.stopDone
}

// Endpoint returns the actual listening address.
// This is particularly useful when using ephemeral ports (port 0).
// Returns format "host:port", e.g., "127.0.0.1:54321"
func (s *Server) Endpoint() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// traceServiceImpl implements the OTLP TraceService gRPC interface.
type traceServiceImpl struct {
	collectortrace.UnimplementedTraceServiceServer
	receiver SpanReceiver
}

// Export handles incoming trace export requests from OTLP clients.
func (t *traceServiceImpl) Export(
	ctx context.Context,
	req *collectortrace.ExportTraceServiceRequest,
) (*collectortrace.ExportTraceServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Pass the resource spans to the receiver
	// Preserve the full OTLP structure: ResourceSpans -> ScopeSpans -> Spans
	if err := t.receiver.ReceiveSpans(ctx, req.ResourceSpans); err != nil {
		return nil, fmt.Errorf("failed to receive spans: %w", err)
	}

	// Return success response
	return &collectortrace.ExportTraceServiceResponse{}, nil
}
