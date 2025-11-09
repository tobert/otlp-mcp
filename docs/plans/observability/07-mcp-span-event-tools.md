# Task 06: MCP Span Event Tools

## Overview

Implement 2 MCP tools for querying spans by their events. Span events capture important moments during a span's lifetime (exceptions, retries, cache hits/misses, etc.). These tools help agents find spans with specific events without retrieving all spans.

**Dependencies:** Bootstrap trace storage (already complete)

## Background: Span Events

**What are span events?**
Events are timestamped annotations within a span that record noteworthy occurrences:
- Exceptions (`exception.type`, `exception.message`, `exception.stacktrace`)
- Retries (`retry.attempt`, `retry.reason`)
- Cache operations (`cache.hit`, `cache.miss`)
- Custom application events

**OTLP Proto:**
```go
type Span struct {
    // ... other fields
    Events []*Event
}

type Event struct {
    TimeUnixNano uint64
    Name         string
    Attributes   []*KeyValue
    DroppedAttributesCount uint32
}
```

**Example use cases:**
- "Find all spans with exception events"
- "Show me spans with retry.attempt > 3"
- "Find spans with cache.miss events during deployment"

---

## Span Event Tools (2 total)

### 1. `query_span_events`

Filter spans by event names and attributes (e.g., find all spans with exception events).

**Parameters:**
```typescript
{
  event_name?: string,           // Event name to match (e.g., "exception")
  event_attribute_key?: string,  // Attribute key (e.g., "exception.type")
  event_attribute_value?: string,// Attribute value (e.g., "NullPointerException")
  trace_id?: string,              // Optional: limit to specific trace
  service_name?: string,          // Optional: limit to specific service
  limit?: number                  // Max results (default: 100)
}
```

**Returns:**
```typescript
{
  spans: Array<{
    span: SpanData,
    matching_events: Array<{
      name: string,
      timestamp: number,
      attributes: {[key: string]: any}
    }>
  }>,
  matched: number,
  returned: number
}
```

**Why this tool:** Find problematic spans without retrieving all spans. Critical for debugging.

**Implementation pattern:**
```go
func (s *Server) handleQuerySpanEvents(args map[string]interface{}) (interface{}, error) {
    eventName, hasEventName := args["event_name"].(string)
    attrKey, hasAttrKey := args["event_attribute_key"].(string)
    attrValue, hasAttrValue := args["event_attribute_value"].(string)

    // Get candidate spans (filtered by trace/service if provided)
    var spans []*storage.Span
    if traceID, ok := args["trace_id"].(string); ok {
        spans = s.traceStorage.GetSpansByTraceID(traceID)
    } else if serviceName, ok := args["service_name"].(string); ok {
        spans = s.traceStorage.GetSpansByService(serviceName)
    } else {
        // Get all recent spans
        spans = s.traceStorage.GetRecentSpans(10000)
    }

    // Filter by event criteria
    matches := []map[string]interface{}{}
    limit := getIntArg(args, "limit", 100)

    for _, span := range spans {
        matchingEvents := []map[string]interface{}{}

        for _, event := range span.Proto.Events {
            // Check event name
            if hasEventName && event.Name != eventName {
                continue
            }

            // Check attributes
            if hasAttrKey {
                attrMatch := false
                for _, attr := range event.Attributes {
                    if attr.Key == attrKey {
                        if !hasAttrValue || attributeValueMatches(attr.Value, attrValue) {
                            attrMatch = true
                            break
                        }
                    }
                }
                if !attrMatch {
                    continue
                }
            }

            // Event matches criteria
            matchingEvents = append(matchingEvents, map[string]interface{}{
                "name": event.Name,
                "timestamp": event.TimeUnixNano,
                "attributes": formatAttributes(event.Attributes),
            })
        }

        // If span has matching events, include it
        if len(matchingEvents) > 0 {
            matches = append(matches, map[string]interface{}{
                "span": formatSpanForMCP(span),
                "matching_events": matchingEvents,
            })

            if len(matches) >= limit {
                break
            }
        }
    }

    return map[string]interface{}{
        "spans": matches,
        "matched": len(matches),
        "returned": len(matches),
    }, nil
}
```

