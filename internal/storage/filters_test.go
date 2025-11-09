package storage

import (
	"testing"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestFilterSpansByTraceID(t *testing.T) {
	spans := []*StoredSpan{
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "span1"},
		{Span: &tracepb.Span{}, TraceID: "trace2", ServiceName: "svc1", SpanName: "span2"},
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc2", SpanName: "span3"},
	}

	result := FilterSpansByTraceID(spans, "trace1")
	if len(result) != 2 {
		t.Errorf("expected 2 spans, got %d", len(result))
	}

	// Empty filter should return all
	result = FilterSpansByTraceID(spans, "")
	if len(result) != 3 {
		t.Errorf("expected 3 spans with empty filter, got %d", len(result))
	}
}

func TestFilterSpansByService(t *testing.T) {
	spans := []*StoredSpan{
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "span1"},
		{Span: &tracepb.Span{}, TraceID: "trace2", ServiceName: "svc1", SpanName: "span2"},
		{Span: &tracepb.Span{}, TraceID: "trace3", ServiceName: "svc2", SpanName: "span3"},
	}

	result := FilterSpansByService(spans, "svc1")
	if len(result) != 2 {
		t.Errorf("expected 2 spans, got %d", len(result))
	}
}

func TestFilterSpansByName(t *testing.T) {
	spans := []*StoredSpan{
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "GET /api"},
		{Span: &tracepb.Span{}, TraceID: "trace2", ServiceName: "svc1", SpanName: "POST /api"},
		{Span: &tracepb.Span{}, TraceID: "trace3", ServiceName: "svc2", SpanName: "GET /api"},
	}

	result := FilterSpansByName(spans, "GET /api")
	if len(result) != 2 {
		t.Errorf("expected 2 spans, got %d", len(result))
	}
}

func TestFilterSpansMultiple(t *testing.T) {
	spans := []*StoredSpan{
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "GET /api"},
		{Span: &tracepb.Span{}, TraceID: "trace2", ServiceName: "svc1", SpanName: "POST /api"},
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc2", SpanName: "GET /api"},
		{Span: &tracepb.Span{}, TraceID: "trace3", ServiceName: "svc1", SpanName: "GET /api"},
	}

	// Filter by service AND span name
	opts := FilterOptions{
		Service:  "svc1",
		SpanName: "GET /api",
	}
	result := FilterSpans(spans, opts)
	if len(result) != 2 {
		t.Errorf("expected 2 spans matching service=svc1 AND name='GET /api', got %d", len(result))
	}

	// Filter by trace ID
	opts = FilterOptions{
		TraceID: "trace1",
	}
	result = FilterSpans(spans, opts)
	if len(result) != 2 {
		t.Errorf("expected 2 spans matching trace1, got %d", len(result))
	}
}

func TestGroupSpansByTraceID(t *testing.T) {
	spans := []*StoredSpan{
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "span1"},
		{Span: &tracepb.Span{}, TraceID: "trace2", ServiceName: "svc1", SpanName: "span2"},
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc2", SpanName: "span3"},
		{Span: &tracepb.Span{}, TraceID: "trace1", ServiceName: "svc1", SpanName: "span4"},
	}

	groups := GroupSpansByTraceID(spans)

	if len(groups) != 2 {
		t.Errorf("expected 2 trace groups, got %d", len(groups))
	}

	if len(groups["trace1"]) != 3 {
		t.Errorf("expected 3 spans in trace1, got %d", len(groups["trace1"]))
	}

	if len(groups["trace2"]) != 1 {
		t.Errorf("expected 1 span in trace2, got %d", len(groups["trace2"]))
	}
}
