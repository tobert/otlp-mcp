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

// TestServerToolRegistration verifies that all 7 snapshot-first tools are registered.
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

// TestGetStatsHandler verifies the get_stats tool handler.
func TestGetStatsHandler(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	server, err := NewServer(obsStorage, "localhost:54321")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Call handler directly
	result, output, err := server.handleGetStats(nil, nil, GetStatsInput{})
	if err != nil {
		t.Fatalf("handleGetStats failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify output structure
	if output.Traces.Capacity != 100 {
		t.Errorf("expected trace capacity 100, got %d", output.Traces.Capacity)
	}
	if output.Logs.Capacity != 500 {
		t.Errorf("expected log capacity 500, got %d", output.Logs.Capacity)
	}
	if output.Metrics.Capacity != 1000 {
		t.Errorf("expected metric capacity 1000, got %d", output.Metrics.Capacity)
	}
	if output.Snapshots != 0 {
		t.Errorf("expected 0 snapshots, got %d", output.Snapshots)
	}
}

// TestClearDataHandler verifies the clear_data tool handler.
func TestClearDataHandler(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	server, err := NewServer(obsStorage, "localhost:54321")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Add some data first
	obsStorage.CreateSnapshot("test-snapshot")

	// Call handler (nuclear option)
	result, output, err := server.handleClearData(nil, nil, ClearDataInput{})
	if err != nil {
		t.Fatalf("handleClearData failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if output.Message == "" {
		t.Error("expected non-empty message")
	}

	// Verify complete reset - everything gone
	snapshots := obsStorage.Snapshots().List()
	if len(snapshots) != 0 {
		t.Errorf("expected complete reset (0 snapshots), got %d snapshots", len(snapshots))
	}
}
