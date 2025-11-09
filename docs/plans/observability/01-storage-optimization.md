# Task 01: Index-Free Storage Architecture

## Overview

Simplify the storage layer by eliminating content indexes. Instead, use snapshot-based position tracking for queries. This approach reduces memory usage, eliminates memory leak risks, and aligns with the snapshot-first query model.

## Why This Is Task 01

This task addresses the memory leak in the bootstrap implementation by eliminating the root cause rather than adding cleanup complexity:
- **Eliminates memory leaks** - Indexes were growing unbounded
- **Reduces complexity** - No eviction callbacks or index maintenance needed
- **Reduces memory usage** - Indexes consumed ~40% of total footprint
- **Aligns with snapshot model** - Position-based queries are the primary use case

## Priority 1: Position-Based Queries Instead of Content Indexes

### Design Rationale

Snapshots track positions in ring buffers, providing natural time-based bookmarks. Instead of indexing by content (trace IDs, service names), queries use position ranges with in-memory filtering. This approach is simpler and sufficient for typical query volumes.

### Current Issue

**File:** `internal/storage/trace_storage.go` (lines 77-80)

The code acknowledges the problem:
```go
// Note: For MVP, the trace index grows unbounded.
// This is acceptable for short agent sessions (minutes to hours).
// Future enhancement: Track which spans are evicted from ring buffer
// and remove them from the index as well.
```

**Issues with content indexes:**
1. **Memory leak** - traceIndex and serviceIndex grow unbounded
2. **Maintenance complexity** - Requires eviction callbacks and synchronization
3. **Memory overhead** - Indexes consume ~40% of total storage footprint
4. **Misaligned with usage** - Snapshot queries use time ranges, not content lookups

### Solution: Position-Based Queries with In-Memory Filtering

**Step 1:** Add position-based queries to **`RingBuffer`**

```go
// internal/storage/ringbuffer.go

// GetRange returns items between start and end positions (inclusive).
// Handles wraparound correctly. Returns empty slice if range is invalid.
func (rb *RingBuffer[T]) GetRange(start, end int) []T {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    if rb.size == 0 || start < 0 || end < start {
        return nil
    }

    // Calculate actual positions in the circular buffer
    result := make([]T, 0, end-start+1)

    for pos := start; pos <= end && pos < rb.head+rb.size; pos++ {
        idx := pos % rb.capacity
        result = append(result, rb.items[idx])
    }

    return result
}

// CurrentPosition returns the current write position (next item will go here).
// This is used by snapshots to bookmark a point in time.
func (rb *RingBuffer[T]) CurrentPosition() int {
    rb.mu.RLock()
    defer rb.mu.RUnlock()
    // Return absolute position (not modulo capacity)
    // This allows snapshots to track total items added
    return rb.head + (rb.size / rb.capacity) * rb.capacity
}
```

**Step 2:** Simplify **`TraceStorage`** - remove content indexes

```go
// internal/storage/trace_storage.go

type TraceStorage struct {
    spans *RingBuffer[*StoredSpan]
    // traceIndex REMOVED
    // serviceIndex REMOVED
    // mu REMOVED (RingBuffer is already thread-safe)
}

func NewTraceStorage(capacity int) *TraceStorage {
    return &TraceStorage{
        spans: NewRingBuffer[*StoredSpan](capacity),
    }
}

// Position-based queries (for snapshots)
func (ts *TraceStorage) GetRange(start, end int) []*StoredSpan {
    return ts.spans.GetRange(start, end)
}

func (ts *TraceStorage) CurrentPosition() int {
    return ts.spans.CurrentPosition()
}
```

### Testing Position-Based Queries

