# Task 03: Storage Optimization

## Overview

Optimize the ring buffer storage layer to prevent memory leaks, improve performance, and handle multiple signal types efficiently.

**Note:** Task renumbered from 04 to 03 after merging events (former Task 03) into MCP tools (now Task 04).

## Priority 1: Fix Index Cleanup (CRITICAL)

### Current Issue

**Memory Leak in Bootstrap Implementation:**

When the ring buffer overwrites an old entry (circular buffer behavior), the indexes are NOT cleaned up. This causes:

1. **Memory leak** - Index maps grow unbounded
2. **Stale data** - Queries return references to overwritten entries
3. **Incorrect results** - Trace/service lookups find deleted spans

**Example Problem:**
```go
// Ring buffer capacity: 10
// Add 11th span → overwrites position 0
// But traceIndex[oldTraceID] still points to position 0!
// Query by oldTraceID returns wrong span (or nil panic)
```

### Current Implementation Review

**File:** `internal/storage/trace_storage.go`

```go
type TraceStorage struct {
    spans       *RingBuffer[Span]
    traceIndex  map[string][]int  // trace_id → positions
    serviceIndex map[string][]int // service_name → positions
    mu          sync.RWMutex
}

func (ts *TraceStorage) AddSpan(span Span) {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    position := ts.spans.Add(span)  // May overwrite old entry!

    // Add to indexes
    ts.traceIndex[span.TraceID] = append(ts.traceIndex[span.TraceID], position)
    ts.serviceIndex[span.ServiceName] = append(ts.serviceIndex[span.ServiceName], position)

    // 🐛 BUG: Old index entries not removed!
}
```

### Solution: Add Eviction Callback

**Step 1:** Modify RingBuffer to support eviction callbacks

```go
// internal/storage/ringbuffer.go

type RingBuffer[T any] struct {
    data     []T
    head     int
    size     int
    capacity int
    onEvict  func(position int, value T) // NEW: callback when item evicted
    mu       sync.RWMutex
}

func (rb *RingBuffer[T]) SetOnEvict(callback func(int, T)) {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    rb.onEvict = callback
}

func (rb *RingBuffer[T]) Add(item T) int {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    position := rb.head

    // If buffer is full, call eviction callback BEFORE overwriting
    if rb.size == rb.capacity && rb.onEvict != nil {
        rb.onEvict(position, rb.data[position])
    }

    rb.data[position] = item
    rb.head = (rb.head + 1) % rb.capacity
    if rb.size < rb.capacity {
        rb.size++
    }

    return position
}
```

**Step 2:** Use eviction callback in TraceStorage

```go
// internal/storage/trace_storage.go

func NewTraceStorage(capacity int) *TraceStorage {
    ts := &TraceStorage{
        spans:        NewRingBuffer[Span](capacity),
        traceIndex:   make(map[string][]int),
        serviceIndex: make(map[string][]int),
    }

    // Set up eviction callback
    ts.spans.SetOnEvict(func(position int, oldSpan Span) {
        ts.removeFromIndexes(position, oldSpan)
    })

    return ts
}

func (ts *TraceStorage) removeFromIndexes(position int, oldSpan Span) {
    // Remove position from trace index
    positions := ts.traceIndex[oldSpan.TraceID]
    ts.traceIndex[oldSpan.TraceID] = removePosition(positions, position)
    if len(ts.traceIndex[oldSpan.TraceID]) == 0 {
        delete(ts.traceIndex, oldSpan.TraceID)
    }

    // Remove position from service index
    positions = ts.serviceIndex[oldSpan.ServiceName]
    ts.serviceIndex[oldSpan.ServiceName] = removePosition(positions, position)
    if len(ts.serviceIndex[oldSpan.ServiceName]) == 0 {
        delete(ts.serviceIndex, oldSpan.ServiceName)
    }
}

func removePosition(positions []int, position int) []int {
    result := make([]int, 0, len(positions))
    for _, p := range positions {
        if p != position {
            result = append(result, p)
        }
    }
    return result
}
```

### Testing the Fix

**Test Case:**
```go
func TestIndexCleanupOnOverwrite(t *testing.T) {
    storage := NewTraceStorage(3) // Small buffer for testing

    // Add 3 spans
    span1 := Span{TraceID: "trace1", ServiceName: "svc1", SpanName: "span1"}
    span2 := Span{TraceID: "trace2", ServiceName: "svc1", SpanName: "span2"}
    span3 := Span{TraceID: "trace3", ServiceName: "svc2", SpanName: "span3"}

    storage.AddSpan(span1)
    storage.AddSpan(span2)
    storage.AddSpan(span3)

    // Verify indexes
    if len(storage.GetSpansByTraceID("trace1")) != 1 {
        t.Error("trace1 should have 1 span")
    }
    if len(storage.GetSpansByService("svc1")) != 2 {
        t.Error("svc1 should have 2 spans")
    }

    // Add 4th span → overwrites span1
    span4 := Span{TraceID: "trace4", ServiceName: "svc2", SpanName: "span4"}
    storage.AddSpan(span4)

    // ✅ trace1 index should be cleaned up
    if len(storage.GetSpansByTraceID("trace1")) != 0 {
        t.Error("trace1 index should be removed")
    }

    // ✅ svc1 index should have 1 span (not 2)
    if len(storage.GetSpansByService("svc1")) != 1 {
        t.Error("svc1 should have 1 span after eviction")
    }

    // ✅ svc2 index should have 2 spans
    if len(storage.GetSpansByService("svc2")) != 2 {
        t.Error("svc2 should have 2 spans")
    }
}
```

