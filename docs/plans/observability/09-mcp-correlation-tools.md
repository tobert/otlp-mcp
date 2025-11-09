# Task 08: MCP Correlation Tools 🔗

## Overview

Implement 3 MCP tools for correlating telemetry signals (traces, logs, metrics) across service boundaries. These tools help agents understand the full story of a request or system behavior by connecting related data.

**Dependencies:** Tasks 01 (Logs), 02 (Metrics), Bootstrap (Traces)

## Background: Signal Correlation

**Why correlation matters:**
- **Traces** show request flow and timing
- **Logs** provide detailed context and errors
- **Metrics** reveal performance and resource usage
- **Together** they tell the complete story

**OpenTelemetry Semantic Conventions:**
```go
// Common correlation fields
trace.id         // Links logs to traces
span.id          // Links logs to specific spans
service.name     // Links all signals for a service
service.instance.id  // Specific instance
```

---

## Correlation Tools (3 total)

### 1. `get_logs_for_trace`

Get all logs associated with a specific trace ID.

**Parameters:**
```typescript
{
  trace_id: string,         // Trace ID in hex format
  include_spans?: boolean,  // Also return the trace's spans (default: true)
  severity?: string         // Optional: filter logs by severity
}
```

**Returns:**
```typescript
{
  trace_id: string,
  spans: SpanData[],       // If include_spans=true
  logs: LogRecord[],
  correlation: {
    total_logs: number,
    logs_with_span_id: number,    // Logs linked to specific spans
    logs_with_trace_only: number, // Logs linked to trace but not span
    span_count: number,
    time_range: {
      start: number,  // Earliest timestamp across spans and logs
      end: number     // Latest timestamp
    }
  }
}
```

**Why this tool:** Complete request context - see traces AND logs together.

**Implementation:**
```go
func (s *Server) handleGetLogsForTrace(args map[string]interface{}) (interface{}, error) {
    traceID, ok := args["trace_id"].(string)
    if !ok || traceID == "" {
        return nil, fmt.Errorf("trace_id is required")
    }

    // Get logs for this trace
    logs := s.logStorage.GetLogsByTraceID(traceID)

    // Optional severity filter
    if severity, ok := args["severity"].(string); ok {
        logs = filterBySeverity(logs, severity)
    }

    // Count logs with span IDs
    logsWithSpanID := 0
    for _, log := range logs {
        if log.SpanID != "" {
            logsWithSpanID++
        }
    }

    result := map[string]interface{}{
        "trace_id": traceID,
        "logs": formatLogsForMCP(logs),
        "correlation": map[string]interface{}{
            "total_logs": len(logs),
            "logs_with_span_id": logsWithSpanID,
            "logs_with_trace_only": len(logs) - logsWithSpanID,
        },
    }

    // Optionally include spans
    includeSpans := getBoolArg(args, "include_spans", true)
    if includeSpans {
        spans := s.traceStorage.GetSpansByTraceID(traceID)
        result["spans"] = formatSpansForMCP(spans)
        result["correlation"].(map[string]interface{})["span_count"] = len(spans)

        // Calculate time range across spans and logs
        startTime, endTime := calculateTimeRange(spans, logs)
        result["correlation"].(map[string]interface{})["time_range"] = map[string]interface{}{
            "start": startTime,
            "end": endTime,
        }
    }

    return result, nil
}

// calculateTimeRange finds the earliest and latest timestamps across spans and logs.
func calculateTimeRange(spans []*storage.Span, logs []*storage.StoredLog) (uint64, uint64) {
    var minTime, maxTime uint64 = math.MaxUint64, 0

    for _, span := range spans {
        if span.StartTimeUnixNano < minTime {
            minTime = span.StartTimeUnixNano
        }
        endTime := span.StartTimeUnixNano + span.DurationNanos
        if endTime > maxTime {
            maxTime = endTime
        }
    }

    for _, log := range logs {
        if log.Timestamp < minTime {
            minTime = log.Timestamp
        }
        if log.Timestamp > maxTime {
            maxTime = log.Timestamp
        }
    }

    if minTime == math.MaxUint64 {
        minTime = 0
    }

    return minTime, maxTime
}
```

---

### 2. `get_metrics_for_service`

Get metrics from the same service as traces/logs (service-level view).

**Parameters:**
```typescript
{
  service_name: string,
  metric_name?: string,       // Optional: specific metric
  metric_type?: string,       // Optional: filter by type
  start_time?: number,        // Unix nanoseconds
  end_time?: number,          // Unix nanoseconds
  limit?: number
}
```

