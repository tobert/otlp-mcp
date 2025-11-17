package storage

import (
	"strings"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// matchesStatusFilter checks if a span matches the status filter criteria
func matchesStatusFilter(span *StoredSpan, filter QueryFilter) bool {
	if span.Span.Status == nil {
		// Treat nil status as UNSET
		if filter.ErrorsOnly {
			return false
		}
		if filter.SpanStatus == "UNSET" || filter.SpanStatus == "STATUS_CODE_UNSET" {
			return true
		}
		return filter.SpanStatus == ""
	}

	statusCode := span.Span.Status.Code

	// errors_only is shortcut for STATUS_CODE_ERROR
	if filter.ErrorsOnly {
		return statusCode == tracepb.Status_STATUS_CODE_ERROR
	}

	// Check explicit status code
	if filter.SpanStatus != "" {
		switch strings.ToUpper(filter.SpanStatus) {
		case "OK", "STATUS_CODE_OK":
			return statusCode == tracepb.Status_STATUS_CODE_OK
		case "ERROR", "STATUS_CODE_ERROR":
			return statusCode == tracepb.Status_STATUS_CODE_ERROR
		case "UNSET", "STATUS_CODE_UNSET":
			return statusCode == tracepb.Status_STATUS_CODE_UNSET
		}
	}

	return true
}

// matchesDurationFilter checks if a span's duration matches the filter criteria
func matchesDurationFilter(span *StoredSpan, filter QueryFilter) bool {
	// Calculate duration in nanoseconds
	duration := span.Span.EndTimeUnixNano - span.Span.StartTimeUnixNano

	// Check minimum duration
	if filter.MinDurationNs != nil {
		if duration < *filter.MinDurationNs {
			return false
		}
	}

	// Check maximum duration
	if filter.MaxDurationNs != nil {
		if duration > *filter.MaxDurationNs {
			return false
		}
	}

	return true
}

// matchesAttributeFilter checks if attributes match the filter criteria
func matchesAttributeFilter(attributes []*commonpb.KeyValue, filter QueryFilter) bool {
	// Check HasAttribute filter
	if filter.HasAttribute != "" {
		found := false
		for _, attr := range attributes {
			if attr.Key == filter.HasAttribute {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check AttributeEquals filter
	if len(filter.AttributeEquals) > 0 {
		// Convert attributes to map for easier lookup
		attrMap := make(map[string]string)
		for _, attr := range attributes {
			attrMap[attr.Key] = getAttributeStringValue(attr.Value)
		}

		// All specified attribute key-value pairs must match
		for key, expectedValue := range filter.AttributeEquals {
			actualValue, exists := attrMap[key]
			if !exists || actualValue != expectedValue {
				return false
			}
		}
	}

	return true
}

// getAttributeStringValue extracts string representation of an attribute value
func getAttributeStringValue(value *commonpb.AnyValue) string {
	if value == nil {
		return ""
	}

	switch v := value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_IntValue:
		return string(rune(v.IntValue))
	case *commonpb.AnyValue_DoubleValue:
		return string(rune(int64(v.DoubleValue)))
	case *commonpb.AnyValue_BoolValue:
		if v.BoolValue {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
