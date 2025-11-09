# Task 04: MCP Log Tools

## Overview

Implement 9 MCP tools for querying and analyzing log data stored in LogStorage. These tools enable agents to efficiently search, filter, and analyze logs with minimal context usage.

**Dependencies:** Task 01 (Logs Support) must be complete

## Log Tools (9 total)

### 1. `get_recent_logs`

Returns N most recent log records with optional offset for pagination.

**Parameters:**
```typescript
{
  limit: number,      // Number of logs to return (default: 100, max: 1000)
  offset?: number     // Skip first N logs (default: 0) - for pagination
}
```

**Returns:**
```typescript
{
  logs: LogRecord[],
  total: number,      // Total logs in buffer
  returned: number    // Number of logs in this response
}
```

**Implementation pattern:**
```go
func (s *Server) handleGetRecentLogs(args map[string]interface{}) (interface{}, error) {
    limit := getIntArg(args, "limit", 100)
    offset := getIntArg(args, "offset", 0)

    if limit > 1000 {
        limit = 1000
    }

    allLogs := s.logStorage.GetRecentLogs(offset + limit)
    if offset >= len(allLogs) {
        return map[string]interface{}{
            "logs": []interface{}{},
            "total": s.logStorage.Stats().LogCount,
            "returned": 0,
        }, nil
    }

    logs := allLogs[offset:]
    return map[string]interface{}{
        "logs": formatLogsForMCP(logs),
        "total": s.logStorage.Stats().LogCount,
        "returned": len(logs),
    }, nil
}
```

---

### 2. `get_logs_by_trace_id`

Fetch all logs correlated with a specific trace ID.

**Parameters:**
```typescript
{
  trace_id: string    // Trace ID in hex format
}
```

**Returns:**
```typescript
{
  logs: LogRecord[],
  trace_id: string,
  count: number
}
```

**Use case:** Find all logs associated with a specific request/trace for correlation.

**Implementation pattern:**
```go
func (s *Server) handleGetLogsByTraceID(args map[string]interface{}) (interface{}, error) {
    traceID, ok := args["trace_id"].(string)
    if !ok || traceID == "" {
        return nil, fmt.Errorf("trace_id is required")
    }

    logs := s.logStorage.GetLogsByTraceID(traceID)

    return map[string]interface{}{
        "logs": formatLogsForMCP(logs),
        "trace_id": traceID,
        "count": len(logs),
    }, nil
}
```

---

### 3. `query_logs`

Filter logs by severity, service, time range, and attributes.

**Parameters:**
```typescript
{
  severity?: string,           // Filter by severity (ERROR, WARN, INFO, etc.)
  service_name?: string,       // Filter by service name
  start_time?: number,         // Unix nanoseconds
  end_time?: number,           // Unix nanoseconds
  limit?: number,              // Max results (default: 100)
  offset?: number              // Pagination offset
}
```

**Returns:**
```typescript
{
  logs: LogRecord[],
  filters_applied: string[],
  matched: number,
  returned: number
}
```

**Implementation pattern:**
```go
func (s *Server) handleQueryLogs(args map[string]interface{}) (interface{}, error) {
    var logs []*storage.StoredLog
    filtersApplied := []string{}

    // Apply filters in order of selectivity
    if severity, ok := args["severity"].(string); ok {
        logs = s.logStorage.GetLogsBySeverity(severity)
        filtersApplied = append(filtersApplied, "severity")
    } else if serviceName, ok := args["service_name"].(string); ok {
        logs = s.logStorage.GetLogsByService(serviceName)
        filtersApplied = append(filtersApplied, "service_name")
    } else {
        logs = s.logStorage.GetRecentLogs(10000) // All logs
    }

    // Apply time range filter
    if startTime, ok := args["start_time"].(float64); ok {
        logs = filterLogsByStartTime(logs, uint64(startTime))
        filtersApplied = append(filtersApplied, "start_time")
    }
    if endTime, ok := args["end_time"].(float64); ok {
        logs = filterLogsByEndTime(logs, uint64(endTime))
        filtersApplied = append(filtersApplied, "end_time")
    }

    matched := len(logs)

    // Apply pagination
    limit := getIntArg(args, "limit", 100)
    offset := getIntArg(args, "offset", 0)
    logs = paginateLogs(logs, offset, limit)

    return map[string]interface{}{
        "logs": formatLogsForMCP(logs),
        "filters_applied": filtersApplied,
        "matched": matched,
        "returned": len(logs),
    }, nil
}
```

---

### 4. `grep_logs`

Search log body and attributes with regex pattern (efficient context usage).

**Parameters:**
```typescript
{
  pattern: string,            // Regex pattern
  case_sensitive?: boolean,   // Default: false
  limit?: number,             // Max results (default: 100)
  service_name?: string       // Pre-filter by service
}
```

**Returns:**
```typescript
{
  matches: Array<{
    log: LogRecord,
    matched_text: string,     // The text that matched
    line_preview: string      // Context around match
  }>,
  pattern: string,
  matched: number,
  returned: number
}
```

