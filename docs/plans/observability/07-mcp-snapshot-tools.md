# Task 07: MCP Snapshot Tools 📸

## Overview

Implement the **revolutionary snapshot system** - the most context-efficient feature in otlp-mcp. Snapshots are named bookmarks of ring buffer positions that enable operation isolation and before/after comparisons with **zero data copying**.

**This is a game-changer for agent workflows.**

## The Problem

**Agent workflow challenge:**
```typescript
// Agent wants to analyze a deployment
1. Deploy new code
2. Run tests
3. Analyze ONLY the telemetry from the deployment
4. Compare before/after

// Traditional approach (INEFFICIENT):
- Retrieve ALL traces/logs/metrics after deployment
- Agent must filter in context
- Wastes tokens on irrelevant data
- Can't isolate specific operations
```

## The Solution: Snapshots

**Snapshot approach (REVOLUTIONARY):**
```typescript
// Before deployment
create_snapshot({name: "before-deploy"})
// → Saves positions: {traces: 1000, logs: 5000, metrics: 8000}

// Deploy and test...

// After deployment
create_snapshot({name: "after-deploy"})
// → Saves positions: {traces: 1150, logs: 5300, metrics: 8500}

// Get ONLY deployment data
get_snapshot_data({
  name: "before-deploy",
  end_snapshot: "after-deploy"
})
// → Returns traces[1000-1150], logs[5000-5300], metrics[8000-8500]
// → ONLY 150 traces, 300 logs, 500 metrics from the deployment!
```

**Benefits:**
- ✨ **Zero-copy**: Snapshots are just position bookmarks (24 bytes)
- 🚀 **Ultra-fast**: No data serialization until query time
- 🎯 **Isolated**: Focus on specific operations
- 📊 **Composable**: Use snapshot ranges in any query tool
- 🏷️ **Named**: Human-readable labels ("before-fix", "test-run-3")

---

## Architecture

### Snapshot Data Structure

```go
// File: internal/storage/snapshot.go

package storage

import (
    "sync"
    "time"
)

// Snapshot represents a named bookmark of ring buffer positions.
type Snapshot struct {
    Name      string
    CreatedAt time.Time

    // Ring buffer positions at snapshot time
    TracePos  int
    LogPos    int
    MetricPos int
}

// SnapshotManager manages named snapshots.
type SnapshotManager struct {
    snapshots map[string]*Snapshot
    mu        sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager() *SnapshotManager {
    return &SnapshotManager{
        snapshots: make(map[string]*Snapshot),
    }
}

// Create creates a new snapshot with current buffer positions.
func (sm *SnapshotManager) Create(name string, tracePos, logPos, metricPos int) *Snapshot {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    snapshot := &Snapshot{
        Name:      name,
        CreatedAt: time.Now(),
        TracePos:  tracePos,
        LogPos:    logPos,
        MetricPos: metricPos,
    }

    sm.snapshots[name] = snapshot
    return snapshot
}

// Get retrieves a snapshot by name.
func (sm *SnapshotManager) Get(name string) *Snapshot {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    return sm.snapshots[name]
}

// List returns all snapshots sorted by creation time.
func (sm *SnapshotManager) List() []*Snapshot {
    sm.mu.RLock()
    defer sm.mu.RUnlock()

    snapshots := make([]*Snapshot, 0, len(sm.snapshots))
    for _, snap := range sm.snapshots {
        snapshots = append(snapshots, snap)
    }

    // Sort by creation time
    sort.Slice(snapshots, func(i, j int) bool {
        return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
    })

    return snapshots
}

// Delete removes a snapshot.
func (sm *SnapshotManager) Delete(name string) bool {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    if _, exists := sm.snapshots[name]; exists {
        delete(sm.snapshots, name)
        return true
    }
    return false
}

// Clear removes all snapshots.
func (sm *SnapshotManager) Clear() {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.snapshots = make(map[string]*Snapshot)
}
```

---

## Snapshot Tools (4 total)

### 1. `create_snapshot`

Save current ring buffer positions with a descriptive name.

**Parameters:**
```typescript
{
  name: string,           // Snapshot name (e.g., "before-deploy", "test-1-start")
  description?: string    // Optional description
}
```