## Priority 2: Optimize for Different Signal Types

### Storage Characteristics

| Signal | Size per Entry | Frequency | Index Needs |
|--------|---------------|-----------|-------------|
| Traces | ~500 bytes | Medium | trace_id, service, span_name |
| Logs | ~200 bytes | High | trace_id, severity, service |
| Metrics | ~150 bytes | Very High | metric_name, service, type |

### Optimizations

**1. Separate Ring Buffers** (already planned)
- TraceStorage (10K capacity)
- LogStorage (50K capacity)
- MetricStorage (100K capacity)

**2. Index Strategies**

**Traces:** Current approach works
- trace_id → positions (primary lookup)
- service_name → positions (filtering)

**Logs:** Add severity index
```go
type LogStorage struct {
    logs          *RingBuffer[LogRecord]
    traceIndex    map[string][]int  // trace_id → positions
    serviceIndex  map[string][]int  // service → positions
    severityIndex map[string][]int  // severity → positions (NEW)
}
```

**Metrics:** Add metric name index
```go
type MetricStorage struct {
    metrics     *RingBuffer[MetricPoint]
    nameIndex   map[string][]int  // metric_name → positions (primary)
    serviceIndex map[string][]int // service → positions
    typeIndex   map[string][]int  // type → positions (gauge/sum/histogram)
}
```

## Priority 3: Memory Usage Monitoring

### Add Stats Tracking

```go
type StorageStats struct {
    Capacity      int           `json:"capacity"`
    SpanCount     int           `json:"span_count"`
    TraceCount    int           `json:"trace_count"`

    // NEW: Memory tracking
    IndexMemory   int64         `json:"index_memory_bytes"`
    DataMemory    int64         `json:"data_memory_bytes"`
    TotalMemory   int64         `json:"total_memory_bytes"`

    // NEW: Index stats
    TraceIndexSize   int        `json:"trace_index_entries"`
    ServiceIndexSize int        `json:"service_index_entries"`
}

func (ts *TraceStorage) Stats() StorageStats {
    ts.mu.RLock()
    defer ts.mu.RUnlock()

    dataMemory := int64(ts.spans.Size() * estimatedSpanSize)

    // Estimate index memory
    indexMemory := int64(0)
    for _, positions := range ts.traceIndex {
        indexMemory += int64(len(positions) * 8) // 8 bytes per int
    }
    for _, positions := range ts.serviceIndex {
        indexMemory += int64(len(positions) * 8)
    }

    return StorageStats{
        Capacity:         ts.spans.Capacity(),
        SpanCount:        ts.spans.Size(),
        TraceCount:       len(ts.traceIndex),
        IndexMemory:      indexMemory,
        DataMemory:       dataMemory,
        TotalMemory:      indexMemory + dataMemory,
        TraceIndexSize:   len(ts.traceIndex),
        ServiceIndexSize: len(ts.serviceIndex),
    }
}
```

## Priority 4: Compression (Future)

For metrics (high volume, repetitive data):

```go
// Compress metric points with delta encoding
type CompressedMetricPoint struct {
    Name      string
    Value     float64
    TimeDelta int64  // Delta from previous point
    // Attributes stored in separate dictionary
}
```

**Deferred to post-observability phase** - adds complexity.

## Success Criteria

1. ✅ Index cleanup on ring buffer overwrite
2. ✅ No memory leaks from stale indexes
3. ✅ Queries return correct results after wraparound
4. ✅ Memory usage tracking in stats
5. ✅ All tests pass including overwrite scenarios
6. ✅ Performance: <1ms for index cleanup on eviction

## Implementation Order

1. **First:** Fix index cleanup (critical bug)
2. **Second:** Add memory tracking to stats
3. **Third:** Apply pattern to logs and metrics storage
4. **Fourth:** Add comprehensive tests
5. **Fifth:** Consider compression (optional)

## Files to Modify

- `internal/storage/ringbuffer.go` - Add eviction callback
- `internal/storage/trace_storage.go` - Implement cleanup
- `internal/storage/trace_storage_test.go` - Add overwrite tests
- `internal/storage/log_storage.go` - New file (same pattern)
- `internal/storage/metric_storage.go` - New file (same pattern)

## Acceptance Criteria

- [ ] Ring buffer supports eviction callbacks
- [ ] TraceStorage cleans up indexes on overwrite
- [ ] LogStorage cleans up indexes on overwrite
- [ ] MetricStorage cleans up indexes on overwrite
- [ ] Stats include memory usage estimates
- [ ] Tests verify no memory leaks
- [ ] Tests verify correct results after buffer wraparound
- [ ] Documentation updated with memory characteristics

---

**Priority:** HIGH (fixes critical memory leak)
**Estimated Effort:** 2-3 hours
**Dependencies:** None (can be done immediately)
