package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
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
	Endpoint        string            `json:"endpoint" jsonschema:"OTLP gRPC endpoint address (accepts traces, logs, and metrics)"`
	Protocol        string            `json:"protocol" jsonschema:"Protocol type (grpc)"`
	EnvironmentVars map[string]string `json:"environment_vars" jsonschema:"Suggested environment variables for configuring applications"`
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

type CreateSnapshotOutput struct {
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

	_ = s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "otlp://snapshots"})

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
	HasAttribute    string            `json:"has_attribute,omitempty" jsonschema:"Filter spans/logs that have this attribute key (e.g., 'http.status_code')"`
	AttributeEquals map[string]string `json:"attribute_equals,omitempty" jsonschema:"Filter by attribute key-value pairs (e.g., {'http.status_code': '500'})"`
}

type QueryOutput struct {
	Traces  []TraceSummary  `json:"traces" jsonschema:"Matching trace spans"`
	Logs    []LogSummary    `json:"logs" jsonschema:"Matching log records"`
	Metrics []MetricSummary `json:"metrics" jsonschema:"Matching metrics"`
	Summary QuerySummary    `json:"summary" jsonschema:"Query result summary"`
}

type QuerySummary struct {
	TraceCount  int      `json:"trace_count" jsonschema:"Number of spans returned"`
	LogCount    int      `json:"log_count" jsonschema:"Number of logs returned"`
	MetricCount int      `json:"metric_count" jsonschema:"Number of metrics returned"`
	Services    []string `json:"services" jsonschema:"Distinct services in results"`
	TraceIDs    []string `json:"trace_ids" jsonschema:"Distinct trace IDs in results"`
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
		ErrorsOnly:      input.ErrorsOnly,
		SpanStatus:      input.SpanStatus,
		MinDurationNs:   input.MinDurationNs,
		MaxDurationNs:   input.MaxDurationNs,
		HasAttribute:    input.HasAttribute,
		AttributeEquals: input.AttributeEquals,
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
	TraceCount    int            `json:"trace_count" jsonschema:"Number of spans"`
	LogCount      int            `json:"log_count" jsonschema:"Number of logs"`
	MetricCount   int            `json:"metric_count" jsonschema:"Number of metrics"`
	Services      []string       `json:"services" jsonschema:"Distinct services"`
	TraceIDs      []string       `json:"trace_ids" jsonschema:"Distinct trace IDs"`
	LogSeverities map[string]int `json:"log_severities" jsonschema:"Log severity counts"`
	MetricNames   []string       `json:"metric_names" jsonschema:"Distinct metric names"`
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
		_ = s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "otlp://snapshots"})
		return &mcp.CallToolResult{}, ManageSnapshotsOutput{
			Action:  "delete",
			Message: fmt.Sprintf("Deleted snapshot '%s'", input.Name),
		}, nil

	case "clear":
		s.storage.Snapshots().Clear()
		_ = s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "otlp://snapshots"})
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

// Tool 10: set_file_source

type SetFileSourceInput struct {
	Directory string `json:"directory" jsonschema:"Path to directory containing OTLP JSONL files (e.g., /tank/otel). Must have traces/, logs/, and/or metrics/ subdirectories."`
	// ActiveOnly when true (default) only loads active files like traces.jsonl,
	// skipping rotated archives like traces-2025-12-09T13-10-56.jsonl.
	// Set to false to load all files including archives.
	ActiveOnly *bool `json:"active_only,omitempty" jsonschema:"Only load active files, skip rotated archives (default: true)"`
}

type SetFileSourceOutput struct {
	Directory   string   `json:"directory" jsonschema:"Directory being watched"`
	WatchedDirs []string `json:"watched_dirs" jsonschema:"Subdirectories being watched (traces, logs, metrics)"`
	Success     bool     `json:"success" jsonschema:"Whether setup succeeded"`
	Message     string   `json:"message,omitempty" jsonschema:"Additional information"`
}

