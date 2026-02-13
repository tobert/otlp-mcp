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
		Version: "0.4.0",
	}, &mcp.ServerOptions{
		Instructions: `OpenTelemetry observability server. Captures OTLP traces, logs, and metrics in memory.

Workflow: get_otlp_endpoint -> set OTEL_EXPORTER_OTLP_ENDPOINT -> run program -> query/snapshot.

Tools: query (filtered search), create_snapshot/get_snapshot_data (before/after), status/recent_activity (polling).
Resources: otlp://endpoint, otlp://stats, otlp://services, otlp://snapshots, otlp://file-sources.`,
		SubscribeHandler:   func(_ context.Context, _ *mcp.SubscribeRequest) error { return nil },
		UnsubscribeHandler: func(_ context.Context, _ *mcp.UnsubscribeRequest) error { return nil },
	})

	// Register all tools and resources
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}
	s.registerResources()

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
// The source is removed from the map under the lock, then stopped
// outside the lock so fs.Stop cannot block other operations.
func (s *Server) RemoveFileSource(directory string) error {
	s.fileSourcesMu.Lock()
	fs, exists := s.fileSources[directory]
	if !exists {
		s.fileSourcesMu.Unlock()
		return fmt.Errorf("directory %s is not being watched", directory)
	}
	delete(s.fileSources, directory)
	s.fileSourcesMu.Unlock()

	fs.Stop()
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
// Sources are collected and the map cleared under the lock, then
// stopped outside the lock so a slow fs.Stop (which waits on
// goroutines) cannot block other file-source operations.
func (s *Server) stopAllFileSources() {
	s.fileSourcesMu.Lock()
	sources := make([]*filereader.FileSource, 0, len(s.fileSources))
	for _, fs := range s.fileSources {
		sources = append(sources, fs)
	}
	clear(s.fileSources)
	s.fileSourcesMu.Unlock()

	for _, fs := range sources {
		fs.Stop()
	}
}