**Why this tool:** Agents can search logs without retrieving all of them, saving massive context.

**Implementation pattern:**
```go
func (s *Server) handleGrepLogs(args map[string]interface{}) (interface{}, error) {
    pattern, ok := args["pattern"].(string)
    if !ok || pattern == "" {
        return nil, fmt.Errorf("pattern is required")
    }

    caseSensitive := getBoolArg(args, "case_sensitive", false)
    limit := getIntArg(args, "limit", 100)

    var regex *regexp.Regexp
    var err error
    if caseSensitive {
        regex, err = regexp.Compile(pattern)
    } else {
        regex, err = regexp.Compile("(?i)" + pattern)
    }
    if err != nil {
        return nil, fmt.Errorf("invalid regex pattern: %w", err)
    }

    // Get logs (optionally filtered by service first)
    var logs []*storage.StoredLog
    if serviceName, ok := args["service_name"].(string); ok {
        logs = s.logStorage.GetLogsByService(serviceName)
    } else {
        logs = s.logStorage.GetRecentLogs(50000) // Search all
    }

    matches := []map[string]interface{}{}
    for _, log := range logs {
        if match := regex.FindString(log.Body); match != "" {
            matches = append(matches, map[string]interface{}{
                "log": formatLogForMCP(log),
                "matched_text": match,
                "line_preview": generatePreview(log.Body, match),
            })

            if len(matches) >= limit {
                break
            }
        }
    }

    return map[string]interface{}{
        "matches": matches,
        "pattern": pattern,
        "matched": len(matches),
        "returned": len(matches),
    }, nil
}
```

---

### 5. `get_log_range`

Get logs from position X to Y in ring buffer (precise windowing).

**Parameters:**
```typescript
{
  start: number,     // Start position in ring buffer
  end: number,       // End position in ring buffer
  severity?: string  // Optional severity filter
}
```

**Returns:**
```typescript
{
  logs: LogRecord[],
  start: number,
  end: number,
  count: number
}
```

**Why this tool:** Enables precise window queries when agent knows positions (e.g., from snapshots).

**Implementation pattern:**
```go
func (s *Server) handleGetLogRange(args map[string]interface{}) (interface{}, error) {
    start := getIntArg(args, "start", 0)
    end := getIntArg(args, "end", 0)

    if start < 0 || end < start {
        return nil, fmt.Errorf("invalid range: start=%d, end=%d", start, end)
    }

    logs := s.logStorage.GetRange(start, end)

    // Optional severity filter
    if severity, ok := args["severity"].(string); ok {
        logs = filterBySeverity(logs, severity)
    }

    return map[string]interface{}{
        "logs": formatLogsForMCP(logs),
        "start": start,
        "end": end,
        "count": len(logs),
    }, nil
}
```

---

### 6. `get_log_range_snapshot`

Get logs between two named snapshots.

**Parameters:**
```typescript
{
  start_snapshot: string,    // Starting snapshot name
  end_snapshot: string,      // Ending snapshot name (optional, defaults to current)
  severity?: string          // Optional severity filter
}
```

**Returns:**
```typescript
{
  logs: LogRecord[],
  start_snapshot: string,
  end_snapshot: string,
  start_position: number,
  end_position: number,
  count: number
}
```

**Why this tool:** Perfect for "get all ERROR logs from this deployment" or "logs between test runs".

**Implementation pattern:**
```go
func (s *Server) handleGetLogRangeSnapshot(args map[string]interface{}) (interface{}, error) {
    startSnapshotName, ok := args["start_snapshot"].(string)
    if !ok {
        return nil, fmt.Errorf("start_snapshot is required")
    }

    startSnapshot := s.snapshotManager.Get(startSnapshotName)
    if startSnapshot == nil {
        return nil, fmt.Errorf("snapshot not found: %s", startSnapshotName)
    }

    endPosition := s.logStorage.CurrentPosition()
    endSnapshotName := "current"

    if endName, ok := args["end_snapshot"].(string); ok {
        endSnapshot := s.snapshotManager.Get(endName)
        if endSnapshot == nil {
            return nil, fmt.Errorf("snapshot not found: %s", endName)
        }
        endPosition = endSnapshot.LogPos
        endSnapshotName = endName
    }

    logs := s.logStorage.GetRange(startSnapshot.LogPos, endPosition)

    // Optional severity filter
    if severity, ok := args["severity"].(string); ok {
        logs = filterBySeverity(logs, severity)
    }

    return map[string]interface{}{
        "logs": formatLogsForMCP(logs),
        "start_snapshot": startSnapshotName,
        "end_snapshot": endSnapshotName,
        "start_position": startSnapshot.LogPos,
        "end_position": endPosition,
        "count": len(logs),
    }, nil
}
```

---

### 7. `get_log_stats`

Get buffer statistics, severity distribution, and time range.

**Parameters:**
```typescript
{
  // No parameters - returns overall stats
}
```

