package webui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tobert/otlp-mcp/internal/storage"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func strVal(s string) *commonpb.AnyValue {
	return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: s}}
}

func TestFormatAttrValue(t *testing.T) {
	tests := []struct {
		name string
		in   *commonpb.AnyValue
		want any
	}{
		{"nil", nil, nil},
		{"string", strVal("hi"), "hi"},
		{"int", &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 42}}, int64(42)},
		{"double", &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 3.5}}, 3.5},
		{"bool", &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}, true},
		{"bytes", &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{0xde, 0xad}}}, "dead"},
		{
			"array",
			&commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{
				Values: []*commonpb.AnyValue{strVal("a"), strVal("b")},
			}}},
			[]any{"a", "b"},
		},
		{
			"kvlist",
			&commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{
				Values: []*commonpb.KeyValue{{Key: "k", Value: strVal("v")}},
			}}},
			map[string]any{"k": "v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttrValue(tt.in, 0)
			// Compare via JSON to keep nested map/slice comparison simple.
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("formatAttrValue(%s) = %s, want %s", tt.name, gotJSON, wantJSON)
			}
		})
	}
}

// TestFormatAttrValueDepthGuard verifies recursion stops at maxAttrDepth so a
// pathologically nested attribute can't blow the stack.
func TestFormatAttrValueDepthGuard(t *testing.T) {
	// At the limit, the value is dropped regardless of content.
	if got := formatAttrValue(strVal("deep"), maxAttrDepth); got != nil {
		t.Errorf("at depth %d, expected nil, got %v", maxAttrDepth, got)
	}
	// Just under the limit, the value still resolves.
	if got := formatAttrValue(strVal("ok"), maxAttrDepth-1); got != "ok" {
		t.Errorf("at depth %d, expected %q, got %v", maxAttrDepth-1, "ok", got)
	}

	// A nested chain deeper than the limit truncates at the boundary rather
	// than recursing forever.
	nest := func(inner *commonpb.AnyValue) *commonpb.AnyValue {
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{
			Values: []*commonpb.KeyValue{{Key: "child", Value: inner}},
		}}}
	}
	v := strVal("bottom")
	for range maxAttrDepth + 5 {
		v = nest(v)
	}
	// Should not panic and should produce a finite structure; the deepest
	// levels collapse to nil once the guard trips.
	out := formatAttrValue(v, 0)
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("marshal nested result: %v", err)
	}
}

// TestDurationMs verifies the clamp: a span whose end precedes its start (clock
// skew, or an unsigned underflow upstream) reports zero, not a wrapped value.
func TestDurationMs(t *testing.T) {
	if got := durationMs(1_000_000, 1_500_000); got != 0.5 {
		t.Errorf("durationMs normal = %v, want 0.5", got)
	}
	if got := durationMs(0, 0); got != 0 {
		t.Errorf("durationMs zero = %v, want 0", got)
	}
	// end < start must clamp to 0 rather than wrap toward ~1.8e10 ms.
	if got := durationMs(1_500_000, 1_000_000); got != 0 {
		t.Errorf("durationMs end<start = %v, want 0", got)
	}
}

// TestTimelineModuleServed confirms the geometry ES module is reachable with a
// JavaScript content type so the browser will execute it as a module.
func TestTimelineModuleServed(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/ui/timeline.mjs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "export function computeTimelineLayout") {
		t.Errorf("module body missing computeTimelineLayout export")
	}
}

// newTestServer wires a Server backed by real in-memory storage and returns an
// httptest server so the {traceId} path routing is exercised end to end.
func newTestServer(t *testing.T) (*storage.ObservabilityStorage, *httptest.Server) {
	t.Helper()
	st := storage.NewObservabilityStorage(100, 100, 100)
	srv := New(st, nil)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return st, ts
}

// TestHandleTraceEmpty verifies a missing trace returns an empty array, not 404,
// so the UI can render "no spans" without special-casing the status code.
func TestHandleTraceEmpty(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/trace/deadbeef")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got []spanDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d spans", len(got))
	}
}

// TestHandleTraceFull builds a span with a parent, status, attributes, events,
// and links, then confirms handleTrace serializes each field.
func TestHandleTraceFull(t *testing.T) {
	st, ts := newTestServer(t)

	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	parentID := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	linkTrace := []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	linkSpan := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	rs := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{Key: "service.name", Value: strVal("checkout")},
				{Key: "host.name", Value: strVal("node-1")},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{{
			Spans: []*tracepb.Span{{
				TraceId:           traceID,
				SpanId:            spanID,
				ParentSpanId:      parentID,
				Name:              "charge",
				Kind:              tracepb.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: 1_000_000_000,
				EndTimeUnixNano:   1_500_000_000,
				Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "declined"},
				Attributes: []*commonpb.KeyValue{
					{Key: "http.method", Value: strVal("POST")},
				},
				Events: []*tracepb.Span_Event{{
					Name:         "retry",
					TimeUnixNano: 1_200_000_000,
					Attributes:   []*commonpb.KeyValue{{Key: "attempt", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 2}}}},
				}},
				Links: []*tracepb.Span_Link{{
					TraceId:    linkTrace,
					SpanId:     linkSpan,
					Attributes: []*commonpb.KeyValue{{Key: "rel", Value: strVal("follows")}},
				}},
			}},
		}},
	}

	if err := st.Traces().ReceiveSpans(context.Background(), []*tracepb.ResourceSpans{rs}); err != nil {
		t.Fatalf("ReceiveSpans: %v", err)
	}

	resp, err := http.Get(ts.URL + "/api/trace/0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var got []spanDetail
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 span, got %d", len(got))
	}
	d := got[0]

	if d.SpanName != "charge" {
		t.Errorf("SpanName = %q, want charge", d.SpanName)
	}
	if d.ParentSpanID != "0909090909090909" {
		t.Errorf("ParentSpanID = %q", d.ParentSpanID)
	}
	if d.Kind != "SERVER" {
		t.Errorf("Kind = %q, want SERVER", d.Kind)
	}
	if d.Status != "ERROR" || d.StatusMessage != "declined" {
		t.Errorf("status = %q/%q, want ERROR/declined", d.Status, d.StatusMessage)
	}
	if d.DurationMs != 500 {
		t.Errorf("DurationMs = %v, want 500", d.DurationMs)
	}
	// Nanosecond fields are decimal strings (BigInt on the client), not numbers.
	if d.StartNs != "1000000000" || d.EndNs != "1500000000" {
		t.Errorf("StartNs/EndNs = %q/%q, want 1000000000/1500000000", d.StartNs, d.EndNs)
	}
	if len(d.Events) == 1 && d.Events[0].TimeNs != "1200000000" {
		t.Errorf("event TimeNs = %q, want 1200000000", d.Events[0].TimeNs)
	}
	if d.ServiceName != "checkout" {
		t.Errorf("ServiceName = %q", d.ServiceName)
	}
	if d.Attributes["http.method"] != "POST" {
		t.Errorf("Attributes[http.method] = %v", d.Attributes["http.method"])
	}
	if d.ResourceAttrs["host.name"] != "node-1" {
		t.Errorf("ResourceAttrs[host.name] = %v", d.ResourceAttrs["host.name"])
	}
	if len(d.Events) != 1 || d.Events[0].Name != "retry" {
		t.Fatalf("events = %+v", d.Events)
	}
	if len(d.Links) != 1 || d.Links[0].SpanID != "0807060504030201" {
		t.Fatalf("links = %+v", d.Links)
	}
}