---

### 2. `get_spans_with_events`

Get spans that have events matching criteria, with full event details.

**Parameters:**
```typescript
{
  min_event_count?: number,     // Minimum number of events (e.g., find spans with 3+ events)
  event_names?: string[],       // List of event names to match
  has_exceptions?: boolean,     // Shortcut for exception events
  trace_id?: string,
  service_name?: string,
  limit?: number
}
```

**Returns:**
```typescript
{
  spans: Array<{
    span: SpanData,
    event_count: number,
    event_summary: {
      [event_name: string]: number  // Event name → count
    },
    has_exceptions: boolean
  }>,
  total_matched: number,
  returned: number
}
```

**Why this tool:** Statistical queries ("how many spans had multiple retry events?") and exception hunting.

**Implementation pattern:**
```go
func (s *Server) handleGetSpansWithEvents(args map[string]interface{}) (interface{}, error) {
    minEventCount := getIntArg(args, "min_event_count", 1)
    hasExceptions := getBoolArg(args, "has_exceptions", false)

    var eventNames []string
    if names, ok := args["event_names"].([]interface{}); ok {
        for _, name := range names {
            if nameStr, ok := name.(string); ok {
                eventNames = append(eventNames, nameStr)
            }
        }
    }

    // Get candidate spans
    var spans []*storage.Span
    if traceID, ok := args["trace_id"].(string); ok {
        spans = s.traceStorage.GetSpansByTraceID(traceID)
    } else if serviceName, ok := args["service_name"].(string); ok {
        spans = s.traceStorage.GetSpansByService(serviceName)
    } else {
        spans = s.traceStorage.GetRecentSpans(10000)
    }

    // Filter and analyze events
    matches := []map[string]interface{}{}
    limit := getIntArg(args, "limit", 100)

    for _, span := range spans {
        events := span.Proto.Events

        // Check min event count
        if len(events) < minEventCount {
            continue
        }

        // Build event summary
        eventSummary := make(map[string]int)
        hasExceptionEvent := false

        for _, event := range events {
            eventSummary[event.Name]++

            if strings.HasPrefix(event.Name, "exception") {
                hasExceptionEvent = true
            }
        }

        // Check exception filter
        if hasExceptions && !hasExceptionEvent {
            continue
        }

        // Check event names filter
        if len(eventNames) > 0 {
            found := false
            for _, name := range eventNames {
                if _, exists := eventSummary[name]; exists {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }

        // Span matches criteria
        matches = append(matches, map[string]interface{}{
            "span": formatSpanForMCP(span),
            "event_count": len(events),
            "event_summary": eventSummary,
            "has_exceptions": hasExceptionEvent,
        })

        if len(matches) >= limit {
            break
        }
    }

    return map[string]interface{}{
        "spans": matches,
        "total_matched": len(matches),
        "returned": len(matches),
    }, nil
}
```

---

## Helper Functions

**File:** `internal/mcpserver/event_helpers.go`

