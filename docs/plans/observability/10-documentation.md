# Task 10: Documentation

## Overview

Update project documentation to reflect the observability phase additions: logs, metrics, 26 new MCP tools, snapshots, and correlation features.

**Dependencies:** All tasks 01-09 complete

---

## Documentation Tasks

### 1. Update Main README

**File:** `README.md`

Add comprehensive examples showing all signals working together.

**New Sections to Add:**

#### Multi-Signal Observability

```markdown
## Multi-Signal Observability 🔭

otlp-mcp now supports **all three core OpenTelemetry signals**:

- **Traces** (10,000 span capacity) - Request flow and timing
- **Logs** (50,000 record capacity) - Detailed context and errors
- **Metrics** (100,000 point capacity) - Performance and resource usage

**26 MCP Tools** enable agents to query, filter, and correlate telemetry data with minimal context usage.

### Snapshot-Based Workflows 📸

**The revolutionary feature:** Named snapshots bookmark buffer positions for perfect operation isolation.

```typescript
// 1. Create "before" snapshot
create_snapshot({name: "before-deploy"})

// 2. Deploy code and run tests
runDeployment()

// 3. Create "after" snapshot
create_snapshot({name: "after-deploy"})

// 4. Get ONLY deployment telemetry
get_snapshot_data({
  name: "before-deploy",
  end_snapshot: "after-deploy"
})
// → Returns only traces/logs/metrics from the deployment
// → 10-100x more context-efficient than retrieving all data
```

### Quick Start (Multi-Signal)

```bash
# 1. Start otlp-mcp server
./otlp-mcp serve

# 2. In Claude Code, query for OTLP endpoints
get_otlp_endpoint()
# → Returns trace, log, and metric gRPC endpoints

# 3. Run your instrumented app
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:54321  # Traces
export OTEL_LOGS_EXPORTER=otlp                       # Logs
export OTEL_METRICS_EXPORTER=otlp                    # Metrics
export OTEL_SERVICE_NAME=my-app

./my-app

# 4. Query telemetry via MCP
get_timeline({service_name: "my-app"})
# → Unified timeline of traces, logs, and metrics
```

### Example Agent Workflows

#### Deployment Analysis

```typescript
// Before deployment
create_snapshot({name: "pre-deploy"})

// Deploy new version
deployApplication()

// After deployment
create_snapshot({name: "post-deploy"})

// Find errors in deployment
grep_logs({
  pattern: "ERROR|FATAL",
  start_snapshot: "pre-deploy",
  end_snapshot: "post-deploy"
})

// Check performance metrics
get_metrics_for_service({
  service_name: "api",
  start_snapshot: "pre-deploy",
  end_snapshot: "post-deploy"
})
```

#### Request Investigation

```typescript
// 1. Find slow requests
const traces = query_traces({
  service_name: "api",
  min_duration: 5000  // > 5 seconds
})

// 2. Get complete context
const context = get_logs_for_trace({
  trace_id: traces[0].trace_id,
  include_spans: true
})

// 3. Analyze timeline
const timeline = get_timeline({
  trace_id: traces[0].trace_id
})
// → Shows spans, logs, and metrics chronologically
```

### MCP Tools (32 total)

**Trace Tools (6)** - From bootstrap phase
- `get_otlp_endpoint`, `get_recent_traces`, `get_trace_by_id`, `query_traces`, `get_stats`, `clear_traces`

**Log Tools (9)** - New in observability phase
- `get_recent_logs`, `get_logs_by_trace_id`, `query_logs`, `grep_logs`, `get_log_range`, `get_log_range_snapshot`, `get_log_stats`, `clear_logs`, `get_log_severities`

**Metric Tools (8)** - New in observability phase
- `get_recent_metrics`, `get_metrics_by_name`, `query_metrics`, `get_metric_range`, `get_metric_range_snapshot`, `get_metric_stats`, `clear_metrics`, `get_metric_names`

**Span Event Tools (2)** - New in observability phase
- `query_span_events`, `get_spans_with_events`

**Snapshot Tools (4)** - Revolutionary feature ⭐
- `create_snapshot`, `list_snapshots`, `get_snapshot_data`, `delete_snapshot`

**Correlation Tools (3)** - Tie signals together
- `get_logs_for_trace`, `get_metrics_for_service`, `get_timeline`

See [TOOLS.md](docs/TOOLS.md) for complete tool documentation.
```