**Returns:**
```typescript
{
  name: string,
  created_at: string,     // ISO timestamp
  positions: {
    traces: number,
    logs: number,
    metrics: number
  },
  sizes: {                // Item counts at each position
    traces: number,
    logs: number,
    metrics: number
  }
}
```

**Implementation:**
```go
func (s *Server) handleCreateSnapshot(args map[string]interface{}) (interface{}, error) {
    name, ok := args["name"].(string)
    if !ok || name == "" {
        return nil, fmt.Errorf("name is required")
    }

    // Get current positions from each storage
    tracePos := s.traceStorage.CurrentPosition()
    logPos := s.logStorage.CurrentPosition()
    metricPos := s.metricStorage.CurrentPosition()

    // Create snapshot
    snapshot := s.snapshotManager.Create(name, tracePos, logPos, metricPos)

    // Get current sizes
    traceStats := s.traceStorage.Stats()
    logStats := s.logStorage.Stats()
    metricStats := s.metricStorage.Stats()

    return map[string]interface{}{
        "name": snapshot.Name,
        "created_at": snapshot.CreatedAt.Format(time.RFC3339),
        "positions": map[string]interface{}{
            "traces": snapshot.TracePos,
            "logs": snapshot.LogPos,
            "metrics": snapshot.MetricPos,
        },
        "sizes": map[string]interface{}{
            "traces": traceStats.SpanCount,
            "logs": logStats.LogCount,
            "metrics": metricStats.MetricCount,
        },
    }, nil
}
```

---

### 2. `list_snapshots`

