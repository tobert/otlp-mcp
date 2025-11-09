# Task 03: OTLP gRPC Receiver

## Why

Need to receive OTLP trace data from instrumented programs. We'll copy and refactor the OTLP gRPC server implementation from otel-cli, adapting it for our ring buffer storage model.

## What

Implement:
- OTLP gRPC service for traces (TraceService)
- Server that binds to localhost:0 (ephemeral port)
- Interface to pass received spans to storage layer
- Proper error handling and logging

## Approach

### Source Material

Copy and refactor from `~/src/otel-cli`:
- `otlpserver/grpcserver.go` - gRPC server setup
- `otlpserver/server.go` - Common server utilities
- Review `otlpclient/protobuf_span.go` for protobuf handling patterns

### Architecture

```
┌─────────────────────────────────────────┐
│ OTLP gRPC Server (localhost:XXXXX)      │
│                                         │
│  ┌────────────────────────────────┐    │
│  │ TraceService                   │    │
│  │ .Export(ExportTraceRequest)    │    │
│  └────────────┬───────────────────┘    │
│               │                         │
│               │ []ResourceSpans         │
│               ▼                         │
│  ┌────────────────────────────────┐    │
│  │ SpanReceiver (interface)       │    │
│  │ .ReceiveSpans(spans)           │    │
│  └────────────┬───────────────────┘    │
│               │                         │
└───────────────┼─────────────────────────┘
                │
                ▼
   ┌────────────────────────────┐
   │ Ring Buffer Storage        │
   │ (implemented in task 04)   │
   └────────────────────────────┘
```

### Key Types

```go
// internal/otlpreceiver/receiver.go
package otlpreceiver

import (
    "context"
    "net"

    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
    "google.golang.org/grpc"
)

// SpanReceiver is the interface for storing received spans
type SpanReceiver interface {
    ReceiveSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error
}

// Server is the OTLP gRPC server
type Server struct {
    listener     net.Listener
    grpcServer   *grpc.Server
    spanReceiver SpanReceiver
}

// Config for the OTLP receiver
type Config struct {
    Host string // e.g., "127.0.0.1"
    Port int    // 0 for ephemeral
}

// NewServer creates a new OTLP gRPC server
func NewServer(cfg Config, receiver SpanReceiver) (*Server, error) {
    // Implementation
}

// Start begins listening for OTLP requests
func (s *Server) Start() error {
    // Bind to host:port
    // Start gRPC server
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
    // Graceful shutdown
}

// Endpoint returns the actual listening address (useful for ephemeral ports)
func (s *Server) Endpoint() string {
    // Returns "127.0.0.1:54321" or similar
}
```

### OTLP Service Implementation

```go
// internal/otlpreceiver/trace_service.go
package otlpreceiver

import (
    "context"

    collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// traceService implements the OTLP TraceService
type traceService struct {
    collectortrace.UnimplementedTraceServiceServer
    receiver SpanReceiver
}

// Export handles incoming trace export requests
func (s *traceService) Export(
    ctx context.Context,
    req *collectortrace.ExportTraceServiceRequest,
) (*collectortrace.ExportTraceServiceResponse, error) {
    if err := s.receiver.ReceiveSpans(ctx, req.ResourceSpans); err != nil {
        return nil, err
    }

    return &collectortrace.ExportTraceServiceResponse{}, nil
}
```

### Error Handling

- Log errors but don't crash the server
- Return proper gRPC error codes
- Use `fmt.Errorf` with `%w` for error wrapping

### Logging

For MVP, simple logging to stderr:
```go
if verbose {
    log.Printf("Received %d resource spans\n", len(req.ResourceSpans))
}
```

## Dependencies

- Task 01 (project-setup) must be complete
- Task 02 (cli-framework) helpful but not required
- Task 04 (ring-buffer) can be stubbed with a mock receiver for testing

## Acceptance Criteria

- [ ] Server starts on localhost:0
- [ ] Actual port can be queried via `Endpoint()`
- [ ] Server accepts OTLP trace export requests
- [ ] Received spans are passed to `SpanReceiver` interface
- [ ] Server shuts down gracefully
- [ ] No panics on malformed requests

## Testing

### Unit Test with Mock Receiver

```go
// internal/otlpreceiver/receiver_test.go
package otlpreceiver

import (
    "context"
    "testing"

    collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

type mockReceiver struct {
    spans []*tracepb.ResourceSpans
}

func (m *mockReceiver) ReceiveSpans(ctx context.Context, spans []*tracepb.ResourceSpans) error {
    m.spans = append(m.spans, spans...)
    return nil
}

func TestOTLPServer(t *testing.T) {
    receiver := &mockReceiver{}

    server, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, receiver)
    if err != nil {
        t.Fatal(err)
    }

    if err := server.Start(); err != nil {
        t.Fatal(err)
    }
    defer server.Stop()

    endpoint := server.Endpoint()
    if endpoint == "" {
        t.Fatal("endpoint is empty")
    }

    // Create gRPC client
    conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close()

    client := collectortrace.NewTraceServiceClient(conn)

    // Send test span
    resp, err := client.Export(context.Background(), &collectortrace.ExportTraceServiceRequest{
        ResourceSpans: []*tracepb.ResourceSpans{
            // Test data
        },
    })
    if err != nil {
        t.Fatal(err)
    }

    if resp == nil {
        t.Fatal("response is nil")
    }

    // Verify receiver got the spans
    if len(receiver.spans) != 1 {
        t.Fatalf("expected 1 resource span, got %d", len(receiver.spans))
    }
}
```

### Integration Test with Real OTLP Client

Can use `otel-cli` from `~/src/otel-cli` to send test spans:

```bash
# Start the server in background
go run ./cmd/otlp-mcp serve &
SERVER_PID=$!

# Wait for startup
sleep 1

# Get endpoint somehow (MCP in future, for now maybe log it?)
ENDPOINT="localhost:54321"

# Send a span
cd ~/src/otel-cli
./otel-cli span --endpoint $ENDPOINT --name "test-span" --service "test-service"

# Check server logs to verify reception

# Cleanup
kill $SERVER_PID
```

## Notes

### Important Implementation Details

1. **Ephemeral Port Discovery**: After binding to `:0`, use `listener.Addr()` to get actual port
2. **Graceful Shutdown**: Use `grpcServer.GracefulStop()` with timeout
3. **Context Handling**: Respect context cancellation in Export handler
4. **Resource Extraction**: ResourceSpans contain resource info (service.name, etc.) - preserve this!

### Data Flow

```
ExportTraceServiceRequest
  └─ []ResourceSpans
       └─ Resource (service.name, etc.)
       └─ []ScopeSpans
            └─ Scope (instrumentation library)
            └─ []Span
                 └─ trace_id, span_id, parent_span_id
                 └─ name, kind, status
                 └─ attributes, events, links
```

Don't flatten this structure - preserve the full hierarchy for the MCP layer to query.

## Status

Status: pending
Depends: 01-project-setup
Next: 04-ring-buffer.md
