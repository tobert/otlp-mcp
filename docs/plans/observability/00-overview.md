# Observability Plan Overview

## Vision

Extend otlp-mcp to support additional OpenTelemetry signals: **logs** and **metrics**, plus enhanced querying capabilities. This enables agents to gain comprehensive visibility into application behavior across multiple observability signals.

## Current State (Bootstrap Complete)

✅ **Traces:** Fully implemented with ring buffer storage and MCP query tools

## Goals

Add support for:

1. **Logs** - Structured and unstructured log records
2. **Metrics** - Counters, gauges, histograms, summaries
3. **Enhanced Querying** - Efficient context-aware tools for all signals

## Architecture Extension

```
┌─────────────────────────────────────────────────────────────────┐
│ otlp-mcp serve                                                  │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  OTLP gRPC Server (localhost:XXXXX)                         │ │
│ │                                                              │ │
│ │  Endpoints:                                                  │ │
│ │  • /v1/traces   ✅ (complete)                                │ │
│ │  • /v1/logs     🆕 (new)                                     │ │
│ │  • /v1/metrics  🆕 (new)                                     │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                           ▼                                     │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  Ring Buffer Storage                                        │ │
│ │                                                              │ │
│ │  • TraceStorage     (10K spans)        ✅                    │ │
│ │  • LogStorage       (50K records)      🆕                    │ │
│ │  • MetricStorage    (100K points)      🆕                    │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                           ▼                                     │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  MCP Server (stdio)                                         │ │
│ │                                                              │ │
│ │  Trace Tools (6) ✅                                          │ │
│ │  Log Tools (6)   🆕                                          │ │
│ │  Metric Tools (6) 🆕                                         │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                           ▼                                     │
│                    Agent (Claude Code)                          │
└─────────────────────────────────────────────────────────────────┘
```

## OpenTelemetry Protocol References

### Official Specifications

**OTLP Protocol:**
- Main Spec: https://opentelemetry.io/docs/specs/otlp/
- Version: 1.9.0 (latest stable)

**Proto Definitions:**
- Repository: https://github.com/open-telemetry/opentelemetry-proto
- Go Package: `go.opentelemetry.io/proto/otlp`

**Signal-Specific Specs:**

1. **Logs:**
   - Spec: https://opentelemetry.io/docs/specs/otel/logs/
   - Data Model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
   - Proto: `go.opentelemetry.io/proto/otlp/logs/v1`
   - Key Types: `LogRecord`, `ResourceLogs`, `ScopeLogs`

2. **Metrics:**
   - Spec: https://opentelemetry.io/docs/specs/otel/metrics/
   - Data Model: https://opentelemetry.io/docs/specs/otel/metrics/data-model/
   - Proto: `go.opentelemetry.io/proto/otlp/metrics/v1`
   - Key Types: `Metric`, `ResourceMetrics`, `ScopeMetrics`
   - Metric Types: `Gauge`, `Sum`, `Histogram`, `ExponentialHistogram`, `Summary`

3. **Span Events (Part of Trace Spec):**
   - Spec: https://opentelemetry.io/docs/specs/otel/trace/api/#add-events
   - Part of Span: `Span.Events[]`
   - Already included in trace data
   - Enhanced querying via MCP tools (not separate endpoint)

### Semantic Conventions

**Resource Attributes:**
- https://opentelemetry.io/docs/specs/semconv/resource/

**Log Attributes:**
- https://opentelemetry.io/docs/specs/semconv/general/logs/

**Metric Conventions:**
- https://opentelemetry.io/docs/specs/semconv/general/metrics/

## Implementation Tasks

### Task 01: Storage Optimization (CRITICAL)
- **Fix index cleanup:** Remove index entries when ring buffer overwrites old records. This is a critical memory leak fix.
- Optimize ring buffer for different signal sizes.
- Add compression for metric data (future consideration).
- Implement LRU eviction strategies (future consideration).
- Memory usage monitoring and limits.
- Prevent memory leaks from stale index entries.

### Task 02: Logs Support
- Implement OTLP logs gRPC endpoint.
- Create LogStorage ring buffer.
- Add log-specific MCP tools.
- Support log severity filtering.
- Handle structured log attributes.

### Task 03: Metrics Support
- Implement OTLP metrics gRPC endpoint.
- Create MetricStorage ring buffer.
- Handle different metric types (gauge, sum, histogram, etc.).
- Add metric aggregation and querying.
- Add metric-specific MCP tools.

### Task 04: MCP Log Tools
- Implement 9 log tools (grep_logs, get_log_range, get_log_range_snapshot, etc.).

### Task 05: MCP Metric Tools
- Implement 8 metric tools (get_metric_range, get_metric_range_snapshot, etc.).

### Task 06: MCP Span Event Tools
- Implement 2 span event tools (query_span_events, get_spans_with_events).

### Task 07: MCP Snapshot Tools
- Implement 4 snapshot tools (create_snapshot, list_snapshots, get_snapshot_data, delete_snapshot).

