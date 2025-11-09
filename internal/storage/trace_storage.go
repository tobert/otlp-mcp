package storage

import (
	"context"
	"fmt"

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

// TraceStorage stores OTLP trace spans without content indexes.
// Queries use position-based ranges with in-memory filtering.
// It implements the otlpreceiver.SpanReceiver interface.
type TraceStorage struct {
	spans *RingBuffer[*StoredSpan]
}

// NewTraceStorage creates a new trace storage with the specified capacity.
func NewTraceStorage(capacity int) *TraceStorage {
	return &TraceStorage{
		spans: NewRingBuffer[*StoredSpan](capacity),
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

// addSpan adds a span to storage.
func (ts *TraceStorage) addSpan(span *StoredSpan) {
	ts.spans.Add(span)
}

// GetRecentSpans returns the N most recent spans in chronological order.
func (ts *TraceStorage) GetRecentSpans(n int) []*StoredSpan {
	return ts.spans.GetRecent(n)
}

// GetAllSpans returns all stored spans in chronological order (oldest to newest).
func (ts *TraceStorage) GetAllSpans() []*StoredSpan {
	return ts.spans.GetAll()
}

// GetSpansByTraceID returns all currently stored spans for a given trace ID.
// This performs an in-memory scan of all stored spans.
func (ts *TraceStorage) GetSpansByTraceID(traceID string) []*StoredSpan {
	all := ts.spans.GetAll()
	var result []*StoredSpan

	for _, span := range all {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}

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
	// Count unique traces by scanning
	all := ts.spans.GetAll()
	traceIDs := make(map[string]struct{})
	for _, span := range all {
		traceIDs[span.TraceID] = struct{}{}
	}

	return StorageStats{
		SpanCount:  ts.spans.Size(),
		Capacity:   ts.spans.Capacity(),
		TraceCount: len(traceIDs),
	}
}

// Clear removes all stored spans.
func (ts *TraceStorage) Clear() {
	ts.spans.Clear()
}

// GetRange returns spans between start and end positions (inclusive).
// Positions are absolute and represent the logical sequence of spans added.
func (ts *TraceStorage) GetRange(start, end int) []*StoredSpan {
	return ts.spans.GetRange(start, end)
}

// CurrentPosition returns the current write position.
// Used by snapshots to bookmark a point in time.
func (ts *TraceStorage) CurrentPosition() int {
	return ts.spans.CurrentPosition()
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
