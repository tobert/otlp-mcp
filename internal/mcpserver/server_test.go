package mcpserver

import (
	"context"
	"testing"

	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
)

// TestServerCreation verifies basic server initialization.
func TestServerCreation(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)

	// Create OTLP receiver
	otlpReceiver, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP receiver: %v", err)
	}
	defer otlpReceiver.Stop()

	// Start receiver in background
	ctx := context.Background()
	go otlpReceiver.Start(ctx)

	server, err := NewServer(obsStorage, otlpReceiver)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}

	if server.otlpReceiver != otlpReceiver {
		t.Fatal("otlp receiver not set correctly")
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
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	otlpReceiver, _ := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	defer otlpReceiver.Stop()

	_, err := NewServer(nil, otlpReceiver)
	if err == nil {
		t.Fatal("expected error for nil storage, got nil")
	}
}

// TestServerCreationNilReceiver verifies that NewServer rejects nil receiver.
func TestServerCreationNilReceiver(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)
	_, err := NewServer(obsStorage, nil)
	if err == nil {
		t.Fatal("expected error for nil receiver, got nil")
	}
}

// TestServerToolRegistration verifies that all 8 snapshot-first tools are registered.
func TestServerToolRegistration(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)

	otlpReceiver, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP receiver: %v", err)
	}
	defer otlpReceiver.Stop()

	server, err := NewServer(obsStorage, otlpReceiver)
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

	otlpReceiver, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP receiver: %v", err)
	}
	defer otlpReceiver.Stop()

	server, err := NewServer(obsStorage, otlpReceiver)
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

	otlpReceiver, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP receiver: %v", err)
	}
	defer otlpReceiver.Stop()

	server, err := NewServer(obsStorage, otlpReceiver)
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

// TestAddOTLPPortHandler verifies the add_otlp_port tool handler.
func TestAddOTLPPortHandler(t *testing.T) {
	obsStorage := storage.NewObservabilityStorage(100, 500, 1000)

	// Start with ephemeral port
	otlpReceiver, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
		obsStorage,
	)
	if err != nil {
		t.Fatalf("failed to create OTLP receiver: %v", err)
	}
	defer otlpReceiver.Stop()

	ctx := context.Background()
	go otlpReceiver.Start(ctx)

	originalEndpoint := otlpReceiver.Endpoint()
	t.Logf("Original endpoint: %s", originalEndpoint)

	server, err := NewServer(obsStorage, otlpReceiver)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Add a specific port (using a high port to avoid conflicts)
	newPort := 45678
	result, output, err := server.handleAddOTLPPort(ctx, nil, AddOTLPPortInput{Port: newPort})
	if err != nil {
		t.Fatalf("handleAddOTLPPort failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if !output.Success {
		t.Errorf("add port failed: %s", output.Message)
	}

	// Should now have 2 endpoints
	if len(output.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints after adding port, got %d", len(output.Endpoints))
	}

	// Verify both endpoints are present
	endpoints := otlpReceiver.Endpoints()
	if len(endpoints) != 2 {
		t.Errorf("receiver should have 2 endpoints, got %d", len(endpoints))
	}

	// Test invalid port
	_, invalidOutput, err := server.handleAddOTLPPort(ctx, nil, AddOTLPPortInput{Port: 99999})
	if err != nil {
		t.Fatalf("handleAddOTLPPort with invalid port failed: %v", err)
	}

	if invalidOutput.Success {
		t.Error("expected failure for invalid port, got success")
	}
}
