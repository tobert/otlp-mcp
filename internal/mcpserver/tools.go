package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/tobert/otlp-mcp/internal/storage"
)

// Tool input and output types for schema inference

// GetOTLPEndpointInput is the input for the get_otlp_endpoint tool (no parameters needed).
type GetOTLPEndpointInput struct{}

// GetOTLPEndpointOutput is the output for the get_otlp_endpoint tool.
type GetOTLPEndpointOutput struct {
	Endpoint string `json:"endpoint" jsonschema:"OTLP gRPC endpoint address (e.g. localhost:54321)"`
	Protocol string `json:"protocol" jsonschema:"Protocol type (always grpc for MVP)"`
}

// GetRecentTracesInput is the input for the get_recent_traces tool.
type GetRecentTracesInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Number of spans to return (default: 100)"`
}

// GetRecentTracesOutput is the output for the get_recent_traces tool.
type GetRecentTracesOutput struct {
	Spans []SpanSummary `json:"spans" jsonschema:"List of recent spans ordered oldest to newest"`
}

// GetTraceByIDInput is the input for the get_trace_by_id tool.
type GetTraceByIDInput struct {
	TraceID string `json:"trace_id" jsonschema:"Trace ID in hex format"`
}

// GetTraceByIDOutput is the output for the get_trace_by_id tool.
type GetTraceByIDOutput struct{
	Spans []SpanSummary `json:"spans" jsonschema:"All spans for the given trace ID"`
}

// QueryTracesInput is the input for the query_traces tool.
type QueryTracesInput struct {
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	SpanName    string `json:"span_name,omitempty" jsonschema:"Filter by span name"`
}

// QueryTracesOutput is the output for the query_traces tool.
type QueryTracesOutput struct {
	Spans []SpanSummary `json:"spans" jsonschema:"Spans matching the filter criteria"`
}

// GetStatsInput is the input for the get_stats tool (no parameters needed).
type GetStatsInput struct{}

// GetStatsOutput is the output for the get_stats tool.
type GetStatsOutput struct {
	SpanCount  int `json:"span_count" jsonschema:"Current number of spans stored"`
	Capacity   int `json:"capacity" jsonschema:"Maximum number of spans that can be stored"`
	TraceCount int `json:"trace_count" jsonschema:"Number of distinct traces"`
}

// ClearTracesInput is the input for the clear_traces tool (no parameters needed).
type ClearTracesInput struct{}

// ClearTracesOutput is the output for the clear_traces tool.
type ClearTracesOutput struct {
	Status string `json:"status" jsonschema:"Status message (always 'cleared')"`
}

// SpanSummary is a simplified view of a span for MCP responses.
type SpanSummary struct {
	TraceID     string         `json:"trace_id" jsonschema:"Trace ID in hex format"`
	SpanID      string         `json:"span_id" jsonschema:"Span ID in hex format"`
	ServiceName string         `json:"service_name" jsonschema:"Service name from resource attributes"`
	SpanName    string         `json:"span_name" jsonschema:"Span operation name"`
	StartTime   uint64         `json:"start_time_unix_nano" jsonschema:"Start time in Unix nanoseconds"`
	EndTime     uint64         `json:"end_time_unix_nano" jsonschema:"End time in Unix nanoseconds"`
	Status      string         `json:"status,omitempty" jsonschema:"Span status code"`
	Attributes  map[string]any `json:"attributes,omitempty" jsonschema:"Span attributes as key-value pairs"`
}

// registerTools registers all MCP tools with the server.
func (s *Server) registerTools() error {
	// get_otlp_endpoint - returns the OTLP gRPC endpoint address
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_otlp_endpoint",
		Description: "Get the OTLP gRPC endpoint address for sending telemetry data",
	}, s.handleGetOTLPEndpoint)

	// get_recent_traces - returns the N most recent spans
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_recent_traces",
		Description: "Get the N most recent trace spans (default: 100)",
	}, s.handleGetRecentTraces)

	// get_trace_by_id - returns all spans for a specific trace ID
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_trace_by_id",
		Description: "Get all spans for a specific trace ID (hex format)",
	}, s.handleGetTraceByID)

	// query_traces - filters traces by service name or span name
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "query_traces",
		Description: "Query traces by service name or span name",
	}, s.handleQueryTraces)

	// get_stats - returns buffer statistics
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_stats",
		Description: "Get buffer statistics (size, capacity, trace count)",
	}, s.handleGetStats)

	// clear_traces - clears all stored traces
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "clear_traces",
		Description: "Clear all stored traces from the buffer",
	}, s.handleClearTraces)

	return nil
}

// Tool handler implementations

// handleGetOTLPEndpoint returns the OTLP gRPC endpoint address.
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

// handleGetRecentTraces returns the N most recent spans.
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

// handleGetTraceByID returns all spans for a specific trace ID.
func (s *Server) handleGetTraceByID(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetTraceByIDInput,
) (*mcp.CallToolResult, GetTraceByIDOutput, error) {
	spans := s.storage.GetSpansByTraceID(input.TraceID)
	summaries := make([]SpanSummary, 0, len(spans))

	for _, span := range spans {
		summaries = append(summaries, s.spanToSummary(span))
	}

	return &mcp.CallToolResult{}, GetTraceByIDOutput{
		Spans: summaries,
	}, nil
}

// handleQueryTraces filters traces by service name or span name.
func (s *Server) handleQueryTraces(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input QueryTracesInput,
) (*mcp.CallToolResult, QueryTracesOutput, error) {
	var spans []*storage.StoredSpan

	if input.ServiceName != "" {
		spans = s.storage.GetSpansByService(input.ServiceName)
	} else if input.SpanName != "" {
		spans = s.storage.GetSpansByName(input.SpanName)
	} else {
		// No filters - return empty
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

// handleGetStats returns buffer statistics.
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

// handleClearTraces clears all stored traces.
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

// spanToSummary converts a StoredSpan to a SpanSummary for MCP responses.
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

	// Extract status
	if span.Span.Status != nil {
		summary.Status = span.Span.Status.Code.String()
	}

	// Extract attributes
	for _, attr := range span.Span.Attributes {
		summary.Attributes[attr.Key] = formatAttributeValue(attr.Value)
	}

	return summary
}

// formatAttributeValue converts an OTLP attribute value to a Go any type.
func formatAttributeValue(value *commonpb.AnyValue) any {
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
		// Convert array to slice
		result := make([]any, len(v.ArrayValue.Values))
		for i, val := range v.ArrayValue.Values {
			result[i] = formatAttributeValue(val)
		}
		return result
	case *commonpb.AnyValue_KvlistValue:
		// Convert key-value list to map
		result := make(map[string]any)
		for _, kv := range v.KvlistValue.Values {
			result[kv.Key] = formatAttributeValue(kv.Value)
		}
		return result
	default:
		return nil
	}
}
