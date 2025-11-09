# Task 06: Integration & End-to-End Testing

## Why

Wire all components together into a working system and validate the complete workflow: start server, receive OTLP data, query via MCP.

## What

Implement:
- Main server startup logic integrating OTLP + MCP
- Graceful shutdown handling
- End-to-end test with real OTLP client
- Documentation of the complete workflow

## Approach

### Main Server Implementation

```go
// internal/cli/serve.go (updated from task 02)
package cli

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/tobert/otlp-mcp/internal/mcpserver"
    "github.com/tobert/otlp-mcp/internal/otlpreceiver"
    "github.com/tobert/otlp-mcp/internal/storage"
    "github.com/urfave/cli/v3"
)

func runServe(c *cli.Context) error {
    cfg := &Config{
        TraceBufferSize:  c.Int("trace-buffer-size"),
        LogBufferSize:    c.Int("log-buffer-size"),
        MetricBufferSize: c.Int("metric-buffer-size"),
        OTLPHost:        c.String("otlp-host"),
        OTLPPort:        c.Int("otlp-port"),
        Verbose:         c.Bool("verbose"),
    }

    if cfg.Verbose {
        log.Printf("Starting OTLP MCP server with config: %+v\n", cfg)
    }

    // 1. Create storage
    traceStorage := storage.NewTraceStorage(cfg.TraceBufferSize)

    // 2. Start OTLP receiver
    otlpServer, err := otlpreceiver.NewServer(
        otlpreceiver.Config{
            Host: cfg.OTLPHost,
            Port: cfg.OTLPPort,
        },
        traceStorage,
    )
    if err != nil {
        return fmt.Errorf("failed to create OTLP server: %w", err)
    }

    if err := otlpServer.Start(); err != nil {
        return fmt.Errorf("failed to start OTLP server: %w", err)
    }

    endpoint := otlpServer.Endpoint()

    if cfg.Verbose {
        log.Printf("OTLP gRPC server listening on %s\n", endpoint)
    }

    // 3. Create MCP server
    mcpServer, err := mcpserver.NewServer(traceStorage, endpoint)
    if err != nil {
        return fmt.Errorf("failed to create MCP server: %w", err)
    }

    // 4. Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        if cfg.Verbose {
            log.Println("Shutdown signal received, stopping servers...")
        }
        cancel()
        otlpServer.Stop()
    }()

    // 5. Run MCP server (blocks on stdin)
    if cfg.Verbose {
        log.Println("MCP server ready on stdio")
    }

    return mcpServer.Run(ctx)
}
```

### Graceful Shutdown

```go
// internal/otlpreceiver/receiver.go
func (s *Server) Stop() error {
    if s.grpcServer != nil {
        // Give it 5 seconds to gracefully stop
        stopped := make(chan struct{})
        go func() {
            s.grpcServer.GracefulStop()
            close(stopped)
        }()

        select {
        case <-stopped:
            return nil
        case <-time.After(5 * time.Second):
            s.grpcServer.Stop() // Force stop
            return fmt.Errorf("graceful stop timeout, forced shutdown")
        }
    }
    return nil
}
```

## End-to-End Test

### Test Script

```bash
#!/bin/bash
# test/e2e.sh

set -e

echo "=== OTLP MCP End-to-End Test ==="

# Build the server
echo "Building otlp-mcp..."
go build -o /tmp/otlp-mcp ./cmd/otlp-mcp

# Build test MCP client
echo "Building test client..."
cat > /tmp/mcp-client.go <<'EOF'
package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
)

func main() {
    scanner := bufio.NewScanner(os.Stdin)

    // Initialize
    send(map[string]any{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": map[string]any{"protocolVersion": "2024-11-05"},
    })

    if !scanner.Scan() { panic("no response") }
    fmt.Fprintf(os.Stderr, "Initialize: %s\n", scanner.Text())

    // Get endpoint
    send(map[string]any{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": map[string]any{
            "name": "get_otlp_endpoint",
        },
    })

    if !scanner.Scan() { panic("no response") }
    var resp struct {
        Result struct {
            Content []struct {
                Text map[string]string `json:"text"`
            } `json:"content"`
        } `json:"result"`
    }
    json.Unmarshal([]byte(scanner.Text()), &resp)
    endpoint := resp.Result.Content[0].Text["endpoint"]
    fmt.Fprintf(os.Stderr, "OTLP Endpoint: %s\n", endpoint)

    // Wait for traces...
    time.Sleep(2 * time.Second)

    // Query traces
    send(map[string]any{
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/call",
        "params": map[string]any{
            "name": "get_recent_traces",
            "arguments": map[string]int{"limit": 10},
        },
    })

    if !scanner.Scan() { panic("no response") }
    fmt.Fprintf(os.Stderr, "Traces: %s\n", scanner.Text())
}

func send(req any) {
    json.NewEncoder(os.Stdout).Encode(req)
}
EOF

go build -o /tmp/mcp-client /tmp/mcp-client.go

# Start server
echo "Starting OTLP MCP server..."
/tmp/otlp-mcp serve --verbose 2>&1 | tee /tmp/server.log &
SERVER_PID=$!

sleep 1

# Extract endpoint from logs
ENDPOINT=$(grep "OTLP gRPC server listening" /tmp/server.log | awk '{print $NF}')
echo "Detected OTLP endpoint: $ENDPOINT"

# Send test span using otel-cli
if command -v otel-cli &> /dev/null; then
    echo "Sending test span..."
    OTEL_EXPORTER_OTLP_ENDPOINT=$ENDPOINT \
    otel-cli span --service e2e-test --name "test-span" --attrs "test=true"
else
    echo "Warning: otel-cli not found, skipping span send"
fi

# Query via MCP
echo "Querying traces via MCP..."
/tmp/mcp-client

# Cleanup
echo "Cleaning up..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null || true

echo "=== Test Complete ==="
```