### Task 08: MCP Correlation Tools
- Implement 3 correlation tools (get_logs_for_trace, get_metrics_for_service, get_timeline).

### Task 09: Integration & Testing
- End-to-end tests for all signals (traces, logs, metrics).
- Multi-signal scenarios.
- Performance testing with high volumes.
- Memory usage validation.

### Task 10: Documentation
- Update README with logs and metrics examples.
- Add multi-signal demo script.
- Document correlation workflows and snapshot usage.
- Add troubleshooting for each signal type.

## MCP Tools (New)

### Log Tools (9 new)

| Tool | Description |
|------|-------------|
| `get_recent_logs` | Returns N most recent log records with optional offset (pagination) |
| `get_logs_by_trace_id` | Fetch logs correlated with a trace ID |
| `query_logs` | Filter by severity, resource, time range, attributes |
| `grep_logs` | Search log body/attributes with regex pattern (efficient context usage) |
| `get_log_range` | Get logs from position X to Y in ring buffer (precise windowing) |
| `get_log_range_snapshot` | Get logs between two named snapshots |
| `get_log_stats` | Buffer stats, severity distribution, time range |
| `clear_logs` | Clear log buffer |
| `get_log_severities` | List all severities in buffer with counts |

### Metric Tools (8 new)

| Tool | Description |
|------|-------------|
| `get_recent_metrics` | Returns N most recent metric points with optional offset (pagination) |
| `get_metrics_by_name` | Fetch all points for a specific metric name with time range |
| `query_metrics` | Filter by name, type, resource, time range, attributes |
| `get_metric_range` | Get metrics from position X to Y in ring buffer (precise windowing) |
| `get_metric_range_snapshot` | Get metrics between two named snapshots |
| `get_metric_stats` | Buffer stats, type distribution, value ranges |
| `clear_metrics` | Clear metric buffer |
| `get_metric_names` | List all metric names in buffer with counts |

### Trace Enhancement Tools (2 new)

| Tool | Description |
|------|-------------|
| `query_span_events` | Filter spans by event names/attributes (e.g., find all spans with exception events) |
| `get_spans_with_events` | Get spans that have events matching criteria |

### Correlation Tools (3 new)

| Tool | Description |
|------|-------------|
| `get_logs_for_trace` | Get logs that share trace_id with spans (requires semantic conventions) |
| `get_metrics_for_service` | Get metrics from same service as traces/logs |
| `get_timeline` | Unified timeline across signals for a service (time-ordered) |

### Snapshot Tools (4 new) - Ultra Context-Efficient

**Problem:** Agent needs to isolate data from a specific operation (e.g., "test run", "deployment", "before/after fix")

**Solution:** Named snapshots that bookmark ring buffer positions

| Tool | Description |
|------|-------------|
| `create_snapshot` | Save current ring buffer positions with a name (e.g., "before-test") |
| `list_snapshots` | Show all snapshots with their ranges and item counts |
| `get_snapshot_data` | Retrieve all data from a snapshot (traces, logs, metrics in range) |
| `delete_snapshot` | Remove a snapshot bookmark (data stays in buffer) |

**Example Workflow:**
```javascript
// 1. Before running test
create_snapshot({name: "before-deploy"})
// → {traces: pos 1000, logs: pos 5000, metrics: pos 8000}

// 2. Agent deploys code, runs tests
// ... traces/logs/metrics flow into buffers ...

// 3. After test completes
create_snapshot({name: "after-deploy"})
// → {traces: pos 1150, logs: pos 5300, metrics: pos 8500}

// 4. Get all data from that deployment
get_snapshot_data({
  name: "before-deploy",
  end_snapshot: "after-deploy"
})
// → Returns traces[1000-1150], logs[5000-5300], metrics[8000-8500]

// 5. Or query parts of snapshot
get_logs_range({
  start: snapshot["before-deploy"].logs,
  end: snapshot["after-deploy"].logs,
  severity: "ERROR"
})
```

**Benefits:**
- **Zero-copy:** Snapshots are just position bookmarks (8 bytes per signal)
- **Fast:** No data serialization until query time
- **Isolated:** Focus on specific time windows/operations
- **Composable:** Use snapshot ranges in any query tool
- **Named:** Human-readable labels ("before-fix", "deployment-1", etc.)

**Implementation:**
```go
type Snapshot struct {
    Name         string
    CreatedAt    time.Time
    TracePos     int  // Position in trace ring buffer
    LogPos       int  // Position in log ring buffer
    MetricPos    int  // Position in metric ring buffer
}
```

### Context Efficiency Tools (Built into all queries)

**All query tools support:**
- **Pagination:** `offset` and `limit` parameters for chunked retrieval
- **Time ranges:** `start_time` and `end_time` for temporal filtering
- **Attribute filtering:** Reduce result set before returning to agent
- **Count-first queries:** Get counts before fetching data (estimate context usage)
- **Snapshot ranges:** Use snapshot positions instead of absolute positions

