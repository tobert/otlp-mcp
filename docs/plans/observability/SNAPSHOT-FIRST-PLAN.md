# 📸 Snapshot-First Observability Plan

## Revolutionary Simplification: From 26 Tools to 5

This plan completely reimagines the observability extension around **snapshots as the primary abstraction**. Instead of 26 signal-specific tools, we provide just 5 intuitive tools that align with how agents actually think about telemetry.

## Core Insight

Agents don't think "get traces, then correlate logs, then query metrics."
They think **"what happened during the deployment?"**

Snapshots make this natural.

## The 5-Tool Architecture

```typescript
// The entire MCP interface - just 5 tools!

1. snapshot.create(name: string)
   // Mark a point in time

2. snapshot.get(from: string, to?: string, options?: FilterOptions)
   // Get all telemetry from a time window

3. snapshot.diff(before: string, after: string)
   // Compare two snapshots (what changed?)

4. telemetry.recent(limit?: number, options?: FilterOptions)
   // Get recent data when no snapshot exists

5. telemetry.search(query: string, from?: string, to?: string)
   // Search across all signals
```

## Architecture (Same Infrastructure, New Interface)

```
┌─────────────────────────────────────────────────────────────────┐
│ otlp-mcp serve                                                  │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  OTLP gRPC Server (localhost:XXXXX)                         │ │
│ │  • /v1/traces   ✅                                           │ │
│ │  • /v1/logs     🆕                                           │ │
│ │  • /v1/metrics  🆕                                           │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        ▼                                        │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  Ring Buffer Storage + Snapshot Manager                     │ │
│ │  • TraceBuffer (10K)                                        │ │
│ │  • LogBuffer (50K)                                          │ │
│ │  • MetricBuffer (100K)                                      │ │
│ │  • SnapshotIndex (positions only - 24 bytes each!)         │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        ▼                                        │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  MCP Server (stdio) - JUST 5 TOOLS!                         │ │
│ │                                                              │ │
│ │  snapshot.create    snapshot.get    snapshot.diff           │ │
│ │  telemetry.recent   telemetry.search                        │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                        ▼                                        │
│                 Agent (Claude/Gemini/GPT)                       │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Tasks (Reordered for Snapshot-First)

### Task 01: Storage Optimization with Snapshot Support (CRITICAL)
**Status**: Must do first - fixes memory leak!

Fix the critical memory leak AND add snapshot infrastructure:
- ✅ Add SetOnEvict callback to prevent index memory leaks
- ✅ Add snapshot position tracking to each buffer
- ✅ Create SnapshotManager to coordinate across buffers

**File**: `internal/storage/snapshot_manager.go`
```go
type SnapshotManager struct {
    snapshots map[string]*Snapshot
    mu        sync.RWMutex
}

type Snapshot struct {
    Name      string
    CreatedAt time.Time
    TracePos  int  // Position in trace buffer
    LogPos    int  // Position in log buffer
    MetricPos int  // Position in metric buffer
}
```

### Task 02: OTLP Receivers for All Signals
**Status**: Parallel after Task 01

Implement the three OTLP endpoints:
- Traces endpoint (already done from bootstrap)
- Logs endpoint (`internal/otlpreceiver/logs.go`)
- Metrics endpoint (`internal/otlpreceiver/metrics.go`)

All three write to their respective ring buffers.

### Task 03: Unified Telemetry Storage
**Status**: Parallel after Task 01

Create storage that understands snapshots:
- `internal/storage/trace_storage.go` (update existing)
- `internal/storage/log_storage.go` (new)
- `internal/storage/metric_storage.go` (new)

Each storage tracks its current position for snapshot creation.

### Task 04: The 5 MCP Tools
**Status**: After Tasks 02-03

Just 5 tools that do everything!

#### Tool 1: `snapshot.create`
```go
func (s *MCPServer) CreateSnapshot(name string) error {
    snapshot := &Snapshot{
        Name:      name,
        CreatedAt: time.Now(),
        TracePos:  s.traceStorage.CurrentPosition(),
        LogPos:    s.logStorage.CurrentPosition(),
        MetricPos: s.metricStorage.CurrentPosition(),
    }
    return s.snapshotManager.Add(snapshot)
}
```

#### Tool 2: `snapshot.get`
The workhorse - returns mixed signals from a time window:
```go
type SnapshotData struct {
    Summary   Summary    `json:"summary"`
    Timeline  []Event    `json:"timeline"`  // Interleaved
    BySignal  BySignal   `json:"by_signal"` // Separated
}

func (s *MCPServer) GetSnapshot(from, to string, opts FilterOptions) (*SnapshotData, error) {
    fromSnap := s.snapshotManager.Get(from)
    toSnap := s.snapshotManager.Get(to) // or current position

    // Get data from each buffer between positions
    traces := s.traceStorage.GetRange(fromSnap.TracePos, toSnap.TracePos)
    logs := s.logStorage.GetRange(fromSnap.LogPos, toSnap.LogPos)
    metrics := s.metricStorage.GetRange(fromSnap.MetricPos, toSnap.MetricPos)

    // Apply filters if provided
    if opts.Severity != "" {
        logs = filterBySeverity(logs, opts.Severity)
    }

    // Build response with automatic correlation
    return &SnapshotData{
        Summary:  buildSummary(traces, logs, metrics),
        Timeline: interleaveByTime(traces, logs, metrics),
        BySignal: BySignal{traces, logs, metrics},
    }, nil
}
```

#### Tool 3: `snapshot.diff`
Compare two snapshots - what changed?
```go
type SnapshotDiff struct {
    NewErrors      []LogEntry    `json:"new_errors"`
    LatencyChange  LatencyDiff   `json:"latency_change"`
    MetricChanges  []MetricDiff  `json:"metric_changes"`
    Summary        string        `json:"summary"`
}