---

### 2. Create TOOLS.md Reference

**File:** `docs/TOOLS.md`

Comprehensive MCP tool reference with examples.

```markdown
# MCP Tools Reference

Complete reference for all 32 MCP tools exposed by otlp-mcp.

## Table of Contents

- [Trace Tools](#trace-tools) (6)
- [Log Tools](#log-tools) (9)
- [Metric Tools](#metric-tools) (8)
- [Span Event Tools](#span-event-tools) (2)
- [Snapshot Tools](#snapshot-tools) (4)
- [Correlation Tools](#correlation-tools) (3)

---

## Trace Tools

### `get_otlp_endpoint`

Returns the OTLP gRPC endpoint addresses for all signals.

**Parameters:** None

**Returns:**
```json
{
  "trace_endpoint": "localhost:54321",
  "log_endpoint": "localhost:54322",
  "metric_endpoint": "localhost:54323"
}
```

**Example:**
```typescript
const endpoints = await mcp.call("get_otlp_endpoint", {})
console.log(`Send traces to ${endpoints.trace_endpoint}`)
```

### `get_recent_traces`

[... document all 32 tools with parameters, returns, and examples ...]

---

## Snapshot Tools ⭐

### `create_snapshot`

Create a named bookmark of current ring buffer positions.

**Parameters:**
```typescript
{
  name: string,          // Required: snapshot name
  description?: string   // Optional: human-readable description
}
```

**Returns:**
```json
{
  "name": "before-deploy",
  "created_at": "2025-11-09T10:30:00Z",
  "positions": {
    "traces": 1000,
    "logs": 5000,
    "metrics": 8000
  },
  "sizes": {
    "traces": 150,
    "logs": 1200,
    "metrics": 3500
  }
}
```

**Example:**
```typescript
// Mark current state before deployment
const snapshot = await mcp.call("create_snapshot", {
  name: "before-deploy",
  description: "Baseline before v2.3.0 deployment"
})
```

[... continue with all 32 tools ...]
```

---

### 3. Create Multi-Signal Demo Script

**File:** `demo-multi-signal.sh`

