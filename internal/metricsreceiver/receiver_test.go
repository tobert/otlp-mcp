package metricsreceiver

import (
	"context"
	"sync"
	"testing"
	"time"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockReceiver is a test implementation of MetricReceiver that records received metrics.
type mockReceiver struct {
	mu      sync.Mutex
	metrics []*metricspb.ResourceMetrics
	err     error // error to return from ReceiveMetrics
}

func (m *mockReceiver) ReceiveMetrics(ctx context.Context, metrics []*metricspb.ResourceMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	m.metrics = append(m.metrics, metrics...)
	return nil
}

func (m *mockReceiver) getMetrics() []*metricspb.ResourceMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metrics
}

func (m *mockReceiver) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.metrics)
}

// TestNewServer verifies server creation.
func TestNewServer(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Stop()

	if server == nil {
		t.Fatal("server is nil")
	}

	endpoint := server.Endpoint()
	if endpoint == "" {
		t.Fatal("endpoint is empty")
	}

	t.Logf("Server endpoint: %s", endpoint)
}

// TestNewServerNilReceiver verifies that NewServer rejects nil receivers.
func TestNewServerNilReceiver(t *testing.T) {
	_, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, nil)
	if err == nil {
		t.Fatal("expected error for nil receiver, got nil")
	}
}

// TestServerStartStop verifies the server can start and stop cleanly.
func TestServerStartStop(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	server.Stop()

	// Wait for Start to return
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Server stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

// TestOTLPExport tests the full flow of sending metrics via OTLP gRPC.
func TestOTLPExport(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	endpoint := server.Endpoint()
	t.Logf("Connecting to server at %s", endpoint)

	// Create gRPC client
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectormetrics.NewMetricsServiceClient(conn)

	// Create test gauge metric
	testMetric := &metricspb.Metric{
		Name:        "test.gauge",
		Description: "A test gauge metric",
		Unit:        "1",
		Data: &metricspb.Metric_Gauge{
			Gauge: &metricspb.Gauge{
				DataPoints: []*metricspb.NumberDataPoint{
					{
						TimeUnixNano: uint64(time.Now().UnixNano()),
						Value: &metricspb.NumberDataPoint_AsDouble{
							AsDouble: 42.0,
						},
						Attributes: []*commonpb.KeyValue{
							{
								Key: "test.key",
								Value: &commonpb.AnyValue{
									Value: &commonpb.AnyValue_StringValue{StringValue: "test-value"},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create resource with service name
	resource := &resourcepb.Resource{
		Attributes: []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"},
				},
			},
		},
	}

	// Build OTLP request
	req := &collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: resource,
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{testMetric},
					},
				},
			},
		},
	}

	// Send the metric
	resp, err := client.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	// Verify receiver got the metric
	time.Sleep(50 * time.Millisecond) // Give receiver time to process

	if receiver.count() != 1 {
		t.Fatalf("expected 1 resource metric, got %d", receiver.count())
	}

	receivedMetrics := receiver.getMetrics()
	if len(receivedMetrics[0].ScopeMetrics) != 1 {
		t.Fatalf("expected 1 scope metric, got %d", len(receivedMetrics[0].ScopeMetrics))
	}

	if len(receivedMetrics[0].ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(receivedMetrics[0].ScopeMetrics[0].Metrics))
	}

	receivedMetric := receivedMetrics[0].ScopeMetrics[0].Metrics[0]
	if receivedMetric.Name != "test.gauge" {
		t.Errorf("expected metric name 'test.gauge', got %q", receivedMetric.Name)
	}

	// Verify it's a gauge
	gauge := receivedMetric.GetGauge()
	if gauge == nil {
		t.Fatal("expected gauge metric, got nil")
	}

	if len(gauge.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(gauge.DataPoints))
	}

	if gauge.DataPoints[0].GetAsDouble() != 42.0 {
		t.Errorf("expected value 42.0, got %f", gauge.DataPoints[0].GetAsDouble())
	}

	// Verify resource attributes
	if receivedMetrics[0].Resource == nil {
		t.Fatal("resource is nil")
	}

	foundServiceName := false
	for _, attr := range receivedMetrics[0].Resource.Attributes {
		if attr.Key == "service.name" {
			if attr.Value.GetStringValue() == "test-service" {
				foundServiceName = true
			}
		}
	}

	if !foundServiceName {
		t.Error("service.name attribute not found in resource")
	}
}

// TestOTLPExportMultipleMetrics verifies handling of multiple metrics.
func TestOTLPExportMultipleMetrics(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := grpc.NewClient(server.Endpoint(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectormetrics.NewMetricsServiceClient(conn)

	// Send 5 separate requests
	for i := 0; i < 5; i++ {
		req := &collectormetrics.ExportMetricsServiceRequest{
			ResourceMetrics: []*metricspb.ResourceMetrics{
				{
					Resource: &resourcepb.Resource{},
					ScopeMetrics: []*metricspb.ScopeMetrics{
						{
							Metrics: []*metricspb.Metric{
								{
									Name: "test.counter",
									Data: &metricspb.Metric_Sum{
										Sum: &metricspb.Sum{
											IsMonotonic:            true,
											AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
											DataPoints: []*metricspb.NumberDataPoint{
												{
													TimeUnixNano: uint64(time.Now().UnixNano()),
													Value: &metricspb.NumberDataPoint_AsInt{
														AsInt: int64(i),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := client.Export(context.Background(), req)
		if err != nil {
			t.Fatalf("Export %d failed: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if receiver.count() != 5 {
		t.Fatalf("expected 5 resource metrics, got %d", receiver.count())
	}
}