Show all snapshots with their positions and ranges.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  snapshots: Array<{
    name: string,
    created_at: string,
    positions: {traces: number, logs: number, metrics: number},
    age_seconds: number
  }>,
  count: number
}
```

**Implementation:**
```go
func (s *Server) handleListSnapshots(args map[string]interface{}) (interface{}, error) {
    snapshots := s.snapshotManager.List()

    snapshotInfos := make([]map[string]interface{}, len(snapshots))
    for i, snap := range snapshots {
        ageSeconds := time.Since(snap.CreatedAt).Seconds()

        snapshotInfos[i] = map[string]interface{}{
            "name": snap.Name,
            "created_at": snap.CreatedAt.Format(time.RFC3339),
            "positions": map[string]interface{}{
                "traces": snap.TracePos,
                "logs": snap.LogPos,
                "metrics": snap.MetricPos,
            },
            "age_seconds": int(ageSeconds),
        }
    }

    return map[string]interface{}{
        "snapshots": snapshotInfos,
        "count": len(snapshots),
    }, nil
}
```

---

### 3. `get_snapshot_data`

Retrieve all telemetry data from a snapshot range (single or between two snapshots).

**Parameters:**
```typescript
{
  name: string,                // Start snapshot name
  end_snapshot?: string,       // Optional end snapshot (defaults to current)
  include_traces?: boolean,    // Default: true
  include_logs?: boolean,      // Default: true
  include_metrics?: boolean    // Default: true
}
```

**Returns:**
```typescript
{
  snapshot_range: {
    start: string,
    end: string,
    duration_seconds: number
  },
  traces: {
    data: SpanData[],
    count: number,
    range: {start: number, end: number}
  },
  logs: {
    data: LogRecord[],
    count: number,
    range: {start: number, end: number}
  },
  metrics: {
    data: MetricData[],
    count: number,
    range: {start: number, end: number}
  },
  total_items: number
}
```

**Why this tool:** One-shot retrieval of all relevant data for an operation. Perfect for deployment analysis.

**Implementation:**
```go
func (s *Server) handleGetSnapshotData(args map[string]interface{}) (interface{}, error) {
    name, ok := args["name"].(string)
    if !ok || name == "" {
        return nil, fmt.Errorf("name is required")
    }

    startSnapshot := s.snapshotManager.Get(name)
    if startSnapshot == nil {
        return nil, fmt.Errorf("snapshot not found: %s", name)
    }

    // Determine end snapshot
    var endSnapshot *Snapshot
    endName := "current"
    if endSnapName, ok := args["end_snapshot"].(string); ok {
        endSnapshot = s.snapshotManager.Get(endSnapName)
        if endSnapshot == nil {
            return nil, fmt.Errorf("end snapshot not found: %s", endSnapName)
        }
        endName = endSnapName
    } else {
        // Use current positions
        endSnapshot = &Snapshot{
            Name:      "current",
            CreatedAt: time.Now(),
            TracePos:  s.traceStorage.CurrentPosition(),
            LogPos:    s.logStorage.CurrentPosition(),
            MetricPos: s.metricStorage.CurrentPosition(),
        }
    }

    duration := endSnapshot.CreatedAt.Sub(startSnapshot.CreatedAt).Seconds()

    // Retrieve data based on include flags
    includeTraces := getBoolArg(args, "include_traces", true)
    includeLogs := getBoolArg(args, "include_logs", true)
    includeMetrics := getBoolArg(args, "include_metrics", true)

    result := map[string]interface{}{
        "snapshot_range": map[string]interface{}{
            "start": name,
            "end": endName,
            "duration_seconds": duration,
        },
    }

    totalItems := 0

    if includeTraces {
        traces := s.traceStorage.GetRange(startSnapshot.TracePos, endSnapshot.TracePos)
        result["traces"] = map[string]interface{}{
            "data": formatSpansForMCP(traces),
            "count": len(traces),
            "range": map[string]interface{}{
                "start": startSnapshot.TracePos,
                "end": endSnapshot.TracePos,
            },
        }
        totalItems += len(traces)
    }

    if includeLogs {
        logs := s.logStorage.GetRange(startSnapshot.LogPos, endSnapshot.LogPos)
        result["logs"] = map[string]interface{}{
            "data": formatLogsForMCP(logs),
            "count": len(logs),
            "range": map[string]interface{}{
                "start": startSnapshot.LogPos,
                "end": endSnapshot.LogPos,
            },
        }
        totalItems += len(logs)
    }

    if includeMetrics {
        metrics := s.metricStorage.GetRange(startSnapshot.MetricPos, endSnapshot.MetricPos)
        result["metrics"] = map[string]interface{}{
            "data": formatMetricsForMCP(metrics),
            "count": len(metrics),
            "range": map[string]interface{}{
                "start": startSnapshot.MetricPos,
                "end": endSnapshot.MetricPos,
            },
        }
        totalItems += len(metrics)
    }

    result["total_items"] = totalItems

    return result, nil
}
```

---

### 4. `delete_snapshot`

Remove a snapshot bookmark (data stays in buffers).

**Parameters:**
```typescript
{
  name: string    // Snapshot name to delete
}
```

**Returns:**
```typescript
{
  deleted: string,
  success: boolean
}
```

**Implementation:**
```go
func (s *Server) handleDeleteSnapshot(args map[string]interface{}) (interface{}, error) {
    name, ok := args["name"].(string)
    if !ok || name == "" {
        return nil, fmt.Errorf("name is required")
    }

    deleted := s.snapshotManager.Delete(name)
    if !deleted {
        return nil, fmt.Errorf("snapshot not found: %s", name)
    }

    return map[string]interface{}{
        "deleted": name,
        "success": true,
    }, nil
}
```

---

## Storage Modifications Required

**Add to each storage type:**

```go
// File: internal/storage/trace_storage.go (and log/metric equivalents)

// CurrentPosition returns the current write position in the ring buffer.
func (ts *TraceStorage) CurrentPosition() int {
    return ts.spans.CurrentPosition()
}

// GetRange returns spans between start and end positions.
func (ts *TraceStorage) GetRange(start, end int) []*Span {
    return ts.spans.GetRange(start, end)
}
```

**Add to RingBuffer:**

```go
// File: internal/storage/ringbuffer.go

// CurrentPosition returns the current write position.
func (rb *RingBuffer[T]) CurrentPosition() int {
    rb.mu.RLock()
    defer rb.mu.RUnlock()
    return rb.head
}

// GetRange returns items between start and end positions.
func (rb *RingBuffer[T]) GetRange(start, end int) []T {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    if start < 0 || end < start {
        return []T{}
    }

    // Handle wraparound in circular buffer
    result := make([]T, 0, end-start)
    for i := start; i < end && i < rb.size; i++ {
        idx := (rb.head - rb.size + i) % rb.capacity
        if idx < 0 {
            idx += rb.capacity
        }
        result = append(result, rb.data[idx])
    }

    return result
}
```

---

## Example Agent Workflows

### Deployment Analysis
```typescript
// 1. Before deployment
create_snapshot({name: "pre-deploy"})

