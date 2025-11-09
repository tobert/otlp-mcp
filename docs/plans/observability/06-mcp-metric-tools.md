# Task 05: MCP Metric Tools

## Overview

Implement 8 MCP tools for querying and analyzing metric data stored in MetricStorage. These tools handle all 5 metric types (Gauge, Sum, Histogram, ExponentialHistogram, Summary) and provide time-series analysis capabilities.

**Dependencies:** Task 02 (Metrics Support) must be complete

## Metric Tools (8 total)

### 1. `get_recent_metrics`

Returns N most recent metric data points with optional offset for pagination.

**Parameters:**
```typescript
{
  limit: number,      // Number of metrics to return (default: 100, max: 1000)
  offset?: number     // Skip first N metrics (default: 0)
}
```

**Returns:**
```typescript
{
  metrics: MetricData[],
  total: number,
  returned: number
}
```

**Implementation pattern:**
```go
func (s *Server) handleGetRecentMetrics(args map[string]interface{}) (interface{}, error) {
    limit := getIntArg(args, "limit", 100)
    offset := getIntArg(args, "offset", 0)

    if limit > 1000 {
        limit = 1000
    }

    allMetrics := s.metricStorage.GetRecentMetrics(offset + limit)
    if offset >= len(allMetrics) {
        return map[string]interface{}{
            "metrics": []interface{}{},
            "total": s.metricStorage.Stats().MetricCount,
            "returned": 0,
        }, nil
    }

    metrics := allMetrics[offset:]
    return map[string]interface{}{
        "metrics": formatMetricsForMCP(metrics),
        "total": s.metricStorage.Stats().MetricCount,
        "returned": len(metrics),
    }, nil
}
```

---

### 2. `get_metrics_by_name`

Fetch all data points for a specific metric name with optional time range.

**Parameters:**
```typescript
{
  metric_name: string,
  start_time?: number,    // Unix nanoseconds
  end_time?: number,      // Unix nanoseconds
  limit?: number          // Max results (default: 1000)
}
```

**Returns:**
```typescript
{
  metric_name: string,
  metrics: MetricData[],
  data_points: number,    // Total data points across all metrics
  time_range: {
    start: number,
    end: number
  }
}
```

**Why this tool:** Time-series analysis of a single metric (e.g., "show me memory_usage over time").

**Implementation pattern:**
```go
func (s *Server) handleGetMetricsByName(args map[string]interface{}) (interface{}, error) {
    metricName, ok := args["metric_name"].(string)
    if !ok || metricName == "" {
        return nil, fmt.Errorf("metric_name is required")
    }

    metrics := s.metricStorage.GetMetricsByName(metricName)

    // Apply time range filter
    if startTime, ok := args["start_time"].(float64); ok {
        metrics = filterMetricsByStartTime(metrics, uint64(startTime))
    }
    if endTime, ok := args["end_time"].(float64); ok {
        metrics = filterMetricsByEndTime(metrics, uint64(endTime))
    }

    // Apply limit
    limit := getIntArg(args, "limit", 1000)
    if len(metrics) > limit {
        metrics = metrics[:limit]
    }

    // Calculate total data points
    totalDataPoints := 0
    for _, m := range metrics {
        totalDataPoints += m.DataPointCount
    }

    // Calculate time range
    var startTS, endTS uint64
    if len(metrics) > 0 {
        startTS = metrics[0].Timestamp
        endTS = metrics[len(metrics)-1].Timestamp
    }

    return map[string]interface{}{
        "metric_name": metricName,
        "metrics": formatMetricsForMCP(metrics),
        "data_points": totalDataPoints,
        "time_range": map[string]interface{}{
            "start": startTS,
            "end": endTS,
        },
    }, nil
}
```

---

### 3. `query_metrics`

Filter metrics by name, type, service, time range, and attributes.

**Parameters:**
```typescript
{
  metric_name?: string,
  metric_type?: string,        // "Gauge", "Sum", "Histogram", etc.
  service_name?: string,
  start_time?: number,
  end_time?: number,
  limit?: number,
  offset?: number
}
```

**Returns:**
```typescript
{
  metrics: MetricData[],
  filters_applied: string[],
  matched: number,
  returned: number
}
```

