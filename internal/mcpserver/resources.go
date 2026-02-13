package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"sort"
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
		MIMEType:    "text/plain",
	}, s.handleEndpointResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://stats",
		Name:        "stats",
		Description: "Ring buffer counts, capacities, and snapshot count.",
		MIMEType:    "text/plain",
	}, s.handleStatsResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://services",
		Name:        "services",
		Description: "Discovered service names across all signal types.",
		MIMEType:    "text/plain",
	}, s.handleServicesResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://snapshots",
		Name:        "snapshots",
		Description: "All snapshots with timestamps and buffer positions.",
		MIMEType:    "text/plain",
	}, s.handleSnapshotsResource)

	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "otlp://file-sources",
		Name:        "file-sources",
		Description: "Active filesystem directories being watched for OTLP JSONL.",
		MIMEType:    "text/plain",
	}, s.handleFileSourcesResource)

	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "otlp://services/{service}",
		Name:        "service-detail",
		Description: "Telemetry overview for a specific service: counts, error rate, recent spans.",
		MIMEType:    "text/plain",
	}, s.handleServiceDetailResource)

	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "otlp://snapshots/{name}",
		Name:        "snapshot-detail",
		Description: "Metadata for a specific snapshot: creation time and buffer positions.",
		MIMEType:    "text/plain",
	}, s.handleSnapshotDetailResource)
}

// ─── Static resource handlers ───────────────────────────────────────────

func (s *Server) handleEndpointResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	endpoint := s.otlpReceiver.Endpoint()
	endpoints := s.otlpReceiver.Endpoints()

	var b strings.Builder
	b.WriteString("OTLP Endpoint\n")
	b.WriteString("═════════════\n")
	fmt.Fprintf(&b, "  Address:   %s\n", endpoint)
	b.WriteString("  Protocol:  grpc\n")
	if len(endpoints) > 1 {
		b.WriteString("\n  Active Ports:\n")
		for _, ep := range endpoints {
			fmt.Fprintf(&b, "    • %s\n", ep)
		}
	}
	b.WriteString("\n  Environment Variables:\n")
	fmt.Fprintf(&b, "    OTEL_EXPORTER_OTLP_ENDPOINT=%s\n", endpoint)
	b.WriteString("    OTEL_EXPORTER_OTLP_PROTOCOL=grpc\n")

	return textResult(req.Params.URI, b.String()), nil
}

func (s *Server) handleStatsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	stats := s.storage.Stats()

	var b strings.Builder
	b.WriteString("Buffer Statistics\n")
	b.WriteString("═════════════════\n")
	b.WriteString("  Signal     Count       Capacity    Usage\n")
	b.WriteString("  ───────    ─────────   ─────────   ─────\n")
	fmt.Fprintf(&b, "  Traces     %-10s  %-10s  %s\n",
		fmtNum(stats.Traces.SpanCount), fmtNum(stats.Traces.Capacity),
		fmtPct(stats.Traces.SpanCount, stats.Traces.Capacity))
	fmt.Fprintf(&b, "  Logs       %-10s  %-10s  %s\n",
		fmtNum(stats.Logs.LogCount), fmtNum(stats.Logs.Capacity),
		fmtPct(stats.Logs.LogCount, stats.Logs.Capacity))
	fmt.Fprintf(&b, "  Metrics    %-10s  %-10s  %s\n",
		fmtNum(stats.Metrics.MetricCount), fmtNum(stats.Metrics.Capacity),
		fmtPct(stats.Metrics.MetricCount, stats.Metrics.Capacity))

	fmt.Fprintf(&b, "\n  Snapshots: %d\n", stats.Snapshots)
	fmt.Fprintf(&b, "  Traces:    %s distinct\n", fmtNum(stats.Traces.TraceCount))
	fmt.Fprintf(&b, "  Metrics:   %d unique names\n", stats.Metrics.UniqueNames)

	if len(stats.Logs.Severities) > 0 {
		b.WriteString("\n  Log Severities:\n")
		// Sort severity keys for stable output
		sevKeys := make([]string, 0, len(stats.Logs.Severities))
		for k := range stats.Logs.Severities {
			sevKeys = append(sevKeys, k)
		}
		sort.Strings(sevKeys)
		for _, sev := range sevKeys {
			fmt.Fprintf(&b, "    %-8s %s\n", sev, fmtNum(stats.Logs.Severities[sev]))
		}
	}

	if len(stats.Metrics.TypeCounts) > 0 {
		b.WriteString("\n  Metric Types:\n")
		typeKeys := make([]string, 0, len(stats.Metrics.TypeCounts))
		for k := range stats.Metrics.TypeCounts {
			typeKeys = append(typeKeys, k)
		}
		sort.Strings(typeKeys)
		for _, mt := range typeKeys {
			fmt.Fprintf(&b, "    %-12s %s\n", mt, fmtNum(stats.Metrics.TypeCounts[mt]))
		}
	}

	return textResult(req.Params.URI, b.String()), nil
}

func (s *Server) handleServicesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	services := s.storage.Services()

	var b strings.Builder
	fmt.Fprintf(&b, "Discovered Services (%d)\n", len(services))
	b.WriteString("═══════════════════════\n")
	if len(services) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, svc := range services {
			fmt.Fprintf(&b, "  • %s\n", svc)
		}
	}

	return textResult(req.Params.URI, b.String()), nil
}

