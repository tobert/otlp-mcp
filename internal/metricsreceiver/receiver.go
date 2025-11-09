package metricsreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/grpc"
)

// MetricReceiver is the interface for storing received metrics.
// Implementations should be thread-safe as Export may be called concurrently.
type MetricReceiver interface {
	ReceiveMetrics(ctx context.Context, metrics []*metricspb.ResourceMetrics) error
}

// Config holds configuration for the OTLP metrics receiver.
type Config struct {
	Host string // e.g., "127.0.0.1"
	Port int    // 0 for ephemeral port assignment
}

// Server is the OTLP gRPC server that receives metric data.
type Server struct {
	listener       net.Listener
	grpcServer     *grpc.Server
	metricReceiver MetricReceiver
	stopOnce       sync.Once
	stopChan       chan struct{}
	stopDone       chan struct{}
}

// NewServer creates a new OTLP gRPC metrics server.
// The server will bind to the configured host and port (use port 0 for ephemeral).
// Received metrics are passed to the MetricReceiver implementation.
func NewServer(cfg Config, receiver MetricReceiver) (*Server, error) {
	if receiver == nil {
		return nil, fmt.Errorf("metric receiver cannot be nil")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	server := &Server{
		listener:       listener,
		grpcServer:     grpcServer,
		metricReceiver: receiver,
		stopChan:       make(chan struct{}),
		stopDone:       make(chan struct{}, 1),
	}

	// Register the metrics service
	metricsService := &metricsServiceImpl{
		receiver: receiver,
	}
	collectormetrics.RegisterMetricsServiceServer(grpcServer, metricsService)

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

// metricsServiceImpl implements the OTLP MetricsService gRPC interface.
type metricsServiceImpl struct {
	collectormetrics.UnimplementedMetricsServiceServer
	receiver MetricReceiver
}

// Export handles incoming metrics export requests from OTLP clients.
func (m *metricsServiceImpl) Export(
	ctx context.Context,
	req *collectormetrics.ExportMetricsServiceRequest,
) (*collectormetrics.ExportMetricsServiceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Pass the resource metrics to the receiver
	// Preserve the full OTLP structure: ResourceMetrics -> ScopeMetrics -> Metrics
	if err := m.receiver.ReceiveMetrics(ctx, req.ResourceMetrics); err != nil {
		return nil, fmt.Errorf("failed to receive metrics: %w", err)
	}

	// Return success response
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}
