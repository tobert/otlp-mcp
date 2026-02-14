package viz

import (
	"strings"
	"testing"
)

func TestRecentTraces_Empty(t *testing.T) {
	result := RecentTraces(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRecentTraces(t *testing.T) {
	traces := []ActivityTrace{
		{TraceID: "aabbccdd11223344", Service: "my-api", RootSpan: "GET /users", Status: "OK", DurationMs: 502},
		{TraceID: "eeff00112233aabb", Service: "my-api", RootSpan: "POST /orders", Status: "ERROR", DurationMs: 1200, ErrorMsg: "timeout"},
	}
	result := RecentTraces(traces)

	if !strings.Contains(result, "Recent Traces (2)") {
		t.Errorf("expected header, got:\n%s", result)
	}
	if !strings.Contains(result, "aabbccdd") {
		t.Errorf("expected truncated trace ID, got:\n%s", result)
	}
	if !strings.Contains(result, "my-api/GET /users") {
		t.Errorf("expected trace label, got:\n%s", result)
	}
	if !strings.Contains(result, "✓") {
		t.Errorf("expected OK icon, got:\n%s", result)
	}
	if !strings.Contains(result, "✗") {
		t.Errorf("expected ERROR icon, got:\n%s", result)
	}
}

func TestRecentErrors_Empty(t *testing.T) {
	result := RecentErrors(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRecentErrors(t *testing.T) {
	errors := []ActivityError{
		{TraceID: "aabbccdd11223344", Service: "db-svc", SpanName: "query", ErrorMsg: "connection refused", Timestamp: 1000},
		{TraceID: "eeff00112233aabb", Service: "cache", SpanName: "get", ErrorMsg: "timeout after 5s", Timestamp: 2000},
	}
	result := RecentErrors(errors)

	if !strings.Contains(result, "Recent Errors (2)") {
		t.Errorf("expected header, got:\n%s", result)
	}
	if !strings.Contains(result, "db-svc/query") {
		t.Errorf("expected error label, got:\n%s", result)
	}
	if !strings.Contains(result, "connection refused") {
		t.Errorf("expected error message, got:\n%s", result)
	}
}

func TestRecentTraces_LongLabel(t *testing.T) {
	traces := []ActivityTrace{
		{TraceID: "aabbccdd", Service: "very-long-service-name", RootSpan: "GET /api/v1/very/long/path/that/exceeds", Status: "OK", DurationMs: 100},
	}
	result := RecentTraces(traces)
	// Should truncate with ellipsis
	if !strings.Contains(result, "…") {
		t.Errorf("expected truncation for long label, got:\n%s", result)
	}
}
