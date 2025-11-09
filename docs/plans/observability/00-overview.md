# Observability Plan Overview - Snapshot-First Design

## Vision

Extend otlp-mcp to support logs and metrics with a **revolutionary snapshot-first interface** that aligns with how agents naturally think about telemetry.

## Current State (Bootstrap Complete)

✅ **Traces:** Fully implemented with ring buffer storage and MCP query tools

## Goals

Add support for:

1. **Logs** - Structured and unstructured log records
2. **Metrics** - Counters, gauges, histograms, summaries
3. **Snapshot-First Interface** - Just 5 tools instead of dozens

## Architecture (Same Infrastructure, New Interface)

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
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        ▼                                        │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  Ring Buffer Storage + Snapshot Manager                     │ │
│ │                                                              │ │
│ │  • TraceStorage     (10K spans)        ✅                    │ │
│ │  • LogStorage       (50K records)      🆕                    │ │
│ │  • MetricStorage    (100K points)      🆕                    │ │
│ │  • SnapshotManager  (position bookmarks) 🆕                  │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        ▼                                        │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │  MCP Server (stdio) - JUST 5 TOOLS!                         │ │
│ │                                                              │ │
│ │  snapshot.create    snapshot.get    snapshot.diff           │ │
│ │  telemetry.recent   telemetry.search                        │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                        ▼                                        │
│                    Agent (Claude/Gemini/GPT)                    │
└─────────────────────────────────────────────────────────────────┘
```

## The 5-Tool Revolution

Instead of dozens of signal-specific tools, we provide just 5 intuitive tools:

1. **`snapshot.create(name)`** - Mark a point in time
2. **`snapshot.get(from, to)`** - Get all telemetry from a time window
3. **`snapshot.diff(before, after)`** - Compare snapshots
4. **`telemetry.recent()`** - Get recent data when no snapshot exists
5. **`telemetry.search(query)`** - Search across all signals

These tools automatically correlate traces, logs, and metrics by time, eliminating manual correlation work.

## Implementation Tasks

### Task 01: Storage Optimization + Snapshot Support (CRITICAL)
- **Fix index cleanup:** Remove index entries when ring buffer overwrites old records. This is a critical memory leak fix.
- Add snapshot position tracking to each buffer
- Implement SnapshotManager for coordination
- Prevent memory leaks from stale index entries

### Task 02: Logs Support
- Implement OTLP logs gRPC endpoint
- Create LogStorage ring buffer with snapshot position tracking
- Support log severity filtering
- Handle structured log attributes

### Task 03: Metrics Support
- Implement OTLP metrics gRPC endpoint
- Create MetricStorage ring buffer with snapshot position tracking
- Handle different metric types (gauge, sum, histogram, etc.)

### Task 04: The 5 Snapshot Tools
- Implement the complete snapshot-first interface:
  - `snapshot.create` - Bookmark buffer positions
  - `snapshot.get` - Retrieve correlated telemetry from time windows
  - `snapshot.diff` - Compare before/after snapshots
  - `telemetry.recent` - Get recent data without snapshots
  - `telemetry.search` - Search across all signals

### Task 05: Integration & Testing
- End-to-end tests for all signals through snapshot interface
- Verify automatic correlation
- Performance testing with high volumes
- Memory usage validation

### Task 06: Documentation
- Update README with snapshot-first examples
- Document the 5-tool workflow
- Highlight 80% complexity reduction
- Add agent-friendly tutorials

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
   - Proto: `go.opentelemetry.io/proto/otlp/logs/v1`

2. **Metrics:**
   - Spec: https://opentelemetry.io/docs/specs/otel/metrics/
   - Proto: `go.opentelemetry.io/proto/otlp/metrics/v1`

## Storage Sizing

| Signal | Buffer Size | Estimated Memory | Rationale |
|--------|-------------|------------------|-----------|
| Traces | 10,000 spans | ~5 MB | Current (proven) |
| Logs | 50,000 records | ~25 MB | 5x traces (smaller but frequent) |
| Metrics | 100,000 points | ~20 MB | 10x traces (compact but high-volume) |
| **Total** | **~160K items** | **~50 MB** | Reasonable for development |

All configurable via CLI flags.

## Success Criteria

1. ✅ OTLP endpoints for traces, logs, and metrics
2. ✅ Ring buffers with memory leak prevention
3. ✅ **Just 5 MCP tools** providing everything
4. ✅ Automatic cross-signal correlation
5. ✅ Zero-copy snapshots (24 bytes each!)
6. ✅ Memory usage ~50 MB
7. ✅ Natural agent workflows
8. ✅ 80% complexity reduction

## Timeline Estimate

Based on bootstrap experience:

- **Task 01:** ~2-3 hours (critical fix + snapshot support)
- **Task 02:** ~2-3 hours (logs implementation)
- **Task 03:** ~3-4 hours (metrics implementation)
- **Task 04:** ~3-4 hours (5 snapshot tools)
- **Task 05:** ~2-3 hours (integration testing)
- **Task 06:** ~1-2 hours (documentation)

**Total: ~14-19 hours** of agent collaboration

## The Philosophy

> "Don't make agents think like SREs. Let them ask 'what happened?' and get a complete answer."

This snapshot-first approach isn't just simpler - it's fundamentally more aligned with how both humans and LLMs think about system behavior.

---

**Status:** Planning complete, ready for implementation
**Next:** Fix memory leak (Task 01) then implement snapshot-first interface
**Priority:** Revolutionary simplification through snapshots