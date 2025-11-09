package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// Server wraps the MCP server with trace storage and OTLP endpoint information.
// It provides tools for agents to query trace data and get the OTLP endpoint address.
type Server struct {
	mcpServer *mcp.Server
	storage   *storage.TraceStorage
	endpoint  string // OTLP gRPC endpoint address (e.g., "localhost:54321")
}

// NewServer creates a new MCP server that exposes trace query tools.
// The otlpEndpoint should be the actual address where the OTLP gRPC server is listening.
func NewServer(traceStorage *storage.TraceStorage, otlpEndpoint string) (*Server, error) {
	if traceStorage == nil {
		return nil, fmt.Errorf("trace storage cannot be nil")
	}

	if otlpEndpoint == "" {
		return nil, fmt.Errorf("otlp endpoint cannot be empty")
	}

	s := &Server{
		storage:  traceStorage,
		endpoint: otlpEndpoint,
	}

	// Create MCP server with implementation metadata
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "otlp-mcp",
		Version: "0.1.0",
	}, nil)

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
