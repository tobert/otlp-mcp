# Observability Plan

**Phase:** Post-Bootstrap Enhancement
**Goal:** Add full OpenTelemetry observability support (logs, metrics, events)

## What This Is

This plan extends the bootstrap MVP to support additional OpenTelemetry signals:

- ✅ **Traces** (complete from bootstrap)
- 🆕 **Logs** (structured log records with grep/search)
- 🆕 **Metrics** (counters, gauges, histograms with time-range queries)
- 🆕 **Span Events** (enhanced querying of existing trace events)
- 🆕 **Context Efficiency** (pagination, windowing, snapshots, precise range queries)

## Plan Structure

```
observability/
├── 00-overview.md             # Architecture and goals (READ THIS FIRST)
├── 01-storage-optimization.md # Ring buffer index cleanup + improvements (CRITICAL FIRST)
├── 02-logs-support.md         # OTLP logs endpoint and storage
├── 03-metrics-support.md      # OTLP metrics endpoint and storage
├── 04-mcp-log-tools.md        # MCP tools for logs
├── 05-mcp-metric-tools.md     # MCP tools for metrics
├── 06-mcp-span-event-tools.md # MCP tools for span events
├── 07-mcp-snapshot-tools.md   # MCP tools for snapshots (revolutionary!)
├── 08-mcp-correlation-tools.md# MCP tools for correlation
├── 09-integration.md          # Multi-signal testing
├── 10-documentation.md        # Update docs for all signals
└── README.md                  # This file
```

## Why This Matters

**Core Observability Signals:**

1. **Traces** answer "what happened?" → Request flow, timing, relationships
2. **Logs** answer "what was the context?" → Detailed events, errors, debug info
3. **Metrics** answer "how much/how many?" → Counts, rates, resource usage

Combined, they provide comprehensive visibility into application behavior. More signals (like profiles) can be added in future phases.

## Example: Full Observability

**Scenario:** Agent debugging a slow API endpoint

```
Agent: "Show me recent traces for the /api/users endpoint"
→ Finds slow trace (500ms)

Agent: "Get logs for trace ID abc123"
→ Sees "Database query took 450ms" warning log

Agent: "Show metrics for database service around that time"
→ Sees connection pool at 95% capacity

Agent: "Aha! The connection pool is saturated. Let me optimize the query..."
```

**With Snapshots:**
```
Agent: "Create snapshot 'before-fix'"
Agent: "Deploy optimized code"
Agent: "Create snapshot 'after-fix'"
Agent: "Compare snapshots - show metrics diff"
→ Connection pool usage: 95% → 60% ✅
```

**Multiple signals** working together with operation isolation = comprehensive understanding.

## Relationship to Bootstrap

**Bootstrap (COMPLETE):**
- Single signal: traces only
- Proof of concept
- Core architecture established
- ~2,000 lines of code

**Observability (IN PROGRESS):**
- Three signals: traces + logs + metrics
- Production-ready observability
- Extends proven architecture
- Est. ~3,000 additional lines

**Shared Infrastructure:**
- OTLP gRPC server (extended)
- Ring buffer storage (pattern reused)
- MCP stdio transport (same)
- CLI framework (same)

## OpenTelemetry Specs

All implementations follow official OTel specs:

- **OTLP Protocol:** https://opentelemetry.io/docs/specs/otlp/
- **Logs Spec:** https://opentelemetry.io/docs/specs/otel/logs/
- **Metrics Spec:** https://opentelemetry.io/docs/specs/otel/metrics/
- **Proto Repo:** https://github.com/open-telemetry/opentelemetry-proto

## Getting Started

1. **Read `00-overview.md`** for full architecture.
2. **Check bootstrap completion** in `../bootstrap/COMPLETE.md`.
3. **Start with Task 01 (Storage Optimization)** to fix the critical memory leak and establish the eviction callback pattern.
4. **Follow the same patterns** as bootstrap implementation.

## Task Dependencies

```
┌─────────────────────┐
│ 01: Storage Opt (C) │
└───────────┬─────────┘
            │
┌───────────▼───────────┐
│ 02: Logs (P)          │
├───────────────────────┤
│ 03: Metrics (P)       │
└───────────┬───────────┘
            │
┌───────────▼───────────┐
│ 04: MCP Log Tools     │
├───────────────────────┤
│ 05: MCP Metric Tools  │
├───────────────────────┤
│ 06: MCP Span Event T. │
├───────────────────────┤
│ 07: MCP Snapshot T.   │
├───────────────────────┤
│ 08: MCP Correlation T.│
└───────────┬───────────┘
            │
┌───────────▼───────────┐
│ 09: Integration       │
└───────────┬───────────┘
            │
┌───────────▼───────────┐
│ 10: Documentation     │
└───────────────────────┘
```

**Legend:** (C) Critical, (P) Parallel

**Implementation Order:**
- **Critical First:** Task 01 (Storage Optimization) fixes a memory leak and is a prerequisite for Tasks 02 and 03.
- **Parallel:** Tasks 02 (Logs) and 03 (Metrics) can be implemented in parallel after Task 01.
- **Sequential:** Tasks 04 through 10 depend on the completion of preceding tasks.

## Success Metrics

When this phase is complete:

- ✅ 3 OTLP endpoints (traces, logs, metrics).
- ✅ 3 ring buffer stores with index cleanup (no memory leaks).
- ✅ 26 new MCP tools total:
  - 9 log tools (Task 04).
  - 8 metric tools (Task 05).
  - 2 span event tools (Task 06).
  - 4 snapshot tools (Task 07 - operation isolation).
  - 3 correlation tools (Task 08).
- ✅ Context-efficient querying (pagination, windowing, filtering).
- ✅ Full observability for agents across multiple signals.
- ✅ ~50 MB total memory footprint.
- ✅ Comprehensive documentation with examples.

## Questions?

See `00-overview.md` for detailed architecture and implementation guidance.

---

**Phase:** Observability Extension
**Status:** Planning complete, ready for implementation
**Estimated Effort:** 15-23 hours of agent collaboration
**Tool Count:** 26 new tools + 6 existing trace tools = 32 total MCP tools
