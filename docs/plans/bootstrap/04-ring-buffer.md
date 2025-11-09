# Task 04: Ring Buffer Storage

## Why

Need efficient, memory-bounded storage for telemetry data. Ring buffers provide O(1) insertion with automatic eviction of oldest entries, perfect for our use case where we want the most recent N spans/logs/metrics.

## What

Implement:
- Generic ring buffer data structure
- Thread-safe operations (agents may query while data arrives)
- Trace-specific storage wrapper
- Query capabilities (by ID, by service, etc.)

## Approach

### Generic Ring Buffer

```go
// internal/storage/ringbuffer.go
package storage

import "sync"

// RingBuffer is a generic thread-safe ring buffer
type RingBuffer[T any] struct {
    mu       sync.RWMutex
    items    []T
    capacity int
    head     int  // next write position
    size     int  // current number of items
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
    return &RingBuffer[T]{
        items:    make([]T, capacity),
        capacity: capacity,
        head:     0,
        size:     0,
    }
}

// Add inserts an item, evicting the oldest if at capacity
func (rb *RingBuffer[T]) Add(item T) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    rb.items[rb.head] = item
    rb.head = (rb.head + 1) % rb.capacity

    if rb.size < rb.capacity {
        rb.size++
    }
}

// GetAll returns all items in chronological order (oldest to newest)
func (rb *RingBuffer[T]) GetAll() []T {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    if rb.size == 0 {
        return nil
    }

    result := make([]T, rb.size)

    if rb.size < rb.capacity {
        // Haven't wrapped yet
        copy(result, rb.items[:rb.size])
    } else {
        // Wrapped - head points to oldest item
        n := copy(result, rb.items[rb.head:])
        copy(result[n:], rb.items[:rb.head])
    }

    return result
}

// GetRecent returns the N most recent items
func (rb *RingBuffer[T]) GetRecent(n int) []T {
    all := rb.GetAll()
    if len(all) <= n {
        return all
    }
    return all[len(all)-n:]
}

// Size returns the current number of items
func (rb *RingBuffer[T]) Size() int {
    rb.mu.RLock()
    defer rb.mu.RUnlock()
    return rb.size
}

// Capacity returns the maximum capacity
func (rb *RingBuffer[T]) Capacity() int {
    return rb.capacity
}

// Clear removes all items
func (rb *RingBuffer[T]) Clear() {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    rb.size = 0
    rb.head = 0
}
```

### Trace Storage

```go
// internal/storage/trace_storage.go
package storage

import (
    "context"
    "fmt"

    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// StoredSpan wraps a protobuf span with indexed fields for querying
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

// TraceStorage stores and indexes OTLP spans
type TraceStorage struct {
    spans       *RingBuffer[*StoredSpan]
    traceIndex  map[string][]*StoredSpan  // trace_id -> spans
    mu          sync.RWMutex               // for index updates
}

// NewTraceStorage creates a new trace storage
func NewTraceStorage(capacity int) *TraceStorage {
    return &TraceStorage{
        spans:      NewRingBuffer[*StoredSpan](capacity),
        traceIndex: make(map[string][]*StoredSpan),
    }
}

// ReceiveSpans implements otlpreceiver.SpanReceiver
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

func (ts *TraceStorage) addSpan(span *StoredSpan) {
    ts.spans.Add(span)

    // Update index
    ts.mu.Lock()
    ts.traceIndex[span.TraceID] = append(ts.traceIndex[span.TraceID], span)
    ts.mu.Unlock()

    // TODO: Evict old entries from index when ring buffer wraps
    // For MVP, index will grow unbounded - acceptable for short sessions
}

// GetRecentSpans returns the N most recent spans
func (ts *TraceStorage) GetRecentSpans(n int) []*StoredSpan {
    return ts.spans.GetRecent(n)
}

// GetSpansByTraceID returns all spans for a given trace ID
func (ts *TraceStorage) GetSpansByTraceID(traceID string) []*StoredSpan {
    ts.mu.RLock()
    defer ts.mu.RUnlock()
    return ts.traceIndex[traceID]
}

// GetSpansByService returns all spans for a given service name
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

// Stats returns storage statistics
func (ts *TraceStorage) Stats() StorageStats {
    ts.mu.RLock()
    defer ts.mu.RUnlock()

    return StorageStats{
        SpanCount:     ts.spans.Size(),
        Capacity:      ts.spans.Capacity(),
        TraceCount:    len(ts.traceIndex),
    }
}

// Clear removes all stored spans
func (ts *TraceStorage) Clear() {
    ts.spans.Clear()

    ts.mu.Lock()
    ts.traceIndex = make(map[string][]*StoredSpan)
    ts.mu.Unlock()
}

type StorageStats struct {
    SpanCount  int
    Capacity   int
    TraceCount int
}

// Helper functions
func extractServiceName(resource *tracepb.Resource) string {
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

func traceIDToString(traceID []byte) string {
    return fmt.Sprintf("%x", traceID)
}

func spanIDToString(spanID []byte) string {
    return fmt.Sprintf("%x", spanID)
}
```