### Alternative: Pure Go E2E Test

```go
// test/e2e_test.go
package test

import (
    "context"
    "testing"
    "time"

    collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    commonpb "go.opentelemetry.io/proto/otlp/common/v1"
    resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
    tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    "github.com/tobert/otlp-mcp/internal/mcpserver"
    "github.com/tobert/otlp-mcp/internal/otlpreceiver"
    "github.com/tobert/otlp-mcp/internal/storage"
)

func TestEndToEnd(t *testing.T) {
    // 1. Setup storage
    traceStorage := storage.NewTraceStorage(1000)

    // 2. Start OTLP server
    otlpServer, err := otlpreceiver.NewServer(
        otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
        traceStorage,
    )
    if err != nil {
        t.Fatal(err)
    }

    if err := otlpServer.Start(); err != nil {
        t.Fatal(err)
    }
    defer otlpServer.Stop()

    endpoint := otlpServer.Endpoint()
    t.Logf("OTLP server listening on %s", endpoint)

    // 3. Send test span
    conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close()

    client := collectortrace.NewTraceServiceClient(conn)

    _, err = client.Export(context.Background(), &collectortrace.ExportTraceServiceRequest{
        ResourceSpans: []*tracepb.ResourceSpans{
            {
                Resource: &resourcepb.Resource{
                    Attributes: []*commonpb.KeyValue{
                        {
                            Key: "service.name",
                            Value: &commonpb.AnyValue{
                                Value: &commonpb.AnyValue_StringValue{StringValue: "e2e-test"},
                            },
                        },
                    },
                },
                ScopeSpans: []*tracepb.ScopeSpans{
                    {
                        Spans: []*tracepb.Span{
                            {
                                TraceId: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
                                SpanId:  []byte{1, 2, 3, 4, 5, 6, 7, 8},
                                Name:    "e2e-test-span",
                            },
                        },
                    },
                },
            },
        },
    })
    if err != nil {
        t.Fatal(err)
    }

    // Give it a moment to process
    time.Sleep(100 * time.Millisecond)

    // 4. Query via storage (MCP would do this)
    recent := traceStorage.GetRecentSpans(10)
    if len(recent) == 0 {
        t.Fatal("no spans found in storage")
    }

    if recent[0].ServiceName != "e2e-test" {
        t.Fatalf("unexpected service name: %s", recent[0].ServiceName)
    }

    if recent[0].SpanName != "e2e-test-span" {
        t.Fatalf("unexpected span name: %s", recent[0].SpanName)
    }

    t.Log("✓ End-to-end test passed")
}
```

## Dependencies

- Tasks 01-05 must be complete

## Acceptance Criteria

- [ ] `otlp-mcp serve` starts successfully
- [ ] OTLP server binds to ephemeral port
- [ ] MCP server responds on stdio
- [ ] Can send OTLP spans to server
- [ ] Spans are stored in buffer
- [ ] Can query spans via MCP tools
- [ ] Graceful shutdown works
- [ ] E2E test passes

## Testing

```bash
# Build
go build ./cmd/otlp-mcp

# Run
./otlp-mcp serve --verbose

# In another terminal, use otel-cli to send spans
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:XXXXX \
  otel-cli span --service my-app --name my-span

# Query via MCP (manually or with test client)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_recent_traces"}}' | \
  ./otlp-mcp serve
```

## Status

Status: pending
Depends: 01-project-setup, 02-cli-framework, 03-otlp-receiver, 04-ring-buffer, 05-mcp-server
Next: 07-documentation.md (optional)
