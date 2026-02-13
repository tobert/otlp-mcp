package test

import (
	"context"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// TestEndToEnd verifies the complete workflow:
// 1. Create storage
// 2. Start OTLP gRPC receiver
// 3. Send trace via OTLP gRPC
// 4. Query storage for the trace
// 5. Verify data integrity
func TestEndToEnd(t *testing.T) {
	// 1. Setup observability storage
	obsStorage := storage.NewObservabilityStorage(1000, 1000, 1000)

	// 2. Start unified OTLP receiver on ephemeral port
	otlpServer, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{
			Host: "127.0.0.1",
			Port: 0, // ephemeral port
		},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in background
	go func() {
		if err := otlpServer.Start(ctx); err != nil {
			t.Logf("OTLP server stopped: %v", err)
		}
	}()
	defer otlpServer.Stop()

	// Get actual endpoint
	endpoint := otlpServer.Endpoint()
	t.Logf("OTLP server listening on %s", endpoint)

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// 3. Create OTLP gRPC client
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectortrace.NewTraceServiceClient(conn)

	// 4. Send test span
	testTraceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	testSpanID := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	_, err = client.Export(context.Background(), &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{StringValue: "e2e-test-service"},
							},
						},
						{
							Key: "deployment.environment",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{StringValue: "test"},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           testTraceID,
								SpanId:            testSpanID,
								Name:              "e2e-test-span",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
								Attributes: []*commonpb.KeyValue{
									{
										Key: "test.type",
										Value: &commonpb.AnyValue{
											Value: &commonpb.AnyValue_StringValue{StringValue: "e2e"},
										},
									},
								},
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_OK,
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to export span: %v", err)
	}

	// Give storage a moment to process
	time.Sleep(100 * time.Millisecond)

	// 5. Query storage and verify data
	traceStorage := obsStorage.Traces()
	recent := traceStorage.GetRecentSpans(10)
	if len(recent) == 0 {
		t.Fatal("no spans found in storage after export")
	}

	// Verify we got our span
	found := false
	for _, span := range recent {
		if span.ServiceName == "e2e-test-service" && span.SpanName == "e2e-test-span" {
			found = true

			// Verify trace ID
			expectedTraceID := "0102030405060708090a0b0c0d0e0f10"
			if span.TraceID != expectedTraceID {
				t.Errorf("expected trace ID %q, got %q", expectedTraceID, span.TraceID)
			}

			// Verify span ID
			expectedSpanID := "0102030405060708"
			if span.SpanID != expectedSpanID {
				t.Errorf("expected span ID %q, got %q", expectedSpanID, span.SpanID)
			}

			break
		}
	}

	if !found {
		t.Fatal("exported span not found in storage")
	}

	// 6. Test query by trace ID
	traceIDStr := "0102030405060708090a0b0c0d0e0f10"
	spansByTraceID := traceStorage.GetSpansByTraceID(traceIDStr)
	if len(spansByTraceID) != 1 {
		t.Fatalf("expected 1 span for trace ID, got %d", len(spansByTraceID))
	}

	// 7. Test query by service name
	spansByService := traceStorage.GetSpansByService("e2e-test-service")
	if len(spansByService) != 1 {
		t.Fatalf("expected 1 span for service name, got %d", len(spansByService))
	}

	// 8. Verify storage stats
	stats := traceStorage.Stats()
	if stats.SpanCount != 1 {
		t.Errorf("expected span count 1, got %d", stats.SpanCount)
	}
	if stats.TraceCount != 1 {
		t.Errorf("expected trace count 1, got %d", stats.TraceCount)
	}
	if stats.Capacity != 1000 {
		t.Errorf("expected capacity 1000, got %d", stats.Capacity)
	}

	t.Log("End-to-end test passed: OTLP -> Storage -> Query")
}

// TestMultipleSpans tests handling of multiple spans across multiple exports.
func TestMultipleSpans(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 100, 100)

	otlpServer, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go otlpServer.Start(ctx)
	defer otlpServer.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := grpc.NewClient(otlpServer.Endpoint(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectortrace.NewTraceServiceClient(conn)

	// Send 10 spans
	for i := 0; i < 10; i++ {
		_, err := client.Export(context.Background(), &collectortrace.ExportTraceServiceRequest{
			ResourceSpans: []*tracepb.ResourceSpans{
				{
					Resource: &resourcepb.Resource{
						Attributes: []*commonpb.KeyValue{
							{
								Key: "service.name",
								Value: &commonpb.AnyValue{
									Value: &commonpb.AnyValue_StringValue{StringValue: "multi-span-test"},
								},
							},
						},
					},
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{
								{
									TraceId:           []byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
									SpanId:            []byte{byte(i), 2, 3, 4, 5, 6, 7, 8},
									Name:              "test-span",
									StartTimeUnixNano: uint64(time.Now().UnixNano()),
									EndTimeUnixNano:   uint64(time.Now().UnixNano()),
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to export span %d: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	stats := obsStorage.Traces().Stats()
	if stats.SpanCount != 10 {
		t.Errorf("expected 10 spans, got %d", stats.SpanCount)
	}

	if stats.TraceCount != 10 {
		t.Errorf("expected 10 traces, got %d", stats.TraceCount)
	}

	t.Log("Multiple spans test passed")
}
