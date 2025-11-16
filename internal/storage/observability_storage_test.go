package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestObservabilityStorage_CreateSnapshot(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Add some data
	addTestTrace(t, obs, "service1", "trace1", "span1")
	addTestLog(t, obs, "service1", "INFO", "test log")
	addTestMetric(t, obs, "service1", "cpu.usage", 42.0)

	// Create snapshot
	err := obs.CreateSnapshot("test-snapshot")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Verify snapshot exists
	snap, err := obs.Snapshots().Get("test-snapshot")
	if err != nil {
		t.Fatalf("Get snapshot failed: %v", err)
	}

	if snap.Name != "test-snapshot" {
		t.Errorf("Expected name 'test-snapshot', got %q", snap.Name)
	}

	// Positions should match current positions
	if snap.TracePos != obs.Traces().CurrentPosition() {
		t.Errorf("Trace position mismatch")
	}
}

func TestObservabilityStorage_GetSnapshotData(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Create initial snapshot
	err := obs.CreateSnapshot("before")
	if err != nil {
		t.Fatalf("CreateSnapshot 'before' failed: %v", err)
	}

	// Add test data
	addTestTrace(t, obs, "service1", "trace1", "span1")
	addTestTrace(t, obs, "service1", "trace1", "span2")
	addTestLog(t, obs, "service1", "INFO", "log1")
	addTestLog(t, obs, "service1", "ERROR", "log2")
	addTestMetric(t, obs, "service1", "requests", 100.0)

	// Create end snapshot
	err = obs.CreateSnapshot("after")
	if err != nil {
		t.Fatalf("CreateSnapshot 'after' failed: %v", err)
	}

	// Get snapshot data
	data, err := obs.GetSnapshotData("before", "after")
	if err != nil {
		t.Fatalf("GetSnapshotData failed: %v", err)
	}

	// Verify data
	if len(data.Traces) != 2 {
		t.Errorf("Expected 2 traces, got %d", len(data.Traces))
	}
	if len(data.Logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(data.Logs))
	}
	if len(data.Metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(data.Metrics))
	}

	// Verify summary
	if data.Summary.SpanCount != 2 {
		t.Errorf("Summary: expected 2 spans, got %d", data.Summary.SpanCount)
	}
	if data.Summary.LogCount != 2 {
		t.Errorf("Summary: expected 2 logs, got %d", data.Summary.LogCount)
	}
	if data.Summary.MetricCount != 1 {
		t.Errorf("Summary: expected 1 metric, got %d", data.Summary.MetricCount)
	}

	// Verify services in summary
	if len(data.Summary.Services) != 1 || data.Summary.Services[0] != "service1" {
		t.Errorf("Summary: expected [service1], got %v", data.Summary.Services)
	}

	// Verify log severities
	if data.Summary.LogSeverities["INFO"] != 1 {
		t.Errorf("Expected 1 INFO log, got %d", data.Summary.LogSeverities["INFO"])
	}
	if data.Summary.LogSeverities["ERROR"] != 1 {
		t.Errorf("Expected 1 ERROR log, got %d", data.Summary.LogSeverities["ERROR"])
	}
}

func TestObservabilityStorage_GetSnapshotData_CurrentEnd(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Create start snapshot
	err := obs.CreateSnapshot("start")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Add data after snapshot
	addTestTrace(t, obs, "service1", "trace1", "span1")

	// Get data with empty end snapshot (should use current)
	data, err := obs.GetSnapshotData("start", "")
	if err != nil {
		t.Fatalf("GetSnapshotData failed: %v", err)
	}

	if len(data.Traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(data.Traces))
	}

	if data.EndSnapshot != "" {
		t.Errorf("Expected empty end snapshot, got %q", data.EndSnapshot)
	}
}