```bash
#!/bin/bash

# otlp-mcp Multi-Signal Demo
# Demonstrates traces, logs, metrics, snapshots, and correlation

set -e

echo "🎯 otlp-mcp Multi-Signal Demo"
echo "=============================="
echo

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if otel-cli is available
if ! command -v ~/src/otel-cli/otel-cli &> /dev/null; then
    echo "❌ otel-cli not found at ~/src/otel-cli/otel-cli"
    echo "   Clone from: https://github.com/tobert/otel-cli"
    exit 1
fi

# Build otlp-mcp
echo -e "${BLUE}📦 Building otlp-mcp...${NC}"
go build -o otlp-mcp ./cmd/otlp-mcp

# Start otlp-mcp server
echo -e "${BLUE}🚀 Starting otlp-mcp server...${NC}"
./otlp-mcp serve &
OTLP_MCP_PID=$!

# Give server time to start
sleep 2

# Get endpoints (via MCP tool - would need client implementation)
echo -e "${BLUE}🔍 Getting OTLP endpoints...${NC}"
# For now, use default ephemeral ports
TRACE_ENDPOINT="localhost:54321"  # Would come from get_otlp_endpoint
LOG_ENDPOINT="localhost:54322"
METRIC_ENDPOINT="localhost:54323"

echo "  Traces:  $TRACE_ENDPOINT"
echo "  Logs:    $LOG_ENDPOINT"
echo "  Metrics: $METRIC_ENDPOINT"
echo

# Simulate deployment workflow
echo -e "${YELLOW}📸 Creating 'before-deploy' snapshot${NC}"
# create_snapshot({name: "before-deploy"})
echo

echo -e "${GREEN}💼 Simulating normal operations...${NC}"
for i in {1..5}; do
    echo "  Request $i..."

    # Send trace
    ~/src/otel-cli/otel-cli span \
        --endpoint "$TRACE_ENDPOINT" \
        --service "demo-api" \
        --name "GET /users" \
        --duration 150ms

    # Send log
    ~/src/otel-cli/otel-cli span \
        --endpoint "$LOG_ENDPOINT" \
        --service "demo-api" \
        --name "INFO" \
        --attrs "severity=INFO,message=Request processed successfully"

    # Send metric
    ~/src/otel-cli/otel-cli span \
        --endpoint "$METRIC_ENDPOINT" \
        --service "demo-api" \
        --name "request_duration" \
        --attrs "value=150"

    sleep 0.5
done
echo

echo -e "${YELLOW}🚀 Deploying new version...${NC}"
sleep 1
echo

echo -e "${GREEN}💼 Simulating post-deployment operations...${NC}"
for i in {1..3}; do
    echo "  Request $i (new version)..."

    # Faster spans
    ~/src/otel-cli/otel-cli span \
        --endpoint "$TRACE_ENDPOINT" \
        --service "demo-api" \
        --name "GET /users" \
        --duration 80ms

    # Success log
    ~/src/otel-cli/otel-cli span \
        --endpoint "$LOG_ENDPOINT" \
        --service "demo-api" \
        --name "INFO" \
        --attrs "severity=INFO,message=Fast path optimization active"

    sleep 0.5
done

# Introduce an error
echo "  Request 4 (error)..."
~/src/otel-cli/otel-cli span \
    --endpoint "$TRACE_ENDPOINT" \
    --service "demo-api" \
    --name "GET /users" \
    --duration 5000ms \
    --attrs "error=true"

~/src/otel-cli/otel-cli span \
    --endpoint "$LOG_ENDPOINT" \
    --service "demo-api" \
    --name "ERROR" \
    --attrs "severity=ERROR,message=Database connection timeout"

echo

echo -e "${YELLOW}📸 Creating 'after-deploy' snapshot${NC}"
# create_snapshot({name: "after-deploy"})
echo

echo -e "${BLUE}📊 Demo complete!${NC}"
echo
echo "Agent can now:"
echo "  1. get_snapshot_data({name: 'before-deploy', end_snapshot: 'after-deploy'})"
echo "  2. grep_logs({pattern: 'ERROR'})"
echo "  3. get_timeline({service_name: 'demo-api'})"
echo "  4. Compare performance metrics before/after deployment"
echo

# Cleanup
echo -e "${BLUE}🧹 Cleaning up...${NC}"
kill $OTLP_MCP_PID
echo "Done!"
```

---

### 4. Update CLAUDE.md

**File:** `CLAUDE.md`

Update architecture section and add observability phase info.

```markdown
## Observability Phase (Current)

**Status:** Implementation planning complete

**Capabilities:**
- OTLP support for logs and metrics (in addition to traces)
- 26 new MCP tools for querying and analyzing telemetry
- Snapshot system for operation isolation (revolutionary!)
- Cross-signal correlation (traces + logs + metrics)
- Context-efficient queries (grep, pagination, snapshots)

**Architecture:**

```
┌─────────────────────────────────────────────────────────────┐
│                     Agent (Claude Code)                      │
└───────────────────────────┬─────────────────────────────────┘
                            │ MCP (stdio)
                            │ 32 tools
┌───────────────────────────┴─────────────────────────────────┐
│                      MCP Server                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Snapshot Manager (named position bookmarks)        │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌──────────┐  ┌─────────┐  ┌──────────┐                   │
│  │  Trace   │  │   Log   │  │  Metric  │                   │
│  │ Storage  │  │ Storage │  │ Storage  │                   │
│  │ (10K)    │  │ (50K)   │  │ (100K)   │                   │
│  └────┬─────┘  └────┬────┘  └────┬─────┘                   │
└───────┼─────────────┼────────────┼─────────────────────────┘
        │ gRPC        │ gRPC       │ gRPC
        │ :54321      │ :54322     │ :54323
┌───────┴─────────────┴────────────┴─────────────────────────┐
│            Instrumented Applications                         │
│  (OpenTelemetry SDK with OTLP exporters)                    │
└─────────────────────────────────────────────────────────────┘
```

**Key Innovations:**

1. **Snapshots** - Zero-copy position bookmarks for operation isolation
2. **Correlation** - Link traces, logs, and metrics automatically
3. **Context Efficiency** - Grep, pagination, snapshot ranges reduce token usage
4. **Multi-Signal Timeline** - Chronological view across all signals

See `docs/plans/observability/` for complete implementation plans.
```

---

### 5. Troubleshooting Guide

**File:** `docs/TROUBLESHOOTING.md`

