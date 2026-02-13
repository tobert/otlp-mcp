package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

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

func readJSON(t *testing.T, result *mcp.ReadResourceResult) map[string]any {
	t.Helper()
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return data
}

func TestEndpointResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleEndpointResource(context.Background(), readReq("otlp://endpoint"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := readJSON(t, result)

	if data["endpoint"] == "" {
		t.Error("expected non-empty endpoint")
	}
	if data["protocol"] != "grpc" {
		t.Errorf("expected protocol grpc, got %v", data["protocol"])
	}
}

func TestStatsResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleStatsResource(context.Background(), readReq("otlp://stats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := readJSON(t, result)

	traces := data["traces"].(map[string]any)
	if int(traces["capacity"].(float64)) != 100 {
		t.Errorf("expected trace capacity 100, got %v", traces["capacity"])
	}
	logs := data["logs"].(map[string]any)
	if int(logs["capacity"].(float64)) != 500 {
		t.Errorf("expected log capacity 500, got %v", logs["capacity"])
	}
}

func TestServicesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleServicesResource(context.Background(), readReq("otlp://services"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := readJSON(t, result)

	if int(data["count"].(float64)) != 0 {
		t.Errorf("expected 0 services, got %v", data["count"])
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
	data := readJSON(t, result)

	if int(data["count"].(float64)) != 2 {
		t.Errorf("expected 2 services, got %v", data["count"])
	}
}

func TestSnapshotsResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleSnapshotsResource(context.Background(), readReq("otlp://snapshots"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := readJSON(t, result)

	if int(data["count"].(float64)) != 0 {
		t.Errorf("expected 0 snapshots, got %v", data["count"])
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
	data := readJSON(t, result)

	if int(data["count"].(float64)) != 2 {
		t.Errorf("expected 2 snapshots, got %v", data["count"])
	}
}

func TestFileSourcesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleFileSourcesResource(context.Background(), readReq("otlp://file-sources"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := readJSON(t, result)

	if int(data["count"].(float64)) != 0 {
		t.Errorf("expected 0 file sources, got %v", data["count"])
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
	data := readJSON(t, result)

	if data["service"] != "my-service" {
		t.Errorf("expected service my-service, got %v", data["service"])
	}
	if int(data["spans"].(float64)) != 1 {
		t.Errorf("expected 1 span, got %v", data["spans"])
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
	data := readJSON(t, result)

	if data["name"] != "test-snap" {
		t.Errorf("expected name test-snap, got %v", data["name"])
	}
	if data["created_at"] == nil {
		t.Error("expected non-nil created_at")
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
		uri, prefix, want string
		err               bool
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
