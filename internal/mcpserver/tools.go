package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// ═══════════════════════════════════════════════════════════════════════════
// SNAPSHOT-FIRST MCP TOOLS
//
// Instead of 18+ signal-specific tools, we provide 5 snapshot-centric tools:
// 1. get_otlp_endpoints - Get all OTLP endpoint addresses
// 2. create_snapshot - Bookmark current state across all buffers
// 3. query - Multi-signal query with optional snapshot time range
// 4. get_snapshot_data - Get all signals between two snapshots
// 5. manage_snapshots - List and delete snapshots
//
// Agents think: "What happened during deployment?" not "Get traces, then logs"
// ═══════════════════════════════════════════════════════════════════════════

// Tool 1: get_otlp_endpoints

type GetOTLPEndpointsInput struct{}

type GetOTLPEndpointsOutput struct {
	TracesEndpoint  string `json:"traces_endpoint" jsonschema:"OTLP gRPC endpoint for traces (e.g. localhost:54321)"`
	LogsEndpoint    string `json:"logs_endpoint" jsonschema:"OTLP gRPC endpoint for logs"`
	MetricsEndpoint string `json:"metrics_endpoint" jsonschema:"OTLP gRPC endpoint for metrics"`
}

func (s *Server) handleGetOTLPEndpoints(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetOTLPEndpointsInput,
) (*mcp.CallToolResult, GetOTLPEndpointsOutput, error) {
	return &mcp.CallToolResult{}, GetOTLPEndpointsOutput{
		TracesEndpoint:  s.endpoints.Traces,
		LogsEndpoint:    s.endpoints.Logs,
		MetricsEndpoint: s.endpoints.Metrics,
	}, nil
}

// Tool 2: create_snapshot

type CreateSnapshotInput struct {
	Name string `json:"name" jsonschema:"Snapshot name (e.g. 'before-deploy', 'test-start')"`
}

type CreateSnapshotOutput struct{
	Name      string `json:"name" jsonschema:"Snapshot name"`
	TracePos  int    `json:"trace_position" jsonschema:"Current trace buffer position"`
	LogPos    int    `json:"log_position" jsonschema:"Current log buffer position"`
	MetricPos int    `json:"metric_position" jsonschema:"Current metric buffer position"`
	Message   string `json:"message" jsonschema:"Success message"`
}

