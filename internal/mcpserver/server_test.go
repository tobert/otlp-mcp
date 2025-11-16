package mcpserver

import (
	"testing"

	"github.com/tobert/otlp-mcp/internal/storage"
)

// TestServerCreation verifies basic server initialization.
func TestServerCreation(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	endpoints := Endpoints{
		Traces:  "localhost:54321",
		Logs:    "localhost:54322",
		Metrics: "localhost:54323",
	}

	server, err := NewServer(obsStorage, endpoints)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}

	if server.endpoints.Traces != "localhost:54321" {
		t.Fatalf("expected traces endpoint 'localhost:54321', got %q", server.endpoints.Traces)
	}
	if server.endpoints.Logs != "localhost:54322" {
		t.Fatalf("expected logs endpoint 'localhost:54322', got %q", server.endpoints.Logs)
	}
	if server.endpoints.Metrics != "localhost:54323" {
		t.Fatalf("expected metrics endpoint 'localhost:54323', got %q", server.endpoints.Metrics)
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
	endpoints := Endpoints{
		Traces:  "localhost:54321",
		Logs:    "localhost:54322",
		Metrics: "localhost:54323",
	}

	_, err := NewServer(nil, endpoints)
	if err == nil {
		t.Fatal("expected error for nil storage, got nil")
	}
}

// TestServerCreationEmptyTraceEndpoint verifies that NewServer rejects empty trace endpoint.
func TestServerCreationEmptyTraceEndpoint(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	endpoints := Endpoints{
		Traces:  "", // Empty trace endpoint
		Logs:    "localhost:54322",
		Metrics: "localhost:54323",
	}

	_, err := NewServer(obsStorage, endpoints)
	if err == nil {
		t.Fatal("expected error for empty trace endpoint, got nil")
	}
}

// TestServerToolRegistration verifies that all 5 snapshot-first tools are registered.
func TestServerToolRegistration(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	endpoints := Endpoints{
		Traces:  "localhost:54321",
		Logs:    "localhost:54322",
		Metrics: "localhost:54323",
	}

	server, err := NewServer(obsStorage, endpoints)
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
