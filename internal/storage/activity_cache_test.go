package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestNewActivityCache(t *testing.T) {
	cache := NewActivityCache()

	if cache == nil {
		t.Fatal("NewActivityCache returned nil")
	}

	// Check initial values
	if cache.SpansReceived() != 0 {
		t.Errorf("expected 0 spans received, got %d", cache.SpansReceived())
	}
	if cache.LogsReceived() != 0 {
		t.Errorf("expected 0 logs received, got %d", cache.LogsReceived())
	}
	if cache.MetricsReceived() != 0 {
		t.Errorf("expected 0 metrics received, got %d", cache.MetricsReceived())
	}
	if cache.Generation() != 0 {
		t.Errorf("expected generation 0, got %d", cache.Generation())
	}
	if cache.RecentErrorCount() != 0 {
		t.Errorf("expected 0 recent errors, got %d", cache.RecentErrorCount())
	}
}

func TestActivityCacheRecordSpan(t *testing.T) {
	cache := NewActivityCache()

	// Create a test span
	span := &StoredSpan{
		TraceID:     "abc123",
		SpanID:      "def456",
		ServiceName: "test-service",
		SpanName:    "test-span",
		Span: &tracepb.Span{
			StartTimeUnixNano: 1000000000,
			EndTimeUnixNano:   2000000000,
			Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
	}

	cache.RecordSpan(span)

	if cache.SpansReceived() != 1 {
		t.Errorf("expected 1 span received, got %d", cache.SpansReceived())
	}
	if cache.Generation() != 1 {
		t.Errorf("expected generation 1, got %d", cache.Generation())
	}

	// Check recent traces
	traces := cache.RecentTraces(5)
	if len(traces) != 1 {
		t.Errorf("expected 1 recent trace, got %d", len(traces))
	}
	if traces[0].TraceID != "abc123" {
		t.Errorf("expected trace ID abc123, got %s", traces[0].TraceID)
	}
	if traces[0].Status != "OK" {
		t.Errorf("expected status OK, got %s", traces[0].Status)
	}
}

func TestActivityCacheRecordSpanWithError(t *testing.T) {
	cache := NewActivityCache()

	// Create an error span
	span := &StoredSpan{
		TraceID:     "error123",
		SpanID:      "span456",
		ServiceName: "test-service",
		SpanName:    "failing-span",
		Span: &tracepb.Span{
			StartTimeUnixNano: 1000000000,
			EndTimeUnixNano:   2000000000,
			Status: &tracepb.Status{
				Code:    tracepb.Status_STATUS_CODE_ERROR,
				Message: "connection refused",
			},
		},
	}

	cache.RecordSpan(span)

	if cache.RecentErrorCount() != 1 {
		t.Errorf("expected 1 recent error, got %d", cache.RecentErrorCount())
	}

	errors := cache.RecentErrors(5)
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
	if errors[0].ErrorMsg != "connection refused" {
		t.Errorf("expected error message 'connection refused', got %s", errors[0].ErrorMsg)
	}

	// Check that trace entry also shows error
	traces := cache.RecentTraces(5)
	if len(traces) != 1 {
		t.Errorf("expected 1 trace, got %d", len(traces))
	}
	if traces[0].Status != "ERROR" {
		t.Errorf("expected trace status ERROR, got %s", traces[0].Status)
	}
}

func TestActivityCacheRecordLog(t *testing.T) {
	cache := NewActivityCache()

	cache.RecordLog()
	cache.RecordLog()
	cache.RecordLog()

	if cache.LogsReceived() != 3 {
		t.Errorf("expected 3 logs received, got %d", cache.LogsReceived())
	}
	if cache.Generation() != 3 {
		t.Errorf("expected generation 3, got %d", cache.Generation())
	}
}

func TestActivityCacheRecordMetric(t *testing.T) {
	cache := NewActivityCache()

	value := 42.5
	metric := &StoredMetric{
		MetricName:   "test.metric",
		ServiceName:  "test-service",
		MetricType:   MetricTypeGauge,
		Timestamp:    1000000000,
		NumericValue: &value,
	}

	cache.RecordMetric(metric)

	if cache.MetricsReceived() != 1 {
		t.Errorf("expected 1 metric received, got %d", cache.MetricsReceived())
	}

	// Check metric peek
	peeked := cache.PeekMetrics([]string{"test.metric"})
	if len(peeked) != 1 {
		t.Errorf("expected 1 peeked metric, got %d", len(peeked))
	}
	if *peeked[0].Value != 42.5 {
		t.Errorf("expected value 42.5, got %f", *peeked[0].Value)
	}
}

func TestActivityCachePeekMetricsNotFound(t *testing.T) {
	cache := NewActivityCache()

	peeked := cache.PeekMetrics([]string{"nonexistent.metric"})
	if len(peeked) != 0 {
		t.Errorf("expected 0 peeked metrics for nonexistent, got %d", len(peeked))
	}
}

func TestActivityCacheClear(t *testing.T) {
	cache := NewActivityCache()

	// Add some data
	span := &StoredSpan{
		TraceID:     "abc123",
		SpanID:      "def456",
		ServiceName: "test-service",
		SpanName:    "test-span",
		Span: &tracepb.Span{
			StartTimeUnixNano: 1000000000,
			EndTimeUnixNano:   2000000000,
			Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
	}
	cache.RecordSpan(span)
	cache.RecordLog()

	value := 42.5
	metric := &StoredMetric{
		MetricName:   "test.metric",
		ServiceName:  "test-service",
		MetricType:   MetricTypeGauge,
		NumericValue: &value,
	}
	cache.RecordMetric(metric)

	// Clear
	cache.Clear()

	// Verify everything is reset
	if cache.SpansReceived() != 0 {
		t.Errorf("expected 0 spans after clear, got %d", cache.SpansReceived())
	}
	if cache.LogsReceived() != 0 {
		t.Errorf("expected 0 logs after clear, got %d", cache.LogsReceived())
	}
	if cache.MetricsReceived() != 0 {
		t.Errorf("expected 0 metrics after clear, got %d", cache.MetricsReceived())
	}
	if cache.Generation() != 0 {
		t.Errorf("expected generation 0 after clear, got %d", cache.Generation())
	}
	if len(cache.RecentTraces(5)) != 0 {
		t.Errorf("expected 0 traces after clear, got %d", len(cache.RecentTraces(5)))
	}
	if len(cache.PeekMetrics([]string{"test.metric"})) != 0 {
		t.Errorf("expected 0 peeked metrics after clear")
	}
}

func TestActivityCacheUptime(t *testing.T) {
	cache := NewActivityCache()

	// Sleep a tiny bit
	time.Sleep(10 * time.Millisecond)

	uptime := cache.UptimeSeconds()
	if uptime < 0.01 {
		t.Errorf("expected uptime > 0.01s, got %f", uptime)
	}
}

func TestActivityCacheTraceUpdate(t *testing.T) {
	cache := NewActivityCache()

	// First span (not root)
	span1 := &StoredSpan{
		TraceID:     "trace123",
		SpanID:      "span1",
		ServiceName: "test-service",
		SpanName:    "child-span",
		Span: &tracepb.Span{
			ParentSpanId:      []byte{1, 2, 3, 4, 5, 6, 7, 8},
			StartTimeUnixNano: 1000000000,
			EndTimeUnixNano:   1500000000,
			Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
	}
	cache.RecordSpan(span1)

	// Root span (no parent)
	span2 := &StoredSpan{
		TraceID:     "trace123",
		SpanID:      "span2",
		ServiceName: "test-service",
		SpanName:    "root-span",
		Span: &tracepb.Span{
			ParentSpanId:      nil, // Root span
			StartTimeUnixNano: 900000000,
			EndTimeUnixNano:   2000000000,
			Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		},
	}
	cache.RecordSpan(span2)

	if cache.SpansReceived() != 2 {
		t.Errorf("expected 2 spans received, got %d", cache.SpansReceived())
	}

	// Should still be 1 trace entry (updated)
	traces := cache.RecentTraces(5)
	if len(traces) != 1 {
		t.Errorf("expected 1 trace entry, got %d", len(traces))
	}

	// Root span should now be the "RootSpan"
	if traces[0].RootSpan != "root-span" {
		t.Errorf("expected RootSpan 'root-span', got '%s'", traces[0].RootSpan)
	}
	if traces[0].SpanCount != 2 {
		t.Errorf("expected SpanCount 2, got %d", traces[0].SpanCount)
	}
}

func TestActivityCacheConcurrency(t *testing.T) {
	cache := NewActivityCache()

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			span := &StoredSpan{
				TraceID:     fmt.Sprintf("trace%d", i),
				SpanID:      fmt.Sprintf("span%d", i),
				ServiceName: "test-service",
				SpanName:    "test-span",
				Span: &tracepb.Span{
					StartTimeUnixNano: 1000000000,
					EndTimeUnixNano:   2000000000,
					Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
				},
			}
			cache.RecordSpan(span)
			cache.RecordLog()

			value := float64(i)
			metric := &StoredMetric{
				MetricName:   "test.metric",
				ServiceName:  "test-service",
				MetricType:   MetricTypeGauge,
				NumericValue: &value,
			}
			cache.RecordMetric(metric)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.SpansReceived()
			_ = cache.LogsReceived()
			_ = cache.MetricsReceived()
			_ = cache.Generation()
			_ = cache.RecentTraces(5)
			_ = cache.RecentErrors(5)
			_ = cache.PeekMetrics([]string{"test.metric"})
		}()
	}
	wg.Wait()

	// Verify counts
	if cache.SpansReceived() != 100 {
		t.Errorf("expected 100 spans, got %d", cache.SpansReceived())
	}
	if cache.LogsReceived() != 100 {
		t.Errorf("expected 100 logs, got %d", cache.LogsReceived())
	}
	if cache.MetricsReceived() != 100 {
		t.Errorf("expected 100 metrics, got %d", cache.MetricsReceived())
	}
}