func (s *Server) handleCreateSnapshot(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateSnapshotInput,
) (*mcp.CallToolResult, CreateSnapshotOutput, error) {
	if input.Name == "" {
		return nil, CreateSnapshotOutput{}, fmt.Errorf("snapshot name cannot be empty")
	}

	err := s.storage.CreateSnapshot(input.Name)
	if err != nil {
		return nil, CreateSnapshotOutput{}, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Get the snapshot we just created to return its positions
	snap, err := s.storage.Snapshots().Get(input.Name)
	if err != nil {
		return nil, CreateSnapshotOutput{}, fmt.Errorf("failed to get created snapshot: %w", err)
	}

	return &mcp.CallToolResult{}, CreateSnapshotOutput{
		Name:      snap.Name,
		TracePos:  snap.TracePos,
		LogPos:    snap.LogPos,
		MetricPos: snap.MetricPos,
		Message:   fmt.Sprintf("Created snapshot '%s' at current buffer positions", input.Name),
	}, nil
}

// Tool 3: query (multi-signal with optional snapshots)

type QueryInput struct {
	ServiceName   string   `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	TraceID       string   `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (hex format)"`
	SpanName      string   `json:"span_name,omitempty" jsonschema:"Filter by span operation name"`
	LogSeverity   string   `json:"log_severity,omitempty" jsonschema:"Filter logs by severity (INFO, WARN, ERROR, etc)"`
	MetricNames   []string `json:"metric_names,omitempty" jsonschema:"Filter metrics by names"`
	StartSnapshot string   `json:"start_snapshot,omitempty" jsonschema:"Start of time range (snapshot name)"`
	EndSnapshot   string   `json:"end_snapshot,omitempty" jsonschema:"End of time range (snapshot name, empty = current)"`
	Limit         int      `json:"limit,omitempty" jsonschema:"Maximum results per signal type (0 = no limit)"`
}

type QueryOutput struct {
	Traces  []TraceSummary  `json:"traces" jsonschema:"Matching trace spans"`
	Logs    []LogSummary    `json:"logs" jsonschema:"Matching log records"`
	Metrics []MetricSummary `json:"metrics" jsonschema:"Matching metrics"`
	Summary QuerySummary    `json:"summary" jsonschema:"Query result summary"`
}

type QuerySummary struct {
	TraceCount   int      `json:"trace_count" jsonschema:"Number of spans returned"`
	LogCount     int      `json:"log_count" jsonschema:"Number of logs returned"`
	MetricCount  int      `json:"metric_count" jsonschema:"Number of metrics returned"`
	Services     []string `json:"services" jsonschema:"Distinct services in results"`
	TraceIDs     []string `json:"trace_ids" jsonschema:"Distinct trace IDs in results"`
}

func (s *Server) handleQuery(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input QueryInput,
) (*mcp.CallToolResult, QueryOutput, error) {
	filter := storage.QueryFilter{
		ServiceName:   input.ServiceName,
		TraceID:       input.TraceID,
		SpanName:      input.SpanName,
		LogSeverity:   input.LogSeverity,
		MetricNames:   input.MetricNames,
		StartSnapshot: input.StartSnapshot,
		EndSnapshot:   input.EndSnapshot,
		Limit:         input.Limit,
	}

	result, err := s.storage.Query(filter)
	if err != nil {
		return nil, QueryOutput{}, fmt.Errorf("query failed: %w", err)
	}

	// Convert to MCP-friendly output
	traces := make([]TraceSummary, len(result.Traces))
	for i, span := range result.Traces {
		traces[i] = spanToTraceSummary(span)
	}

	logs := make([]LogSummary, len(result.Logs))
	for i, log := range result.Logs {
		logs[i] = logToSummary(log)
	}

	metrics := make([]MetricSummary, len(result.Metrics))
	for i, metric := range result.Metrics {
		metrics[i] = metricToSummary(metric)
	}

	return &mcp.CallToolResult{}, QueryOutput{
		Traces:  traces,
		Logs:    logs,
		Metrics: metrics,
		Summary: QuerySummary{
			TraceCount:  result.Summary.SpanCount,
			LogCount:    result.Summary.LogCount,
			MetricCount: result.Summary.MetricCount,
			Services:    result.Summary.Services,
			TraceIDs:    result.Summary.TraceIDs,
		},
	}, nil
}

// Tool 4: get_snapshot_data

type GetSnapshotDataInput struct {
	StartSnapshot string `json:"start_snapshot" jsonschema:"Start snapshot name"`
	EndSnapshot   string `json:"end_snapshot,omitempty" jsonschema:"End snapshot name (empty = current)"`
}

type GetSnapshotDataOutput struct {
	StartSnapshot string          `json:"start_snapshot" jsonschema:"Start snapshot name"`
	EndSnapshot   string          `json:"end_snapshot" jsonschema:"End snapshot name"`
	TimeRange     TimeRange       `json:"time_range" jsonschema:"Time window of the data"`
	Traces        []TraceSummary  `json:"traces" jsonschema:"All traces in time range"`
	Logs          []LogSummary    `json:"logs" jsonschema:"All logs in time range"`
	Metrics       []MetricSummary `json:"metrics" jsonschema:"All metrics in time range"`
	Summary       DataSummary     `json:"summary" jsonschema:"Data summary"`
}

type TimeRange struct {
	StartTime string `json:"start_time" jsonschema:"Start time (RFC3339)"`
	EndTime   string `json:"end_time" jsonschema:"End time (RFC3339)"`
	Duration  string `json:"duration" jsonschema:"Duration as string"`
}

type DataSummary struct {
	TraceCount   int              `json:"trace_count" jsonschema:"Number of spans"`
	LogCount     int              `json:"log_count" jsonschema:"Number of logs"`
	MetricCount  int              `json:"metric_count" jsonschema:"Number of metrics"`
	Services     []string         `json:"services" jsonschema:"Distinct services"`
	TraceIDs     []string         `json:"trace_ids" jsonschema:"Distinct trace IDs"`
	LogSeverities map[string]int  `json:"log_severities" jsonschema:"Log severity counts"`
	MetricNames  []string         `json:"metric_names" jsonschema:"Distinct metric names"`
}

func (s *Server) handleGetSnapshotData(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetSnapshotDataInput,
) (*mcp.CallToolResult, GetSnapshotDataOutput, error) {
	if input.StartSnapshot == "" {
		return nil, GetSnapshotDataOutput{}, fmt.Errorf("start_snapshot is required")
	}

	data, err := s.storage.GetSnapshotData(input.StartSnapshot, input.EndSnapshot)
	if err != nil {
		return nil, GetSnapshotDataOutput{}, fmt.Errorf("failed to get snapshot data: %w", err)
	}

	// Convert to MCP-friendly output
	traces := make([]TraceSummary, len(data.Traces))
	for i, span := range data.Traces {
		traces[i] = spanToTraceSummary(span)
	}

	logs := make([]LogSummary, len(data.Logs))
	for i, log := range data.Logs {
		logs[i] = logToSummary(log)
	}

	metrics := make([]MetricSummary, len(data.Metrics))
	for i, metric := range data.Metrics {
		metrics[i] = metricToSummary(metric)
	}

	return &mcp.CallToolResult{}, GetSnapshotDataOutput{
		StartSnapshot: data.StartSnapshot,
		EndSnapshot:   data.EndSnapshot,
		TimeRange: TimeRange{
			StartTime: data.TimeRange.StartTime.Format("2006-01-02T15:04:05.999Z07:00"),
			EndTime:   data.TimeRange.EndTime.Format("2006-01-02T15:04:05.999Z07:00"),
			Duration:  data.TimeRange.Duration,
		},
		Traces:  traces,
		Logs:    logs,
		Metrics: metrics,
		Summary: DataSummary{
			TraceCount:    data.Summary.SpanCount,
			LogCount:      data.Summary.LogCount,
			MetricCount:   data.Summary.MetricCount,
			Services:      data.Summary.Services,
			TraceIDs:      data.Summary.TraceIDs,
			LogSeverities: data.Summary.LogSeverities,
			MetricNames:   data.Summary.MetricNames,
		},
	}, nil
}

// Tool 5: manage_snapshots

type ManageSnapshotsInput struct {
	Action string `json:"action" jsonschema:"Action: 'list', 'delete', or 'clear'"`
	Name   string `json:"name,omitempty" jsonschema:"Snapshot name (required for 'delete')"`
}

type ManageSnapshotsOutput struct {
	Action    string   `json:"action" jsonschema:"Action performed"`
	Snapshots []string `json:"snapshots,omitempty" jsonschema:"List of snapshot names (for 'list')"`
	Message   string   `json:"message" jsonschema:"Status message"`
}

func (s *Server) handleManageSnapshots(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ManageSnapshotsInput,
) (*mcp.CallToolResult, ManageSnapshotsOutput, error) {
	switch input.Action {
	case "list":
		snapshots := s.storage.Snapshots().List()
		return &mcp.CallToolResult{}, ManageSnapshotsOutput{
			Action:    "list",
			Snapshots: snapshots,
			Message:   fmt.Sprintf("Found %d snapshots", len(snapshots)),
		}, nil

	case "delete":
		if input.Name == "" {
			return nil, ManageSnapshotsOutput{}, fmt.Errorf("snapshot name required for delete action")
		}
		err := s.storage.Snapshots().Delete(input.Name)
		if err != nil {
			return nil, ManageSnapshotsOutput{}, fmt.Errorf("failed to delete snapshot: %w", err)
		}
		return &mcp.CallToolResult{}, ManageSnapshotsOutput{
			Action:  "delete",
			Message: fmt.Sprintf("Deleted snapshot '%s'", input.Name),
		}, nil

	case "clear":
		s.storage.Snapshots().Clear()
		return &mcp.CallToolResult{}, ManageSnapshotsOutput{
			Action:  "clear",
			Message: "Cleared all snapshots",
		}, nil

	default:
		return nil, ManageSnapshotsOutput{}, fmt.Errorf("invalid action: %s (must be 'list', 'delete', or 'clear')", input.Action)
	}
}

// Register all tools

func (s *Server) registerTools() error {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_otlp_endpoints",
		Description: "Get OTLP gRPC endpoint addresses for traces, logs, and metrics",
	}, s.handleGetOTLPEndpoints)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_snapshot",
		Description: "Create a named snapshot of current buffer positions across all signal types",
	}, s.handleCreateSnapshot)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "query",
		Description: "Query telemetry data across traces, logs, and metrics with optional filters and snapshot time range",
	}, s.handleQuery)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_snapshot_data",
		Description: "Get all telemetry data between two snapshots (time-based query)",
	}, s.handleGetSnapshotData)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "manage_snapshots",
		Description: "List, delete, or clear snapshots",
	}, s.handleManageSnapshots)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// OUTPUT TYPES - Simplified views of telemetry data
