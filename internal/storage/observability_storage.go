package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// ObservabilityStorage provides unified access to all telemetry signals (traces, logs, metrics)
// with snapshot support for time-based queries. This is the primary interface for MCP tools.
type ObservabilityStorage struct {
	traces    *TraceStorage
	logs      *LogStorage
	metrics   *MetricStorage
	snapshots *SnapshotManager
}

// NewObservabilityStorage creates a unified storage layer with the specified capacities.
func NewObservabilityStorage(traceCapacity, logCapacity, metricCapacity int) *ObservabilityStorage {
	return &ObservabilityStorage{
		traces:    NewTraceStorage(traceCapacity),
		logs:      NewLogStorage(logCapacity),
		metrics:   NewMetricStorage(metricCapacity),
		snapshots: NewSnapshotManager(),
	}
}

// Traces returns the underlying trace storage for receiver integration.
func (os *ObservabilityStorage) Traces() *TraceStorage {
	return os.traces
}

// Logs returns the underlying log storage for receiver integration.
func (os *ObservabilityStorage) Logs() *LogStorage {
	return os.logs
}

// Metrics returns the underlying metric storage for receiver integration.
func (os *ObservabilityStorage) Metrics() *MetricStorage {
	return os.metrics
}

// Snapshots returns the snapshot manager.
func (os *ObservabilityStorage) Snapshots() *SnapshotManager {
	return os.snapshots
}

// CreateSnapshot creates a named snapshot of current buffer positions.
// This allows querying "what happened between snapshot A and snapshot B?"
func (os *ObservabilityStorage) CreateSnapshot(name string) error {
	tracePos := os.traces.CurrentPosition()
	logPos := os.logs.CurrentPosition()
	metricPos := os.metrics.CurrentPosition()

	return os.snapshots.Create(name, tracePos, logPos, metricPos)
}

// SnapshotData represents all telemetry data between two points in time.
type SnapshotData struct {
	StartSnapshot string              `json:"start_snapshot"`
	EndSnapshot   string              `json:"end_snapshot"`
	TimeRange     TimeRange           `json:"time_range"`
	Traces        []*StoredSpan       `json:"traces"`
	Logs          []*StoredLog        `json:"logs"`
	Metrics       []*StoredMetric     `json:"metrics"`
	Summary       SnapshotDataSummary `json:"summary"`
}

// TimeRange represents a time window.
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

// SnapshotDataSummary provides quick stats about the snapshot data.
type SnapshotDataSummary struct {
	SpanCount     int            `json:"span_count"`
	LogCount      int            `json:"log_count"`
	MetricCount   int            `json:"metric_count"`
	Services      []string       `json:"services"`
	TraceIDs      []string       `json:"trace_ids"`
	LogSeverities map[string]int `json:"log_severities"`
	MetricNames   []string       `json:"metric_names"`
}