func TestObservabilityStorage_Query(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Add test data with different services
	// TraceID will be hex-encoded: []byte("trace1") -> "74726163653"
	addTestTrace(t, obs, "service1", "trace1", "http.request")
	addTestTrace(t, obs, "service2", "trace2", "db.query")
	addTestLogWithTrace(t, obs, "service1", "trace1", "INFO", "log for trace1")
	addTestLog(t, obs, "service1", "INFO", "info log")
	addTestLog(t, obs, "service1", "ERROR", "error log")
	addTestMetric(t, obs, "service1", "cpu.usage", 50.0)
	addTestMetric(t, obs, "service2", "memory.usage", 75.0)

	// Get the actual hex-encoded trace ID for filtering
	trace1HexID := fmt.Sprintf("%x", []byte("trace1"))

	tests := []struct {
		name           string
		filter         QueryFilter
		expectedTraces int
		expectedLogs   int
		expectedMetrics int
	}{
		{
			name:           "no filter returns all",
			filter:         QueryFilter{},
			expectedTraces: 2,
			expectedLogs:   3,
			expectedMetrics: 2,
		},
		{
			name: "filter by service",
			filter: QueryFilter{
				ServiceName: "service1",
			},
			expectedTraces: 1,
			expectedLogs:   3,
			expectedMetrics: 1,
		},
		{
			name: "filter by trace ID",
			filter: QueryFilter{
				TraceID: trace1HexID,
			},
			expectedTraces: 1,
			expectedLogs:   1, // One log has trace1
			expectedMetrics: 0, // Metrics don't have trace IDs
		},
		{
			name: "filter by log severity",
			filter: QueryFilter{
				LogSeverity: "ERROR",
			},
			expectedTraces: 2,
			expectedLogs:   1,
			expectedMetrics: 2,
		},
		{
			name: "filter by metric name",
			filter: QueryFilter{
				MetricNames: []string{"cpu.usage"},
			},
			expectedTraces: 2,
			expectedLogs:   3,
			expectedMetrics: 1,
		},
		{
			name: "apply limit",
			filter: QueryFilter{
				Limit: 1,
			},
			expectedTraces: 1,
			expectedLogs:   1,
			expectedMetrics: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obs.Query(tt.filter)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if len(result.Traces) != tt.expectedTraces {
				t.Errorf("Expected %d traces, got %d", tt.expectedTraces, len(result.Traces))
			}
			if len(result.Logs) != tt.expectedLogs {
				t.Errorf("Expected %d logs, got %d", tt.expectedLogs, len(result.Logs))
			}
			if len(result.Metrics) != tt.expectedMetrics {
				t.Errorf("Expected %d metrics, got %d", tt.expectedMetrics, len(result.Metrics))
			}
		})
	}
}

