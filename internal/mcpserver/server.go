package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// Server wraps the MCP server with observability storage and OTLP receiver.
// It provides snapshot-first tools for agents to query telemetry data across all signal types.
type Server struct {
	mcpServer    *mcp.Server
	storage      *storage.ObservabilityStorage
	otlpReceiver *otlpreceiver.UnifiedServer // OTLP receiver for dynamic port rebinding
}

// NewServer creates a new MCP server that exposes snapshot-first observability tools.
// The otlpReceiver provides the OTLP endpoint and enables dynamic port rebinding.
func NewServer(obsStorage *storage.ObservabilityStorage, otlpReceiver *otlpreceiver.UnifiedServer) (*Server, error) {
	if obsStorage == nil {
		return nil, fmt.Errorf("observability storage cannot be nil")
	}

	if otlpReceiver == nil {
		return nil, fmt.Errorf("OTLP receiver cannot be nil")
	}

	s := &Server{
		storage:      obsStorage,
		otlpReceiver: otlpReceiver,
	}

	// Create MCP server with implementation metadata
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "otlp-mcp",
		Title:   "OpenTelemetry Observability for Agents",
		Version: "0.3.0", // v0.3.0: doctor + richer query filters
	}, &mcp.ServerOptions{
		Instructions: `🔭 OpenTelemetry Observability via MCP

This server enables real-time observability for OpenTelemetry-instrumented programs:
• Captures OTLP traces, logs, and metrics in memory (no external dependencies!)
• Provides snapshot-based temporal queries ("what happened during deployment?")
• Dynamic port management - add/remove OTLP ports on-demand
• Powerful query filters - find errors, slow operations, specific attributes
• Perfect for debugging, testing, performance analysis, and understanding system behavior

💡 Quick Start Workflow:
1. get_otlp_endpoint - Get the OTLP endpoint address (or add_otlp_port for specific port)
2. Run your instrumented program: OTEL_EXPORTER_OTLP_ENDPOINT=<endpoint> ./yourapp
3. create_snapshot before and after key events (e.g., 'before-test', 'after-deploy')
4. query with filters to find specific telemetry (errors_only, min_duration_ns, attributes)
5. get_snapshot_data to analyze what happened between snapshots

🎯 Use Cases:
• Find failures: query({errors_only: true})
• Find slow operations: query({min_duration_ns: 500000000})
• Find HTTP errors: query({attribute_equals: {"http.status_code": "500"}})
• Compare before/after: snapshots + get_snapshot_data
• Observing test runs: see exactly what traces/logs/metrics your tests generated

📌 Pro Tip: Use this whenever you see OpenTelemetry, OTLP, tracing, or observability!
The unified endpoint accepts traces + logs + metrics on a single port.`,
	})

	// Register all tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return s, nil
}

// Run starts the MCP server on stdio transport.
// This method blocks until the context is cancelled or EOF is received on stdin.
func (s *Server) Run(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}