// GetSnapshotData retrieves all telemetry data between two snapshots.
// If endSnapshot is empty, uses current positions.
func (os *ObservabilityStorage) GetSnapshotData(startSnapshot, endSnapshot string) (*SnapshotData, error) {
	// Get start snapshot
	startSnap, err := os.snapshots.Get(startSnapshot)
	if err != nil {
		return nil, fmt.Errorf("start snapshot: %w", err)
	}

	// Get end snapshot (or use current positions)
	var endSnap *Snapshot
	if endSnapshot == "" {
		endSnap = &Snapshot{
			Name:      "current",
			CreatedAt: time.Now(),
			TracePos:  os.traces.CurrentPosition(),
			LogPos:    os.logs.CurrentPosition(),
			MetricPos: os.metrics.CurrentPosition(),
		}
	} else {
		endSnap, err = os.snapshots.Get(endSnapshot)
		if err != nil {
			return nil, fmt.Errorf("end snapshot: %w", err)
		}
	}

	// Validate time ordering
	if endSnap.CreatedAt.Before(startSnap.CreatedAt) {
		return nil, fmt.Errorf("end snapshot (%s) is before start snapshot (%s)",
			endSnap.CreatedAt.Format(time.RFC3339),
			startSnap.CreatedAt.Format(time.RFC3339))
	}

	// Get data ranges from each buffer
	// Subtract 1 from end position because GetRange is inclusive on both ends
	// but snapshots mark "up to but not including this position"
	traces := os.traces.GetRange(startSnap.TracePos, endSnap.TracePos-1)
	logs := os.logs.GetRange(startSnap.LogPos, endSnap.LogPos-1)
	metrics := os.metrics.GetRange(startSnap.MetricPos, endSnap.MetricPos-1)

	// Build summary
	summary := buildSnapshotSummary(traces, logs, metrics)

	return &SnapshotData{
		StartSnapshot: startSnapshot,
		EndSnapshot:   endSnapshot,
		TimeRange: TimeRange{
			StartTime: startSnap.CreatedAt,
			EndTime:   endSnap.CreatedAt,
			Duration:  endSnap.CreatedAt.Sub(startSnap.CreatedAt).String(),
		},
		Traces:  traces,
		Logs:    logs,
		Metrics: metrics,
		Summary: summary,
	}, nil
}

// QueryFilter specifies multi-signal query criteria.
type QueryFilter struct {
	// Basic filters
	ServiceName   string   `json:"service_name,omitempty"`
	TraceID       string   `json:"trace_id,omitempty"`
	SpanName      string   `json:"span_name,omitempty"`
	LogSeverity   string   `json:"log_severity,omitempty"`
	MetricNames   []string `json:"metric_names,omitempty"`
	StartSnapshot string   `json:"start_snapshot,omitempty"`
	EndSnapshot   string   `json:"end_snapshot,omitempty"`
	Limit         int      `json:"limit,omitempty"` // 0 = no limit

	// Status filters
	ErrorsOnly bool   `json:"errors_only,omitempty"`
	SpanStatus string `json:"span_status,omitempty"` // "OK", "ERROR", "UNSET"

	// Duration filters (nanoseconds)
	MinDurationNs *uint64 `json:"min_duration_ns,omitempty"`
	MaxDurationNs *uint64 `json:"max_duration_ns,omitempty"`

	// Attribute filters
	HasAttribute    string            `json:"has_attribute,omitempty"`
	AttributeEquals map[string]string `json:"attribute_equals,omitempty"`
}

// QueryResult contains filtered telemetry data across all signals.
type QueryResult struct {
	Filter  QueryFilter         `json:"filter"`
	Traces  []*StoredSpan       `json:"traces"`
	Logs    []*StoredLog        `json:"logs"`
	Metrics []*StoredMetric     `json:"metrics"`
	Summary SnapshotDataSummary `json:"summary"`
}

// Query performs a multi-signal query with optional snapshot-based time range.
func (os *ObservabilityStorage) Query(filter QueryFilter) (*QueryResult, error) {
	var traces []*StoredSpan
	var logs []*StoredLog
	var metrics []*StoredMetric

	// Determine data range
	if filter.StartSnapshot != "" {
		// Snapshot-based query
		data, err := os.GetSnapshotData(filter.StartSnapshot, filter.EndSnapshot)
		if err != nil {
			return nil, err
		}
		traces = data.Traces
		logs = data.Logs
		metrics = data.Metrics
	} else {
		// Full buffer scan
		traces = os.traces.GetAllSpans()
		logs = os.logs.GetAllLogs()
		metrics = os.metrics.GetAllMetrics()
	}

	// Apply filters to traces
	traces = filterTraces(traces, filter)

	// Apply filters to logs
	logs = filterLogs(logs, filter)

	// Apply filters to metrics
	metrics = filterMetrics(metrics, filter)

	// Apply limit if specified
	if filter.Limit > 0 {
		if len(traces) > filter.Limit {
			traces = traces[:filter.Limit]
		}
		if len(logs) > filter.Limit {
			logs = logs[:filter.Limit]
		}
		if len(metrics) > filter.Limit {
			metrics = metrics[:filter.Limit]
		}
	}

	summary := buildSnapshotSummary(traces, logs, metrics)

	return &QueryResult{
		Filter:  filter,
		Traces:  traces,
		Logs:    logs,
		Metrics: metrics,
		Summary: summary,
	}, nil
}

