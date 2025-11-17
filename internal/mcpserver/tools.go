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
// Instead of 18+ signal-specific tools, we provide 9 snapshot-centric tools:
// 1. get_otlp_endpoint - Get the unified OTLP endpoint (one for all signals)
// 2. add_otlp_port - Add listening ports dynamically (multi-port support)
// 3. remove_otlp_port - Remove ports when no longer needed
// 4. create_snapshot - Bookmark current state across all buffers
// 5. query - Multi-signal query with optional snapshot time range
// 6. get_snapshot_data - Get all signals between two snapshots
// 7. manage_snapshots - List and delete snapshots
// 8. get_stats - Buffer health dashboard
// 9. clear_data - Nuclear reset (wipes everything)
//
// Agents think: "What happened during deployment?" not "Get traces, then logs"
// Dynamic port management: add/remove ports on-demand for long-running programs!
// ═══════════════════════════════════════════════════════════════════════════

// Tool 1: get_otlp_endpoint

type GetOTLPEndpointInput struct{}

type GetOTLPEndpointOutput struct {
	Endpoint         string            `json:"endpoint" jsonschema:"OTLP gRPC endpoint address (accepts traces, logs, and metrics)"`
	Protocol         string            `json:"protocol" jsonschema:"Protocol type (grpc)"`
	EnvironmentVars  map[string]string `json:"environment_vars" jsonschema:"Suggested environment variables for configuring applications"`
}

func (s *Server) handleGetOTLPEndpoint(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetOTLPEndpointInput,
) (*mcp.CallToolResult, GetOTLPEndpointOutput, error) {
	endpoint := s.otlpReceiver.Endpoint()
	return &mcp.CallToolResult{}, GetOTLPEndpointOutput{
		Endpoint: endpoint,
		Protocol: "grpc",
		EnvironmentVars: map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint,
			"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		},
	}, nil
}

// Tool 2: add_otlp_port

type AddOTLPPortInput struct {
	Port int `json:"port" jsonschema:"Port to add (1-65535)"`
}

type AddOTLPPortOutput struct {
	Endpoints []string `json:"endpoints" jsonschema:"All active OTLP endpoint addresses"`
	Success   bool     `json:"success" jsonschema:"Whether port addition succeeded"`
	Message   string   `json:"message,omitempty" jsonschema:"Additional information or error message"`
}

func (s *Server) handleAddOTLPPort(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input AddOTLPPortInput,
) (*mcp.CallToolResult, AddOTLPPortOutput, error) {
	// Validate port range
	if input.Port < 1 || input.Port > 65535 {
		return &mcp.CallToolResult{}, AddOTLPPortOutput{
			Endpoints: s.otlpReceiver.Endpoints(),
			Success:   false,
			Message:   fmt.Sprintf("invalid port %d: must be between 1 and 65535", input.Port),
		}, nil
	}

	// Attempt to add port
	if err := s.otlpReceiver.AddPort(ctx, input.Port); err != nil {
		return &mcp.CallToolResult{}, AddOTLPPortOutput{
			Endpoints: s.otlpReceiver.Endpoints(),
			Success:   false,
			Message:   fmt.Sprintf("failed to add port: %v", err),
		}, nil
	}

	endpoints := s.otlpReceiver.Endpoints()
	return &mcp.CallToolResult{}, AddOTLPPortOutput{
		Endpoints: endpoints,
		Success:   true,
		Message:   fmt.Sprintf("successfully added port %d - now listening on %d ports", input.Port, len(endpoints)),
	}, nil
}

// Tool 3: remove_otlp_port

type RemoveOTLPPortInput struct {
	Port int `json:"port" jsonschema:"Port to remove (1-65535)"`
}

type RemoveOTLPPortOutput struct {
	Endpoints []string `json:"endpoints" jsonschema:"Remaining active OTLP endpoint addresses"`
	Success   bool     `json:"success" jsonschema:"Whether port removal succeeded"`
	Message   string   `json:"message,omitempty" jsonschema:"Additional information or error message"`
}

func (s *Server) handleRemoveOTLPPort(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RemoveOTLPPortInput,
) (*mcp.CallToolResult, RemoveOTLPPortOutput, error) {
	// Validate port range
	if input.Port < 1 || input.Port > 65535 {
		return &mcp.CallToolResult{}, RemoveOTLPPortOutput{
			Endpoints: s.otlpReceiver.Endpoints(),
			Success:   false,
			Message:   fmt.Sprintf("invalid port %d: must be between 1 and 65535", input.Port),
		}, nil
	}

	// Attempt to remove port
	if err := s.otlpReceiver.RemovePort(input.Port); err != nil {
		return &mcp.CallToolResult{}, RemoveOTLPPortOutput{
			Endpoints: s.otlpReceiver.Endpoints(),
			Success:   false,
			Message:   fmt.Sprintf("failed to remove port: %v", err),
		}, nil
	}

	endpoints := s.otlpReceiver.Endpoints()
	return &mcp.CallToolResult{}, RemoveOTLPPortOutput{
		Endpoints: endpoints,
		Success:   true,
		Message:   fmt.Sprintf("successfully removed port %d - now listening on %d ports", input.Port, len(endpoints)),
	}, nil
}