func (s *Server) handleSetFileSource(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input SetFileSourceInput,
) (*mcp.CallToolResult, SetFileSourceOutput, error) {
	if input.Directory == "" {
		return &mcp.CallToolResult{}, SetFileSourceOutput{
			Success: false,
			Message: "directory is required",
		}, nil
	}

	// Default activeOnly to true if not specified
	activeOnly := true
	if input.ActiveOnly != nil {
		activeOnly = *input.ActiveOnly
	}

	if err := s.AddFileSource(ctx, input.Directory, activeOnly); err != nil {
		return &mcp.CallToolResult{}, SetFileSourceOutput{
			Directory: input.Directory,
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	// Get stats for the newly added source
	stats := s.FileSourceStats()
	var watchedDirs []string
	for _, stat := range stats {
		if stat.Directory == input.Directory {
			watchedDirs = stat.WatchedDirs
			break
		}
	}

	_ = s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "otlp://file-sources"})

	return &mcp.CallToolResult{}, SetFileSourceOutput{
		Directory:   input.Directory,
		WatchedDirs: watchedDirs,
		Success:     true,
		Message:     fmt.Sprintf("Now watching %s for OTLP JSONL files. Data loaded into ring buffers.", input.Directory),
	}, nil
}

// Tool 11: remove_file_source

type RemoveFileSourceInput struct {
	Directory string `json:"directory" jsonschema:"Directory to stop watching"`
}

type RemoveFileSourceOutput struct {
	Directory string `json:"directory" jsonschema:"Directory that was removed"`
	Success   bool   `json:"success" jsonschema:"Whether removal succeeded"`
	Message   string `json:"message,omitempty" jsonschema:"Additional information"`
}

func (s *Server) handleRemoveFileSource(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RemoveFileSourceInput,
) (*mcp.CallToolResult, RemoveFileSourceOutput, error) {
	if input.Directory == "" {
		return &mcp.CallToolResult{}, RemoveFileSourceOutput{
			Success: false,
			Message: "directory is required",
		}, nil
	}

	if err := s.RemoveFileSource(input.Directory); err != nil {
		return &mcp.CallToolResult{}, RemoveFileSourceOutput{
			Directory: input.Directory,
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	_ = s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "otlp://file-sources"})

	return &mcp.CallToolResult{}, RemoveFileSourceOutput{
		Directory: input.Directory,
		Success:   true,
		Message:   fmt.Sprintf("Stopped watching %s. Previously loaded data remains in buffers.", input.Directory),
	}, nil
}

// Tool 12: list_file_sources

type ListFileSourcesInput struct{}

type FileSourceInfo struct {
	Directory    string   `json:"directory" jsonschema:"Directory path"`
	WatchedDirs  []string `json:"watched_dirs" jsonschema:"Subdirectories being watched"`
	FilesTracked int      `json:"files_tracked" jsonschema:"Number of files being tracked"`
}

type ListFileSourcesOutput struct {
	Sources []FileSourceInfo `json:"sources" jsonschema:"Active file sources"`
	Count   int              `json:"count" jsonschema:"Number of active file sources"`
}

