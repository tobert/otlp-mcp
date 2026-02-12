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

func TestEndpointResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleEndpointResource(context.Background(), readReq("otlp://endpoint"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}

	var data struct {
		Endpoint  string            `json:"endpoint"`
		Protocol  string            `json:"protocol"`
		Endpoints []string          `json:"endpoints"`
		EnvVars   map[string]string `json:"environment_vars"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Endpoint == "" {
		t.Error("expected non-empty endpoint")
	}
	if data.Protocol != "grpc" {
		t.Errorf("expected protocol grpc, got %s", data.Protocol)
	}
	if len(data.Endpoints) == 0 {
		t.Error("expected at least one endpoint")
	}
	if data.EnvVars["OTEL_EXPORTER_OTLP_ENDPOINT"] == "" {
		t.Error("expected OTEL_EXPORTER_OTLP_ENDPOINT env var")
	}
}

func TestStatsResource(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleStatsResource(context.Background(), readReq("otlp://stats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data struct {
		Traces struct {
			Capacity int `json:"capacity"`
		} `json:"traces"`
		Logs struct {
			Capacity int `json:"capacity"`
		} `json:"logs"`
		Metrics struct {
			Capacity int `json:"capacity"`
		} `json:"metrics"`
		Snapshots int `json:"snapshot_count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Traces.Capacity != 100 {
		t.Errorf("expected trace capacity 100, got %d", data.Traces.Capacity)
	}
	if data.Logs.Capacity != 500 {
		t.Errorf("expected log capacity 500, got %d", data.Logs.Capacity)
	}
}

func TestServicesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleServicesResource(context.Background(), readReq("otlp://services"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data struct {
		Services []string `json:"services"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Count != 0 {
		t.Errorf("expected 0 services, got %d", data.Count)
	}
}

func TestServicesResourceWithData(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Add spans with different services
	srv.storage.ReceiveSpans(ctx, []*tracepb.ResourceSpans{
		makeResourceSpan("svc-alpha", "op1"),
		makeResourceSpan("svc-beta", "op2"),
	})

	result, err := srv.handleServicesResource(ctx, readReq("otlp://services"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data struct {
		Services []string `json:"services"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Count != 2 {
		t.Errorf("expected 2 services, got %d", data.Count)
	}
	// Should be sorted
	if len(data.Services) == 2 && data.Services[0] != "svc-alpha" {
		t.Errorf("expected sorted services, got %v", data.Services)
	}
}

func TestSnapshotsResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleSnapshotsResource(context.Background(), readReq("otlp://snapshots"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data struct {
		Snapshots []snapshotInfo `json:"snapshots"`
		Count     int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Count != 0 {
		t.Errorf("expected 0 snapshots, got %d", data.Count)
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

	var data struct {
		Snapshots []snapshotInfo `json:"snapshots"`
		Count     int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Count != 2 {
		t.Errorf("expected 2 snapshots, got %d", data.Count)
	}
}

func TestFileSourcesResourceEmpty(t *testing.T) {
	srv := newTestServer(t)
	result, err := srv.handleFileSourcesResource(context.Background(), readReq("otlp://file-sources"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data struct {
		Sources []FileSourceInfo `json:"sources"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Count != 0 {
		t.Errorf("expected 0 file sources, got %d", data.Count)
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

	var data struct {
		Service   string `json:"service"`
		SpanCount int    `json:"span_count"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Service != "my-service" {
		t.Errorf("expected service my-service, got %s", data.Service)
	}
	if data.SpanCount != 1 {
		t.Errorf("expected 1 span, got %d", data.SpanCount)
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

	var data snapshotInfo
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Name != "test-snap" {
		t.Errorf("expected name test-snap, got %s", data.Name)
	}
	if data.CreatedAt == "" {
		t.Error("expected non-empty created_at")
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
