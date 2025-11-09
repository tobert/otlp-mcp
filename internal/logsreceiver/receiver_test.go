package logsreceiver

import (
	"context"
	"sync"
	"testing"
	"time"

	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockReceiver is a test implementation of LogReceiver that records received logs.
type mockReceiver struct {
	mu   sync.Mutex
	logs []*logspb.ResourceLogs
	err  error // error to return from ReceiveLogs
}

func (m *mockReceiver) ReceiveLogs(ctx context.Context, logs []*logspb.ResourceLogs) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	m.logs = append(m.logs, logs...)
	return nil
}

func (m *mockReceiver) getLogs() []*logspb.ResourceLogs {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs
}

func (m *mockReceiver) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.logs)
}

// TestNewServer verifies server creation.
func TestNewServer(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Stop()

	if server == nil {
		t.Fatal("server is nil")
	}

	endpoint := server.Endpoint()
	if endpoint == "" {
		t.Fatal("endpoint is empty")
	}

	t.Logf("Server endpoint: %s", endpoint)
}

// TestNewServerNilReceiver verifies that NewServer rejects nil receivers.
func TestNewServerNilReceiver(t *testing.T) {
	_, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, nil)
	if err == nil {
		t.Fatal("expected error for nil receiver, got nil")
	}
}

// TestServerStartStop verifies the server can start and stop cleanly.
func TestServerStartStop(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	server.Stop()

	// Wait for Start to return
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Server stopped with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

// TestOTLPExport tests the full flow of sending logs via OTLP gRPC.
func TestOTLPExport(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	endpoint := server.Endpoint()
	t.Logf("Connecting to server at %s", endpoint)

	// Create gRPC client
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectorlogs.NewLogsServiceClient(conn)

	// Create test log record
	testLog := &logspb.LogRecord{
		TimeUnixNano:         uint64(time.Now().UnixNano()),
		ObservedTimeUnixNano: uint64(time.Now().UnixNano()),
		SeverityNumber:       logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		SeverityText:         "INFO",
		Body: &commonpb.AnyValue{
			Value: &commonpb.AnyValue_StringValue{StringValue: "test log message"},
		},
		Attributes: []*commonpb.KeyValue{
			{
				Key: "test.key",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{StringValue: "test-value"},
				},
			},
		},
	}

	// Create resource with service name
	resource := &resourcepb.Resource{
		Attributes: []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"},
				},
			},
		},
	}

	// Build OTLP request
	req := &collectorlogs.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: resource,
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{testLog},
					},
				},
			},
		},
	}

	// Send the log
	resp, err := client.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	// Verify receiver got the log
	time.Sleep(50 * time.Millisecond) // Give receiver time to process

	if receiver.count() != 1 {
		t.Fatalf("expected 1 resource log, got %d", receiver.count())
	}

	receivedLogs := receiver.getLogs()
	if len(receivedLogs[0].ScopeLogs) != 1 {
		t.Fatalf("expected 1 scope log, got %d", len(receivedLogs[0].ScopeLogs))
	}

	if len(receivedLogs[0].ScopeLogs[0].LogRecords) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(receivedLogs[0].ScopeLogs[0].LogRecords))
	}

	receivedLog := receivedLogs[0].ScopeLogs[0].LogRecords[0]
	if receivedLog.Body.GetStringValue() != "test log message" {
		t.Errorf("expected log body 'test log message', got %q", receivedLog.Body.GetStringValue())
	}

	// Verify resource attributes
	if receivedLogs[0].Resource == nil {
		t.Fatal("resource is nil")
	}

	foundServiceName := false
	for _, attr := range receivedLogs[0].Resource.Attributes {
		if attr.Key == "service.name" {
			if attr.Value.GetStringValue() == "test-service" {
				foundServiceName = true
			}
		}
	}

	if !foundServiceName {
		t.Error("service.name attribute not found in resource")
	}
}

// TestOTLPExportMultipleLogs verifies handling of multiple log records.
func TestOTLPExportMultipleLogs(t *testing.T) {
	receiver := &mockReceiver{}

	server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := grpc.NewClient(server.Endpoint(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	client := collectorlogs.NewLogsServiceClient(conn)

	// Send 5 separate requests
	for i := 0; i < 5; i++ {
		req := &collectorlogs.ExportLogsServiceRequest{
			ResourceLogs: []*logspb.ResourceLogs{
				{
					Resource: &resourcepb.Resource{},
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{
								{
									TimeUnixNano:         uint64(time.Now().UnixNano()),
									ObservedTimeUnixNano: uint64(time.Now().UnixNano()),
									SeverityNumber:       logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
									Body: &commonpb.AnyValue{
										Value: &commonpb.AnyValue_StringValue{StringValue: "test log"},
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := client.Export(context.Background(), req)
		if err != nil {
			t.Fatalf("Export %d failed: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if receiver.count() != 5 {
		t.Fatalf("expected 5 resource logs, got %d", receiver.count())
	}
}