**Returns:**
```typescript
{
  service_name: string,
  metrics: MetricData[],
  summary: {
    metric_count: number,
    unique_metric_names: number,
    metric_types: string[],
    time_range: {
      start: number,
      end: number
    }
  }
}
```

**Why this tool:** See service health metrics alongside traces/logs.

**Implementation:**
```go
func (s *Server) handleGetMetricsForService(args map[string]interface{}) (interface{}, error) {
    serviceName, ok := args["service_name"].(string)
    if !ok || serviceName == "" {
        return nil, fmt.Errorf("service_name is required")
    }

    // Get all metrics for this service
    metrics := s.metricStorage.GetMetricsByService(serviceName)

    // Optional filters
    if metricName, ok := args["metric_name"].(string); ok {
        metrics = filterByMetricName(metrics, metricName)
    }

    if metricType, ok := args["metric_type"].(string); ok {
        mt := parseMetricType(metricType)
        metrics = filterByMetricType(metrics, mt)
    }

    // Time range filter
    if startTime, ok := args["start_time"].(float64); ok {
        metrics = filterMetricsByStartTime(metrics, uint64(startTime))
    }
    if endTime, ok := args["end_time"].(float64); ok {
        metrics = filterMetricsByEndTime(metrics, uint64(endTime))
    }

    // Limit
    limit := getIntArg(args, "limit", 1000)
    if len(metrics) > limit {
        metrics = metrics[:limit]
    }

    // Build summary
    uniqueNames := make(map[string]bool)
    metricTypes := make(map[string]bool)
    var minTime, maxTime uint64 = math.MaxUint64, 0

    for _, m := range metrics {
        uniqueNames[m.MetricName] = true
        metricTypes[m.MetricType.String()] = true

        if m.Timestamp < minTime {
            minTime = m.Timestamp
        }
        if m.Timestamp > maxTime {
            maxTime = m.Timestamp
        }
    }

    typeList := make([]string, 0, len(metricTypes))
    for t := range metricTypes {
        typeList = append(typeList, t)
    }

    if minTime == math.MaxUint64 {
        minTime = 0
    }

    return map[string]interface{}{
        "service_name": serviceName,
        "metrics": formatMetricsForMCP(metrics),
        "summary": map[string]interface{}{
            "metric_count": len(metrics),
            "unique_metric_names": len(uniqueNames),
            "metric_types": typeList,
            "time_range": map[string]interface{}{
                "start": minTime,
                "end": maxTime,
            },
        },
    }, nil
}
```

---

### 3. `get_timeline`

Unified timeline across all signals for a service or trace (time-ordered events).

**Parameters:**
```typescript
{
  service_name?: string,   // Service-level timeline
  trace_id?: string,       // Trace-level timeline (preferred over service)
  start_time?: number,
  end_time?: number,
  limit?: number           // Max events (default: 500)
}
```

**Returns:**
```typescript
{
  timeline: Array<{
    timestamp: number,
    type: "span" | "log" | "metric",
    signal: SpanData | LogRecord | MetricData,
    summary: string  // Human-readable event summary
  }>,
  range: {
    start: number,
    end: number,
    duration_ms: number
  },
  counts: {
    spans: number,
    logs: number,
    metrics: number,
    total: number
  }
}
```

**Why this tool:** Complete chronological view of everything that happened. Perfect for root cause analysis.