// AllStats returns comprehensive statistics across all signal types.
type AllStats struct {
	Traces    StorageStats       `json:"traces"`
	Logs      LogStorageStats    `json:"logs"`
	Metrics   MetricStorageStats `json:"metrics"`
	Snapshots int                `json:"snapshot_count"`
}

// Stats returns comprehensive statistics for all storage.
func (os *ObservabilityStorage) Stats() AllStats {
	return AllStats{
		Traces:    os.traces.Stats(),
		Logs:      os.logs.Stats(),
		Metrics:   os.metrics.Stats(),
		Snapshots: os.snapshots.Count(),
	}
}

// Clear removes all telemetry data AND snapshots.
// This is a complete reset - use sparingly. For normal cleanup,
// delete individual snapshots with manage_snapshots instead.
func (os *ObservabilityStorage) Clear() {
	os.traces.Clear()
	os.logs.Clear()
	os.metrics.Clear()
	os.snapshots.Clear()
}

// Receiver interface implementations for OTLP servers

// ReceiveSpans implements the trace receiver interface.
func (os *ObservabilityStorage) ReceiveSpans(ctx context.Context, resourceSpans []*tracepb.ResourceSpans) error {
	return os.traces.ReceiveSpans(ctx, resourceSpans)
}

// ReceiveLogs implements the logs receiver interface.
func (os *ObservabilityStorage) ReceiveLogs(ctx context.Context, resourceLogs []*logspb.ResourceLogs) error {
	return os.logs.ReceiveLogs(ctx, resourceLogs)
}

// ReceiveMetrics implements the metrics receiver interface.
func (os *ObservabilityStorage) ReceiveMetrics(ctx context.Context, resourceMetrics []*metricspb.ResourceMetrics) error {
	return os.metrics.ReceiveMetrics(ctx, resourceMetrics)
}

// Helper functions