// ═══════════════════════════════════════════════════════════════════════════

type TraceSummary struct {
	TraceID     string         `json:"trace_id" jsonschema:"Trace ID (hex)"`
	SpanID      string         `json:"span_id" jsonschema:"Span ID (hex)"`
	ServiceName string         `json:"service_name" jsonschema:"Service name"`
	SpanName    string         `json:"span_name" jsonschema:"Span operation name"`
	StartTime   uint64         `json:"start_time_unix_nano" jsonschema:"Start time (Unix nanoseconds)"`
	EndTime     uint64         `json:"end_time_unix_nano" jsonschema:"End time (Unix nanoseconds)"`
	Status      string         `json:"status,omitempty" jsonschema:"Span status code"`
	Attributes  map[string]any `json:"attributes,omitempty" jsonschema:"Span attributes"`
}

type LogSummary struct {
	TraceID     string         `json:"trace_id,omitempty" jsonschema:"Associated trace ID (hex)"`
	SpanID      string         `json:"span_id,omitempty" jsonschema:"Associated span ID (hex)"`
	ServiceName string         `json:"service_name" jsonschema:"Service name"`
	Severity    string         `json:"severity" jsonschema:"Severity text (INFO, ERROR, etc)"`
	SeverityNum int32          `json:"severity_number" jsonschema:"Severity number"`
	Body        string         `json:"body" jsonschema:"Log message body"`
	Timestamp   uint64         `json:"timestamp_unix_nano" jsonschema:"Timestamp (Unix nanoseconds)"`
	Attributes  map[string]any `json:"attributes,omitempty" jsonschema:"Log attributes"`
}

