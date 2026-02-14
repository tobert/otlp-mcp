package viz

import (
	"strings"
	"testing"
)

func TestStatsOverview(t *testing.T) {
	stats := BufferStats{
		SpanCount:      1204,
		SpanCapacity:   10000,
		LogCount:       4891,
		LogCapacity:    50000,
		MetricCount:    2003,
		MetricCapacity: 100000,
		SnapshotCount:  3,
	}
	result := StatsOverview(stats)

	if !strings.Contains(result, "Buffer Health") {
		t.Errorf("expected 'Buffer Health' header, got:\n%s", result)
	}
	if !strings.Contains(result, "Traces") {
		t.Errorf("expected 'Traces' label, got:\n%s", result)
	}
	if !strings.Contains(result, "Logs") {
		t.Errorf("expected 'Logs' label, got:\n%s", result)
	}
	if !strings.Contains(result, "Metrics") {
		t.Errorf("expected 'Metrics' label, got:\n%s", result)
	}
	if !strings.Contains(result, "Snapshots: 3") {
		t.Errorf("expected 'Snapshots: 3', got:\n%s", result)
	}
	if !strings.Contains(result, "1,204") {
		t.Errorf("expected formatted count '1,204', got:\n%s", result)
	}
}

func TestStatsOverview_Empty(t *testing.T) {
	stats := BufferStats{
		SpanCapacity:   10000,
		LogCapacity:    50000,
		MetricCapacity: 100000,
	}
	result := StatsOverview(stats)
	if !strings.Contains(result, "Buffer Health") {
		t.Errorf("expected header even for empty buffers, got:\n%s", result)
	}
	// All bars should be empty dots
	if !strings.Contains(result, "[....................]") {
		t.Errorf("expected empty bar, got:\n%s", result)
	}
}

func TestServiceSummary_Empty(t *testing.T) {
	result := ServiceSummary(nil, 80)
	if result != "" {
		t.Errorf("expected empty string for nil services, got %q", result)
	}
}

func TestServiceSummary(t *testing.T) {
	services := []ServiceStats{
		{Name: "my-api", SpanCount: 28, ErrorCount: 2},
		{Name: "db-svc", SpanCount: 10, ErrorCount: 0},
		{Name: "cache-svc", SpanCount: 4, ErrorCount: 0},
	}
	result := ServiceSummary(services, 80)

	if !strings.Contains(result, "3 active") {
		t.Errorf("expected '3 active', got:\n%s", result)
	}
	if !strings.Contains(result, "42 spans") {
		t.Errorf("expected '42 spans' total, got:\n%s", result)
	}
	if !strings.Contains(result, "my-api") {
		t.Errorf("expected 'my-api', got:\n%s", result)
	}
	if !strings.Contains(result, "(2 errors)") {
		t.Errorf("expected '(2 errors)', got:\n%s", result)
	}
	// db-svc should not have errors suffix
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "db-svc") && strings.Contains(line, "error") {
			t.Errorf("db-svc should not have errors, got: %s", line)
		}
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{10000, "10,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := formatCount(tt.input)
		if got != tt.expected {
			t.Errorf("formatCount(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
