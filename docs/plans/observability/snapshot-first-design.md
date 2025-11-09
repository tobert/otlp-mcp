# 📸 Snapshot-First Design: Simplifying Agent Telemetry UX

## The Insight

Instead of 26+ tools for different signals, what if snapshots were the **primary** way agents interact with telemetry? This could dramatically simplify the cognitive model.

## Current Design (Signal-Centric)

```typescript
// Agent needs to understand 26 tools:
get_recent_traces()
get_recent_logs()
get_recent_metrics()
query_traces()
query_logs()
query_metrics()
// ... 20 more tools
```

**Problem**: Agents must:
1. Know which tool to use
2. Make multiple queries
3. Manually correlate signals
4. Remember different parameter formats

## Proposed: Snapshot-First Design

### Core Concept: "What happened during X?"

Most agent questions are actually about **time windows**, not specific signals:
- "What happened during the deployment?"
- "Show me the test run results"
- "What errors occurred in the last operation?"

### Simplified Tool Set (Just 5 Tools!)

```typescript
// 1. Mark points in time
snapshot.create(name: string)

// 2. Get everything from a time window
snapshot.get(
  from: "before-test",
  to: "after-test",
  options?: {
    signals?: ["traces", "logs", "metrics"], // default: all
    filter?: {
      severity?: "ERROR",      // cross-signal filtering
      service?: "api-server",
      contains?: "timeout"     // grep across all text
    }
  }
)

// 3. Compare snapshots
snapshot.diff(
  before: "pre-deploy",
  after: "post-deploy"
)
// Returns: what changed (new errors, metric shifts, latency changes)

// 4. Recent data (when no snapshot exists)
telemetry.recent(
  limit?: 100,
  signals?: ["all"]
)

// 5. Search across everything
telemetry.search(
  query: "error OR timeout",
  from?: "snapshot-name",
  to?: "now"
)
```

## Why This Works Better for LLMs

### 1. Natural Mental Model
Agents think in **operations** not signals:
```typescript
// Natural:
"Show me what happened during the test"

// Unnatural:
"Get traces, then get logs with matching trace_id, then get metrics from the same time range"
```

### 2. Automatic Correlation
```typescript
// Old way (manual correlation):
traces = get_traces_by_time(start, end)
trace_ids = traces.map(t => t.trace_id)
logs = get_logs_by_trace_ids(trace_ids)
metrics = get_metrics_by_time(start, end)

// Snapshot way (automatic):
data = snapshot.get("test-start", "test-end")
// data.traces, data.logs, data.metrics all correlated
```

### 3. Progressive Disclosure
```typescript
// Start broad
data = snapshot.get("before", "after")
// → "Found 150 traces, 823 logs (12 errors), 450 metric points"

// Drill down if needed
data = snapshot.get("before", "after", {
  filter: { severity: "ERROR" }
})
// → "12 error logs with 3 related traces"
```

## The Hybrid Intelligence Pattern

### Mixed Signal Returns
Instead of separate queries, return **unified telemetry objects**:

```typescript
interface TelemetrySnapshot {
  summary: {
    traces: { count: 150, errors: 3, p95_latency: 234ms },
    logs: { count: 823, errors: 12, warnings: 45 },
    metrics: {
      cpu_usage: { avg: 45%, max: 78% },
      memory: { avg: 1.2GB, max: 1.8GB }
    }
  },

  // Interleaved by time
  timeline: [
    { time: "10:00:00", type: "trace", data: {...} },
    { time: "10:00:01", type: "log", severity: "ERROR", message: "..." },
    { time: "10:00:01", type: "metric", name: "cpu", value: 78 },
  ],

  // Or separated if needed
  by_signal: {
    traces: [...],
    logs: [...],
    metrics: [...]
  }
}
```

### Smart Aggregation
The tool could pre-analyze patterns:

```typescript
{
  insights: {
    correlation: "3 traces failed when CPU > 75%",
    anomaly: "Memory spike coincided with errors",
    pattern: "All errors from same service: auth-api"
  }
}
```

## Real Agent Workflow

### Current (26 tools)
```typescript
// Agent internal monologue:
// "User wants to know what went wrong...
//  Should I use query_traces? get_logs_by_time?
//  Maybe grep_logs? Or get_metrics_for_service?
//  Let me try query_traces first..."

traces = query_traces({service: "api"})
// "Hmm, found traces but need logs..."
logs = get_logs_by_trace_id(traces[0].trace_id)
// "Now metrics..."
metrics = get_metrics_by_time(...)
// "How do I correlate these?"
```

### Snapshot-First (5 tools)
```typescript
// Agent internal monologue:
// "User wants to know what went wrong...
//  I'll get the snapshot from that time period"

data = snapshot.get("before-issue", "after-issue")
// "Here's everything: 3 failed traces, 12 error logs,
//  and CPU spiked to 95%. The errors correlate with
//  the CPU spike. Let me investigate further..."

details = snapshot.get("before-issue", "after-issue", {
  filter: { severity: "ERROR" }
})
// "All errors are from the auth service timeout..."
```

## Implementation Strategy

### Phase 1: Snapshot-First Tools
Implement the 5 core tools that cover 90% of use cases:
1. `snapshot.create`
2. `snapshot.get`
3. `snapshot.diff`
4. `telemetry.recent`
5. `telemetry.search`

### Phase 2: Signal-Specific Escape Hatches
Keep a few specialized tools for advanced queries:
- `metrics.aggregate` (for time-series math)
- `traces.topology` (for dependency graphs)
- `logs.grep` (for complex regex)

### Phase 3: Intelligent Aggregation
Add smart features:
- Automatic anomaly detection
- Cross-signal correlation insights
- Pattern recognition

## Benefits Summary

### For Agents
- **5 tools instead of 26** (80% reduction in complexity)
- **Natural workflow** (operations not signals)
- **Automatic correlation** (no manual joining)
- **Progressive disclosure** (summary → details)

### For Context Efficiency
- **One query instead of many** (fewer round trips)
- **Built-in filtering** (less data transferred)
- **Smart summaries** (insights without raw data)

### For Users
- **Simpler prompts** ("show me the deployment")
- **Faster results** (agent knows what to do)
- **Better insights** (correlation built-in)

## The Philosophical Shift

From: "Query individual signals and correlate manually"
To: "Show me what happened during this operation"

This aligns with how humans think about observability - we care about **what happened**, not about the artificial separation of signals.

## Questions to Resolve

1. **Should summaries be automatic?** (Always include insights?)
2. **How smart should filtering be?** (ML-powered or rule-based?)
3. **Should we interleave or separate signals?** (Or both?)
4. **What about real-time streaming?** (Live tail of snapshot?)

## Recommendation

Start with snapshot-first design but keep escape hatches for power users. This gives us:
- **Simple default path** (5 tools for 90% of cases)
- **Power when needed** (specialized tools available)
- **Room to grow** (can add intelligence over time)

---

*This could be the difference between agents struggling with 26 tools and agents naturally understanding telemetry.*