## Dependencies

- Task 01 (project-setup) must be complete
- Works independently of tasks 02-03, but integrates with 03

## Acceptance Criteria

- [ ] Generic `RingBuffer[T]` implementation works correctly
- [ ] `TraceStorage` implements `SpanReceiver` interface
- [ ] Spans can be added and retrieved
- [ ] Recent spans ordered correctly (oldest to newest)
- [ ] Query by trace ID works
- [ ] Query by service name works
- [ ] Thread-safe under concurrent access
- [ ] Clear() properly resets state

## Testing

### Ring Buffer Tests

```go
// internal/storage/ringbuffer_test.go
package storage

import "testing"

func TestRingBuffer(t *testing.T) {
    rb := NewRingBuffer[int](3)

    // Test basic add/get
    rb.Add(1)
    rb.Add(2)
    rb.Add(3)

    all := rb.GetAll()
    if len(all) != 3 {
        t.Fatalf("expected 3 items, got %d", len(all))
    }

    // Test wrapping
    rb.Add(4)
    all = rb.GetAll()
    if len(all) != 3 || all[0] != 2 || all[2] != 4 {
        t.Fatalf("unexpected items after wrap: %v", all)
    }
}

func TestRingBufferGetRecent(t *testing.T) {
    rb := NewRingBuffer[int](10)
    for i := 0; i < 5; i++ {
        rb.Add(i)
    }

    recent := rb.GetRecent(3)
    if len(recent) != 3 || recent[0] != 2 || recent[2] != 4 {
        t.Fatalf("unexpected recent items: %v", recent)
    }
}

func TestRingBufferConcurrent(t *testing.T) {
    rb := NewRingBuffer[int](1000)

    // Concurrent writes
    done := make(chan bool)
    for i := 0; i < 10; i++ {
        go func(start int) {
            for j := 0; j < 100; j++ {
                rb.Add(start*100 + j)
            }
            done <- true
        }(i)
    }

    // Concurrent reads
    for i := 0; i < 5; i++ {
        go func() {
            for j := 0; j < 100; j++ {
                _ = rb.GetAll()
                _ = rb.Size()
            }
            done <- true
        }()
    }

    // Wait for all goroutines
    for i := 0; i < 15; i++ {
        <-done
    }

    // Should have exactly 1000 items
    if rb.Size() != 1000 {
        t.Fatalf("expected 1000 items, got %d", rb.Size())
    }
}
```

### Trace Storage Tests

```go
// internal/storage/trace_storage_test.go
package storage

import (
    "context"
    "testing"

    commonpb "go.opentelemetry.io/proto/otlp/common/v1"
    resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestTraceStorage(t *testing.T) {
    ts := NewTraceStorage(100)

    // Create test span
    rs := &tracepb.ResourceSpans{
        Resource: &resourcepb.Resource{
            Attributes: []*commonpb.KeyValue{
                {
                    Key: "service.name",
                    Value: &commonpb.AnyValue{
                        Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"},
                    },
                },
            },
        },
        ScopeSpans: []*tracepb.ScopeSpans{
            {
                Spans: []*tracepb.Span{
                    {
                        TraceId: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
                        SpanId:  []byte{1, 2, 3, 4, 5, 6, 7, 8},
                        Name:    "test-span",
                    },
                },
            },
        },
    }

    // Add span
    err := ts.ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs})
    if err != nil {
        t.Fatal(err)
    }

    // Verify storage
    recent := ts.GetRecentSpans(10)
    if len(recent) != 1 {
        t.Fatalf("expected 1 span, got %d", len(recent))
    }

    if recent[0].ServiceName != "test-service" {
        t.Fatalf("unexpected service name: %s", recent[0].ServiceName)
    }

    // Test query by trace ID
    spans := ts.GetSpansByTraceID(traceIDToString(rs.ScopeSpans[0].Spans[0].TraceId))
    if len(spans) != 1 {
        t.Fatalf("expected 1 span by trace ID, got %d", len(spans))
    }
}
```

## Notes

### Future Improvements

- **Index Eviction**: Currently index grows unbounded - should evict entries when ring buffer wraps
- **Additional Indexes**: Could index by span name, status code, duration, etc.
- **Query Language**: More sophisticated filtering (AND/OR conditions)
- **Statistics**: Track min/max/avg span durations, error rates, etc.

### Memory Considerations

For default capacity of 10,000 spans:
- Average span size: ~1-2 KB
- Total memory: ~10-20 MB
- Very reasonable for modern systems

## Status

Status: pending
Depends: 01-project-setup
Next: 05-mcp-server.md
