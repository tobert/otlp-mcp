package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// Server wraps the MCP server with observability storage and OTLP endpoint information.
// It provides snapshot-first tools for agents to query telemetry data across all signal types.
type Server struct {
	mcpServer *mcp.Server
	storage   *storage.ObservabilityStorage
	endpoint  string // Single OTLP gRPC endpoint for all signal types
}

// NewServer creates a new MCP server that exposes snapshot-first observability tools.
// The endpoint should be the OTLP gRPC address that accepts all signal types.
func NewServer(obsStorage *storage.ObservabilityStorage, endpoint string) (*Server, error) {
	if obsStorage == nil {
		return nil, fmt.Errorf("observability storage cannot be nil")
	}

	if endpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint cannot be empty")
	}

	s := &Server{
		storage:  obsStorage,
		endpoint: endpoint,
	}

	// Create MCP server with implementation metadata
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "otlp-mcp",
		Version: "0.2.0", // Bumped for snapshot-first redesign
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