**Implementation:**
```go
func (s *Server) handleGetTimeline(args map[string]interface{}) (interface{}, error) {
    var spans []*storage.Span
    var logs []*storage.StoredLog
    var metrics []*storage.StoredMetric

    // Determine scope (trace or service)
    if traceID, ok := args["trace_id"].(string); ok {
        // Trace-level timeline
        spans = s.traceStorage.GetSpansByTraceID(traceID)
        logs = s.logStorage.GetLogsByTraceID(traceID)
        // Note: Metrics don't have trace_id, skip for trace timeline
    } else if serviceName, ok := args["service_name"].(string); ok {
        // Service-level timeline
        spans = s.traceStorage.GetSpansByService(serviceName)
        logs = s.logStorage.GetLogsByService(serviceName)
        metrics = s.metricStorage.GetMetricsByService(serviceName)
    } else {
        return nil, fmt.Errorf("either trace_id or service_name is required")
    }

    // Apply time range filters
    if startTime, ok := args["start_time"].(float64); ok {
        spans = filterSpansByStartTime(spans, uint64(startTime))
        logs = filterLogsByStartTime(logs, uint64(startTime))
        metrics = filterMetricsByStartTime(metrics, uint64(startTime))
    }
    if endTime, ok := args["end_time"].(float64); ok {
        spans = filterSpansByEndTime(spans, uint64(endTime))
        logs = filterLogsByEndTime(logs, uint64(endTime))
        metrics = filterMetricsByEndTime(metrics, uint64(endTime))
    }

    // Build unified timeline
    timeline := []map[string]interface{}{}

    for _, span := range spans {
        timeline = append(timeline, map[string]interface{}{
            "timestamp": span.StartTimeUnixNano,
            "type": "span",
            "signal": formatSpanForMCP(span),
            "summary": fmt.Sprintf("Span: %s (duration: %s)",
                span.Name,
                time.Duration(span.DurationNanos)),
        })
    }

    for _, log := range logs {
        timeline = append(timeline, map[string]interface{}{
            "timestamp": log.Timestamp,
            "type": "log",
            "signal": formatLogForMCP(log),
            "summary": fmt.Sprintf("[%s] %s", log.Severity, truncate(log.Body, 80)),
        })
    }

    for _, metric := range metrics {
        timeline = append(timeline, map[string]interface{}{
            "timestamp": metric.Timestamp,
            "type": "metric",
            "signal": formatMetricForMCP(metric),
            "summary": fmt.Sprintf("Metric: %s = %v",
                metric.MetricName,
                formatMetricValue(metric)),
        })
    }

    // Sort by timestamp
    sort.Slice(timeline, func(i, j int) bool {
        return timeline[i]["timestamp"].(uint64) < timeline[j]["timestamp"].(uint64)
    })

    // Apply limit
    limit := getIntArg(args, "limit", 500)
    if len(timeline) > limit {
        timeline = timeline[:limit]
    }

    // Calculate range
    var startTS, endTS uint64
    if len(timeline) > 0 {
        startTS = timeline[0]["timestamp"].(uint64)
        endTS = timeline[len(timeline)-1]["timestamp"].(uint64)
    }
    durationMS := float64(endTS-startTS) / 1e6

    return map[string]interface{}{
        "timeline": timeline,
        "range": map[string]interface{}{
            "start": startTS,
            "end": endTS,
            "duration_ms": durationMS,
        },
        "counts": map[string]interface{}{
            "spans": len(spans),
            "logs": len(logs),
            "metrics": len(metrics),
            "total": len(timeline),
        },
    }, nil
}

// truncate truncates a string to maxLen with ellipsis.
func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}

// formatMetricValue formats a metric value for display.
func formatMetricValue(m *storage.StoredMetric) string {
    if m.NumericValue != nil {
        return fmt.Sprintf("%.2f", *m.NumericValue)
    }
    if m.Count != nil && m.Sum != nil {
        return fmt.Sprintf("count=%d sum=%.2f", *m.Count, *m.Sum)
    }
    return "N/A"
}
```

---

## Helper Functions

**File:** `internal/mcpserver/correlation_helpers.go`

```go
package mcpserver

import "github.com/tobert/otlp-mcp/internal/storage"

// Filter functions for time ranges

func filterSpansByStartTime(spans []*storage.Span, startTime uint64) []*storage.Span {
    result := make([]*storage.Span, 0)
    for _, span := range spans {
        if span.StartTimeUnixNano >= startTime {
            result = append(result, span)
        }
    }
    return result
}

func filterSpansByEndTime(spans []*storage.Span, endTime uint64) []*storage.Span {
    result := make([]*storage.Span, 0)
    for _, span := range spans {
        spanEndTime := span.StartTimeUnixNano + span.DurationNanos
        if spanEndTime <= endTime {
            result = append(result, span)
        }
    }
    return result
}

func filterLogsByStartTime(logs []*storage.StoredLog, startTime uint64) []*storage.StoredLog {
    result := make([]*storage.StoredLog, 0)
    for _, log := range logs {
        if log.Timestamp >= startTime {
            result = append(result, log)
        }
    }
    return result
}

func filterLogsByEndTime(logs []*storage.StoredLog, endTime uint64) []*storage.StoredLog {
    result := make([]*storage.StoredLog, 0)
    for _, log := range logs {
        if log.Timestamp <= endTime {
            result = append(result, log)
        }
    }
    return result
}

func filterMetricsByStartTime(metrics []*storage.StoredMetric, startTime uint64) []*storage.StoredMetric {
    result := make([]*storage.StoredMetric, 0)
    for _, metric := range metrics {
        if metric.Timestamp >= startTime {
            result = append(result, metric)
        }
    }
    return result
}

func filterMetricsByEndTime(metrics []*storage.StoredMetric, endTime uint64) []*storage.StoredMetric {
    result := make([]*storage.StoredMetric, 0)
    for _, metric := range metrics {
        if metric.Timestamp <= endTime {
            result = append(result, metric)
        }
    }
    return result
}
```