// Tool 4: create_snapshot

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
	// Basic filters
	ServiceName   string   `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	TraceID       string   `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (hex format)"`
	SpanName      string   `json:"span_name,omitempty" jsonschema:"Filter by span operation name"`
	LogSeverity   string   `json:"log_severity,omitempty" jsonschema:"Filter logs by severity (INFO, WARN, ERROR, etc)"`
	MetricNames   []string `json:"metric_names,omitempty" jsonschema:"Filter metrics by names"`
	StartSnapshot string   `json:"start_snapshot,omitempty" jsonschema:"Start of time range (snapshot name)"`
	EndSnapshot   string   `json:"end_snapshot,omitempty" jsonschema:"End of time range (snapshot name, empty = current)"`
	Limit         int      `json:"limit,omitempty" jsonschema:"Maximum results per signal type (0 = no limit)"`

	// Status filters (NEW)
	ErrorsOnly bool   `json:"errors_only,omitempty" jsonschema:"Only return spans with error status (shortcut for span_status=ERROR)"`
	SpanStatus string `json:"span_status,omitempty" jsonschema:"Filter by span status: OK, ERROR, or UNSET"`

	// Duration filters in nanoseconds (NEW)
	MinDurationNs *uint64 `json:"min_duration_ns,omitempty" jsonschema:"Minimum span duration in nanoseconds (e.g., 500000000 for 500ms)"`
	MaxDurationNs *uint64 `json:"max_duration_ns,omitempty" jsonschema:"Maximum span duration in nanoseconds"`

	// Attribute filters (NEW)
	HasAttribute   string            `json:"has_attribute,omitempty" jsonschema:"Filter spans/logs that have this attribute key (e.g., 'http.status_code')"`
	AttributeEquals map[string]string `json:"attribute_equals,omitempty" jsonschema:"Filter by attribute key-value pairs (e.g., {'http.status_code': '500'})"`
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
		// Basic filters
		ServiceName:   input.ServiceName,
		TraceID:       input.TraceID,
		SpanName:      input.SpanName,
		LogSeverity:   input.LogSeverity,
		MetricNames:   input.MetricNames,
		StartSnapshot: input.StartSnapshot,
		EndSnapshot:   input.EndSnapshot,
		Limit:         input.Limit,

		// New filters
		ErrorsOnly:       input.ErrorsOnly,
		SpanStatus:       input.SpanStatus,
		MinDurationNs:    input.MinDurationNs,
		MaxDurationNs:    input.MaxDurationNs,
		HasAttribute:     input.HasAttribute,
		AttributeEquals:  input.AttributeEquals,
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

// Tool 6: get_stats

type GetStatsInput struct{}

type GetStatsOutput struct {
	Traces    StorageStats       `json:"traces" jsonschema:"Trace storage statistics"`
	Logs      LogStorageStats    `json:"logs" jsonschema:"Log storage statistics"`
	Metrics   MetricStorageStats `json:"metrics" jsonschema:"Metric storage statistics"`
	Snapshots int                `json:"snapshot_count" jsonschema:"Number of snapshots"`
}

type StorageStats struct {
	SpanCount  int `json:"span_count" jsonschema:"Current number of spans"`
	Capacity   int `json:"capacity" jsonschema:"Maximum spans capacity"`
	TraceCount int `json:"trace_count" jsonschema:"Number of distinct traces"`
}

type LogStorageStats struct {
	LogCount     int            `json:"log_count" jsonschema:"Current number of logs"`
	Capacity     int            `json:"capacity" jsonschema:"Maximum logs capacity"`
	TraceCount   int            `json:"trace_count" jsonschema:"Logs linked to traces"`
	ServiceCount int            `json:"service_count" jsonschema:"Distinct services"`
	Severities   map[string]int `json:"severities" jsonschema:"Severity level counts"`
}

type MetricStorageStats struct {
	MetricCount  int            `json:"metric_count" jsonschema:"Current number of metrics"`
	Capacity     int            `json:"capacity" jsonschema:"Maximum metrics capacity"`
	UniqueNames  int            `json:"unique_names" jsonschema:"Distinct metric names"`
	ServiceCount int            `json:"service_count" jsonschema:"Distinct services"`
	TypeCounts   map[string]int `json:"type_counts" jsonschema:"Counts by metric type"`
}

