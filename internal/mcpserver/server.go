package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// Endpoints contains the OTLP gRPC endpoint addresses for each signal type.
type Endpoints struct {
	Traces  string
	Logs    string
	Metrics string
}

// Server wraps the MCP server with observability storage and OTLP endpoint information.
// It provides snapshot-first tools for agents to query telemetry data across all signal types.
type Server struct {
	mcpServer *mcp.Server
	storage   *storage.ObservabilityStorage
	endpoints Endpoints
}

// NewServer creates a new MCP server that exposes snapshot-first observability tools.
func NewServer(obsStorage *storage.ObservabilityStorage, endpoints Endpoints) (*Server, error) {
	if obsStorage == nil {
		return nil, fmt.Errorf("observability storage cannot be nil")
	}

	if endpoints.Traces == "" {
		return nil, fmt.Errorf("trace endpoint cannot be empty")
	}

	s := &Server{
		storage:   obsStorage,
		endpoints: endpoints,
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