```go
package mcpserver

import (
    "strings"
    commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// attributeValueMatches checks if an attribute value matches the expected string.
func attributeValueMatches(attrValue *commonpb.AnyValue, expected string) bool {
    if attrValue == nil {
        return false
    }

    switch v := attrValue.Value.(type) {
    case *commonpb.AnyValue_StringValue:
        return v.StringValue == expected
    case *commonpb.AnyValue_IntValue:
        return fmt.Sprintf("%d", v.IntValue) == expected
    case *commonpb.AnyValue_DoubleValue:
        return fmt.Sprintf("%f", v.DoubleValue) == expected
    case *commonpb.AnyValue_BoolValue:
        return fmt.Sprintf("%t", v.BoolValue) == expected
    default:
        return false
    }
}

// formatAttributes converts protobuf attributes to MCP-friendly map.
func formatAttributes(attrs []*commonpb.KeyValue) map[string]interface{} {
    result := make(map[string]interface{})
    for _, attr := range attrs {
        result[attr.Key] = extractAttributeValue(attr.Value)
    }
    return result
}

// extractAttributeValue extracts the value from an AnyValue.
func extractAttributeValue(value *commonpb.AnyValue) interface{} {
    if value == nil {
        return nil
    }

    switch v := value.Value.(type) {
    case *commonpb.AnyValue_StringValue:
        return v.StringValue
    case *commonpb.AnyValue_IntValue:
        return v.IntValue
    case *commonpb.AnyValue_DoubleValue:
        return v.DoubleValue
    case *commonpb.AnyValue_BoolValue:
        return v.BoolValue
    case *commonpb.AnyValue_ArrayValue:
        return "<array>"  // Simplified for now
    case *commonpb.AnyValue_KvlistValue:
        return "<kvlist>" // Simplified for now
    default:
        return nil
    }
}
```

---

## Tool Registration

**File:** `internal/mcpserver/span_event_tools.go`

```go
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (s *Server) registerSpanEventTools() error {
    tools := []struct{
        name string
        description string
        schema map[string]interface{}
        handler func(map[string]interface{}) (interface{}, error)
    }{
        {
            name: "query_span_events",
            description: "Find spans by event names and attributes (e.g., exceptions)",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "event_name": map[string]interface{}{"type": "string"},
                    "event_attribute_key": map[string]interface{}{"type": "string"},
                    "event_attribute_value": map[string]interface{}{"type": "string"},
                    "trace_id": map[string]interface{}{"type": "string"},
                    "service_name": map[string]interface{}{"type": "string"},
                    "limit": map[string]interface{}{"type": "number"},
                },
            },
            handler: s.handleQuerySpanEvents,
        },
        {
            name: "get_spans_with_events",
            description: "Get spans with event criteria and statistics",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "min_event_count": map[string]interface{}{"type": "number"},
                    "event_names": map[string]interface{}{
                        "type": "array",
                        "items": map[string]interface{}{"type": "string"},
                    },
                    "has_exceptions": map[string]interface{}{"type": "boolean"},
                    "trace_id": map[string]interface{}{"type": "string"},
                    "service_name": map[string]interface{}{"type": "string"},
                    "limit": map[string]interface{}{"type": "number"},
                },
            },
            handler: s.handleGetSpansWithEvents,
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

## Acceptance Criteria

- [ ] Both span event tools implemented
- [ ] Event name filtering working
- [ ] Event attribute filtering working (key and value)
- [ ] Exception detection working
- [ ] Event summary statistics accurate
- [ ] Tool registration working
- [ ] Error handling for invalid inputs
- [ ] Unit tests for each tool
- [ ] Integration tests with real span events

## Files to Create

- `internal/mcpserver/span_event_tools.go` - Tool implementations
- `internal/mcpserver/span_event_tools_test.go` - Unit tests
- `internal/mcpserver/event_helpers.go` - Event formatting/filtering helpers

## Files to Modify

- `internal/mcpserver/server.go` - Call `registerSpanEventTools()` during initialization

## Testing Notes

**Test scenarios:**
1. Query by event name only
2. Query by event attribute key/value
3. Exception event detection
4. Multiple events per span
5. Spans with no events
6. Event summary statistics
7. Min event count filtering
8. Event names array filtering
9. Combined filters (trace_id + event criteria)

**Test data:**
Create spans with various events:
- Exception events with stack traces
- Retry events with attempt counts
- Cache hit/miss events
- Custom application events

---

**Status:** Ready to implement
**Dependencies:** Bootstrap trace storage (complete)
**Next:** Task 07 (MCP Snapshot Tools) - The revolutionary feature!
