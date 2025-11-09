# Observability Plan - Handoff Notes

**Date:** 2025-11-09
**Session End:** Running low on context
**Next Agent:** Continue creating remaining task files

## What's Complete ✅

### Plan Files (4/8 complete)
- ✅ `00-overview.md` (370 lines) - Full architecture, all 26 tools spec'd
- ✅ `01-logs-support.md` (500+ lines) - Complete logs implementation spec
- ✅ `03-storage-optimization.md` (337 lines) - Memory leak fix detailed
- ✅ `README.md` (150 lines) - Phase overview

### Bootstrap Phase
- ✅ MVP complete and validated
- ✅ 6 trace MCP tools working
- ✅ Published to GitHub

## What's Needed ❌

### Missing Task Files (4 remaining)

**Task 02: Metrics Support** (HIGH PRIORITY)
- OTLP metrics gRPC endpoint
- MetricStorage with ring buffer
- Handle different metric types (Gauge, Sum, Histogram, ExponentialHistogram, Summary)
- Index by metric name, service, type
- Follow same pattern as Task 01 (logs)
- Proto: `go.opentelemetry.io/proto/otlp/metrics/v1`
- Estimated: 600+ lines (more complex than logs)

**Task 04: MCP Tools** (LARGEST TASK)
- 26 new MCP tools total:
  - 9 log tools (grep_logs, get_log_range, get_log_range_snapshot, etc.)
  - 8 metric tools (get_metric_range, get_metric_range_snapshot, etc.)
  - 2 span event tools (query_span_events, get_spans_with_events)
  - 4 snapshot tools (create_snapshot, list_snapshots, get_snapshot_data, delete_snapshot)
  - 3 correlation tools (get_logs_for_trace, get_metrics_for_service, get_timeline)
- All specs are in 00-overview.md (lines 143-250)
- Use MCP Go SDK patterns from bootstrap
- Estimated: 800+ lines

**Task 05: Integration & Testing** (MEDIUM PRIORITY)
- End-to-end tests for all signals
- Multi-signal scenarios
- Snapshot workflow tests
- Performance testing with high volumes
- Memory usage validation
- Follow pattern from `test/e2e_test.go` in bootstrap
- Estimated: 400+ lines

**Task 06: Documentation** (LOW PRIORITY)
- Update main README with logs/metrics examples
- Create multi-signal demo script (like demo.sh but for all signals)
- Document snapshot workflows
- Add troubleshooting for logs and metrics
- Update CLAUDE.md if needed
- Estimated: 300+ lines

## Key Design Decisions Made

### Snapshot Tools (Revolutionary Feature)
- Zero-copy: Just position bookmarks (24 bytes per snapshot)
- Named snapshots: "before-test", "after-deploy", etc.
- Operation isolation: Get only data from specific time window
- See 00-overview.md lines 184-250 for full spec

### Context Efficiency
- All tools support pagination (offset + limit)
- Time-range filtering (start_time + end_time)
- grep_logs for regex search
- Range queries (get_log_range, get_metric_range)
- Snapshot-based ranges

### Storage Optimization (Task 03)
- Critical memory leak fix via eviction callbacks
- Should be implemented FIRST before logs/metrics
- Pattern: RingBuffer.SetOnEvict() → removeFromIndexes()

### Language
- Removed "three signals" throughout
- Now: "multiple signals", "all signals"
- Future-proof for profiles, baggage, etc.

## Implementation Order Recommended

1. **FIRST:** Task 03 (Storage Optimization) - fixes memory leak
2. **THEN:** Task 01 (Logs) & Task 02 (Metrics) - can be parallel
3. **THEN:** Task 04 (MCP Tools) - needs 01 & 02 complete
4. **THEN:** Task 05 (Integration)
5. **LAST:** Task 06 (Documentation)

## Context for Next Agent

**User Request:** "Create the missing task files"

**What to Do:**
1. Create `02-metrics-support.md` following `01-logs-support.md` pattern
2. Create `04-mcp-tools.md` with all 26 tool implementations
3. Create `05-integration.md` with test scenarios
4. Create `06-documentation.md` with doc updates

**Reference Files:**
- `01-logs-support.md` - Pattern for Task 02
- `00-overview.md` lines 143-250 - Full tool specs for Task 04
- `test/e2e_test.go` (bootstrap) - Pattern for Task 05
- `README.md` - Pattern for Task 06

**Total Lines to Write:** ~2,100 lines across 4 files

## Quick Reference

**OTLP Specs:**
- Logs: https://opentelemetry.io/docs/specs/otel/logs/
- Metrics: https://opentelemetry.io/docs/specs/otel/metrics/
- OTLP: https://opentelemetry.io/docs/specs/otlp/ (v1.9.0)

**Proto Packages:**
- Logs: `go.opentelemetry.io/proto/otlp/logs/v1`
- Metrics: `go.opentelemetry.io/proto/otlp/metrics/v1`

**MCP SDK:**
- `github.com/modelcontextprotocol/go-sdk`
- See `internal/mcpserver/server.go` for patterns

## Session Summary

**What We Accomplished:**
- ✅ Marked bootstrap complete (147 lines doc)
- ✅ Created observability overview (370 lines)
- ✅ Designed snapshot tools (revolutionary)
- ✅ Detailed storage optimization fix (337 lines)
- ✅ Completed logs support spec (500+ lines)
- ✅ Pushed to GitHub (main branch)
- ✅ Total: 1,500+ lines of planning

**What's Left:**
- ❌ 4 task files (~2,100 lines)
- ❌ Then ready for implementation!

---

**Next Agent:** Pick up here and create remaining task files (02, 04, 05, 06)
