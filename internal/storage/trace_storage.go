package storage

import (
	"context"
	"fmt"
	"sync"

	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// StoredSpan wraps a protobuf span with indexed fields for efficient querying.
// It preserves the full OTLP hierarchy: ResourceSpans -> ScopeSpans -> Span.
type StoredSpan struct {
	ResourceSpan *tracepb.ResourceSpans
	ScopeSpan    *tracepb.ScopeSpans
	Span         *tracepb.Span

	// Indexed fields for fast lookup
	TraceID     string
	SpanID      string
	ServiceName string
	SpanName    string
}

// TraceStorage stores and indexes OTLP trace spans.
// It implements the otlpreceiver.SpanReceiver interface.
type TraceStorage struct {
	spans      *RingBuffer[*StoredSpan]
	traceIndex map[string][]*StoredSpan // trace_id -> spans
	mu         sync.RWMutex              // protects traceIndex
}

// NewTraceStorage creates a new trace storage with the specified capacity.
func NewTraceStorage(capacity int) *TraceStorage {
	return &TraceStorage{
		spans:      NewRingBuffer[*StoredSpan](capacity),
		traceIndex: make(map[string][]*StoredSpan),
	}
}

// ReceiveSpans implements otlpreceiver.SpanReceiver.
// It stores received spans and updates indexes for querying.
func (ts *TraceStorage) ReceiveSpans(ctx context.Context, resourceSpans []*tracepb.ResourceSpans) error {
	for _, rs := range resourceSpans {
		serviceName := extractServiceName(rs.Resource)

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				stored := &StoredSpan{
					ResourceSpan: rs,
					ScopeSpan:    ss,
					Span:         span,
					TraceID:      traceIDToString(span.TraceId),
					SpanID:       spanIDToString(span.SpanId),
					ServiceName:  serviceName,
					SpanName:     span.Name,
				}

				ts.addSpan(stored)
			}
		}
	}

	return nil
}

// addSpan adds a span to storage and updates the trace index.
func (ts *TraceStorage) addSpan(span *StoredSpan) {
	ts.spans.Add(span)

	// Update trace index
	ts.mu.Lock()
	ts.traceIndex[span.TraceID] = append(ts.traceIndex[span.TraceID], span)
	ts.mu.Unlock()

	// Note: For MVP, the trace index grows unbounded.
	// This is acceptable for short agent sessions (minutes to hours).
	// Future enhancement: Track which spans are evicted from ring buffer
	// and remove them from the index as well.
}

// GetRecentSpans returns the N most recent spans in chronological order.
func (ts *TraceStorage) GetRecentSpans(n int) []*StoredSpan {
	return ts.spans.GetRecent(n)
}

// GetAllSpans returns all stored spans in chronological order (oldest to newest).
func (ts *TraceStorage) GetAllSpans() []*StoredSpan {
	return ts.spans.GetAll()
}

// GetSpansByTraceID returns all spans for a given trace ID.
// Returns nil if no spans are found for the trace ID.
func (ts *TraceStorage) GetSpansByTraceID(traceID string) []*StoredSpan {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	spans := ts.traceIndex[traceID]
	if len(spans) == 0 {
		return nil
	}

	// Return a copy to avoid concurrent modification issues
	result := make([]*StoredSpan, len(spans))
	copy(result, spans)
	return result
}

// GetSpansByService returns all spans for a given service name.
// This performs a linear scan of all stored spans.
func (ts *TraceStorage) GetSpansByService(serviceName string) []*StoredSpan {
	all := ts.spans.GetAll()
	var result []*StoredSpan

	for _, span := range all {
		if span.ServiceName == serviceName {
			result = append(result, span)
		}
	}

	return result
}

// GetSpansByName returns all spans with a given span name.
// This performs a linear scan of all stored spans.
func (ts *TraceStorage) GetSpansByName(spanName string) []*StoredSpan {
	all := ts.spans.GetAll()
	var result []*StoredSpan

	for _, span := range all {
		if span.SpanName == spanName {
			result = append(result, span)
		}
	}

	return result
}

// Stats returns current storage statistics.
func (ts *TraceStorage) Stats() StorageStats {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return StorageStats{
		SpanCount:  ts.spans.Size(),
		Capacity:   ts.spans.Capacity(),
		TraceCount: len(ts.traceIndex),
	}
}

// Clear removes all stored spans and resets indexes.
func (ts *TraceStorage) Clear() {
	ts.spans.Clear()

	ts.mu.Lock()
	ts.traceIndex = make(map[string][]*StoredSpan)
	ts.mu.Unlock()
}

// StorageStats contains statistics about trace storage.
type StorageStats struct {
	SpanCount  int // Current number of spans stored
	Capacity   int // Maximum number of spans that can be stored
	TraceCount int // Number of distinct traces
}

// extractServiceName extracts the service.name attribute from an OTLP resource.
// Returns "unknown" if the service name is not found.
func extractServiceName(resource *resourcepb.Resource) string {
	if resource == nil {
		return "unknown"
	}

	for _, attr := range resource.Attributes {
		if attr.Key == "service.name" {
			if sv := attr.Value.GetStringValue(); sv != "" {
				return sv
			}
		}
	}

	return "unknown"
}

// traceIDToString converts a trace ID byte array to a hex string.
func traceIDToString(traceID []byte) string {
	return fmt.Sprintf("%x", traceID)
}

// spanIDToString converts a span ID byte array to a hex string.
func spanIDToString(spanID []byte) string {
	return fmt.Sprintf("%x", spanID)
}
