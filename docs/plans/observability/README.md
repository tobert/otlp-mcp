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
├── 00-overview.md          # Architecture and goals (READ THIS FIRST)
├── 01-logs-support.md      # OTLP logs endpoint and storage
├── 02-metrics-support.md   # OTLP metrics endpoint and storage
├── 03-storage-optimization.md # Ring buffer index cleanup + improvements
├── 04-mcp-tools.md         # 20 new tools: logs, metrics, events, correlation
├── 05-integration.md       # Multi-signal testing
├── 06-documentation.md     # Update docs for all signals
└── README.md               # This file
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

1. **Read `00-overview.md`** for full architecture
2. **Check bootstrap completion** in `../bootstrap/COMPLETE.md`
3. **Start with Task 01** (logs support)
4. **Follow the same patterns** as bootstrap implementation

## Task Dependencies

```
┌─────────────────┐
│ 01: Logs        │──┐
└─────────────────┘  │
                     │
┌─────────────────┐  │
│ 02: Metrics     │──┤
└─────────────────┘  │
                     ├──► 04: MCP Tools ──► 05: Integration ──► 06: Docs
┌─────────────────┐  │
│ 03: Storage Opt │──┘
└─────────────────┘
```

**Parallel work possible:** Tasks 01-03 can be done concurrently
**Sequential:** 04-06 depend on 01-03 completion
**Critical first:** Task 03 (storage optimization) fixes memory leak - can be done immediately

## Success Metrics

When this phase is complete:

- ✅ 3 OTLP endpoints (traces, logs, metrics)
- ✅ 3 ring buffer stores with index cleanup (no memory leaks)
- ✅ 26 new MCP tools total:
  - 9 log tools (including grep, snapshot queries)
  - 8 metric tools (including time-range, snapshot support)
  - 2 span event tools
  - 4 snapshot tools (operation isolation)
  - 3 correlation tools
- ✅ Context-efficient querying (pagination, windowing, filtering)
- ✅ Full observability for agents across multiple signals
- ✅ ~50 MB total memory footprint
- ✅ Comprehensive documentation with examples

## Questions?

See `00-overview.md` for detailed architecture and implementation guidance.

---

**Phase:** Observability Extension
**Status:** Planning complete, ready for implementation
**Estimated Effort:** 13-19 hours of agent collaboration
**Tool Count:** 26 new tools + 6 existing trace tools = 32 total MCP tools
