package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// registerResources registers all MCP resources and resource templates.
func (s *Server) registerResources() {
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

// ─── Static resource handlers ───────────────────────────────────────────

func (s *Server) handleEndpointResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	data := map[string]any{
		"endpoint":  s.otlpReceiver.Endpoint(),
		"protocol":  "grpc",
		"endpoints": s.otlpReceiver.Endpoints(),
		"env": map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": s.otlpReceiver.Endpoint(),
			"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		},
	}
	return jsonResult(req.Params.URI, data)
}

func (s *Server) handleStatsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	stats := s.storage.Stats()
	data := map[string]any{
		"traces": map[string]any{
			"count":    stats.Traces.SpanCount,
			"capacity": stats.Traces.Capacity,
			"distinct": stats.Traces.TraceCount,
		},
		"logs": map[string]any{
			"count":      stats.Logs.LogCount,
			"capacity":   stats.Logs.Capacity,
			"severities": stats.Logs.Severities,
		},
		"metrics": map[string]any{
			"count":        stats.Metrics.MetricCount,
			"capacity":     stats.Metrics.Capacity,
			"unique_names": stats.Metrics.UniqueNames,
			"types":        stats.Metrics.TypeCounts,
		},
		"snapshots": stats.Snapshots,
	}
	return jsonResult(req.Params.URI, data)
}

func (s *Server) handleServicesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	services := s.storage.Services()
	data := map[string]any{
		"services": services,
		"count":    len(services),
	}
	return jsonResult(req.Params.URI, data)
}

func (s *Server) handleSnapshotsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	names := s.storage.Snapshots().List()
	snapshots := make([]map[string]any, 0, len(names))
	for _, name := range names {
		snap, err := s.storage.Snapshots().Get(name)
		if err != nil {
			continue
		}
		snapshots = append(snapshots, map[string]any{
			"name":       snap.Name,
			"created_at": snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"positions":  map[string]int{"traces": snap.TracePos, "logs": snap.LogPos, "metrics": snap.MetricPos},
		})
	}
	data := map[string]any{
		"snapshots": snapshots,
		"count":     len(snapshots),
	}
	return jsonResult(req.Params.URI, data)
}

func (s *Server) handleFileSourcesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	stats := s.FileSourceStats()
	sources := make([]map[string]any, len(stats))
	for i, stat := range stats {
		sources[i] = map[string]any{
			"directory":     stat.Directory,
			"watched_dirs":  stat.WatchedDirs,
			"files_tracked": stat.FilesTracked,
		}
	}
	data := map[string]any{
		"sources": sources,
		"count":   len(sources),
	}
	return jsonResult(req.Params.URI, data)
}

// ─── Resource template handlers ─────────────────────────────────────────

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

	data := map[string]any{
		"service":      serviceName,
		"spans":        result.Summary.SpanCount,
		"logs":         result.Summary.LogCount,
		"metrics":      result.Summary.MetricCount,
		"trace_ids":    result.Summary.TraceIDs,
		"metric_names": result.Summary.MetricNames,
	}
	if len(result.Summary.LogSeverities) > 0 {
		data["log_severities"] = result.Summary.LogSeverities
	}
	return jsonResult(req.Params.URI, data)
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

	data := map[string]any{
		"name":       snap.Name,
		"created_at": snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"positions":  map[string]int{"traces": snap.TracePos, "logs": snap.LogPos, "metrics": snap.MetricPos},
	}
	return jsonResult(req.Params.URI, data)
}

// ─── Helpers ────────────────────────────────────────────────────────────

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

func jsonResult(uri string, data any) (*mcp.ReadResourceResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal resource data: %w", err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:  uri,
			Text: string(b),
		}},
	}, nil
}
