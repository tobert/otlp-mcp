package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// makeTestSpan creates a test span with the given parameters.
func makeTestSpan(traceID, spanID []byte, serviceName, spanName string) *tracepb.ResourceSpans {
	return &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key: "service.name",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: serviceName},
					},
				},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            spanID,
						Name:              spanName,
						Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
						StartTimeUnixNano: uint64(time.Now().UnixNano()),
						EndTimeUnixNano:   uint64(time.Now().UnixNano()),
					},
				},
			},
		},
	}
}

// TestTraceStorageBasic tests basic span storage and retrieval.
func TestTraceStorageBasic(t *testing.T) {
	ts := NewTraceStorage(100)

	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	rs := makeTestSpan(traceID, spanID, "test-service", "test-span")

	err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
	if err != nil {
		t.Fatalf("ReceiveSpans failed: %v", err)
	}

	// Verify storage
	recent := ts.GetRecentSpans(10)
	if len(recent) != 1 {
		t.Fatalf("expected 1 span, got %d", len(recent))
	}

	if recent[0].ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got %q", recent[0].ServiceName)
	}

	if recent[0].SpanName != "test-span" {
		t.Errorf("expected span name 'test-span', got %q", recent[0].SpanName)
	}

	// Verify stats
	stats := ts.Stats()
	if stats.SpanCount != 1 {
		t.Errorf("expected span count 1, got %d", stats.SpanCount)
	}
	if stats.TraceCount != 1 {
		t.Errorf("expected trace count 1, got %d", stats.TraceCount)
	}
}

// TestTraceStorageGetByTraceID tests querying spans by trace ID.
func TestTraceStorageGetByTraceID(t *testing.T) {
	ts := NewTraceStorage(100)

	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// Add multiple spans with same trace ID
	for i := 0; i < 3; i++ {
		spanID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8}
		rs := makeTestSpan(traceID, spanID, "test-service", fmt.Sprintf("span-%d", i))
		err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
		if err != nil {
			t.Fatalf("ReceiveSpans failed: %v", err)
		}
	}

	// Query by trace ID
	traceIDStr := traceIDToString(traceID)
	spans := ts.GetSpansByTraceID(traceIDStr)

	if len(spans) != 3 {
		t.Fatalf("expected 3 spans for trace ID, got %d", len(spans))
	}

	// Verify all spans have correct trace ID
	for i, span := range spans {
		if span.TraceID != traceIDStr {
			t.Errorf("span %d has wrong trace ID: %s", i, span.TraceID)
		}
	}

	// Query for non-existent trace ID
	notFound := ts.GetSpansByTraceID("nonexistent")
	if notFound != nil {
		t.Errorf("expected nil for non-existent trace ID, got %v", notFound)
	}
}

// TestTraceStorageGetByService tests querying spans by service name.
func TestTraceStorageGetByService(t *testing.T) {
	ts := NewTraceStorage(100)

	// Add spans from different services
	services := []string{"service-a", "service-b", "service-a", "service-c", "service-a"}

	for i, svc := range services {
		traceID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8}
		rs := makeTestSpan(traceID, spanID, svc, fmt.Sprintf("span-%d", i))
		err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
		if err != nil {
			t.Fatalf("ReceiveSpans failed: %v", err)
		}
	}

	// Query for service-a (should have 3 spans)
	spans := ts.GetSpansByService("service-a")
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans for service-a, got %d", len(spans))
	}

	// Query for service-b (should have 1 span)
	spans = ts.GetSpansByService("service-b")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span for service-b, got %d", len(spans))
	}

	// Query for non-existent service
	spans = ts.GetSpansByService("non-existent")
	if len(spans) != 0 {
		t.Fatalf("expected 0 spans for non-existent service, got %d", len(spans))
	}
}

// TestTraceStorageGetByName tests querying spans by span name.
func TestTraceStorageGetByName(t *testing.T) {
	ts := NewTraceStorage(100)

	// Add spans with different names
	names := []string{"GET /api/users", "POST /api/users", "GET /api/users", "DELETE /api/users"}

	for i, name := range names {
		traceID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8}
		rs := makeTestSpan(traceID, spanID, "api-service", name)
		err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
		if err != nil {
			t.Fatalf("ReceiveSpans failed: %v", err)
		}
	}

	// Query for "GET /api/users" (should have 2 spans)
	spans := ts.GetSpansByName("GET /api/users")
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans for 'GET /api/users', got %d", len(spans))
	}

	// Verify all returned spans have correct name
	for _, span := range spans {
		if span.SpanName != "GET /api/users" {
			t.Errorf("expected span name 'GET /api/users', got %q", span.SpanName)
		}
	}
}

// TestTraceStorageRingBufferWrapping tests that old spans are evicted when capacity is reached.
func TestTraceStorageRingBufferWrapping(t *testing.T) {
	ts := NewTraceStorage(5) // Small capacity for testing

	// Add 10 spans (should evict first 5)
	for i := 0; i < 10; i++ {
		traceID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8}
		rs := makeTestSpan(traceID, spanID, "test-service", fmt.Sprintf("span-%d", i))
		err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
		if err != nil {
			t.Fatalf("ReceiveSpans failed: %v", err)
		}
	}

	stats := ts.Stats()
	if stats.SpanCount != 5 {
		t.Errorf("expected 5 spans after wrapping, got %d", stats.SpanCount)
	}

	// GetAllSpans should return the 5 most recent
	all := ts.GetAllSpans()
	if len(all) != 5 {
		t.Fatalf("expected 5 spans from GetAllSpans, got %d", len(all))
	}

	// Verify we have spans 5-9 (0-4 were evicted)
	for i, span := range all {
		expectedName := fmt.Sprintf("span-%d", i+5)
		if span.SpanName != expectedName {
			t.Errorf("at index %d: expected %q, got %q", i, expectedName, span.SpanName)
		}
	}

	// Note: Trace index still has all 10 traces (unbounded for MVP)
	if stats.TraceCount != 10 {
		t.Logf("Note: trace index has %d traces (unbounded for MVP)", stats.TraceCount)
	}
}