**Implementation pattern:**
```go
func (s *Server) handleQueryMetrics(args map[string]interface{}) (interface{}, error) {
    var metrics []*storage.StoredMetric
    filtersApplied := []string{}

    // Apply filters in order of selectivity
    if metricName, ok := args["metric_name"].(string); ok {
        metrics = s.metricStorage.GetMetricsByName(metricName)
        filtersApplied = append(filtersApplied, "metric_name")
    } else if metricType, ok := args["metric_type"].(string); ok {
        mt := parseMetricType(metricType)
        metrics = s.metricStorage.GetMetricsByType(mt)
        filtersApplied = append(filtersApplied, "metric_type")
    } else if serviceName, ok := args["service_name"].(string); ok {
        metrics = s.metricStorage.GetMetricsByService(serviceName)
        filtersApplied = append(filtersApplied, "service_name")
    } else {
        metrics = s.metricStorage.GetRecentMetrics(100000) // All metrics
    }

    // Apply time range filters
    if startTime, ok := args["start_time"].(float64); ok {
        metrics = filterMetricsByStartTime(metrics, uint64(startTime))
        filtersApplied = append(filtersApplied, "start_time")
    }
    if endTime, ok := args["end_time"].(float64); ok {
        metrics = filterMetricsByEndTime(metrics, uint64(endTime))
        filtersApplied = append(filtersApplied, "end_time")
    }

    matched := len(metrics)

    // Apply pagination
    limit := getIntArg(args, "limit", 100)
    offset := getIntArg(args, "offset", 0)
    metrics = paginateMetrics(metrics, offset, limit)

    return map[string]interface{}{
        "metrics": formatMetricsForMCP(metrics),
        "filters_applied": filtersApplied,
        "matched": matched,
        "returned": len(metrics),
    }, nil
}
```

---

### 4. `get_metric_range`

Get metrics from position X to Y in ring buffer (precise windowing).

**Parameters:**
```typescript
{
  start: number,           // Start position in ring buffer
  end: number,             // End position in ring buffer
  metric_name?: string     // Optional name filter
}
```

**Returns:**
```typescript
{
  metrics: MetricData[],
  start: number,
  end: number,
  count: number
}
```

**Why this tool:** Precise window queries when agent knows positions (e.g., from snapshots).

**Implementation pattern:**
```go
func (s *Server) handleGetMetricRange(args map[string]interface{}) (interface{}, error) {
    start := getIntArg(args, "start", 0)
    end := getIntArg(args, "end", 0)

    if start < 0 || end < start {
        return nil, fmt.Errorf("invalid range: start=%d, end=%d", start, end)
    }

    metrics := s.metricStorage.GetRange(start, end)

    // Optional name filter
    if metricName, ok := args["metric_name"].(string); ok {
        metrics = filterByMetricName(metrics, metricName)
    }

    return map[string]interface{}{
        "metrics": formatMetricsForMCP(metrics),
        "start": start,
        "end": end,
        "count": len(metrics),
    }, nil
}
```

---

### 5. `get_metric_range_snapshot`

Get metrics between two named snapshots.

**Parameters:**
```typescript
{
  start_snapshot: string,
  end_snapshot?: string,      // Optional, defaults to current
  metric_name?: string,       // Optional name filter
  metric_type?: string        // Optional type filter
}
```

**Returns:**
```typescript
{
  metrics: MetricData[],
  start_snapshot: string,
  end_snapshot: string,
  start_position: number,
  end_position: number,
  count: number
}
```

**Why this tool:** "Show me all HTTP request count metrics during the deployment" or "latency between test runs".