**Test Case:**
```go
func TestPositionBasedQueries(t *testing.T) {
    storage := NewTraceStorage(100)

    // Capture position before adding spans
    pos1 := storage.CurrentPosition()

    // Add some spans
    storage.AddSpan(span1)
    storage.AddSpan(span2)
    storage.AddSpan(span3)

    pos2 := storage.CurrentPosition()

    // Add more spans
    storage.AddSpan(span4)
    storage.AddSpan(span5)

    pos3 := storage.CurrentPosition()

    // Get range between pos1 and pos2 (should be 3 spans)
    spans := storage.GetRange(pos1, pos2-1)
    if len(spans) != 3 {
        t.Errorf("expected 3 spans, got %d", len(spans))
    }

    // Get range between pos2 and pos3 (should be 2 spans)
    spans = storage.GetRange(pos2, pos3-1)
    if len(spans) != 2 {
        t.Errorf("expected 2 spans, got %d", len(spans))
    }

    // Filter by trace ID in-memory (no index needed)
    allSpans := storage.GetRange(pos1, pos3-1)
    filtered := FilterByTraceID(allSpans, "trace-123")
}
```

## Priority 2: Filtering Utilities

### Storage Characteristics

| Signal | Size per Entry | Frequency | Typical Query Size |
|--------|---------------|-----------|-------------------|
| Traces | ~500 bytes | Medium | 100-1000 spans |
| Logs | ~200 bytes | High | 500-5000 logs |
| Metrics | ~150 bytes | Very High | 1000-10000 points |

**Key insight:** Linear filtering of 1000 items takes microseconds. Network latency is ~50-200ms. The filtering overhead is negligible compared to network transfer.

### Storage Design (Simplified)

All three signal types follow the same pattern without content indexes:

```go
type TraceStorage struct {
    spans *RingBuffer[*StoredSpan]
}

type LogStorage struct {
    logs *RingBuffer[*LogRecord]
}

type MetricStorage struct {
    metrics *RingBuffer[*MetricPoint]
}
```

### Filtering Functions (In-Memory)

```go
// internal/storage/filters.go

func FilterSpansByTraceID(spans []*StoredSpan, traceID string) []*StoredSpan {
    result := make([]*StoredSpan, 0, len(spans)/10) // estimate 10% match
    for _, span := range spans {
        if span.TraceID == traceID {
            result = append(result, span)
        }
    }
    return result
}

func FilterSpansByService(spans []*StoredSpan, service string) []*StoredSpan {
    result := make([]*StoredSpan, 0, len(spans)/5) // estimate 20% match
    for _, span := range spans {
        if span.ServiceName == service {
            result = append(result, span)
        }
    }
    return result
}

func FilterLogsBySeverity(logs []*LogRecord, severity string) []*LogRecord {
    result := make([]*LogRecord, 0, len(logs)/20) // estimate 5% match
    for _, log := range logs {
        if log.Severity == severity {
            result = append(result, log)
        }
    }
    return result
}

func FilterMetricsByName(metrics []*MetricPoint, name string) []*MetricPoint {
    result := make([]*MetricPoint, 0, len(metrics)/100) // estimate 1% match
    for _, metric := range metrics {
        if metric.Name == name {
            result = append(result, metric)
        }
    }
    return result
}

// Combine multiple filters with AND logic
func FilterSpans(spans []*StoredSpan, opts FilterOptions) []*StoredSpan {
    result := spans
    if opts.TraceID != "" {
        result = FilterSpansByTraceID(result, opts.TraceID)
    }
    if opts.Service != "" {
        result = FilterSpansByService(result, opts.Service)
    }
    if opts.SpanName != "" {
        result = FilterSpansByName(result, opts.SpanName)
    }
    return result
}
```

## Priority 3: Snapshot Manager

### Position Tracking Implementation