func (s *MCPServer) DiffSnapshots(before, after string) (*SnapshotDiff, error) {
    beforeData := s.GetSnapshot(before, before, FilterOptions{})
    afterData := s.GetSnapshot(after, after, FilterOptions{})

    return &SnapshotDiff{
        NewErrors:     findNewErrors(beforeData.BySignal.Logs, afterData.BySignal.Logs),
        LatencyChange: compareLatency(beforeData.BySignal.Traces, afterData.BySignal.Traces),
        MetricChanges: compareMetrics(beforeData.BySignal.Metrics, afterData.BySignal.Metrics),
        Summary:       generateDiffSummary(...),
    }, nil
}
```

#### Tool 4: `telemetry.recent`
When there's no snapshot, get recent data:
```go
func (s *MCPServer) GetRecent(limit int, opts FilterOptions) (*SnapshotData, error) {
    // Get last N items from each buffer
    traces := s.traceStorage.GetRecent(limit)
    logs := s.logStorage.GetRecent(limit)
    metrics := s.metricStorage.GetRecent(limit)

    return buildSnapshotData(traces, logs, metrics, opts), nil
}
```

#### Tool 5: `telemetry.search`
Search across all signals:
```go
func (s *MCPServer) Search(query string, from, to string) (*SearchResults, error) {
    // Search in all buffers
    matchingTraces := s.traceStorage.Search(query)
    matchingLogs := s.logStorage.Search(query)
    matchingMetrics := s.metricStorage.Search(query)

    return &SearchResults{
        Traces:  matchingTraces,
        Logs:    matchingLogs,
        Metrics: matchingMetrics,
        Count:   len(matchingTraces) + len(matchingLogs) + len(matchingMetrics),
    }, nil
}
```

### Task 05: Smart Aggregation & Correlation
**Status**: After Task 04

Add intelligence to the responses:
- Automatic error correlation (errors that happened together)
- Anomaly detection (unusual patterns)
- Insights generation ("CPU spike coincided with errors")

### Task 06: Integration Testing
**Status**: After Task 05

Test the complete flow:
1. Start server
2. Send mixed telemetry
3. Create snapshots
4. Query snapshot data
5. Verify correlation

### Task 07: Documentation
**Status**: Final

Update docs with the revolutionary simplicity:
- Show the 5-tool workflow
- Provide agent examples
- Highlight the 80% complexity reduction

## Example Agent Workflows

### Debugging a Deployment
```typescript
// Before deployment
await mcp.call("snapshot.create", { name: "pre-deploy" })

// Deploy...
await deployApplication()

// After deployment
await mcp.call("snapshot.create", { name: "post-deploy" })

// Get everything that happened
const data = await mcp.call("snapshot.get", {
    from: "pre-deploy",
    to: "post-deploy"
})

console.log(data.summary)
// "150 traces (3 errors), 823 logs (12 errors), CPU peaked at 95%"

// Agent: "I see 3 failed requests and high CPU. Let me investigate..."
const errors = await mcp.call("snapshot.get", {
    from: "pre-deploy",
    to: "post-deploy",
    options: { severity: "ERROR" }
})
```

### Comparing Before/After
```typescript
const diff = await mcp.call("snapshot.diff", {
    before: "pre-optimization",
    after: "post-optimization"
})

console.log(diff.summary)
// "Latency improved 34%, memory usage down 20%, no new errors"
```

### Quick Investigation
```typescript
// No snapshot? No problem!
const recent = await mcp.call("telemetry.recent", { limit: 100 })

// Search everything
const errors = await mcp.call("telemetry.search", {
    query: "timeout OR error"
})
```

## Why This Is Revolutionary

### For Agents
- **5 tools instead of 26** - Massive cognitive simplification
- **Natural workflow** - Matches how they think about problems
- **Automatic correlation** - No manual joining needed

### For Context Windows
- **Single query** - Get everything at once
- **Smart filtering** - Only relevant data returned
- **Progressive disclosure** - Summary first, details on demand

### For Users
- **Intuitive prompts** - "Show me the deployment"
- **Faster results** - Agent knows exactly what to do
- **Better insights** - Correlation is automatic

## Migration Path

1. **Keep existing buffers** - Infrastructure doesn't change
2. **Add snapshot layer** - Thin coordination on top
3. **Implement 5 tools** - Replace the 26
4. **Gradual rollout** - Can run both interfaces initially

## Success Metrics

- ✅ 80% reduction in tool count (26 → 5)
- ✅ 90% of queries handled by snapshot.get
- ✅ Single-query correlation vs multi-query manual
- ✅ Agent success rate improvement
- ✅ User satisfaction with simplicity

## The Philosophy

> "Don't make agents think like systems engineers. Let them think like investigators asking 'what happened?'"

This isn't just fewer tools - it's a fundamental rethinking of how agents interact with observability data. By centering on time windows and operations rather than signal types, we align with natural thought patterns.

## Next Steps

1. **Fix memory leak** (Task 01) - Still critical!
2. **Build snapshot manager** - Core abstraction
3. **Implement 5 tools** - Replace complexity with simplicity
4. **Test with agents** - Verify the improvement
5. **Document success** - Show the world a better way

---

*This snapshot-first approach could become the standard for agent-observability interaction. It's not just simpler - it's more aligned with how both humans and LLMs think about system behavior.*