**Implementation pattern:**
```go
func (s *Server) handleGetMetricRangeSnapshot(args map[string]interface{}) (interface{}, error) {
    startSnapshotName, ok := args["start_snapshot"].(string)
    if !ok {
        return nil, fmt.Errorf("start_snapshot is required")
    }

    startSnapshot := s.snapshotManager.Get(startSnapshotName)
    if startSnapshot == nil {
        return nil, fmt.Errorf("snapshot not found: %s", startSnapshotName)
    }

    endPosition := s.metricStorage.CurrentPosition()
    endSnapshotName := "current"

    if endName, ok := args["end_snapshot"].(string); ok {
        endSnapshot := s.snapshotManager.Get(endName)
        if endSnapshot == nil {
            return nil, fmt.Errorf("snapshot not found: %s", endName)
        }
        endPosition = endSnapshot.MetricPos
        endSnapshotName = endName
    }

    metrics := s.metricStorage.GetRange(startSnapshot.MetricPos, endPosition)

    // Optional filters
    if metricName, ok := args["metric_name"].(string); ok {
        metrics = filterByMetricName(metrics, metricName)
    }
    if metricType, ok := args["metric_type"].(string); ok {
        mt := parseMetricType(metricType)
        metrics = filterByMetricType(metrics, mt)
    }

    return map[string]interface{}{
        "metrics": formatMetricsForMCP(metrics),
        "start_snapshot": startSnapshotName,
        "end_snapshot": endSnapshotName,
        "start_position": startSnapshot.MetricPos,
        "end_position": endPosition,
        "count": len(metrics),
    }, nil
}
```

---

### 6. `get_metric_stats`

Get buffer statistics, type distribution, and value ranges.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  metric_count: number,
  capacity: number,
  utilization: number,
  unique_names: number,
  service_count: number,
  type_counts: {[key: string]: number},    // Type → count
  total_data_points: number,
  oldest_timestamp: number,
  newest_timestamp: number
}
```

**Why this tool:** Understand buffer state and metric distribution before querying.

**Implementation pattern:**
```go
func (s *Server) handleGetMetricStats(args map[string]interface{}) (interface{}, error) {
    stats := s.metricStorage.Stats()

    // Calculate time range
    recentMetrics := s.metricStorage.GetRecentMetrics(1)
    oldestMetrics := s.metricStorage.GetOldest(1)

    var oldestTS, newestTS uint64
    if len(oldestMetrics) > 0 {
        oldestTS = oldestMetrics[0].Timestamp
    }
    if len(recentMetrics) > 0 {
        newestTS = recentMetrics[0].Timestamp
    }

    utilization := 0.0
    if stats.Capacity > 0 {
        utilization = float64(stats.MetricCount) / float64(stats.Capacity) * 100
    }

    return map[string]interface{}{
        "metric_count": stats.MetricCount,
        "capacity": stats.Capacity,
        "utilization": utilization,
        "unique_names": stats.UniqueNames,
        "service_count": stats.ServiceCount,
        "type_counts": stats.TypeCounts,
        "total_data_points": stats.TotalDataPoints,
        "oldest_timestamp": oldestTS,
        "newest_timestamp": newestTS,
    }, nil
}
```

---

### 7. `clear_metrics`

Clear the metric buffer.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  cleared: number,
  success: boolean
}
```

**Implementation pattern:**
```go
func (s *Server) handleClearMetrics(args map[string]interface{}) (interface{}, error) {
    stats := s.metricStorage.Stats()
    clearedCount := stats.MetricCount

    s.metricStorage.Clear()

    return map[string]interface{}{
        "cleared": clearedCount,
        "success": true,
    }, nil
}
```

---

### 8. `get_metric_names`

List all metric names currently in buffer with counts and types.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  metric_names: Array<{
    name: string,
    count: number,
    type: string,          // Gauge, Sum, Histogram, etc.
    latest_value: number,  // For Gauge/Sum
    service: string
  }>,
  total_unique: number
}
```

**Why this tool:** Quick overview of available metrics without retrieving all data.

**Implementation pattern:**
```go
func (s *Server) handleGetMetricNames(args map[string]interface{}) (interface{}, error) {
    names := s.metricStorage.GetMetricNames()

    metricInfos := []map[string]interface{}{}
    for _, name := range names {
        metrics := s.metricStorage.GetMetricsByName(name)
        if len(metrics) == 0 {
            continue
        }

        latest := metrics[len(metrics)-1]
        info := map[string]interface{}{
            "name": name,
            "count": len(metrics),
            "type": latest.MetricType.String(),
            "service": latest.ServiceName,
        }

        // Add latest value for Gauge/Sum
        if latest.NumericValue != nil {
            info["latest_value"] = *latest.NumericValue
        }

        metricInfos = append(metricInfos, info)
    }

    return map[string]interface{}{
        "metric_names": metricInfos,
        "total_unique": len(names),
    }, nil
}
```

---

## Metric Formatting Helpers

**File:** `internal/mcpserver/metric_formatting.go`

```go
package mcpserver