// TestTraceStorageClear tests the Clear method.
func TestTraceStorageClear(t *testing.T) {
	ts := NewTraceStorage(100)

	// Add some spans
	for i := 0; i < 5; i++ {
		traceID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID := []byte{byte(i), 2, 3, 4, 5, 6, 7, 8}
		rs := makeTestSpan(traceID, spanID, "test-service", fmt.Sprintf("span-%d", i))
		err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
		if err != nil {
			t.Fatalf("ReceiveSpans failed: %v", err)
		}
	}

	stats := ts.Stats()
	if stats.SpanCount != 5 {
		t.Fatalf("expected 5 spans before clear, got %d", stats.SpanCount)
	}

	ts.Clear()

	stats = ts.Stats()
	if stats.SpanCount != 0 {
		t.Errorf("expected 0 spans after clear, got %d", stats.SpanCount)
	}
	if stats.TraceCount != 0 {
		t.Errorf("expected 0 traces after clear, got %d", stats.TraceCount)
	}

	all := ts.GetAllSpans()
	if all != nil {
		t.Errorf("expected nil from GetAllSpans after clear, got %v", all)
	}
}

// TestTraceStorageNoServiceName tests handling of resources without service.name.
func TestTraceStorageNoServiceName(t *testing.T) {
	ts := NewTraceStorage(100)

	// Create resource without service.name attribute
	rs := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key: "some.other.attribute",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "value"},
					},
				},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
						SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
						Name:              "test-span",
						StartTimeUnixNano: uint64(time.Now().UnixNano()),
						EndTimeUnixNano:   uint64(time.Now().UnixNano()),
					},
				},
			},
		},
	}

	err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
	if err != nil {
		t.Fatalf("ReceiveSpans failed: %v", err)
	}

	spans := ts.GetRecentSpans(1)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Should default to "unknown"
	if spans[0].ServiceName != "unknown" {
		t.Errorf("expected service name 'unknown', got %q", spans[0].ServiceName)
	}
}

// TestTraceStorageConcurrent tests thread-safety under concurrent access.
func TestTraceStorageConcurrent(t *testing.T) {
	ts := NewTraceStorage(1000)

	var wg sync.WaitGroup

	// Concurrent writers
	writers := 10
	spansPerWriter := 10

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for i := 0; i < spansPerWriter; i++ {
				traceID := []byte{byte(writerID), byte(i), 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
				spanID := []byte{byte(writerID), byte(i), 3, 4, 5, 6, 7, 8}
				rs := makeTestSpan(traceID, spanID, fmt.Sprintf("service-%d", writerID), fmt.Sprintf("span-%d", i))

				err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
				if err != nil {
					t.Errorf("ReceiveSpans failed: %v", err)
				}
			}
		}(w)
	}

	// Concurrent readers
	readers := 5
	readsPerReader := 20

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < readsPerReader; i++ {
				_ = ts.GetAllSpans()
				_ = ts.GetRecentSpans(10)
				_ = ts.GetSpansByService("service-0")
				_ = ts.Stats()
			}
		}()
	}

	wg.Wait()

	// Verify we got all spans
	stats := ts.Stats()
	expected := writers * spansPerWriter
	if stats.SpanCount != expected {
		t.Errorf("expected %d spans after concurrent writes, got %d", expected, stats.SpanCount)
	}
}

// TestTraceStorageMultipleSpansInResourceSpans tests handling multiple spans in a single ResourceSpans.
func TestTraceStorageMultipleSpansInResourceSpans(t *testing.T) {
	ts := NewTraceStorage(100)

	// Create ResourceSpans with multiple spans
	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	rs := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key: "service.name",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{StringValue: "multi-span-service"},
					},
				},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
						Name:              "span-1",
						StartTimeUnixNano: uint64(time.Now().UnixNano()),
						EndTimeUnixNano:   uint64(time.Now().UnixNano()),
					},
					{
						TraceId:           traceID,
						SpanId:            []byte{2, 2, 3, 4, 5, 6, 7, 8},
						Name:              "span-2",
						StartTimeUnixNano: uint64(time.Now().UnixNano()),
						EndTimeUnixNano:   uint64(time.Now().UnixNano()),
					},
					{
						TraceId:           traceID,
						SpanId:            []byte{3, 2, 3, 4, 5, 6, 7, 8},
						Name:              "span-3",
						StartTimeUnixNano: uint64(time.Now().UnixNano()),
						EndTimeUnixNano:   uint64(time.Now().UnixNano()),
					},
				},
			},
		},
	}

	err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
	if err != nil {
		t.Fatalf("ReceiveSpans failed: %v", err)
	}

	// Should have stored all 3 spans
	stats := ts.Stats()
	if stats.SpanCount != 3 {
		t.Fatalf("expected 3 spans, got %d", stats.SpanCount)
	}

	// All should be queryable by trace ID
	spans := ts.GetSpansByTraceID(traceIDToString(traceID))
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans for trace ID, got %d", len(spans))
	}

	// All should have same service name
	for _, span := range spans {
		if span.ServiceName != "multi-span-service" {
			t.Errorf("expected service name 'multi-span-service', got %q", span.ServiceName)
		}
	}
}
