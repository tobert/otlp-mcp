package mcpserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/filereader"
	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// Server wraps the MCP server with observability storage and OTLP receiver.
// It provides snapshot-first tools for agents to query telemetry data across all signal types.
type Server struct {
	mcpServer    *mcp.Server
	storage      *storage.ObservabilityStorage
	otlpReceiver *otlpreceiver.UnifiedServer // OTLP receiver for dynamic port rebinding

	// File sources - directories being watched for OTLP JSONL files
	fileSourcesMu sync.RWMutex
	fileSources   map[string]*filereader.FileSource
	verbose       bool
}

// ServerOptions configures the MCP server.
type ServerOptions struct {
	Verbose bool // Enable verbose logging
}

// NewServer creates a new MCP server that exposes snapshot-first observability tools.
// The otlpReceiver provides the OTLP endpoint and enables dynamic port rebinding.
func NewServer(obsStorage *storage.ObservabilityStorage, otlpReceiver *otlpreceiver.UnifiedServer, opts ...ServerOptions) (*Server, error) {
	if obsStorage == nil {
		return nil, fmt.Errorf("observability storage cannot be nil")
	}

	if otlpReceiver == nil {
		return nil, fmt.Errorf("OTLP receiver cannot be nil")
	}

	var verbose bool
	if len(opts) > 0 {
		verbose = opts[0].Verbose
	}

	s := &Server{
		storage:      obsStorage,
		otlpReceiver: otlpReceiver,
		fileSources:  make(map[string]*filereader.FileSource),
		verbose:      verbose,
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
	err := s.mcpServer.Run(ctx, transport)

	// Stop all file sources on shutdown
	s.stopAllFileSources()

	return err
}

// MCPServer returns the underlying mcp.Server for use with alternative transports.
// This enables the server to be used with StreamableHTTPHandler for HTTP transport.
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}

// Shutdown performs cleanup when using non-stdio transports.
// For stdio transport, this cleanup is handled by Run() automatically.
func (s *Server) Shutdown() {
	s.stopAllFileSources()
}

// AddFileSource adds a new file source that reads OTLP JSONL from a directory.
// When activeOnly is true, only active files (e.g., traces.jsonl) are loaded,
// skipping rotated archives (e.g., traces-2025-12-09T13-10-56.jsonl).
// Returns an error if the directory is already being watched.
func (s *Server) AddFileSource(ctx context.Context, directory string, activeOnly bool) error {
	s.fileSourcesMu.Lock()
	defer s.fileSourcesMu.Unlock()

	if _, exists := s.fileSources[directory]; exists {
		return fmt.Errorf("directory %s is already being watched", directory)
	}

	fs, err := filereader.New(filereader.Config{
		Directory:  directory,
		Verbose:    s.verbose,
		ActiveOnly: activeOnly,
	}, s.storage)
	if err != nil {
		return fmt.Errorf("failed to create file source: %w", err)
	}

	if err := fs.Start(ctx); err != nil {
		return fmt.Errorf("failed to start file source: %w", err)
	}

	s.fileSources[directory] = fs
	return nil
}

// RemoveFileSource stops and removes a file source.
func (s *Server) RemoveFileSource(directory string) error {
	s.fileSourcesMu.Lock()
	defer s.fileSourcesMu.Unlock()

	fs, exists := s.fileSources[directory]
	if !exists {
		return fmt.Errorf("directory %s is not being watched", directory)
	}

	fs.Stop()
	delete(s.fileSources, directory)
	return nil
}

// ListFileSources returns all active file source directories.
func (s *Server) ListFileSources() []string {
	s.fileSourcesMu.RLock()
	defer s.fileSourcesMu.RUnlock()

	dirs := make([]string, 0, len(s.fileSources))
	for dir := range s.fileSources {
		dirs = append(dirs, dir)
	}
	return dirs
}

// FileSourceStats returns stats for all file sources.
func (s *Server) FileSourceStats() []filereader.Stats {
	s.fileSourcesMu.RLock()
	defer s.fileSourcesMu.RUnlock()

	stats := make([]filereader.Stats, 0, len(s.fileSources))
	for _, fs := range s.fileSources {
		stats = append(stats, fs.Stats())
	}
	return stats
}

// stopAllFileSources stops all file sources (called on shutdown).
func (s *Server) stopAllFileSources() {
	s.fileSourcesMu.Lock()
	defer s.fileSourcesMu.Unlock()

	for dir, fs := range s.fileSources {
		fs.Stop()
		delete(s.fileSources, dir)
	}
}