## Storage Sizing

Following bootstrap defaults scaled appropriately:

| Signal | Buffer Size | Estimated Memory | Rationale |
|--------|-------------|------------------|-----------|
| Traces | 10,000 spans | ~5 MB | ✅ Current (proven) |
| Logs | 50,000 records | ~25 MB | 🆕 5x traces (logs are smaller but more frequent) |
| Metrics | 100,000 points | ~20 MB | 🆕 10x traces (metrics are compact but high-volume) |
| **Total** | **~160K items** | **~50 MB** | Reasonable for local development |

All configurable via CLI flags.

## Ring Buffer Eviction Policy

**When buffer reaches capacity:**

1. **Oldest entries are overwritten** (FIFO ring buffer behavior)
2. **Index cleanup triggered** via eviction callback (prevents memory leaks)
3. **No warning/error** - silent overwrite (expected behavior)
4. **Stats track overwrites** - `total_received` vs `current_count` shows dropped items
5. **No backpressure** - OTLP server never rejects data due to full buffer

**Rationale:** Agent sessions are ephemeral. Losing old data is acceptable. Alternative (rejecting new data) would break instrumented programs.

**Future:** Add configurable policies (newest-first, priority-based, etc.)

## Success Criteria

Observability phase complete when:

1. ✅ OTLP server accepts logs via gRPC
2. ✅ OTLP server accepts metrics via gRPC
3. ✅ All signals stored in separate ring buffers
4. ✅ Index cleanup prevents memory leaks
5. ✅ 9 log tools working via MCP (including grep and snapshot queries)
6. ✅ 8 metric tools working via MCP (including time-range and snapshot support)
7. ✅ 2 span event tools working via MCP
8. ✅ 4 snapshot tools working via MCP (create, list, get, delete)
9. ✅ 3 correlation tools link signals together
10. ✅ Pagination and windowing work for all signals
11. ✅ Snapshots enable operation isolation and before/after comparisons
12. ✅ Agent can analyze multiple signals together (traces, logs, metrics)
13. ✅ Memory usage stays within expected bounds (~50 MB)
14. ✅ Documentation covers all signals with examples

## Non-Goals

Not in this phase:
- HTTP OTLP endpoints (gRPC only, like bootstrap)
- Persistent storage (still memory-only)
- Metric aggregation across time windows (just raw storage)
- Log/metric sampling (store everything until buffer full)
- Remote connections (still localhost only)
- WebSocket MCP transport (stdio only)

## Future Enhancements

After observability phase:
- **Profiles signal** (OTLP 1.9.0 new signal type - CPU/memory profiling)
- HTTP OTLP support (HTTP/protobuf transport)
- WebSocket MCP transport (multi-client support)
- Metric aggregation and downsampling
- Log pattern detection and anomaly detection
- Export to common formats (JSON, Prometheus, Loki, etc.)
- Grafana/Prometheus integration
- Advanced eviction policies (priority-based, TTL-based)
- Persistent storage (disk/database backend)

## Dependencies

**New Go packages needed:**
```go
// Already have these from bootstrap:
// - go.opentelemetry.io/proto/otlp/trace/v1
// - go.opentelemetry.io/proto/otlp/common/v1
// - go.opentelemetry.io/proto/otlp/resource/v1

// Need to add:
import (
    logspb "go.opentelemetry.io/proto/otlp/logs/v1"
    metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
    collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
    collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)
```

**No new external dependencies** - all part of OpenTelemetry proto package we already use.

## Timeline Estimate

Based on bootstrap experience:

- **Task 01 (Storage Optimization):** ~2-3 hours (critical index cleanup + optimization)
- **Task 02 (Logs Support):** ~2-3 hours (similar to trace implementation)
- **Task 03 (Metrics Support):** ~3-4 hours (more complex data model)
- **Task 04 (MCP Log Tools):** ~1-2 hours (9 new tools)
- **Task 05 (MCP Metric Tools):** ~1-2 hours (8 new tools)
- **Task 06 (MCP Span Event Tools):** ~1 hour (2 new tools)
- **Task 07 (MCP Snapshot Tools):** ~2-3 hours (4 new tools - revolutionary!)
- **Task 08 (MCP Correlation Tools):** ~1-2 hours (3 new tools)
- **Task 09 (Integration & Testing):** ~2-3 hours (testing and validation)
- **Task 10 (Documentation):** ~1-2 hours (extend existing docs)

**Total: ~15-23 hours** of agent collaboration

## Notes

- Logs and metrics follow same patterns as traces.
- MCP tool structure is consistent across signals.
- Storage layer is generic (ring buffer reuse).
- Learning from bootstrap makes this phase faster.
- OpenTelemetry specs are well-documented.
- Go protobuf types are well-maintained.

---

**Status:** Planning complete, ready for implementation.
**Next:** Begin with **Task 01: Storage Optimization** to fix the critical memory leak and establish the eviction callback pattern.
**Priority:** Task 01 is **CRITICAL** and must be completed first.