func (s *Server) handleListFileSources(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListFileSourcesInput,
) (*mcp.CallToolResult, ListFileSourcesOutput, error) {
	stats := s.FileSourceStats()

	sources := make([]FileSourceInfo, len(stats))
	for i, stat := range stats {
		sources[i] = FileSourceInfo{
			Directory:    stat.Directory,
			WatchedDirs:  stat.WatchedDirs,
			FilesTracked: stat.FilesTracked,
		}
	}

	return &mcp.CallToolResult{}, ListFileSourcesOutput{
		Sources: sources,
		Count:   len(sources),
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// FAST POLLING TOOLS - Optimized for frequent status checks
// ═══════════════════════════════════════════════════════════════════════════

// Tool: status (fast polling, ~100ms)

type StatusInput struct{}

type StatusOutput struct {
	SpansReceived    uint64  `json:"spans_received" jsonschema:"Total spans received since startup"`
	LogsReceived     uint64  `json:"logs_received" jsonschema:"Total logs received since startup"`
	MetricsReceived  uint64  `json:"metrics_received" jsonschema:"Total metrics received since startup"`
	RecentErrorCount int     `json:"recent_error_count" jsonschema:"Number of recent errors tracked (max 100)"`
	Generation       uint64  `json:"generation" jsonschema:"Change counter - incremented on any telemetry receipt"`
	UptimeSeconds    float64 `json:"uptime_seconds" jsonschema:"Server uptime in seconds"`
}

func (s *Server) handleStatus(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input StatusInput,
) (*mcp.CallToolResult, StatusOutput, error) {
	cache := s.storage.ActivityCache()

	return &mcp.CallToolResult{}, StatusOutput{
		SpansReceived:    cache.SpansReceived(),
		LogsReceived:     cache.LogsReceived(),
		MetricsReceived:  cache.MetricsReceived(),
		RecentErrorCount: cache.RecentErrorCount(),
		Generation:       cache.Generation(),
		UptimeSeconds:    cache.UptimeSeconds(),
	}, nil
}

// Tool: recent_activity (rich polling, 1-5s)

// MaxMetricPeekNames is the maximum number of metric names allowed in a peek request.
const MaxMetricPeekNames = 20

// DefaultWindowDurationMs is the default window duration for activity summary.
const DefaultWindowDurationMs = 60000 // 1 minute

type RecentActivityInput struct {
	MetricNames []string `json:"metric_names,omitempty" jsonschema:"Metric names to peek (max 20, empty = none)"`
}

type RecentActivityOutput struct {
	RecentTraces     []ActivityTraceSummary `json:"recent_traces" jsonschema:"Most recent 5 traces"`
	RecentErrors     []ActivityErrorSummary `json:"recent_errors" jsonschema:"Most recent 5 errors (separate from traces)"`
	Throughput       ActivityThroughput     `json:"throughput" jsonschema:"Throughput counters (caller computes rates)"`
	MetricsPeek      []ActivityMetricPeek   `json:"metrics_peek,omitempty" jsonschema:"Current values for requested metrics"`
	WindowDurationMs int64                  `json:"window_duration_ms" jsonschema:"Window duration in milliseconds"`
}

type ActivityTraceSummary struct {
	TraceID    string  `json:"trace_id" jsonschema:"Trace ID"`
	Service    string  `json:"service" jsonschema:"Service name"`
	RootSpan   string  `json:"root_span" jsonschema:"Root span name (or first span seen)"`
	Status     string  `json:"status" jsonschema:"Status: OK, ERROR, or UNSET"`
	DurationMs float64 `json:"duration_ms" jsonschema:"Duration in milliseconds"`
	ErrorMsg   string  `json:"error_msg,omitempty" jsonschema:"Error message if status is ERROR"`
}

type ActivityErrorSummary struct {
	TraceID   string `json:"trace_id" jsonschema:"Trace ID"`
	Service   string `json:"service" jsonschema:"Service name"`
	SpanName  string `json:"span_name" jsonschema:"Span name where error occurred"`
	ErrorMsg  string `json:"error_msg" jsonschema:"Error message"`
	Timestamp uint64 `json:"timestamp_unix_nano" jsonschema:"Error timestamp (Unix nanoseconds)"`
}

type ActivityThroughput struct {
	TotalSpans   uint64 `json:"total_spans" jsonschema:"Total spans received"`
	TotalLogs    uint64 `json:"total_logs" jsonschema:"Total logs received"`
	TotalMetrics uint64 `json:"total_metrics" jsonschema:"Total metrics received"`
}

type ActivityMetricPeek struct {
	Name        string             `json:"name" jsonschema:"Metric name"`
	Type        string             `json:"type" jsonschema:"Metric type"`
	Value       *float64           `json:"value,omitempty" jsonschema:"Current value (Gauge/Sum)"`
	Count       *uint64            `json:"count,omitempty" jsonschema:"Count (Histogram)"`
	Sum         *float64           `json:"sum,omitempty" jsonschema:"Sum (Histogram)"`
	Min         *float64           `json:"min,omitempty" jsonschema:"Min value (Histogram)"`
	Max         *float64           `json:"max,omitempty" jsonschema:"Max value (Histogram)"`
	Percentiles map[string]float64 `json:"percentiles,omitempty" jsonschema:"Percentiles: p50, p95, p99 (Histogram)"`
}

func (s *Server) handleRecentActivity(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RecentActivityInput,
) (*mcp.CallToolResult, RecentActivityOutput, error) {
	// Validate metric names limit
	if len(input.MetricNames) > MaxMetricPeekNames {
		return nil, RecentActivityOutput{}, fmt.Errorf("too many metric names: max %d allowed, got %d", MaxMetricPeekNames, len(input.MetricNames))
	}

	cache := s.storage.ActivityCache()

	// Get recent traces (5)
	recentTraces := cache.RecentTraces(5)
	traces := make([]ActivityTraceSummary, len(recentTraces))
	for i, t := range recentTraces {
		traces[i] = ActivityTraceSummary{
			TraceID:    t.TraceID,
			Service:    t.Service,
			RootSpan:   t.RootSpan,
			Status:     t.Status,
			DurationMs: t.DurationMs,
			ErrorMsg:   t.ErrorMsg,
		}
	}

	// Get recent errors (5)
	recentErrors := cache.RecentErrors(5)
	errors := make([]ActivityErrorSummary, len(recentErrors))
	for i, e := range recentErrors {
		errors[i] = ActivityErrorSummary{
			TraceID:   e.TraceID,
			Service:   e.Service,
			SpanName:  e.SpanName,
			ErrorMsg:  e.ErrorMsg,
			Timestamp: e.Timestamp,
		}
	}

	// Get metric peek
	var metricsPeek []ActivityMetricPeek
	if len(input.MetricNames) > 0 {
		peeked := cache.PeekMetrics(input.MetricNames)
		metricsPeek = make([]ActivityMetricPeek, len(peeked))
		for i, p := range peeked {
			metricsPeek[i] = ActivityMetricPeek{
				Name:        p.Name,
				Type:        p.Type.String(),
				Value:       p.Value,
				Count:       p.Count,
				Sum:         p.Sum,
				Min:         p.Min,
				Max:         p.Max,
				Percentiles: p.Percentiles,
			}
		}
	}

	return &mcp.CallToolResult{}, RecentActivityOutput{
		RecentTraces: traces,
		RecentErrors: errors,
		Throughput: ActivityThroughput{
			TotalSpans:   cache.SpansReceived(),
			TotalLogs:    cache.LogsReceived(),
			TotalMetrics: cache.MetricsReceived(),
		},
		MetricsPeek:      metricsPeek,
		WindowDurationMs: DefaultWindowDurationMs,
	}, nil
}

// Register all tools

func (s *Server) registerTools() error {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_otlp_endpoint",
		Description: "Get OTLP endpoint address. Set OTEL_EXPORTER_OTLP_ENDPOINT=<result> to instrument programs.",
	}, s.handleGetOTLPEndpoint)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_otlp_port",
		Description: "Add a listening port to the OTLP receiver without disrupting existing connections.",
	}, s.handleAddOTLPPort)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "remove_otlp_port",
		Description: "Remove a listening port from the OTLP receiver. Cannot remove the last port.",
	}, s.handleRemoveOTLPPort)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_snapshot",
		Description: "Bookmark current buffer positions for before/after comparison across all signals.",
	}, s.handleCreateSnapshot)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "query",
		Description: "Search traces, logs, metrics with filters: service, trace_id, errors_only, duration, attributes, snapshot ranges.",
	}, s.handleQuery)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_snapshot_data",
		Description: "Get all telemetry between two snapshots for before/after analysis.",
	}, s.handleGetSnapshotData)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "manage_snapshots",
		Description: "List, delete, or clear snapshots. Actions: 'list', 'delete', 'clear'.",
	}, s.handleManageSnapshots)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_stats",
		Description: "Buffer health: span/log/metric counts, capacities, snapshot count.",
	}, s.handleGetStats)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "clear_data",
		Description: "Wipe ALL telemetry data and snapshots. Irreversible.",
	}, s.handleClearData)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "set_file_source",
		Description: "Load OTLP JSONL from a collector file exporter directory. Watches for new files.",
	}, s.handleSetFileSource)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "remove_file_source",
		Description: "Stop watching a directory for OTLP data. Loaded data stays in buffers.",
	}, s.handleRemoveFileSource)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_file_sources",
		Description: "List watched directories and file tracking stats.",
	}, s.handleListFileSources)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "status",
		Description: "Fast poll: monotonic counters (spans/logs/metrics), generation counter, error count, uptime.",
	}, s.handleStatus)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "recent_activity",
		Description: "Recent 5 traces, 5 errors, throughput, optional metric peek (pass metric_names, max 20).",
	}, s.handleRecentActivity)

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
	MetricName  string   `json:"metric_name" jsonschema:"Metric name"`
	ServiceName string   `json:"service_name" jsonschema:"Service name"`
	MetricType  string   `json:"metric_type" jsonschema:"Metric type (Gauge, Sum, Histogram, etc)"`
	Timestamp   uint64   `json:"timestamp_unix_nano" jsonschema:"Timestamp (Unix nanoseconds)"`
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