func buildSnapshotSummary(traces []*StoredSpan, logs []*StoredLog, metrics []*StoredMetric) SnapshotDataSummary {
	serviceSet := make(map[string]struct{})
	traceIDSet := make(map[string]struct{})
	metricNameSet := make(map[string]struct{})
	logSeverities := make(map[string]int)

	// Aggregate from traces
	for _, span := range traces {
		serviceSet[span.ServiceName] = struct{}{}
		traceIDSet[span.TraceID] = struct{}{}
	}

	// Aggregate from logs
	for _, log := range logs {
		serviceSet[log.ServiceName] = struct{}{}
		if log.TraceID != "" {
			traceIDSet[log.TraceID] = struct{}{}
		}
		logSeverities[log.Severity]++
	}

	// Aggregate from metrics
	for _, metric := range metrics {
		serviceSet[metric.ServiceName] = struct{}{}
		metricNameSet[metric.MetricName] = struct{}{}
	}

	// Convert sets to sorted slices
	services := make([]string, 0, len(serviceSet))
	for svc := range serviceSet {
		services = append(services, svc)
	}
	sort.Strings(services)

	traceIDs := make([]string, 0, len(traceIDSet))
	for tid := range traceIDSet {
		traceIDs = append(traceIDs, tid)
	}
	sort.Strings(traceIDs)

	metricNames := make([]string, 0, len(metricNameSet))
	for name := range metricNameSet {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	return SnapshotDataSummary{
		SpanCount:     len(traces),
		LogCount:      len(logs),
		MetricCount:   len(metrics),
		Services:      services,
		TraceIDs:      traceIDs,
		LogSeverities: logSeverities,
		MetricNames:   metricNames,
	}
}

func filterTraces(traces []*StoredSpan, filter QueryFilter) []*StoredSpan {
	// Check if ANY filter is set that applies to traces
	hasServiceFilter := filter.ServiceName != ""
	hasTraceIDFilter := filter.TraceID != ""
	hasSpanNameFilter := filter.SpanName != ""
	hasStatusFilter := filter.ErrorsOnly || filter.SpanStatus != ""
	hasDurationFilter := filter.MinDurationNs != nil || filter.MaxDurationNs != nil
	hasAttributeFilter := filter.HasAttribute != "" || len(filter.AttributeEquals) > 0

	// If no filters, return all
	if !hasServiceFilter && !hasTraceIDFilter && !hasSpanNameFilter &&
		!hasStatusFilter && !hasDurationFilter && !hasAttributeFilter {
		return traces
	}

	result := make([]*StoredSpan, 0)
	for _, span := range traces {
		// Must match ALL specified filters
		if hasServiceFilter && span.ServiceName != filter.ServiceName {
			continue
		}
		if hasTraceIDFilter && span.TraceID != filter.TraceID {
			continue
		}
		if hasSpanNameFilter && span.SpanName != filter.SpanName {
			continue
		}

		// Status filter
		if hasStatusFilter {
			if !matchesStatusFilter(span, filter) {
				continue
			}
		}

		// Duration filter
		if hasDurationFilter {
			if !matchesDurationFilter(span, filter) {
				continue
			}
		}

		// Attribute filter
		if hasAttributeFilter {
			if !matchesAttributeFilter(span.Span.Attributes, filter) {
				continue
			}
		}

		result = append(result, span)
	}
	return result
}

func filterLogs(logs []*StoredLog, filter QueryFilter) []*StoredLog {
	// Check if ANY filter is set that applies to logs
	hasServiceFilter := filter.ServiceName != ""
	hasTraceIDFilter := filter.TraceID != ""
	hasSeverityFilter := filter.LogSeverity != ""
	hasAttributeFilter := filter.HasAttribute != "" || len(filter.AttributeEquals) > 0

	// If no filters that could match logs, return all
	if !hasServiceFilter && !hasTraceIDFilter && !hasSeverityFilter && !hasAttributeFilter {
		return logs
	}

	result := make([]*StoredLog, 0)
	for _, log := range logs {
		// Must match ALL specified filters
		if hasServiceFilter && log.ServiceName != filter.ServiceName {
			continue
		}
		if hasTraceIDFilter && log.TraceID != filter.TraceID {
			continue
		}
		if hasSeverityFilter && log.Severity != filter.LogSeverity {
			continue
		}

		// Attribute filter
		if hasAttributeFilter && log.LogRecord != nil {
			if !matchesAttributeFilter(log.LogRecord.Attributes, filter) {
				continue
			}
		}

		result = append(result, log)
	}
	return result
}

func filterMetrics(metrics []*StoredMetric, filter QueryFilter) []*StoredMetric {
	// Check if ANY filter is set that applies to metrics
	hasServiceFilter := filter.ServiceName != ""
	hasMetricNamesFilter := len(filter.MetricNames) > 0

	// If TraceID filter is set, metrics can't match (they don't have trace IDs)
	if filter.TraceID != "" {
		return []*StoredMetric{}
	}

	// If no filters that could match metrics, return all
	if !hasServiceFilter && !hasMetricNamesFilter {
		return metrics
	}

	result := make([]*StoredMetric, 0)
	for _, metric := range metrics {
		// Must match ALL specified filters
		if hasServiceFilter && metric.ServiceName != filter.ServiceName {
			continue
		}
		if hasMetricNamesFilter {
			found := false
			for _, name := range filter.MetricNames {
				if metric.MetricName == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, metric)
	}
	return result
}
