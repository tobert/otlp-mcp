package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// newTestServer creates a Server with small buffers for testing.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	recv, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("create receiver: %v", err)
	}
	t.Cleanup(recv.Stop)
	go recv.Start(context.Background())

	srv, err := NewServer(obsStorage, recv)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv
}

func readReq(uri string) *mcp.ReadResourceRequest {
	return &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: uri},
	}
}

func requireText(t *testing.T, result *mcp.ReadResourceResult) string {
	t.Helper()
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}
	return result.Contents[0].Text
}

func TestEndpointResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleEndpointResource(context.Background(), readReq("otlp://endpoint"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "OTLP Endpoint") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "Address:") {
		t.Error("expected Address field")
	}
	if !strings.Contains(text, "Protocol:  grpc") {
		t.Error("expected grpc protocol")
	}
	if !strings.Contains(text, "OTEL_EXPORTER_OTLP_ENDPOINT=") {
		t.Error("expected env var")
	}
}

func TestStatsResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleStatsResource(context.Background(), readReq("otlp://stats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Buffer Statistics") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "Traces") {
		t.Error("expected Traces row")
	}
	if !strings.Contains(text, "Logs") {
		t.Error("expected Logs row")
	}
	if !strings.Contains(text, "Metrics") {
		t.Error("expected Metrics row")
	}
	if !strings.Contains(text, "100") {
		t.Error("expected capacity numbers")
	}
}

func TestServicesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleServicesResource(context.Background(), readReq("otlp://services"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Discovered Services (0)") {
		t.Error("expected header with count 0")
	}
	if !strings.Contains(text, "(none)") {
		t.Error("expected (none) for empty list")
	}
}

func TestServicesResourceWithData(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	srv.storage.ReceiveSpans(ctx, []*tracepb.ResourceSpans{
		makeResourceSpan("svc-alpha", "op1"),
		makeResourceSpan("svc-beta", "op2"),
	})

	result, err := srv.handleServicesResource(ctx, readReq("otlp://services"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Discovered Services (2)") {
		t.Errorf("expected 2 services, got:\n%s", text)
	}
	if !strings.Contains(text, "svc-alpha") {
		t.Error("expected svc-alpha")
	}
	if !strings.Contains(text, "svc-beta") {
		t.Error("expected svc-beta")
	}
}

func TestSnapshotsResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleSnapshotsResource(context.Background(), readReq("otlp://snapshots"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Snapshots (0)") {
		t.Error("expected header with count 0")
	}
	if !strings.Contains(text, "(none)") {
		t.Error("expected (none)")
	}
}

func TestSnapshotsResourceWithData(t *testing.T) {
	srv := newTestServer(t)
	srv.storage.CreateSnapshot("snap-a")
	srv.storage.CreateSnapshot("snap-b")

	result, err := srv.handleSnapshotsResource(context.Background(), readReq("otlp://snapshots"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Snapshots (2)") {
		t.Errorf("expected 2 snapshots, got:\n%s", text)
	}
	if !strings.Contains(text, "snap-a") {
		t.Error("expected snap-a")
	}
	if !strings.Contains(text, "snap-b") {
		t.Error("expected snap-b")
	}
}

func TestFileSourcesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleFileSourcesResource(context.Background(), readReq("otlp://file-sources"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "File Sources (0)") {
		t.Error("expected header with count 0")
	}
	if !strings.Contains(text, "(none)") {
		t.Error("expected (none)")
	}
}

func TestServiceDetailResource(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	srv.storage.ReceiveSpans(ctx, []*tracepb.ResourceSpans{
		makeResourceSpan("my-service", "GET /api"),
	})

	result, err := srv.handleServiceDetailResource(ctx, readReq("otlp://services/my-service"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Service: my-service") {
		t.Error("expected service header")
	}
	if !strings.Contains(text, "Spans:") {
		t.Error("expected Spans field")
	}
}

func TestServiceDetailResourceNotFound(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.handleServiceDetailResource(context.Background(), readReq("otlp://services/nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
}

func TestSnapshotDetailResource(t *testing.T) {
	srv := newTestServer(t)
	srv.storage.CreateSnapshot("test-snap")

	result, err := srv.handleSnapshotDetailResource(context.Background(), readReq("otlp://snapshots/test-snap"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := requireText(t, result)

	if !strings.Contains(text, "Snapshot: test-snap") {
		t.Error("expected snapshot header")
	}
	if !strings.Contains(text, "Created:") {
		t.Error("expected Created field")
	}
	if !strings.Contains(text, "Trace Pos:") {
		t.Error("expected Trace Pos field")
	}
}

func TestSnapshotDetailResourceNotFound(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.handleSnapshotDetailResource(context.Background(), readReq("otlp://snapshots/nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent snapshot")
	}
}

func TestExtractURIParam(t *testing.T) {
	tests := []struct {
		uri    string
		prefix string
		want   string
		err    bool
	}{
		{"otlp://services/my-svc", "otlp://services/", "my-svc", false},
		{"otlp://services/url%20encoded", "otlp://services/", "url encoded", false},
		{"otlp://services/", "otlp://services/", "", true},
		{"otlp://wrong/path", "otlp://services/", "", true},
	}
	for _, tt := range tests {
		got, err := extractURIParam(tt.uri, tt.prefix)
		if tt.err && err == nil {
			t.Errorf("extractURIParam(%q, %q): expected error", tt.uri, tt.prefix)
		}
		if !tt.err && err != nil {
			t.Errorf("extractURIParam(%q, %q): unexpected error: %v", tt.uri, tt.prefix, err)
		}
		if got != tt.want {
			t.Errorf("extractURIParam(%q, %q) = %q, want %q", tt.uri, tt.prefix, got, tt.want)
		}
	}
}

func TestFmtNum(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{10000, "10,000"},
		{100000, "100,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := fmtNum(tt.n)
		if got != tt.want {
			t.Errorf("fmtNum(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// makeResourceSpan creates a minimal ResourceSpans for testing.
func makeResourceSpan(serviceName, spanName string) *tracepb.ResourceSpans {
	return &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: serviceName}}},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{{
			Spans: []*tracepb.Span{{
				TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
				Name:              spanName,
				StartTimeUnixNano: 1000000000,
				EndTimeUnixNano:   2000000000,
			}},
		}},
	}
}
