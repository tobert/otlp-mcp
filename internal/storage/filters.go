package storage

// FilterOptions contains options for filtering telemetry data.
type FilterOptions struct {
	TraceID  string
	Service  string
	SpanName string
	Severity string
	MinTime  int64
	MaxTime  int64
}

// FilterSpansByTraceID returns spans matching the specified trace ID.
func FilterSpansByTraceID(spans []*StoredSpan, traceID string) []*StoredSpan {
	if traceID == "" {
		return spans
	}

	result := make([]*StoredSpan, 0, len(spans)/10) // estimate 10% match rate
	for _, span := range spans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}
	return result
}

// FilterSpansByService returns spans matching the specified service name.
func FilterSpansByService(spans []*StoredSpan, service string) []*StoredSpan {
	if service == "" {
		return spans
	}

	result := make([]*StoredSpan, 0, len(spans)/5) // estimate 20% match rate
	for _, span := range spans {
		if span.ServiceName == service {
			result = append(result, span)
		}
	}
	return result
}

// FilterSpansByName returns spans matching the specified span name.
func FilterSpansByName(spans []*StoredSpan, spanName string) []*StoredSpan {
	if spanName == "" {
		return spans
	}

	result := make([]*StoredSpan, 0, len(spans)/10) // estimate 10% match rate
	for _, span := range spans {
		if span.SpanName == spanName {
			result = append(result, span)
		}
	}
	return result
}

// FilterSpans applies multiple filters using AND logic.
// Empty filter values are ignored.
func FilterSpans(spans []*StoredSpan, opts FilterOptions) []*StoredSpan {
	result := spans

	if opts.TraceID != "" {
		result = FilterSpansByTraceID(result, opts.TraceID)
	}

	if opts.Service != "" {
		result = FilterSpansByService(result, opts.Service)
	}

	if opts.SpanName != "" {
		result = FilterSpansByName(result, opts.SpanName)
	}

	return result
}

// GroupSpansByTraceID groups spans by their trace ID.
// Returns a map of trace ID to spans.
func GroupSpansByTraceID(spans []*StoredSpan) map[string][]*StoredSpan {
	traces := make(map[string][]*StoredSpan)

	for _, span := range spans {
		traces[span.TraceID] = append(traces[span.TraceID], span)
	}

	return traces
}
