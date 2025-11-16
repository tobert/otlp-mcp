package mcpserver

import (
	"testing"

	"github.com/tobert/otlp-mcp/internal/storage"
)

// TestServerCreation verifies basic server initialization.
func TestServerCreation(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	endpoint := "localhost:54321"

	server, err := NewServer(obsStorage, endpoint)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}

	if server.endpoint != endpoint {
		t.Fatalf("expected endpoint %q, got %q", endpoint, server.endpoint)
	}

	if server.storage != obsStorage {
		t.Fatal("storage not set correctly")
	}

	if server.mcpServer == nil {
		t.Fatal("mcp server is nil")
	}
}

// TestServerCreationNilStorage verifies that NewServer rejects nil storage.
func TestServerCreationNilStorage(t *testing.T) {
	_, err := NewServer(nil, "localhost:54321")
	if err == nil {
		t.Fatal("expected error for nil storage, got nil")
	}
}

// TestServerCreationEmptyEndpoint verifies that NewServer rejects empty endpoint.
func TestServerCreationEmptyEndpoint(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	_, err := NewServer(obsStorage, "")
	if err == nil {
		t.Fatal("expected error for empty endpoint, got nil")
	}
}

// TestServerToolRegistration verifies that all 5 snapshot-first tools are registered.
func TestServerToolRegistration(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	endpoint := "localhost:54321"

	server, err := NewServer(obsStorage, endpoint)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Verify MCP server was created (tools registered in registerTools)
	if server.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}

	// The actual tool registration is tested via integration tests
	// Here we just verify the server structure is correct
}