---

## Tool Registration

**File:** `internal/mcpserver/correlation_tools.go`

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Server) registerCorrelationTools() error {
    tools := []struct{
        name string
        description string
        schema map[string]interface{}
        handler func(map[string]interface{}) (interface{}, error)
    }{
        {
            name: "get_logs_for_trace",
            description: "Get logs correlated with a trace ID (complete request context)",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "trace_id": map[string]interface{}{"type": "string"},
                    "include_spans": map[string]interface{}{"type": "boolean"},
                    "severity": map[string]interface{}{"type": "string"},
                },
                "required": []string{"trace_id"},
            },
            handler: s.handleGetLogsForTrace,
        },
        {
            name: "get_metrics_for_service",
            description: "Get metrics from a service (service health view)",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "service_name": map[string]interface{}{"type": "string"},
                    "metric_name": map[string]interface{}{"type": "string"},
                    "metric_type": map[string]interface{}{"type": "string"},
                    "start_time": map[string]interface{}{"type": "number"},
                    "end_time": map[string]interface{}{"type": "number"},
                    "limit": map[string]interface{}{"type": "number"},
                },
                "required": []string{"service_name"},
            },
            handler: s.handleGetMetricsForService,
        },
        {
            name: "get_timeline",
            description: "Unified timeline across all signals (chronological view)",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "service_name": map[string]interface{}{"type": "string"},
                    "trace_id": map[string]interface{}{"type": "string"},
                    "start_time": map[string]interface{}{"type": "number"},
                    "end_time": map[string]interface{}{"type": "number"},
                    "limit": map[string]interface{}{"type": "number"},
                },
            },
            handler: s.handleGetTimeline,
        },
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

## Example Workflows

### Request Investigation
```typescript
// 1. Find problematic trace
const traces = query_traces({service_name: "api", min_duration: 5000})
const slowTrace = traces[0]

// 2. Get complete context for that request
const context = get_logs_for_trace({
  trace_id: slowTrace.trace_id,
  include_spans: true
})

// Analyze: spans show WHERE time was spent, logs show WHY
```

### Service Health Check
```typescript
// 1. Get service timeline
const timeline = get_timeline({
  service_name: "database",
  start_time: last_hour,
  end_time: now
})

// 2. See errors in context
timeline.filter(e => e.type === "log" && e.signal.severity === "ERROR")

// 3. Check corresponding metrics
const metrics = get_metrics_for_service({
  service_name: "database",
  start_time: last_hour
})
```

---

## Acceptance Criteria

- [ ] All 3 correlation tools implemented
- [ ] Trace-to-logs correlation working
- [ ] Service-level queries working across signals
- [ ] Timeline sorting by timestamp working
- [ ] Time range filtering working
- [ ] Tool registration working
- [ ] Summary generation working (human-readable)
- [ ] Error handling for missing data
- [ ] Unit tests for each tool
- [ ] Integration tests with multi-signal data

## Files to Create

- `internal/mcpserver/correlation_tools.go` - Tool implementations
- `internal/mcpserver/correlation_tools_test.go` - Unit tests
- `internal/mcpserver/correlation_helpers.go` - Filtering and formatting helpers

## Files to Modify

- `internal/mcpserver/server.go` - Call `registerCorrelationTools()` during initialization

## Testing Notes

**Test scenarios:**
1. Trace with correlated logs
2. Trace without logs
3. Service with all signal types
4. Service with only some signals
5. Timeline with mixed signals
6. Time range filtering
7. Empty results handling
8. Large timeline (pagination needed)
9. Metric value formatting
10. Summary string generation

---

**Status:** Ready to implement
**Dependencies:** Tasks 01 (Logs), 02 (Metrics), Bootstrap (Traces)
**Next:** Task 09 (Integration & Testing)

---

## Why Correlation Matters 🔗

**Without correlation:**
- Analyze traces in isolation
- Search logs separately
- View metrics independently
- **Miss the connections**

**With correlation:**
- See complete request story (trace + logs)
- Understand service health (all signals)
- Timeline shows causation
- **Root cause analysis in seconds**

Correlation tools turn isolated signals into actionable insights. 💡
