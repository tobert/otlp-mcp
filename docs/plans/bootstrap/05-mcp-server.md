# Task 05: MCP Server Implementation

## Why

The MCP (Model Context Protocol) server is how agents interact with the telemetry data. It exposes tools for querying traces and resources for accessing configuration. This runs on stdio and provides the agent's interface to the OTLP data.

## What

Implement:
- MCP server using the official Go SDK (`github.com/modelcontextprotocol/go-sdk`)
- stdio transport for agent communication
- MCP tools for trace querying
- Integration with TraceStorage

## Approach

### Using the Official SDK

**Great news**: There is an official MCP Go SDK maintained in collaboration with Google!

- **Package**: `github.com/modelcontextprotocol/go-sdk`
- **Documentation**: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
- **Benefits**: Schema inference, declarative tools, built-in stdio transport

This significantly simplifies implementation compared to building from scratch.

### MCP Server Structure

```go
// internal/mcpserver/server.go
package mcpserver

import (
    "context"
    "fmt"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/tobert/otlp-mcp/internal/storage"
)

// Server wraps the MCP server with our storage
type Server struct {
    mcpServer *mcp.Server
    storage   *storage.TraceStorage
    endpoint  string // OTLP endpoint to expose
}

// NewServer creates a new MCP server
func NewServer(storage *storage.TraceStorage, otlpEndpoint string) (*Server, error) {
    s := &Server{
        storage:  storage,
        endpoint: otlpEndpoint,
    }

    // Create MCP server
    s.mcpServer = mcp.NewServer(&mcp.Implementation{
        Name:    "otlp-mcp",
        Version: "0.1.0",
    }, nil)

    // Register tools
    if err := s.registerTools(); err != nil {
        return nil, fmt.Errorf("failed to register tools: %w", err)
    }

    return s, nil
}

// Run starts the MCP server on stdio (blocks until EOF)
func (s *Server) Run(ctx context.Context) error {
    return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
```

### Tool Definitions with Input/Output Types

```go
// internal/mcpserver/tools.go
package mcpserver

import (
    "context"
    "fmt"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool input/output types (used for schema inference)

type GetOTLPEndpointInput struct{}

type GetOTLPEndpointOutput struct {
    Endpoint string `json:"endpoint" jsonschema:"description=OTLP gRPC endpoint address"`
    Protocol string `json:"protocol" jsonschema:"description=Protocol type (grpc)"`
}

type GetRecentTracesInput struct {
    Limit int `json:"limit,omitempty" jsonschema:"description=Number of spans to return,default=100"`
}

type GetRecentTracesOutput struct {
    Spans []SpanSummary `json:"spans"`
}

type SpanSummary struct {
    TraceID     string            `json:"trace_id"`
    SpanID      string            `json:"span_id"`
    ServiceName string            `json:"service_name"`
    SpanName    string            `json:"span_name"`
    StartTime   uint64            `json:"start_time_unix_nano"`
    EndTime     uint64            `json:"end_time_unix_nano"`
    Status      string            `json:"status,omitempty"`
    Attributes  map[string]any    `json:"attributes,omitempty"`
}

type GetTraceByIDInput struct {
    TraceID string `json:"trace_id" jsonschema:"description=Trace ID in hex format,required=true"`
}

type GetTraceByIDOutput struct {
    Spans []SpanSummary `json:"spans"`
}

type QueryTracesInput struct {
    ServiceName string `json:"service_name,omitempty" jsonschema:"description=Filter by service name"`
    SpanName    string `json:"span_name,omitempty" jsonschema:"description=Filter by span name"`
}

type QueryTracesOutput struct {
    Spans []SpanSummary `json:"spans"`
}

type GetStatsInput struct{}

type GetStatsOutput struct {
    SpanCount  int `json:"span_count"`
    Capacity   int `json:"capacity"`
    TraceCount int `json:"trace_count"`
}

type ClearTracesInput struct{}

type ClearTracesOutput struct {
    Status string `json:"status"`
}

// Register all tools
func (s *Server) registerTools() error {
    // get_otlp_endpoint
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "get_otlp_endpoint",
        Description: "Get the OTLP gRPC endpoint address for sending telemetry",
    }, s.handleGetOTLPEndpoint); err != nil {
        return err
    }

    // get_recent_traces
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "get_recent_traces",
        Description: "Get the N most recent trace spans",
    }, s.handleGetRecentTraces); err != nil {
        return err
    }

    // get_trace_by_id
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "get_trace_by_id",
        Description: "Get all spans for a specific trace ID",
    }, s.handleGetTraceByID); err != nil {
        return err
    }

    // query_traces
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "query_traces",
        Description: "Query traces by service name or span name",
    }, s.handleQueryTraces); err != nil {
        return err
    }

    // get_stats
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "get_stats",
        Description: "Get buffer statistics (size, capacity, trace count)",
    }, s.handleGetStats); err != nil {
        return err
    }

    // clear_traces
    if err := mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "clear_traces",
        Description: "Clear all stored traces from the buffer",
    }, s.handleClearTraces); err != nil {
        return err
    }

    return nil
}

// Tool handlers

func (s *Server) handleGetOTLPEndpoint(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetOTLPEndpointInput,
) (*mcp.CallToolResult, GetOTLPEndpointOutput, error) {
    return &mcp.CallToolResult{}, GetOTLPEndpointOutput{
        Endpoint: s.endpoint,
        Protocol: "grpc",
    }, nil
}

func (s *Server) handleGetRecentTraces(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetRecentTracesInput,
) (*mcp.CallToolResult, GetRecentTracesOutput, error) {
    limit := input.Limit
    if limit == 0 {
        limit = 100
    }

    spans := s.storage.GetRecentSpans(limit)
    summaries := make([]SpanSummary, len(spans))

    for i, span := range spans {
        summaries[i] = s.spanToSummary(span)
    }

    return &mcp.CallToolResult{}, GetRecentTracesOutput{
        Spans: summaries,
    }, nil
}

func (s *Server) handleGetTraceByID(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetTraceByIDInput,
) (*mcp.CallToolResult, GetTraceByIDOutput, error) {
    spans := s.storage.GetSpansByTraceID(input.TraceID)
    summaries := make([]SpanSummary, len(spans))

    for i, span := range spans {
        summaries[i] = s.spanToSummary(span)
    }

    return &mcp.CallToolResult{}, GetTraceByIDOutput{
        Spans: summaries,
    }, nil
}

func (s *Server) handleQueryTraces(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input QueryTracesInput,
) (*mcp.CallToolResult, QueryTracesOutput, error) {
    var spans []*storage.StoredSpan

    if input.ServiceName != "" {
        spans = s.storage.GetSpansByService(input.ServiceName)
    } else {
        // TODO: support span name filtering
        spans = []*storage.StoredSpan{}
    }

    summaries := make([]SpanSummary, len(spans))
    for i, span := range spans {
        summaries[i] = s.spanToSummary(span)
    }

    return &mcp.CallToolResult{}, QueryTracesOutput{
        Spans: summaries,
    }, nil
}

func (s *Server) handleGetStats(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetStatsInput,
) (*mcp.CallToolResult, GetStatsOutput, error) {
    stats := s.storage.Stats()

    return &mcp.CallToolResult{}, GetStatsOutput{
        SpanCount:  stats.SpanCount,
        Capacity:   stats.Capacity,
        TraceCount: stats.TraceCount,
    }, nil
}

func (s *Server) handleClearTraces(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input ClearTracesInput,
) (*mcp.CallToolResult, ClearTracesOutput, error) {
    s.storage.Clear()

    return &mcp.CallToolResult{}, ClearTracesOutput{
        Status: "cleared",
    }, nil
}

// Helper: convert StoredSpan to SpanSummary
func (s *Server) spanToSummary(span *storage.StoredSpan) SpanSummary {
    summary := SpanSummary{
        TraceID:     span.TraceID,
        SpanID:      span.SpanID,
        ServiceName: span.ServiceName,
        SpanName:    span.SpanName,
        StartTime:   span.Span.StartTimeUnixNano,
        EndTime:     span.Span.EndTimeUnixNano,
        Attributes:  make(map[string]any),
    }

    if span.Span.Status != nil {
        summary.Status = span.Span.Status.Code.String()
    }

    // Extract attributes
    for _, attr := range span.Span.Attributes {
        summary.Attributes[attr.Key] = formatAttributeValue(attr.Value)
    }

    return summary
}

func formatAttributeValue(value *commonpb.AnyValue) any {
    switch v := value.Value.(type) {
    case *commonpb.AnyValue_StringValue:
        return v.StringValue
    case *commonpb.AnyValue_IntValue:
        return v.IntValue
    case *commonpb.AnyValue_DoubleValue:
        return v.DoubleValue
    case *commonpb.AnyValue_BoolValue:
        return v.BoolValue
    default:
        return nil
    }
}
```