import "github.com/tobert/otlp-mcp/internal/storage"

// formatMetricsForMCP converts StoredMetric slice to MCP-friendly format.
func formatMetricsForMCP(metrics []*storage.StoredMetric) []map[string]interface{} {
    result := make([]map[string]interface{}, len(metrics))
    for i, m := range metrics {
        result[i] = formatMetricForMCP(m)
    }
    return result
}

// formatMetricForMCP converts a single StoredMetric to MCP format.
func formatMetricForMCP(m *storage.StoredMetric) map[string]interface{} {
    formatted := map[string]interface{}{
        "metric_name": m.MetricName,
        "service_name": m.ServiceName,
        "metric_type": m.MetricType.String(),
        "timestamp": m.Timestamp,
        "data_point_count": m.DataPointCount,
    }

    // Add type-specific fields
    if m.NumericValue != nil {
        formatted["value"] = *m.NumericValue
    }
    if m.Count != nil {
        formatted["count"] = *m.Count
    }
    if m.Sum != nil {
        formatted["sum"] = *m.Sum
    }

    return formatted
}

// parseMetricType converts string to MetricType.
func parseMetricType(typeStr string) storage.MetricType {
    switch typeStr {
    case "Gauge":
        return storage.MetricTypeGauge
    case "Sum":
        return storage.MetricTypeSum
    case "Histogram":
        return storage.MetricTypeHistogram
    case "ExponentialHistogram":
        return storage.MetricTypeExponentialHistogram
    case "Summary":
        return storage.MetricTypeSummary
    default:
        return storage.MetricTypeUnknown
    }
}
```

---

## Tool Registration

**File:** `internal/mcpserver/metric_tools.go`

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Server) registerMetricTools() error {
    tools := []struct{
        name string
        description string
        schema map[string]interface{}
        handler func(map[string]interface{}) (interface{}, error)
    }{
        {
            name: "get_recent_metrics",
            description: "Get N most recent metrics with pagination",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "limit": map[string]interface{}{"type": "number"},
                    "offset": map[string]interface{}{"type": "number"},
                },
            },
            handler: s.handleGetRecentMetrics,
        },
        {
            name: "get_metrics_by_name",
            description: "Get all data points for a metric name with time range",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "metric_name": map[string]interface{}{"type": "string"},
                    "start_time": map[string]interface{}{"type": "number"},
                    "end_time": map[string]interface{}{"type": "number"},
                    "limit": map[string]interface{}{"type": "number"},
                },
                "required": []string{"metric_name"},
            },
            handler: s.handleGetMetricsByName,
        },
        // ... register remaining 6 tools
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

- [ ] All 8 metric tools implemented
- [ ] All 5 metric types handled correctly (Gauge, Sum, Histogram, ExponentialHistogram, Summary)
- [ ] Tool registration working
- [ ] Pagination working correctly
- [ ] Time range filtering working
- [ ] Type-based queries working
- [ ] Range queries working with snapshot support
- [ ] Stats tools returning accurate data
- [ ] Error handling for invalid inputs
- [ ] Unit tests for each tool
- [ ] Integration tests with all metric types

## Files to Create

- `internal/mcpserver/metric_tools.go` - Tool implementations
- `internal/mcpserver/metric_tools_test.go` - Unit tests
- `internal/mcpserver/metric_formatting.go` - Metric formatting helpers

## Files to Modify

- `internal/mcpserver/server.go` - Call `registerMetricTools()` during initialization
- `internal/storage/metric_storage.go` - Add `GetRange()`, `GetOldest()`, `CurrentPosition()` methods

## Testing Notes

**Test scenarios:**
1. Pagination with various limits
2. Time-series queries for single metric
3. Type filtering (all 5 types)
4. Service filtering
5. Time range filtering
6. Range queries with positions
7. Snapshot range queries
8. Stats with different metric types
9. Empty buffer edge cases
10. Mixed metric types in buffer

---

**Status:** Ready to implement
**Dependencies:** Task 02 (Metrics Support), Task 07 (Snapshot Tools)
**Next:** Task 06 (MCP Span Event Tools)