func (s *Server) handleSnapshotsResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	names := s.storage.Snapshots().List()

	var b strings.Builder
	fmt.Fprintf(&b, "Snapshots (%d)\n", len(names))
	b.WriteString("═════════════\n")

	if len(names) == 0 {
		b.WriteString("  (none)\n")
	} else {
		// Find max name width for alignment
		nameW := 4
		for _, name := range names {
			if len(name) > nameW {
				nameW = len(name)
			}
		}

		fmt.Fprintf(&b, "  %-*s  %-24s  %6s  %6s  %6s\n", nameW, "Name", "Created", "Traces", "Logs", "Metric")
		fmt.Fprintf(&b, "  %-*s  %-24s  %6s  %6s  %6s\n", nameW, strings.Repeat("─", nameW), "────────────────────────", "──────", "──────", "──────")

		for _, name := range names {
			snap, err := s.storage.Snapshots().Get(name)
			if err != nil {
				continue
			}
			created := snap.CreatedAt.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "  %-*s  %-24s  %6d  %6d  %6d\n",
				nameW, snap.Name, created, snap.TracePos, snap.LogPos, snap.MetricPos)
		}
	}

	return textResult(req.Params.URI, b.String()), nil
}

func (s *Server) handleFileSourcesResource(
	ctx context.Context,
	req *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	stats := s.FileSourceStats()

	var b strings.Builder
	fmt.Fprintf(&b, "File Sources (%d)\n", len(stats))
	b.WriteString("═════════════════\n")

	if len(stats) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, stat := range stats {
			fmt.Fprintf(&b, "  %s\n", stat.Directory)
			fmt.Fprintf(&b, "    Files tracked: %d\n", stat.FilesTracked)
			if len(stat.WatchedDirs) > 0 {
				b.WriteString("    Watching:\n")
				for _, dir := range stat.WatchedDirs {
					fmt.Fprintf(&b, "      • %s\n", dir)
				}
			}
		}
	}

	return textResult(req.Params.URI, b.String()), nil
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

	var b strings.Builder
	fmt.Fprintf(&b, "Service: %s\n", serviceName)
	b.WriteString(strings.Repeat("═", len(serviceName)+9) + "\n")
	fmt.Fprintf(&b, "  Spans:    %s\n", fmtNum(result.Summary.SpanCount))
	fmt.Fprintf(&b, "  Logs:     %s\n", fmtNum(result.Summary.LogCount))
	fmt.Fprintf(&b, "  Metrics:  %s\n", fmtNum(result.Summary.MetricCount))

	if len(result.Summary.LogSeverities) > 0 {
		b.WriteString("\n  Log Severities:\n")
		sevKeys := make([]string, 0, len(result.Summary.LogSeverities))
		for k := range result.Summary.LogSeverities {
			sevKeys = append(sevKeys, k)
		}
		sort.Strings(sevKeys)
		for _, sev := range sevKeys {
			fmt.Fprintf(&b, "    %-8s %s\n", sev, fmtNum(result.Summary.LogSeverities[sev]))
		}
	}

	if len(result.Summary.TraceIDs) > 0 {
		b.WriteString("\n  Trace IDs:\n")
		limit := len(result.Summary.TraceIDs)
		if limit > 10 {
			limit = 10
		}
		for _, tid := range result.Summary.TraceIDs[:limit] {
			fmt.Fprintf(&b, "    %s\n", tid)
		}
		if len(result.Summary.TraceIDs) > 10 {
			fmt.Fprintf(&b, "    ... and %d more\n", len(result.Summary.TraceIDs)-10)
		}
	}

	if len(result.Summary.MetricNames) > 0 {
		b.WriteString("\n  Metric Names:\n")
		limit := len(result.Summary.MetricNames)
		if limit > 20 {
			limit = 20
		}
		for _, name := range result.Summary.MetricNames[:limit] {
			fmt.Fprintf(&b, "    %s\n", name)
		}
		if len(result.Summary.MetricNames) > 20 {
			fmt.Fprintf(&b, "    ... and %d more\n", len(result.Summary.MetricNames)-20)
		}
	}

	return textResult(req.Params.URI, b.String()), nil
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

	var b strings.Builder
	fmt.Fprintf(&b, "Snapshot: %s\n", snap.Name)
	b.WriteString(strings.Repeat("═", len(snap.Name)+10) + "\n")
	fmt.Fprintf(&b, "  Created:    %s\n", snap.CreatedAt.Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(&b, "  Trace Pos:  %d\n", snap.TracePos)
	fmt.Fprintf(&b, "  Log Pos:    %d\n", snap.LogPos)
	fmt.Fprintf(&b, "  Metric Pos: %d\n", snap.MetricPos)

	return textResult(req.Params.URI, b.String()), nil
}

// ─── Helpers ────────────────────────────────────────────────────────────

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

// textResult wraps a string in a ReadResourceResult.
func textResult(uri, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:  uri,
			Text: text,
		}},
	}
}

// fmtNum formats an integer with comma separators (e.g. 10,000).
func fmtNum(n int) string {
	if n < 0 {
		return "-" + fmtNum(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

// fmtPct formats a percentage like "62%" or "100%".
func fmtPct(count, capacity int) string {
	if capacity == 0 {
		return "─"
	}
	pct := float64(count) / float64(capacity) * 100
	return fmt.Sprintf("%.0f%%", pct)
}
