package otlpreceiver

import (
	"context"
	"sync"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockReceiver is a test implementation of SpanReceiver that records received spans.
type mockReceiver struct {
	mu    sync.Mutex
	spans []*tracepb.ResourceSpans
	err   error // error to return from ReceiveSpans
}

func (m *mockReceiver) ReceiveSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	m.spans = append(m.spans, spans...)
	return nil
}

func (m *mockReceiver) getSpans() []*tracepb.ResourceSpans {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.spans
}

func (m *mockReceiver) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.spans)
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

// TestOTLPExport tests the full flow of sending spans via OTLP gRPC.
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

	client := collectortrace.NewTraceServiceClient(conn)

	// Create test span
	traceID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}

	testSpan := &tracepb.Span{
		TraceId:           traceID,
		SpanId:            spanID,
		Name:              "test-span",
		Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
		StartTimeUnixNano: uint64(time.Now().UnixNano()),
		EndTimeUnixNano:   uint64(time.Now().UnixNano()),
		Attributes: []*commonpb.KeyValue{
			{
				Key: "test.key",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{StringValue: "test-value"},
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
	req := &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: resource,
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{testSpan},
					},
				},
			},
		},
	}

	// Send the span
	resp, err := client.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	// Verify receiver got the span
	time.Sleep(50 * time.Millisecond) // Give receiver time to process

	if receiver.count() != 1 {
		t.Fatalf("expected 1 resource span, got %d", receiver.count())
	}

	receivedSpans := receiver.getSpans()
	if len(receivedSpans[0].ScopeSpans) != 1 {
		t.Fatalf("expected 1 scope span, got %d", len(receivedSpans[0].ScopeSpans))
	}

	if len(receivedSpans[0].ScopeSpans[0].Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(receivedSpans[0].ScopeSpans[0].Spans))
	}

	receivedSpan := receivedSpans[0].ScopeSpans[0].Spans[0]
	if receivedSpan.Name != "test-span" {
		t.Errorf("expected span name 'test-span', got %q", receivedSpan.Name)
	}

	// Verify resource attributes
	if receivedSpans[0].Resource == nil {
		t.Fatal("resource is nil")
	}

	foundServiceName := false
	for _, attr := range receivedSpans[0].Resource.Attributes {
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

// TestOTLPExportMultipleSpans verifies handling of multiple spans.
func TestOTLPExportMultipleSpans(t *testing.T) {
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

	client := collectortrace.NewTraceServiceClient(conn)

	// Send 5 separate requests
	for i := 0; i < 5; i++ {
		req := &collectortrace.ExportTraceServiceRequest{
			ResourceSpans: []*tracepb.ResourceSpans{
				{
					Resource: &resourcepb.Resource{},
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{
								{
									TraceId:           []byte{byte(i), 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
									SpanId:            []byte{byte(i), 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
									Name:              "test-span",
									StartTimeUnixNano: uint64(time.Now().UnixNano()),
									EndTimeUnixNano:   uint64(time.Now().UnixNano()),
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
		t.Fatalf("expected 5 resource spans, got %d", receiver.count())
	}
}