**Returns:**
```typescript
{
  log_count: number,
  capacity: number,
  utilization: number,           // Percentage
  trace_count: number,            // Distinct traces
  service_count: number,          // Distinct services
  severities: {[key: string]: number},  // Severity → count
  oldest_timestamp: number,       // Unix nanos
  newest_timestamp: number        // Unix nanos
}
```

**Why this tool:** Helps agent understand buffer state before querying.

**Implementation pattern:**
```go
func (s *Server) handleGetLogStats(args map[string]interface{}) (interface{}, error) {
    stats := s.logStorage.Stats()

    // Calculate time range
    recentLogs := s.logStorage.GetRecentLogs(1)
    oldestLogs := s.logStorage.GetOldest(1)

    var oldestTS, newestTS uint64
    if len(oldestLogs) > 0 {
        oldestTS = oldestLogs[0].Timestamp
    }
    if len(recentLogs) > 0 {
        newestTS = recentLogs[0].Timestamp
    }

    utilization := 0.0
    if stats.Capacity > 0 {
        utilization = float64(stats.LogCount) / float64(stats.Capacity) * 100
    }

    return map[string]interface{}{
        "log_count": stats.LogCount,
        "capacity": stats.Capacity,
        "utilization": utilization,
        "trace_count": stats.TraceCount,
        "service_count": stats.ServiceCount,
        "severities": stats.Severities,
        "oldest_timestamp": oldestTS,
        "newest_timestamp": newestTS,
    }, nil
}
```

---

### 8. `clear_logs`

Clear the log buffer.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  cleared: number,     // Number of logs cleared
  success: boolean
}
```

**Implementation pattern:**
```go
func (s *Server) handleClearLogs(args map[string]interface{}) (interface{}, error) {
    stats := s.logStorage.Stats()
    clearedCount := stats.LogCount

    s.logStorage.Clear()

    return map[string]interface{}{
        "cleared": clearedCount,
        "success": true,
    }, nil
}
```

---

### 9. `get_log_severities`

List all severities currently in buffer with counts.

**Parameters:**
```typescript
{
  // No parameters
}
```

**Returns:**
```typescript
{
  severities: Array<{
    name: string,
    count: number,
    percentage: number
  }>,
  total_logs: number
}
```

**Why this tool:** Quick overview of log distribution without retrieving logs.

**Implementation pattern:**
```go
func (s *Server) handleGetLogSeverities(args map[string]interface{}) (interface{}, error) {
    stats := s.logStorage.Stats()

    severities := []map[string]interface{}{}
    totalLogs := stats.LogCount

    for severity, count := range stats.Severities {
        percentage := 0.0
        if totalLogs > 0 {
            percentage = float64(count) / float64(totalLogs) * 100
        }

        severities = append(severities, map[string]interface{}{
            "name": severity,
            "count": count,
            "percentage": percentage,
        })
    }

    return map[string]interface{}{
        "severities": severities,
        "total_logs": totalLogs,
    }, nil
}
```

---

## Tool Registration

**File:** `internal/mcpserver/log_tools.go`

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Server) registerLogTools() error {
    tools := []struct{
        name string
        description string
        schema map[string]interface{}
        handler func(map[string]interface{}) (interface{}, error)
    }{
        {
            name: "get_recent_logs",
            description: "Get N most recent logs with pagination",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "limit": map[string]interface{}{"type": "number"},
                    "offset": map[string]interface{}{"type": "number"},
                },
            },
            handler: s.handleGetRecentLogs,
        },
        {
            name: "get_logs_by_trace_id",
            description: "Get logs correlated with a trace ID",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "trace_id": map[string]interface{}{"type": "string"},
                },
                "required": []string{"trace_id"},
            },
            handler: s.handleGetLogsByTraceID,
        },
        // ... register remaining 7 tools
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

## Acceptance Criteria

- [ ] All 9 log tools implemented
- [ ] Tool registration working
- [ ] Pagination working correctly
- [ ] Regex grep working with case sensitivity option
- [ ] Range queries working with snapshot support
- [ ] Stats tools returning accurate data
- [ ] Error handling for invalid inputs
- [ ] Unit tests for each tool
- [ ] Integration tests with real log data

## Files to Create

- `internal/mcpserver/log_tools.go` - Tool implementations
- `internal/mcpserver/log_tools_test.go` - Unit tests
- `internal/mcpserver/formatting.go` - Log formatting helpers (shared with other tools)

## Files to Modify

- `internal/mcpserver/server.go` - Call `registerLogTools()` during initialization
- `internal/storage/log_storage.go` - Add `GetRange()`, `GetOldest()`, `CurrentPosition()` methods

## Testing Notes

**Test scenarios:**
1. Pagination with offset/limit
2. Trace ID correlation
3. Severity filtering
4. Time range filtering
5. Regex grep with special characters
6. Range queries with invalid positions
7. Snapshot range queries
8. Stats accuracy with buffer wraparound
9. Empty buffer edge cases

---

**Status:** Ready to implement
**Dependencies:** Task 01 (Logs Support), Task 07 (Snapshot Tools for range queries)
**Next:** Task 05 (MCP Metric Tools)
