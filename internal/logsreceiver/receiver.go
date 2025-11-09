package logsreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"

	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
)

// LogReceiver is the interface for storing received log records.
// Implementations should be thread-safe as Export may be called concurrently.
type LogReceiver interface {
	ReceiveLogs(ctx context.Context, logs []*logspb.ResourceLogs) error
}

// Config holds configuration for the OTLP logs receiver.
type Config struct {
	Host string // e.g., "127.0.0.1"
	Port int    // 0 for ephemeral port assignment
}

// Server is the OTLP gRPC server that receives log data.
type Server struct {
	listener    net.Listener
	grpcServer  *grpc.Server
	logReceiver LogReceiver
	stopOnce    sync.Once
	stopChan    chan struct{}
	stopDone    chan struct{}
}

// NewServer creates a new OTLP gRPC logs server.
// The server will bind to the configured host and port (use port 0 for ephemeral).
// Received logs are passed to the LogReceiver implementation.
func NewServer(cfg Config, receiver LogReceiver) (*Server, error) {
	if receiver == nil {
		return nil, fmt.Errorf("log receiver cannot be nil")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	server := &Server{
		listener:    listener,
		grpcServer:  grpcServer,
		logReceiver: receiver,
		stopChan:    make(chan struct{}),
		stopDone:    make(chan struct{}, 1),
	}

	// Register the logs service
	logsService := &logsServiceImpl{
		receiver: receiver,
	}
	collectorlogs.RegisterLogsServiceServer(grpcServer, logsService)

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

// logsServiceImpl implements the OTLP LogsService gRPC interface.
type logsServiceImpl struct {
	collectorlogs.UnimplementedLogsServiceServer
	receiver LogReceiver
}

// Export handles incoming logs export requests from OTLP clients.
func (l *logsServiceImpl) Export(
	ctx context.Context,
	req *collectorlogs.ExportLogsServiceRequest,
) (*collectorlogs.ExportLogsServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Pass the resource logs to the receiver
	// Preserve the full OTLP structure: ResourceLogs -> ScopeLogs -> LogRecords
	if err := l.receiver.ReceiveLogs(ctx, req.ResourceLogs); err != nil {
		return nil, fmt.Errorf("failed to receive logs: %w", err)
	}

	// Return success response
	return &collectorlogs.ExportLogsServiceResponse{}, nil
}
