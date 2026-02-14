package viz

import (
	"strings"
	"testing"
)

func TestWaterfall_Empty(t *testing.T) {
	result := Waterfall(nil, 80)
	if result != "" {
		t.Errorf("expected empty string for nil input, got %q", result)
	}
	result = Waterfall([]SpanInfo{}, 80)
	if result != "" {
		t.Errorf("expected empty string for empty input, got %q", result)
	}
}

func TestWaterfall_SingleSpan(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "abc123", SpanID: "s1", ServiceName: "my-svc", SpanName: "GET /", StartNano: 1000, EndNano: 2000},
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "Trace abc123") {
		t.Errorf("expected trace header, got:\n%s", result)
	}
	if !strings.Contains(result, "1 spans") {
		t.Errorf("expected '1 spans' in header, got:\n%s", result)
	}
	if !strings.Contains(result, "my-svc.GET /") {
		t.Errorf("expected span label, got:\n%s", result)
	}
}

func TestWaterfall_ParentChild(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "aabbcc", SpanID: "root", ParentID: "", ServiceName: "api", SpanName: "GET /users", StartNano: 0, EndNano: 500_000_000},
		{TraceID: "aabbcc", SpanID: "child1", ParentID: "root", ServiceName: "db", SpanName: "query", StartNano: 10_000_000, EndNano: 100_000_000},
		{TraceID: "aabbcc", SpanID: "child2", ParentID: "root", ServiceName: "cache", SpanName: "get", StartNano: 5_000_000, EndNano: 15_000_000},
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "3 spans") {
		t.Errorf("expected '3 spans', got:\n%s", result)
	}
	if !strings.Contains(result, "├─") || !strings.Contains(result, "└─") {
		t.Errorf("expected tree connectors, got:\n%s", result)
	}
}

func TestWaterfall_ErrorSpan(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "err1", SpanID: "s1", ServiceName: "svc", SpanName: "op", StartNano: 0, EndNano: 1000, StatusCode: "ERROR"},
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "!! ERR") {
		t.Errorf("expected error indicator, got:\n%s", result)
	}
}

func TestWaterfall_DeepNesting(t *testing.T) {
	spans := make([]SpanInfo, 6)
	for i := range spans {
		parent := ""
		if i > 0 {
			parent = spans[i-1].SpanID
		}
		spans[i] = SpanInfo{
			TraceID:     "deep1",
			SpanID:      string(rune('a' + i)),
			ParentID:    parent,
			ServiceName: "svc",
			SpanName:    "op",
			StartNano:   uint64(i * 100),
			EndNano:     uint64((i + 1) * 100),
		}
	}
	result := Waterfall(spans, 100)
	// Should have multiple levels of indentation
	lines := strings.Split(result, "\n")
	if len(lines) < 7 { // header + 6 spans
		t.Errorf("expected 7+ lines, got %d:\n%s", len(lines), result)
	}
}

func TestWaterfall_ZeroDuration(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "zero1", SpanID: "s1", ServiceName: "svc", SpanName: "instant", StartNano: 1000, EndNano: 1000},
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "0ns") {
		t.Errorf("expected 0ns duration, got:\n%s", result)
	}
	// Should still render a bar (all #'s for zero-duration)
	if !strings.Contains(result, "##") {
		t.Errorf("expected filled bar for zero-duration, got:\n%s", result)
	}
}

func TestWaterfall_MissingRoot(t *testing.T) {
	// All spans have parents not in the set
	spans := []SpanInfo{
		{TraceID: "orphan1", SpanID: "s1", ParentID: "missing", ServiceName: "svc", SpanName: "op1", StartNano: 0, EndNano: 100},
		{TraceID: "orphan1", SpanID: "s2", ParentID: "missing", ServiceName: "svc", SpanName: "op2", StartNano: 50, EndNano: 150},
	}
	result := Waterfall(spans, 80)
	if result == "" {
		t.Error("expected non-empty result for orphaned spans")
	}
	if !strings.Contains(result, "2 spans") {
		t.Errorf("expected '2 spans', got:\n%s", result)
	}
}

func TestWaterfall_LongSpanName(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "long1", SpanID: "s1", ServiceName: "my-very-long-service-name", SpanName: "GET /api/v1/users/search/by-email", StartNano: 0, EndNano: 1000},
	}
	result := Waterfall(spans, 80)
	// Should truncate with ellipsis
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if len(line) > 82 { // small tolerance
			t.Errorf("line too long (%d chars): %q", len(line), line)
		}
	}
}

func TestWaterfall_MultipleTraces(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "trace1", SpanID: "s1", ServiceName: "svc", SpanName: "op1", StartNano: 0, EndNano: 100},
		{TraceID: "trace2", SpanID: "s2", ServiceName: "svc", SpanName: "op2", StartNano: 200, EndNano: 300},
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "Trace trace1") {
		t.Errorf("expected trace1 header, got:\n%s", result)
	}
	if !strings.Contains(result, "Trace trace2") {
		t.Errorf("expected trace2 header, got:\n%s", result)
	}
}

func TestWaterfall_OverflowTraces(t *testing.T) {
	var spans []SpanInfo
	for i := 0; i < 7; i++ {
		spans = append(spans, SpanInfo{
			TraceID:     string(rune('a' + i)),
			SpanID:      string(rune('a' + i)),
			ServiceName: "svc",
			SpanName:    "op",
			StartNano:   uint64(i * 100),
			EndNano:     uint64(i*100 + 50),
		})
	}
	result := Waterfall(spans, 80)
	if !strings.Contains(result, "+2 more traces") {
		t.Errorf("expected overflow message, got:\n%s", result)
	}
}

func TestWaterfall_EndBeforeStart(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "bad1", SpanID: "s1", ServiceName: "svc", SpanName: "op", StartNano: 5000, EndNano: 1000},
	}
	result := Waterfall(spans, 80)
	// Should not panic, should render with 0 duration
	if !strings.Contains(result, "0ns") {
		t.Errorf("expected 0ns for bad span timing, got:\n%s", result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		nanos    uint64
		expected string
	}{
		{0, "0ns"},
		{500, "0µs"},              // 0.5µs rounds to 0
		{5000, "5µs"},             // 5µs
		{1_500_000, "2ms"},        // 1.5ms rounds to 2
		{500_000_000, "500ms"},    // 500ms
		{1_500_000_000, "1.5s"},   // 1.5s
		{60_000_000_000, "60.0s"}, // 60s
	}
	for _, tt := range tests {
		got := formatDuration(tt.nanos)
		if got != tt.expected {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.nanos, got, tt.expected)
		}
	}
}