```go
// internal/storage/snapshot_manager.go

type SnapshotManager struct {
    snapshots map[string]*Snapshot
    mu        sync.RWMutex
}

type Snapshot struct {
    Name      string
    CreatedAt time.Time
    TracePos  int // Position in trace buffer
    LogPos    int // Position in log buffer
    MetricPos int // Position in metric buffer
}

func NewSnapshotManager() *SnapshotManager {
    return &SnapshotManager{
        snapshots: make(map[string]*Snapshot),
    }
}

func (sm *SnapshotManager) Create(name string, tracePos, logPos, metricPos int) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if _, exists := sm.snapshots[name]; exists {
        return fmt.Errorf("snapshot %s already exists", name)
    }

    sm.snapshots[name] = &Snapshot{
        Name:      name,
        CreatedAt: time.Now(),
        TracePos:  tracePos,
        LogPos:    logPos,
        MetricPos: metricPos,
    }

    return nil
}

func (sm *SnapshotManager) Get(name string) (*Snapshot, error) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()

    snap, exists := sm.snapshots[name]
    if !exists {
        return nil, fmt.Errorf("snapshot %s not found", name)
    }

    return snap, nil
}

func (sm *SnapshotManager) List() []string {
    sm.mu.RLock()
    defer sm.mu.RUnlock()

    names := make([]string, 0, len(sm.snapshots))
    for name := range sm.snapshots {
        names = append(names, name)
    }
    return names
}

func (sm *SnapshotManager) Delete(name string) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if _, exists := sm.snapshots[name]; !exists {
        return fmt.Errorf("snapshot %s not found", name)
    }

    delete(sm.snapshots, name)
    return nil
}
```

### Memory Tracking (Simplified)

```go
type StorageStats struct {
    Capacity   int   `json:"capacity"`
    Size       int   `json:"size"`
    DataMemory int64 `json:"data_memory_bytes"`
}

func (ts *TraceStorage) Stats() StorageStats {
    size := ts.spans.Size()

    return StorageStats{
        Capacity:   ts.spans.Capacity(),
        Size:       size,
        DataMemory: int64(size * estimatedSpanSize),
    }
}
```

## Performance Characteristics

### Query Performance

**Typical snapshot query:**
- Range size: 100-5000 items
- Filter scan: O(n) where n = range size
- Time: <1ms for 5000 items
- Network latency: 50-200ms
- **Conclusion:** Filtering overhead is negligible

**Search across all buffers:**
- Full scan: O(n) where n = buffer size (10K spans)
- Time: ~1ms for 10K items
- Use case: Rare needle-in-haystack searches
- **Conclusion:** Acceptable for debugging scenarios

### Memory Comparison

**Old (with indexes):**
- Ring buffer: 500 KB
- Trace index: 250 KB
- Service index: 100 KB
- **Total: 850 KB**

**New (index-free):**
- Ring buffer: 500 KB
- Snapshots: 2.4 KB (100 snapshots × 24 bytes)
- **Total: 502 KB**

**Savings: 348 KB (40% reduction)**

## Implementation Order

1. **Add position queries to RingBuffer** (`GetRange`, `CurrentPosition`)
2. **Remove indexes from TraceStorage** (delete traceIndex, serviceIndex, mu)
3. **Create filtering utilities** (`internal/storage/filters.go`)
4. **Build SnapshotManager** (`internal/storage/snapshot_manager.go`)
5. **Update tests** to use position-based queries
6. **Verify memory profile** (no unbounded growth)

## Definition of Done

- [ ] `GetRange(start, end)` added to `internal/storage/ringbuffer.go`
- [ ] `CurrentPosition()` added to `internal/storage/ringbuffer.go`
- [ ] All indexes removed from `internal/storage/trace_storage.go`
- [ ] Filtering functions created in `internal/storage/filters.go`
- [ ] `SnapshotManager` implemented in `internal/storage/snapshot_manager.go`
- [ ] Tests updated to use position-based queries
- [ ] Performance verified: <1ms for 5K item scan
- [ ] Memory verified: No unbounded growth
- [ ] All existing tests pass

---

**Priority:** High (addresses memory leak and simplifies architecture)
**Estimated Effort:** 2-3 hours
**Dependencies:** None (can be done immediately)
**Impact:** 40% memory reduction, significant complexity reduction, eliminates memory leak risk