### Why This Approach is Better

1. **Schema inference**: SDK generates JSON schemas from struct tags automatically
2. **Type safety**: Input validation happens at the SDK layer
3. **Less boilerplate**: No manual JSON-RPC handling, no manual tool registration lists
4. **Maintainability**: Following official patterns means easier updates
5. **Features**: Get resources, prompts, and other MCP features for free when we need them

## Dependencies

- Task 01 (project-setup) must be complete
- Task 04 (ring-buffer) must be complete
- New dependency: `github.com/modelcontextprotocol/go-sdk`

## Acceptance Criteria

- [ ] MCP server dependency added to go.mod
- [ ] Server initializes with stdio transport
- [ ] `get_otlp_endpoint` tool returns correct endpoint
- [ ] `get_recent_traces` tool returns spans from storage
- [ ] `get_trace_by_id` tool filters by trace ID
- [ ] `query_traces` tool filters by service name
- [ ] `get_stats` tool returns buffer statistics
- [ ] `clear_traces` tool clears buffer
- [ ] Schemas are correctly inferred from struct tags

## Testing

### Unit Tests

```go
// internal/mcpserver/server_test.go
package mcpserver

import (
    "context"
    "testing"

    "github.com/tobert/otlp-mcp/internal/storage"
)

func TestServerCreation(t *testing.T) {
    traceStorage := storage.NewTraceStorage(100)
    server, err := NewServer(traceStorage, "localhost:12345")

    if err != nil {
        t.Fatalf("failed to create server: %v", err)
    }

    if server == nil {
        t.Fatal("server is nil")
    }

    if server.endpoint != "localhost:12345" {
        t.Fatalf("unexpected endpoint: %s", server.endpoint)
    }
}

// Note: Full integration tests with MCP client in task 06
```

### Manual Testing

The MCP SDK handles the protocol, so manual testing will happen in task 06 when we wire everything together. For now, verify that:
- Server initializes without errors
- All tools are registered
- No panics during startup

## Notes

### SDK Benefits

- **Automatic schema generation** from Go structs with `jsonschema` tags
- **Built-in stdio transport** - just pass `&mcp.StdioTransport{}`
- **Protocol compliance** - SDK handles all JSON-RPC details
- **Future-proof** - Resources and prompts can be added easily

### Important Imports

```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)
```

### Struct Tag Format

```go
type Example struct {
    Field string `json:"field" jsonschema:"description=Field description,required=true"`
}
```

## Status

Status: pending
Depends: 01-project-setup, 04-ring-buffer
Next: 06-integration.md