func TestObservabilityStorage_Query_WithSnapshots(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Create before snapshot
	err := obs.CreateSnapshot("before")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Add data in window
	addTestTrace(t, obs, "service1", "trace1", "span1")
	addTestLog(t, obs, "service1", "INFO", "log in window")

	// Create after snapshot
	err = obs.CreateSnapshot("after")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Add data outside window
	addTestTrace(t, obs, "service2", "trace2", "span2")
	addTestLog(t, obs, "service2", "ERROR", "log outside window")

	// Query within snapshot range
	result, err := obs.Query(QueryFilter{
		StartSnapshot: "before",
		EndSnapshot:   "after",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should only see data within snapshots
	if len(result.Traces) != 1 {
		t.Errorf("Expected 1 trace in window, got %d", len(result.Traces))
	}
	if len(result.Logs) != 1 {
		t.Errorf("Expected 1 log in window, got %d", len(result.Logs))
	}
	if result.Traces[0].ServiceName != "service1" {
		t.Errorf("Expected service1 trace, got %s", result.Traces[0].ServiceName)
	}
}

func TestObservabilityStorage_Stats(t *testing.T) {
	obs := NewObservabilityStorage(100, 200, 300)

	// Add some data
	addTestTrace(t, obs, "service1", "trace1", "span1")
	addTestLog(t, obs, "service1", "INFO", "log1")
	addTestMetric(t, obs, "service1", "cpu", 50.0)

	// Create a snapshot
	err := obs.CreateSnapshot("test")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	stats := obs.Stats()

	// Verify capacities
	if stats.Traces.Capacity != 100 {
		t.Errorf("Expected trace capacity 100, got %d", stats.Traces.Capacity)
	}
	if stats.Logs.Capacity != 200 {
		t.Errorf("Expected log capacity 200, got %d", stats.Logs.Capacity)
	}
	if stats.Metrics.Capacity != 300 {
		t.Errorf("Expected metric capacity 300, got %d", stats.Metrics.Capacity)
	}

	// Verify counts
	if stats.Traces.SpanCount != 1 {
		t.Errorf("Expected 1 span, got %d", stats.Traces.SpanCount)
	}
	if stats.Logs.LogCount != 1 {
		t.Errorf("Expected 1 log, got %d", stats.Logs.LogCount)
	}
	if stats.Metrics.MetricCount != 1 {
		t.Errorf("Expected 1 metric, got %d", stats.Metrics.MetricCount)
	}
	if stats.Snapshots != 1 {
		t.Errorf("Expected 1 snapshot, got %d", stats.Snapshots)
	}
}

func TestObservabilityStorage_Clear(t *testing.T) {
	obs := NewObservabilityStorage(100, 100, 100)

	// Add data
	addTestTrace(t, obs, "service1", "trace1", "span1")
	addTestLog(t, obs, "service1", "INFO", "log1")
	addTestMetric(t, obs, "service1", "cpu", 50.0)
	obs.CreateSnapshot("test")

	// Clear everything (nuclear option)
	obs.Clear()

	// Verify complete reset - everything gone
	stats := obs.Stats()
	if stats.Traces.SpanCount != 0 {
		t.Errorf("Expected 0 spans after clear, got %d", stats.Traces.SpanCount)
	}
	if stats.Logs.LogCount != 0 {
		t.Errorf("Expected 0 logs after clear, got %d", stats.Logs.LogCount)
	}
	if stats.Metrics.MetricCount != 0 {
		t.Errorf("Expected 0 metrics after clear, got %d", stats.Metrics.MetricCount)
	}
	if stats.Snapshots != 0 {
		t.Errorf("Expected 0 snapshots after clear (complete reset), got %d", stats.Snapshots)
	}
}

// Helper functions to add test data

func addTestTrace(t *testing.T, obs *ObservabilityStorage, service, traceID, spanName string) {
	t.Helper()

	resourceSpans := []*tracepb.ResourceSpans{
		{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{
					{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}}},
				},
			},
			ScopeSpans: []*tracepb.ScopeSpans{
				{
					Spans: []*tracepb.Span{
						{
							TraceId:           []byte(traceID),
							SpanId:            []byte("span123"),
							Name:              spanName,
							StartTimeUnixNano: uint64(time.Now().UnixNano()),
							EndTimeUnixNano:   uint64(time.Now().UnixNano()),
						},
					},
				},
			},
		},
	}

	err := obs.ReceiveSpans(context.Background(), resourceSpans)
	if err != nil {
		t.Fatalf("ReceiveSpans failed: %v", err)
	}
}

func addTestLog(t *testing.T, obs *ObservabilityStorage, service, severity, body string) {
	t.Helper()
	addTestLogWithTrace(t, obs, service, "", severity, body)
}

func addTestLogWithTrace(t *testing.T, obs *ObservabilityStorage, service, traceID, severity, body string) {
	t.Helper()

	logRecord := &logspb.LogRecord{
		TimeUnixNano: uint64(time.Now().UnixNano()),
		SeverityText: severity,
		Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: body}},
	}

	if traceID != "" {
		logRecord.TraceId = []byte(traceID)
	}

	resourceLogs := []*logspb.ResourceLogs{
		{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{
					{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}}},
				},
			},
			ScopeLogs: []*logspb.ScopeLogs{
				{
					LogRecords: []*logspb.LogRecord{logRecord},
				},
			},
		},
	}

	err := obs.ReceiveLogs(context.Background(), resourceLogs)
	if err != nil {
		t.Fatalf("ReceiveLogs failed: %v", err)
	}
}

func addTestMetric(t *testing.T, obs *ObservabilityStorage, service, metricName string, value float64) {
	t.Helper()

	resourceMetrics := []*metricspb.ResourceMetrics{
		{
			Resource: &resourcepb.Resource{
				Attributes: []*commonpb.KeyValue{
					{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: service}}},
				},
			},
			ScopeMetrics: []*metricspb.ScopeMetrics{
				{
					Metrics: []*metricspb.Metric{
						{
							Name: metricName,
							Data: &metricspb.Metric_Gauge{
								Gauge: &metricspb.Gauge{
									DataPoints: []*metricspb.NumberDataPoint{
										{
											TimeUnixNano: uint64(time.Now().UnixNano()),
											Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: value},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err := obs.ReceiveMetrics(context.Background(), resourceMetrics)
	if err != nil {
		t.Fatalf("ReceiveMetrics failed: %v", err)
	}
}
