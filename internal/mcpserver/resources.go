package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// registerResources registers all MCP resources and resource templates.
func (s *Server) registerResources() {
	// Static resources
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://endpoint",
		Name:        "endpoint",
		Description: "OTLP gRPC endpoint address, active ports, and environment variable suggestions.",
		MIMEType:    "application/json",
	}, s.handleEndpointResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://stats",
		Name:        "stats",
		Description: "Ring buffer counts, capacities, and snapshot count.",
		MIMEType:    "application/json",
	}, s.handleStatsResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://services",
		Name:        "services",
		Description: "Discovered service names across all signal types.",
		MIMEType:    "application/json",
	}, s.handleServicesResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://snapshots",
		Name:        "snapshots",
		Description: "All snapshots with timestamps and buffer positions.",
		MIMEType:    "application/json",
	}, s.handleSnapshotsResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://file-sources",
		Name:        "file-sources",
		Description: "Active filesystem directories being watched for OTLP JSONL.",
		MIMEType:    "application/json",
	}, s.handleFileSourcesResource)

	// Resource templates
	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "otlp://services/{service}",
		Name:        "service-detail",
		Description: "Telemetry overview for a specific service: counts, error rate, recent spans.",
		MIMEType:    "application/json",
	}, s.handleServiceDetailResource)

	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "otlp://snapshots/{name}",
		Name:        "snapshot-detail",
		Description: "Metadata for a specific snapshot: creation time and buffer positions.",
		MIMEType:    "application/json",
	}, s.handleSnapshotDetailResource)
}

// Static resource handlers

func (s *Server) handleEndpointResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	data := struct {
		Endpoint  string            `json:"endpoint"`
		Protocol  string            `json:"protocol"`
		Endpoints []string          `json:"endpoints"`
		EnvVars   map[string]string `json:"environment_vars"`
	}{
		Endpoint:  s.otlpReceiver.Endpoint(),
		Protocol:  "grpc",
		Endpoints: s.otlpReceiver.Endpoints(),
		EnvVars: map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": s.otlpReceiver.Endpoint(),
			"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		},
	}
	return jsonResourceResult(req.Params.URI, data)
}

func (s *Server) handleStatsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	return jsonResourceResult(req.Params.URI, s.storage.Stats())
}

func (s *Server) handleServicesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	services := s.storage.Services()
	data := struct {
		Services []string `json:"services"`
		Count    int      `json:"count"`
	}{
		Services: services,
		Count:    len(services),
	}
	return jsonResourceResult(req.Params.URI, data)
}

type snapshotInfo struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	TracePos  int    `json:"trace_position"`
	LogPos    int    `json:"log_position"`
	MetricPos int    `json:"metric_position"`
}

func (s *Server) handleSnapshotsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	names := s.storage.Snapshots().List()
	snapshots := make([]snapshotInfo, 0, len(names))
	for _, name := range names {
		snap, err := s.storage.Snapshots().Get(name)
		if err != nil {
			continue // may have been deleted between List and Get
		}
		snapshots = append(snapshots, snapshotInfo{
			Name:      snap.Name,
			CreatedAt: snap.CreatedAt.Format(time.RFC3339Nano),
			TracePos:  snap.TracePos,
			LogPos:    snap.LogPos,
			MetricPos: snap.MetricPos,
		})
	}
	data := struct {
		Snapshots []snapshotInfo `json:"snapshots"`
		Count     int            `json:"count"`
	}{
		Snapshots: snapshots,
		Count:     len(snapshots),
	}
	return jsonResourceResult(req.Params.URI, data)
}

func (s *Server) handleFileSourcesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	stats := s.FileSourceStats()
	sources := make([]FileSourceInfo, len(stats))
	for i, stat := range stats {
		sources[i] = FileSourceInfo{
			Directory:    stat.Directory,
			WatchedDirs:  stat.WatchedDirs,
			FilesTracked: stat.FilesTracked,
		}
	}
	data := struct {
		Sources []FileSourceInfo `json:"sources"`
		Count   int              `json:"count"`
	}{
		Sources: sources,
		Count:   len(sources),
	}
	return jsonResourceResult(req.Params.URI, data)
}

// Resource template handlers

func (s *Server) handleServiceDetailResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	serviceName, err := extractURIParam(req.Params.URI, "otlp://services/")
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	result, err := s.storage.Query(storage.QueryFilter{
		ServiceName: serviceName,
	})
	if err != nil {
		return nil, fmt.Errorf("query service data: %w", err)
	}

	if result.Summary.SpanCount == 0 && result.Summary.LogCount == 0 && result.Summary.MetricCount == 0 {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data := struct {
		Service     string   `json:"service"`
		SpanCount   int      `json:"span_count"`
		LogCount    int      `json:"log_count"`
		MetricCount int      `json:"metric_count"`
		TraceIDs    []string `json:"trace_ids"`
		MetricNames []string `json:"metric_names"`
	}{
		Service:     serviceName,
		SpanCount:   result.Summary.SpanCount,
		LogCount:    result.Summary.LogCount,
		MetricCount: result.Summary.MetricCount,
		TraceIDs:    result.Summary.TraceIDs,
		MetricNames: result.Summary.MetricNames,
	}
	return jsonResourceResult(req.Params.URI, data)
}

func (s *Server) handleSnapshotDetailResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	name, err := extractURIParam(req.Params.URI, "otlp://snapshots/")
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	snap, err := s.storage.Snapshots().Get(name)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data := snapshotInfo{
		Name:      snap.Name,
		CreatedAt: snap.CreatedAt.Format(time.RFC3339Nano),
		TracePos:  snap.TracePos,
		LogPos:    snap.LogPos,
		MetricPos: snap.MetricPos,
	}
	return jsonResourceResult(req.Params.URI, data)
}

// Helpers

// extractURIParam extracts the parameter value from a URI by stripping the prefix
// and URL-decoding the remainder.
func extractURIParam(uri, prefix string) (string, error) {
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("invalid URI: %s", uri)
	}
	param := strings.TrimPrefix(uri, prefix)
	if param == "" {
		return "", fmt.Errorf("empty parameter in URI: %s", uri)
	}
	decoded, err := url.PathUnescape(param)
	if err != nil {
		return "", fmt.Errorf("invalid encoding in URI: %w", err)
	}
	return decoded, nil
}

// jsonResourceResult marshals data to JSON and wraps it in a ReadResourceResult.
func jsonResourceResult(uri string, data any) (*mcp.ReadResourceResult, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal resource data: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:  uri,
			Text: string(jsonBytes),
		}},
	}, nil
}