type MetricSummary struct {
	MetricName  string  `json:"metric_name" jsonschema:"Metric name"`
	ServiceName string  `json:"service_name" jsonschema:"Service name"`
	MetricType  string  `json:"metric_type" jsonschema:"Metric type (Gauge, Sum, Histogram, etc)"`
	Timestamp   uint64  `json:"timestamp_unix_nano" jsonschema:"Timestamp (Unix nanoseconds)"`
	Value       *float64 `json:"value,omitempty" jsonschema:"Numeric value (for Gauge/Sum)"`
	Count       *uint64  `json:"count,omitempty" jsonschema:"Count (for Histogram)"`
	Sum         *float64 `json:"sum,omitempty" jsonschema:"Sum (for Histogram)"`
	DataPoints  int      `json:"data_point_count" jsonschema:"Number of data points"`
}

// Conversion functions

func spanToTraceSummary(span *storage.StoredSpan) TraceSummary {
	summary := TraceSummary{
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

	// Extract key attributes (limit to prevent huge payloads)
	for i, attr := range span.Span.Attributes {
		if i >= 20 { // Limit to 20 attributes
			break
		}
		summary.Attributes[attr.Key] = formatAttributeValue(attr.Value)
	}

	return summary
}

func logToSummary(log *storage.StoredLog) LogSummary {
	summary := LogSummary{
		TraceID:     log.TraceID,
		SpanID:      log.SpanID,
		ServiceName: log.ServiceName,
		Severity:    log.Severity,
		SeverityNum: log.SeverityNum,
		Body:        log.Body,
		Timestamp:   log.Timestamp,
		Attributes:  make(map[string]any),
	}

	// Extract attributes from log record
	if log.LogRecord != nil {
		for i, attr := range log.LogRecord.Attributes {
			if i >= 20 { // Limit to 20 attributes
				break
			}
			summary.Attributes[attr.Key] = formatAttributeValue(attr.Value)
		}
	}

	return summary
}

func metricToSummary(metric *storage.StoredMetric) MetricSummary {
	return MetricSummary{
		MetricName:  metric.MetricName,
		ServiceName: metric.ServiceName,
		MetricType:  metric.MetricType.String(),
		Timestamp:   metric.Timestamp,
		Value:       metric.NumericValue,
		Count:       metric.Count,
		Sum:         metric.Sum,
		DataPoints:  metric.DataPointCount,
	}
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
