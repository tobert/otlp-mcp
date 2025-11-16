package otlpreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

// UnifiedReceiver defines the interface for receiving all OTLP signal types.
// This is typically implemented by storage.ObservabilityStorage.
type UnifiedReceiver interface {
	ReceiveSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error
	ReceiveLogs(ctx context.Context, logs []*logspb.ResourceLogs) error
	ReceiveMetrics(ctx context.Context, metrics []*metricspb.ResourceMetrics) error
}

// UnifiedServer is a single OTLP gRPC server that handles all three signal types.
// This simplifies application configuration - only one endpoint needed.
// It can listen on multiple ports simultaneously via AddPort().
type UnifiedServer struct {
	host        string
	listeners   []net.Listener
	grpcServers []*grpc.Server
	receiver    UnifiedReceiver
	mu          sync.Mutex // protects port additions
	ctx         context.Context
	stopOnce    sync.Once
	stopChan    chan struct{}
	stopDone    chan struct{}
}

// NewUnifiedServer creates a new OTLP gRPC server that accepts all signal types.
// The server will bind to the configured host and port (use port 0 for ephemeral).
// All received telemetry is passed to the UnifiedReceiver implementation.
func NewUnifiedServer(cfg Config, receiver UnifiedReceiver) (*UnifiedServer, error) {
	if receiver == nil {
		return nil, fmt.Errorf("receiver cannot be nil")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	// Register all three OTLP services on the gRPC server
	collectortrace.RegisterTraceServiceServer(grpcServer, &unifiedTraceService{receiver: receiver})
	collectorlogs.RegisterLogsServiceServer(grpcServer, &unifiedLogsService{receiver: receiver})
	collectormetrics.RegisterMetricsServiceServer(grpcServer, &unifiedMetricsService{receiver: receiver})

	server := &UnifiedServer{
		host:        cfg.Host,
		listeners:   []net.Listener{listener},
		grpcServers: []*grpc.Server{grpcServer},
		receiver:    receiver,
		stopChan:    make(chan struct{}),
		stopDone:    make(chan struct{}, 1),
	}

	return server, nil
}

// Start begins serving OTLP requests on the primary listener.
// This method blocks until Stop is called. It should typically be run in a goroutine.
func (s *UnifiedServer) Start(ctx context.Context) error {
	s.ctx = ctx

	// Handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			s.Stop()
		case <-s.stopChan:
			// Stop was called directly
		}
	}()

	// Serve on primary listener (index 0)
	if len(s.listeners) == 0 {
		return fmt.Errorf("no listeners available")
	}

	err := s.grpcServers[0].Serve(s.listeners[0])
	s.stopDone <- struct{}{}
	return err
}

// Stop initiates graceful shutdown of all servers.
// Safe to call multiple times.
func (s *UnifiedServer) Stop() {
	s.stopOnce.Do(func() {
		for _, grpcServer := range s.grpcServers {
			grpcServer.GracefulStop()
		}
		close(s.stopChan)
	})
}

// StopWait stops the server and waits for shutdown to complete.
func (s *UnifiedServer) StopWait() {
	s.Stop()
	<-s.stopDone
}

// Endpoint returns the primary listening address.
// This is particularly useful when using ephemeral ports (port 0).
// Returns format "host:port", e.g., "127.0.0.1:54321"
func (s *UnifiedServer) Endpoint() string {
	if len(s.listeners) == 0 {
		return ""
	}
	return s.listeners[0].Addr().String()
}

// Endpoints returns all listening addresses.
func (s *UnifiedServer) Endpoints() []string {
	endpoints := make([]string, len(s.listeners))
	for i, listener := range s.listeners {
		endpoints[i] = listener.Addr().String()
	}
	return endpoints
}

// AddPort adds a new listening port to the server without disrupting existing connections.
// This allows the server to accept OTLP data on multiple ports simultaneously.
// All buffered data is shared across all ports.
func (s *UnifiedServer) AddPort(ctx context.Context, port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create new listener on requested port
	addr := fmt.Sprintf("%s:%d", s.host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}

	// Create new gRPC server
	grpcServer := grpc.NewServer()

	// Register all three OTLP services with existing receiver (shared storage)
	collectortrace.RegisterTraceServiceServer(grpcServer, &unifiedTraceService{receiver: s.receiver})
	collectorlogs.RegisterLogsServiceServer(grpcServer, &unifiedLogsService{receiver: s.receiver})
	collectormetrics.RegisterMetricsServiceServer(grpcServer, &unifiedMetricsService{receiver: s.receiver})

	// Add to lists
	s.listeners = append(s.listeners, listener)
	s.grpcServers = append(s.grpcServers, grpcServer)

	// Start serving on new port in background
	go func() {
		_ = grpcServer.Serve(listener)
	}()

	// Health check: verify the port is actually accepting connections
	checkAddr := listener.Addr().String()
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", checkAddr, 10*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}

	return fmt.Errorf("port %s failed health check - not accepting connections after 50ms", checkAddr)
}

// Service implementations

type unifiedTraceService struct {
	collectortrace.UnimplementedTraceServiceServer
	receiver UnifiedReceiver
}

func (t *unifiedTraceService) Export(
	ctx context.Context,
	req *collectortrace.ExportTraceServiceRequest,
) (*collectortrace.ExportTraceServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := t.receiver.ReceiveSpans(ctx, req.ResourceSpans); err != nil {
		return nil, fmt.Errorf("failed to receive spans: %w", err)
	}

	return &collectortrace.ExportTraceServiceResponse{}, nil
}

type unifiedLogsService struct {
	collectorlogs.UnimplementedLogsServiceServer
	receiver UnifiedReceiver
}

func (l *unifiedLogsService) Export(
	ctx context.Context,
	req *collectorlogs.ExportLogsServiceRequest,
) (*collectorlogs.ExportLogsServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := l.receiver.ReceiveLogs(ctx, req.ResourceLogs); err != nil {
		return nil, fmt.Errorf("failed to receive logs: %w", err)
	}

	return &collectorlogs.ExportLogsServiceResponse{}, nil
}

type unifiedMetricsService struct {
	collectormetrics.UnimplementedMetricsServiceServer
	receiver UnifiedReceiver
}

func (m *unifiedMetricsService) Export(
	ctx context.Context,
	req *collectormetrics.ExportMetricsServiceRequest,
) (*collectormetrics.ExportMetricsServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := m.receiver.ReceiveMetrics(ctx, req.ResourceMetrics); err != nil {
		return nil, fmt.Errorf("failed to receive metrics: %w", err)
	}

	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}