// 2. Deploy new version
runDeployment()

// 3. After deployment
create_snapshot({name: "post-deploy"})

// 4. Get all deployment telemetry
const data = get_snapshot_data({
  name: "pre-deploy",
  end_snapshot: "post-deploy"
})

// 5. Analyze errors
const errors = grep_logs({
  pattern: "ERROR|FATAL",
  start_snapshot: "pre-deploy",
  end_snapshot: "post-deploy"
})
```

### Test Run Comparison
```typescript
// Test 1
create_snapshot({name: "test-1-start"})
runTests()
create_snapshot({name: "test-1-end"})

// Test 2 (after fix)
create_snapshot({name: "test-2-start"})
runTests()
create_snapshot({name: "test-2-end"})

// Compare metrics
const test1Metrics = get_snapshot_data({
  name: "test-1-start",
  end_snapshot: "test-1-end",
  include_traces: false,
  include_logs: false
})

const test2Metrics = get_snapshot_data({
  name: "test-2-start",
  end_snapshot: "test-2-end",
  include_traces: false,
  include_logs: false
})
```

---

## Tool Registration

**File:** `internal/mcpserver/snapshot_tools.go`

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Server) registerSnapshotTools() error {
    tools := []struct{
        name string
        description string
        schema map[string]interface{}
        handler func(map[string]interface{}) (interface{}, error)
    }{
        {
            name: "create_snapshot",
            description: "Create named snapshot of current buffer positions",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "name": map[string]interface{}{"type": "string"},
                    "description": map[string]interface{}{"type": "string"},
                },
                "required": []string{"name"},
            },
            handler: s.handleCreateSnapshot,
        },
        // ... register remaining 3 tools
    }

    for _, tool := range tools {
        if err := s.mcpServer.AddTool(mcp.Tool{
            Name: tool.name,
            Description: tool.description,
            InputSchema: tool.schema,
        }, tool.handler); err != nil {
            return err
        }
    }

    return nil
}
```

---

## Acceptance Criteria

- [ ] SnapshotManager implemented with thread-safe operations
- [ ] All 4 snapshot tools implemented
- [ ] Ring buffer position tracking working
- [ ] Range queries working across all signal types
- [ ] Snapshot creation/deletion working
- [ ] Multi-signal data retrieval working
- [ ] Selective signal inclusion working (include_traces, etc.)
- [ ] Tool registration working
- [ ] Unit tests for SnapshotManager
- [ ] Unit tests for each tool
- [ ] Integration tests with real workflows
- [ ] Edge case handling (deleted snapshots, invalid ranges)

## Files to Create

- `internal/storage/snapshot.go` - Snapshot and SnapshotManager
- `internal/storage/snapshot_test.go` - Unit tests
- `internal/mcpserver/snapshot_tools.go` - Tool implementations
- `internal/mcpserver/snapshot_tools_test.go` - Unit tests

## Files to Modify

- `internal/storage/ringbuffer.go` - Add `CurrentPosition()` and `GetRange()`
- `internal/storage/trace_storage.go` - Add position/range methods
- `internal/storage/log_storage.go` - Add position/range methods
- `internal/storage/metric_storage.go` - Add position/range methods
- `internal/mcpserver/server.go` - Add SnapshotManager and register tools
- `internal/cli/serve.go` - Initialize SnapshotManager

## Testing Notes

**Test scenarios:**
1. Create snapshot and verify positions
2. List snapshots (sorted by time)
3. Get data between two snapshots
4. Get data from snapshot to current
5. Delete snapshot
6. Invalid snapshot names
7. Snapshot with empty buffers
8. Multiple snapshots in sequence
9. Buffer wraparound handling
10. Selective signal retrieval

---

**Status:** Ready to implement
**Dependencies:** All storage layers (Tasks 01, 02, bootstrap)
**Next:** Task 08 (MCP Correlation Tools)

---

## Why This Matters 🌟

**Before snapshots:**
- Agent retrieves all data, filters in context
- Wastes tokens on irrelevant telemetry
- Can't isolate specific operations
- No before/after comparisons

**With snapshots:**
- Agent bookmarks operations precisely
- Retrieves only relevant data
- Perfect operation isolation
- Easy before/after analysis
- **10-100x context efficiency gains**

This is the most innovative feature in otlp-mcp. 🚀
