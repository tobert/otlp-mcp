package storage

import (
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestMatchesStatusFilter(t *testing.T) {
	testCases := []struct {
		name     string
		span     *StoredSpan
		filter   QueryFilter
		expected bool
	}{
		{
			name: "errors_only_true_with_error_status",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR},
				},
			},
			filter:   QueryFilter{ErrorsOnly: true},
			expected: true,
		},
		{
			name: "errors_only_true_with_ok_status",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
				},
			},
			filter:   QueryFilter{ErrorsOnly: true},
			expected: false,
		},
		{
			name: "span_status_ok",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
				},
			},
			filter:   QueryFilter{SpanStatus: "OK"},
			expected: true,
		},
		{
			name: "span_status_error",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR},
				},
			},
			filter:   QueryFilter{SpanStatus: "ERROR"},
			expected: true,
		},
		{
			name: "span_status_unset",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_UNSET},
				},
			},
			filter:   QueryFilter{SpanStatus: "UNSET"},
			expected: true,
		},
		{
			name: "no_status_filter",
			span: &StoredSpan{
				Span: &tracepb.Span{
					Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
				},
			},
			filter:   QueryFilter{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := matchesStatusFilter(tc.span, tc.filter)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestMatchesDurationFilter(t *testing.T) {
	now := time.Now()
	minDuration := uint64(100)
	maxDuration := uint64(200)

	testCases := []struct {
		name     string
		span     *StoredSpan
		filter   QueryFilter
		expected bool
	}{
		{
			name: "duration_within_range",
			span: &StoredSpan{
				Span: &tracepb.Span{
					StartTimeUnixNano: uint64(now.UnixNano()),
					EndTimeUnixNano:   uint64(now.Add(150 * time.Nanosecond).UnixNano()),
				},
			},
			filter: QueryFilter{
				MinDurationNs: &minDuration,
				MaxDurationNs: &maxDuration,
			},
			expected: true,
		},
		{
			name: "duration_below_min",
			span: &StoredSpan{
				Span: &tracepb.Span{
					StartTimeUnixNano: uint64(now.UnixNano()),
					EndTimeUnixNano:   uint64(now.Add(50 * time.Nanosecond).UnixNano()),
				},
			},
			filter: QueryFilter{
				MinDurationNs: &minDuration,
			},
			expected: false,
		},
		{
			name: "duration_above_max",
			span: &StoredSpan{
				Span: &tracepb.Span{
					StartTimeUnixNano: uint64(now.UnixNano()),
					EndTimeUnixNano:   uint64(now.Add(250 * time.Nanosecond).UnixNano()),
				},
			},
			filter: QueryFilter{
				MaxDurationNs: &maxDuration,
			},
			expected: false,
		},
		{
			name: "no_duration_filter",
			span: &StoredSpan{
				Span: &tracepb.Span{
					StartTimeUnixNano: uint64(now.UnixNano()),
					EndTimeUnixNano:   uint64(now.Add(150 * time.Nanosecond).UnixNano()),
				},
			},
			filter:   QueryFilter{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := matchesDurationFilter(tc.span, tc.filter)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestMatchesAttributeFilter(t *testing.T) {
	attributes := []*commonpb.KeyValue{
		{
			Key:   "http.method",
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}},
		},
		{
			Key:   "http.status_code",
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 200}},
		},
	}

	testCases := []struct {
		name     string
		filter   QueryFilter
		expected bool
	}{
		{
			name: "has_attribute_present",
			filter: QueryFilter{
				HasAttribute: "http.method",
			},
			expected: true,
		},
		{
			name: "has_attribute_absent",
			filter: QueryFilter{
				HasAttribute: "db.statement",
			},
			expected: false,
		},
		{
			name: "attribute_equals_match",
			filter: QueryFilter{
				AttributeEquals: map[string]string{
					"http.method":      "GET",
					"http.status_code": "200",
				},
			},
			expected: true,
		},
		{
			name: "attribute_equals_no_match",
			filter: QueryFilter{
				AttributeEquals: map[string]string{
					"http.method":      "POST",
					"http.status_code": "200",
				},
			},
			expected: false,
		},
		{
			name:     "no_attribute_filter",
			filter:   QueryFilter{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := matchesAttributeFilter(attributes, tc.filter)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}