```markdown
# Troubleshooting Guide

## Common Issues

### No Data Appearing in Buffers

**Symptoms:** MCP tools return empty results

**Checks:**
1. Verify OTLP endpoints are correct:
   ```typescript
   get_otlp_endpoint()
   ```

2. Check if application is sending data:
   ```bash
   # Test with otel-cli
   ~/src/otel-cli/otel-cli span \
       --endpoint localhost:54321 \
       --service test \
       --name test-span
   ```

3. Verify buffer capacity not exceeded:
   ```typescript
   get_stats()  // Check utilization
   ```

### Snapshots Not Working

**Symptoms:** `get_snapshot_data` returns no data

**Checks:**
1. Verify snapshot exists:
   ```typescript
   list_snapshots()
   ```

2. Check position ranges:
   ```typescript
   // Ensure end position > start position
   const snapshot = list_snapshots().snapshots[0]
   console.log(snapshot.positions)
   ```

3. Verify data exists in range:
   ```typescript
   get_stats()  // Check total data count
   ```

### Memory Usage High

**Symptoms:** otlp-mcp consuming excessive memory

**Checks:**
1. Verify index cleanup is working:
   ```bash
   # Run memory leak test
   go test ./test -run TestMemoryLeak
   ```

2. Check buffer utilization:
   ```typescript
   get_log_stats()
   get_metric_stats()
   ```

3. Reduce buffer sizes in serve command (future enhancement)

### Correlation Not Working

**Symptoms:** `get_logs_for_trace` returns no logs

**Checks:**
1. Verify logs have trace_id set:
   ```typescript
   get_recent_logs({limit: 10})
   // Check if trace_id field is populated
   ```

2. Ensure instrumentation includes trace context in logs:
   ```go
   // Application must propagate trace context to logs
   logger.Info("message",
       "trace_id", span.SpanContext().TraceID(),
       "span_id", span.SpanContext().SpanID())
   ```

## Performance Tuning

### Slow Queries

If queries are slow:

1. Use pagination:
   ```typescript
   get_recent_logs({limit: 100, offset: 0})
   ```

2. Use snapshots for ranges:
   ```typescript
   get_log_range_snapshot({
       start_snapshot: "before",
       end_snapshot: "after"
   })
   ```

3. Filter early:
   ```typescript
   grep_logs({pattern: "ERROR", service_name: "api"})
   // Pre-filter by service before grep
   ```

### Buffer Wraparound

Understanding buffer behavior:

- **FIFO:** Oldest data is overwritten when buffer is full
- **No warnings:** Silent overwrite (expected behavior)
- **Check stats:** `total_received` vs `current_count` shows drops

```typescript
const stats = get_stats()
const dropped = stats.total_received - stats.current_count
console.log(`Dropped ${dropped} spans due to buffer wraparound`)
```

## Getting Help

1. Check docs: `docs/plans/observability/00-overview.md`
2. Run tests: `go test ./test/... -v`
3. Open issue: https://github.com/tobert/otlp-mcp/issues
```

---

## Acceptance Criteria

- [ ] README updated with multi-signal examples
- [ ] TOOLS.md created with all 32 tool docs
- [ ] Multi-signal demo script created and tested
- [ ] CLAUDE.md updated with observability architecture
- [ ] Troubleshooting guide created
- [ ] All examples tested and verified
- [ ] Screenshots/diagrams added (optional)
- [ ] Links verified
- [ ] Formatting consistent with project style

## Files to Create

- `docs/TOOLS.md` - Complete tool reference
- `docs/TROUBLESHOOTING.md` - Troubleshooting guide
- `demo-multi-signal.sh` - Multi-signal demo script

## Files to Modify

- `README.md` - Add multi-signal examples and capabilities
- `CLAUDE.md` - Update architecture and current phase
- `docs/plans/observability/README.md` - Add completion notes

---

**Status:** Ready to implement
**Dependencies:** All tasks 01-09 complete
**Final Task:** Update jj description and push to GitHub

---

## Documentation Philosophy

**For users:**
- Examples before explanations
- Show the "why" not just the "how"
- Progressive disclosure (simple → advanced)

**For agents:**
- Tool descriptions are prompts
- Include use cases and workflows
- Demonstrate integration patterns

**For developers:**
- Architecture diagrams
- Implementation notes in task files
- Testing strategies included