func (s *Server) handleGetStats(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetStatsInput,
) (*mcp.CallToolResult, GetStatsOutput, error) {
	stats := s.storage.Stats()

	return &mcp.CallToolResult{}, GetStatsOutput{
		Traces: StorageStats{
			SpanCount:  stats.Traces.SpanCount,
			Capacity:   stats.Traces.Capacity,
			TraceCount: stats.Traces.TraceCount,
		},
		Logs: LogStorageStats{
			LogCount:     stats.Logs.LogCount,
			Capacity:     stats.Logs.Capacity,
			TraceCount:   stats.Logs.TraceCount,
			ServiceCount: stats.Logs.ServiceCount,
			Severities:   stats.Logs.Severities,
		},
		Metrics: MetricStorageStats{
			MetricCount:  stats.Metrics.MetricCount,
			Capacity:     stats.Metrics.Capacity,
			UniqueNames:  stats.Metrics.UniqueNames,
			ServiceCount: stats.Metrics.ServiceCount,
			TypeCounts:   stats.Metrics.TypeCounts,
		},
		Snapshots: stats.Snapshots,
	}, nil
}

// Tool 7: clear_data

type ClearDataInput struct{}

type ClearDataOutput struct {
	Message string `json:"message" jsonschema:"Confirmation message"`
}

func (s *Server) handleClearData(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ClearDataInput,
) (*mcp.CallToolResult, ClearDataOutput, error) {
	s.storage.Clear()

	return &mcp.CallToolResult{}, ClearDataOutput{
		Message: "Cleared all telemetry data and snapshots (complete reset)",
	}, nil
}

// Register all tools

func (s *Server) registerTools() error {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_otlp_endpoint",
		Description: "🚀 START HERE: Get the unified OTLP (OpenTelemetry Protocol) endpoint address. Call this FIRST when working with OpenTelemetry instrumentation, then set OTEL_EXPORTER_OTLP_ENDPOINT=<endpoint> when running programs. Single port accepts traces + logs + metrics from any OTLP-compatible instrumentation (OpenTelemetry SDKs, auto-instrumentation, etc.).",
	}, s.handleGetOTLPEndpoint)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_otlp_port",
		Description: "Add an additional listening port to the OTLP receiver without disrupting existing connections. Useful when Claude Code restarts and you need to listen on a port that running programs are using. The server will accept OpenTelemetry telemetry on all added ports simultaneously. Example: add_otlp_port(40187) to listen on a specific port your application expects.",
	}, s.handleAddOTLPPort)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "remove_otlp_port",
		Description: "Remove a listening port from the OTLP receiver. The server on that port is gracefully stopped. Cannot remove the last port - at least one must remain active. Useful for cleaning up ports that are no longer needed after programs finish.",
	}, s.handleRemoveOTLPPort)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_snapshot",
		Description: "Bookmark this moment in time with a descriptive name (e.g. 'before-deploy', 'test-start', 'after-fix'). Creates a reference point across all OpenTelemetry signals (traces, logs, metrics) so you can compare before/after or query time windows. Think: Git commit for live telemetry. Essential for temporal analysis and debugging.",
	}, s.handleCreateSnapshot)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "query",
		Description: "Search across all OpenTelemetry signals (traces, logs, metrics) with optional filters. Use for ad-hoc observability exploration: filter by service name, trace_id for debugging distributed requests, severity for error analysis, or combine with snapshot time ranges for temporal analysis. Perfect for answering 'show me all ERROR logs' or 'find traces for service X'.",
	}, s.handleQuery)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_snapshot_data",
		Description: "Get everything that happened between two snapshots - perfect for before/after observability analysis. Ask: 'What traces/logs/metrics appeared during deployment?' or 'What changed between test runs?'. Unlocks temporal reasoning: snapshot 'before-deploy' + 'after-deploy' = complete picture of system behavior changes. The foundation of snapshot-driven debugging.",
	}, s.handleGetSnapshotData)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "manage_snapshots",
		Description: "Housekeeping for your observability timeline - list your captured moments ('list'), delete specific bookmarks ('delete'), or clear all snapshots ('clear'). Prefer deleting individual snapshots as you finish analyzing them - keeps your timeline clean without losing data.",
	}, s.handleManageSnapshots)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_stats",
		Description: "Buffer health dashboard - check OpenTelemetry data capacity, current usage, and snapshot count. Use before long-running tests/observations to ensure buffers won't wrap and lose early data. Answers: 'Am I capturing telemetry?', 'How much history do I have?', and 'Will my buffers overflow?'. Shows span/log/metric counts and capacity limits.",
	}, s.handleGetStats)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "clear_data",
		Description: "Nuclear option - wipes ALL telemetry data and snapshots. Use sparingly, only for complete resets. For normal cleanup, delete individual snapshots with manage_snapshots instead - it's surgical vs. scorched earth.",
	}, s.handleClearData)

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
