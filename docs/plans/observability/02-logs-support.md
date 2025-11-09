# Task 02: Logs Support

## Overview

Implement OTLP logs gRPC endpoint and **`LogStorage`** to receive, store, and query log records from instrumented applications.

**Pattern:** Follow the same architecture as traces (from bootstrap).
- OTLP gRPC receiver for logs.
- Ring buffer storage with indexing.
- Integration with main `serve` command.

## Prerequisite

- **Task 01: Storage Optimization** must be complete. This task relies on the **`SetOnEvict`** callback pattern established in **`internal/storage/ringbuffer.go`** to prevent memory leaks.

## Goals

1. Accept OTLP log records via gRPC.
2. Store logs in a ring buffer (50,000 record capacity).
3. Index by `trace_id`, `severity`, and `service name`.
4. Prepare for MCP query tools (Task 04).

## OpenTelemetry Log Specifications

**OTLP Logs Spec:**
- Protocol: https://opentelemetry.io/docs/specs/otel/logs/
- Data Model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
- Proto: `go.opentelemetry.io/proto/otlp/logs/v1`

**Key Types:**
```go
import (
    logspb "go.opentelemetry.io/proto/otlp/logs/v1"
    collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
)

// LogRecord structure (from proto)
type LogRecord struct {
    TimeUnixNano         uint64           // Timestamp
    ObservedTimeUnixNano uint64           // Observation time
    SeverityNumber       SeverityNumber   // DEBUG, INFO, WARN, ERROR, FATAL
    SeverityText         string           // Human-readable severity
    Body                 *AnyValue        // Log message (string or structured)
    Attributes           []*KeyValue      // Additional attributes
    TraceId              []byte           // Optional trace correlation
    SpanId               []byte           // Optional span correlation
    // ... more fields
}
```

**Severity Levels:**
```go
SEVERITY_NUMBER_UNSPECIFIED = 0
SEVERITY_NUMBER_TRACE       = 1
SEVERITY_NUMBER_DEBUG       = 5
SEVERITY_NUMBER_INFO        = 9
SEVERITY_NUMBER_WARN        = 13
SEVERITY_NUMBER_ERROR       = 17
SEVERITY_NUMBER_FATAL       = 21
```

## Implementation

### Step 1: Create LogStorage

**File:** `internal/storage/log_storage.go`

```go
package storage

import (
    "context"
    "fmt"
    "sync"

    logspb "go.opentelemetry.io/proto/otlp/logs/v1"
    resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// StoredLog wraps a protobuf log record with indexed fields.
type StoredLog struct {
    ResourceLog *logspb.ResourceLogs
    ScopeLog    *logspb.ScopeLogs
    LogRecord   *logspb.LogRecord

    // Indexed fields for fast lookup
    TraceID     string
    SpanID      string
    ServiceName string
    Severity    string
    SeverityNum int32
    Body        string // Extracted from AnyValue
    Timestamp   uint64 // TimeUnixNano
}

// LogStorage stores and indexes OTLP log records.
type LogStorage struct {
    logs          *RingBuffer[*StoredLog]
    traceIndex    map[string][]*StoredLog // trace_id → logs
    severityIndex map[string][]*StoredLog // severity → logs
    serviceIndex  map[string][]*StoredLog // service → logs
    mu            sync.RWMutex             // protects indexes
}

// NewLogStorage creates a new log storage with the specified capacity.
func NewLogStorage(capacity int) *LogStorage {
    ls := &LogStorage{
        logs:          NewRingBuffer[*StoredLog](capacity),
        traceIndex:    make(map[string][]*StoredLog),
        severityIndex: make(map[string][]*StoredLog),
        serviceIndex:  make(map[string][]*StoredLog),
    }

    // PATTERN: Use the SetOnEvict callback to ensure indexes are cleaned up
    // when the ring buffer overwrites old data. This prevents memory leaks.
    // This pattern is established in Task 01.
    ls.logs.SetOnEvict(func(position int, oldLog *StoredLog) {
        ls.removeFromIndexes(position, oldLog)
    })

    return ls
}

// ReceiveLogs implements the log receiver interface.
func (ls *LogStorage) ReceiveLogs(ctx context.Context, resourceLogs []*logspb.ResourceLogs) error {
    for _, rl := range resourceLogs {
        serviceName := extractServiceName(rl.Resource)

        for _, sl := range rl.ScopeLogs {
            for _, log := range sl.LogRecords {
                stored := &StoredLog{
                    ResourceLog: rl,
                    ScopeLog:    sl,
                    LogRecord:   log,
                    TraceID:     traceIDToString(log.TraceId),
                    SpanID:      spanIDToString(log.SpanId),
                    ServiceName: serviceName,
                    Severity:    log.SeverityText,
                    SeverityNum: int32(log.SeverityNumber),
                    Body:        extractLogBody(log.Body),
                    Timestamp:   log.TimeUnixNano,
                }

                ls.addLog(stored)
            }
        }
    }

    return nil
}

// addLog adds a log to storage and updates indexes.
func (ls *LogStorage) addLog(log *StoredLog) {
    ls.logs.Add(log)

    ls.mu.Lock()
    defer ls.mu.Unlock()

    // Update trace index (if log has trace_id)
    if log.TraceID != "" {
        ls.traceIndex[log.TraceID] = append(ls.traceIndex[log.TraceID], log)
    }

    // Update severity index
    if log.Severity != "" {
        ls.severityIndex[log.Severity] = append(ls.severityIndex[log.Severity], log)
    }

    // Update service index
    ls.serviceIndex[log.ServiceName] = append(ls.serviceIndex[log.ServiceName], log)
}

// removeFromIndexes cleans up index entries when a log is evicted.
func (ls *LogStorage) removeFromIndexes(position int, oldLog *StoredLog) {
    // Remove from trace index
    if oldLog.TraceID != "" {
        positions := ls.traceIndex[oldLog.TraceID]
        ls.traceIndex[oldLog.TraceID] = removeLogPosition(positions, position)
        if len(ls.traceIndex[oldLog.TraceID]) == 0 {
            delete(ls.traceIndex, oldLog.TraceID)
        }
    }

    // Remove from severity index
    if oldLog.Severity != "" {
        positions := ls.severityIndex[oldLog.Severity]
        ls.severityIndex[oldLog.Severity] = removeLogPosition(positions, position)
        if len(ls.severityIndex[oldLog.Severity]) == 0 {
            delete(ls.severityIndex, oldLog.Severity)
        }
    }

    // Remove from service index
    positions := ls.serviceIndex[oldLog.ServiceName]
    ls.serviceIndex[oldLog.ServiceName] = removeLogPosition(positions, position)
    if len(ls.serviceIndex[oldLog.ServiceName]) == 0 {
        delete(ls.serviceIndex, oldLog.ServiceName)
    }
}

// GetRecentLogs returns the N most recent logs.
func (ls *LogStorage) GetRecentLogs(n int) []*StoredLog {
    return ls.logs.GetRecent(n)
}

// GetLogsByTraceID returns all logs for a given trace ID.
func (ls *LogStorage) GetLogsByTraceID(traceID string) []*StoredLog {
    ls.mu.RLock()
    defer ls.mu.RUnlock()

    logs := ls.traceIndex[traceID]
    if len(logs) == 0 {
        return nil
    }

    result := make([]*StoredLog, len(logs))
    copy(result, logs)
    return result
}

// GetLogsBySeverity returns all logs matching a severity level.
func (ls *LogStorage) GetLogsBySeverity(severity string) []*StoredLog {
    ls.mu.RLock()
    defer ls.mu.RUnlock()

    logs := ls.severityIndex[severity]
    if len(logs) == 0 {
        return nil
    }

    result := make([]*StoredLog, len(logs))
    copy(result, logs)
    return result
}

// GetLogsByService returns all logs for a given service.
func (ls *LogStorage) GetLogsByService(serviceName string) []*StoredLog {
    ls.mu.RLock()
    defer ls.mu.RUnlock()

    logs := ls.serviceIndex[serviceName]
    if len(logs) == 0 {
        return nil
    }

    result := make([]*StoredLog, len(logs))
    copy(result, logs)
    return result
}

// Stats returns current storage statistics.
func (ls *LogStorage) Stats() LogStorageStats {
    ls.mu.RLock()
    defer ls.mu.RUnlock()

    // Count unique severities
    severities := make(map[string]int)
    for severity, logs := range ls.severityIndex {
        severities[severity] = len(logs)
    }

    return LogStorageStats{
        LogCount:     ls.logs.Size(),
        Capacity:     ls.logs.Capacity(),
        TraceCount:   len(ls.traceIndex),
        ServiceCount: len(ls.serviceIndex),
        Severities:   severities,
    }
}

// Clear removes all logs and resets indexes.
func (ls *LogStorage) Clear() {
    ls.logs.Clear()

    ls.mu.Lock()
    ls.traceIndex = make(map[string][]*StoredLog)
    ls.severityIndex = make(map[string][]*StoredLog)
    ls.serviceIndex = make(map[string][]*StoredLog)
    ls.mu.Unlock()
}

// LogStorageStats contains statistics about log storage.
type LogStorageStats struct {
    LogCount     int            // Current number of logs stored
    Capacity     int            // Maximum number of logs
    TraceCount   int            // Number of distinct traces
    ServiceCount int            // Number of distinct services
    Severities   map[string]int // Severity → count
}

// extractLogBody extracts the string body from an AnyValue.
func extractLogBody(body *AnyValue) string {
    if body == nil {
        return ""
    }

    if sv := body.GetStringValue(); sv != "" {
        return sv
    }

    // For structured logs, convert to string representation
    // (simplified - could be more sophisticated)
    return fmt.Sprintf("%v", body)
}

func removeLogPosition(positions []*StoredLog, position int) []*StoredLog {
    // Note: This is comparing pointer addresses, not positions
    // May need to track positions differently
    result := make([]*StoredLog, 0, len(positions))
    for _, log := range positions {
        // This logic needs refinement based on how positions are tracked
        result = append(result, log)
    }
    return result
}
```

### Step 2: Create OTLP Logs Receiver

**File:** `internal/logsreceiver/receiver.go`

```go
package logsreceiver

import (
    "context"
    "fmt"
    "net"

    collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
    logspb "go.opentelemetry.io/proto/otlp/logs/v1"
    "google.golang.org/grpc"
)

// LogReceiver defines the interface for receiving logs.
type LogReceiver interface {
    ReceiveLogs(ctx context.Context, logs []*logspb.ResourceLogs) error
}

// Server implements the OTLP logs gRPC service.
type Server struct {
    collectorlogs.UnimplementedLogsServiceServer
    receiver LogReceiver
    grpcServer *grpc.Server
    listener   net.Listener
    endpoint   string
}

// Config holds configuration for the logs receiver.
type Config struct {
    Host string // Bind address (default: 127.0.0.1)
    Port int    // Port (0 for ephemeral)
}

// NewServer creates a new OTLP logs receiver.
func NewServer(cfg Config, receiver LogReceiver) (*Server, error) {
    if cfg.Host == "" {
        cfg.Host = "127.0.0.1"
    }

    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
    if err != nil {
        return nil, fmt.Errorf("failed to listen: %w", err)
    }

    grpcServer := grpc.NewServer()

    s := &Server{
        receiver:   receiver,
        grpcServer: grpcServer,
        listener:   listener,
        endpoint:   listener.Addr().String(),
    }

    collectorlogs.RegisterLogsServiceServer(grpcServer, s)

    return s, nil
}

// Export implements the OTLP logs service.
func (s *Server) Export(ctx context.Context, req *collectorlogs.ExportLogsServiceRequest) (*collectorlogs.ExportLogsServiceResponse, error) {
    if err := s.receiver.ReceiveLogs(ctx, req.ResourceLogs); err != nil {
        return nil, err
    }

    return &collectorlogs.ExportLogsServiceResponse{}, nil
}

// Start starts the gRPC server.
func (s *Server) Start(ctx context.Context) error {
    return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the server.
func (s *Server) Stop() {
    s.grpcServer.GracefulStop()
}

// Endpoint returns the server's endpoint address.
func (s *Server) Endpoint() string {
    return s.endpoint
}
```

### Step 3: Integration with Main Serve Command

**File:** `internal/cli/serve.go` (modify existing)

Add logs receiver alongside trace receiver:

```go
// In runServe function:

// Create log storage
logStorage := storage.NewLogStorage(50000) // 50K logs

// Create logs receiver
logsReceiver, err := logsreceiver.NewServer(
    logsreceiver.Config{
        Host: "127.0.0.1",
        Port: 0, // Use same ephemeral approach as traces
    },
    logStorage,
)
if err != nil {
    return fmt.Errorf("failed to create logs receiver: %w", err)
}

// Start logs receiver
go func() {
    if err := logsReceiver.Start(ctx); err != nil {
        log.Printf("Logs receiver stopped: %v", err)
    }
}()
defer logsReceiver.Stop()

// Pass logStorage to MCP server (for future query tools)
```

## Testing

### Unit Tests

**File:** `internal/storage/log_storage_test.go`

```go
func TestLogStorage(t *testing.T) {
    storage := NewLogStorage(100)

    // Test adding logs
    log1 := &StoredLog{
        TraceID:     "trace1",
        ServiceName: "svc1",
        Severity:    "ERROR",
        Body:        "Database connection failed",
    }
    storage.addLog(log1)

    // Test retrieval
    logs := storage.GetLogsByTraceID("trace1")
    if len(logs) != 1 {
        t.Errorf("expected 1 log, got %d", len(logs))
    }

    // Test severity filtering
    errorLogs := storage.GetLogsBySeverity("ERROR")
    if len(errorLogs) != 1 {
        t.Errorf("expected 1 error log, got %d", len(errorLogs))
    }
}

func TestLogStorageIndexCleanup(t *testing.T) {
    storage := NewLogStorage(3) // Small buffer

    // Add 3 logs
    for i := 0; i < 3; i++ {
        storage.addLog(&StoredLog{
            TraceID:     fmt.Sprintf("trace%d", i),
            ServiceName: "svc1",
            Severity:    "INFO",
        })
    }

    // Add 4th log (overwrites first)
    storage.addLog(&StoredLog{
        TraceID:     "trace3",
        ServiceName: "svc2",
        Severity:    "ERROR",
    })

    // trace0 should be removed from index
    logs := storage.GetLogsByTraceID("trace0")
    if len(logs) != 0 {
        t.Error("trace0 index should be cleaned up")
    }
}
```

### Integration Test

**File:** `test/logs_e2e_test.go`

```go
func TestLogsEndToEnd(t *testing.T) {
    // Create storage
    logStorage := storage.NewLogStorage(1000)

    // Create and start receiver
    receiver, err := logsreceiver.NewServer(
        logsreceiver.Config{Host: "127.0.0.1", Port: 0},
        logStorage,
    )
    if err != nil {
        t.Fatalf("failed to create receiver: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go receiver.Start(ctx)
    defer receiver.Stop()

    time.Sleep(100 * time.Millisecond)

    // Create gRPC client
    conn, err := grpc.NewClient(receiver.Endpoint(),
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatalf("failed to create client: %v", err)
    }
    defer conn.Close()

    client := collectorlogs.NewLogsServiceClient(conn)

    // Send test log
    _, err = client.Export(context.Background(), &collectorlogs.ExportLogsServiceRequest{
        ResourceLogs: []*logspb.ResourceLogs{
            {
                Resource: &resourcepb.Resource{
                    Attributes: []*commonpb.KeyValue{
                        {
                            Key: "service.name",
                            Value: &commonpb.AnyValue{
                                Value: &commonpb.AnyValue_StringValue{
                                    StringValue: "test-service",
                                },
                            },
                        },
                    },
                },
                ScopeLogs: []*logspb.ScopeLogs{
                    {
                        LogRecords: []*logspb.LogRecord{
                            {
                                TimeUnixNano:   uint64(time.Now().UnixNano()),
                                SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
                                SeverityText:   "ERROR",
                                Body: &commonpb.AnyValue{
                                    Value: &commonpb.AnyValue_StringValue{
                                        StringValue: "Test error message",
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
    })

    if err != nil {
        t.Fatalf("failed to export logs: %v", err)
    }

    time.Sleep(100 * time.Millisecond)

    // Verify storage
    logs := logStorage.GetRecentLogs(10)
    if len(logs) == 0 {
        t.Fatal("no logs found in storage")
    }

    if logs[0].Body != "Test error message" {
        t.Errorf("expected 'Test error message', got %q", logs[0].Body)
    }
}
```

## Definition of Done

- [ ] The **`LogStorage`** struct is created in **`internal/storage/log_storage.go`** with a ring buffer and indexes for `trace_id`, `severity`, and `service_name`.
- [ ] The **`NewLogStorage`** function correctly sets up the **`SetOnEvict`** callback to prevent index memory leaks.
- [ ] The **`removeFromIndexes`** function is implemented and correctly removes evicted logs from all indexes.
- [ ] The OTLP logs gRPC receiver is implemented in **`internal/logsreceiver/receiver.go`**.
- [ ] The `serve` command in **`internal/cli/serve.go`** is updated to initialize and start the logs receiver.
- [ ] Unit tests in **`internal/storage/log_storage_test.go`** are created and pass, including the **`TestLogStorageIndexCleanup`** test.
- [ ] An end-to-end integration test is created in **`test/logs_e2e_test.go`** and passes.
- [ ] The **`LogStorage.Stats()`** method is implemented and provides accurate counts.

## Files to Create

- `internal/storage/log_storage.go`
- `internal/storage/log_storage_test.go`
- `internal/logsreceiver/receiver.go`
- `internal/logsreceiver/receiver_test.go`
- `test/logs_e2e_test.go`

## Files to Modify

- `internal/cli/serve.go` - Add logs receiver startup
- `go.mod` - Already has logs proto dependencies

## Dependencies

All dependencies already in project from bootstrap:
- `go.opentelemetry.io/proto/otlp/logs/v1`
- `go.opentelemetry.io/proto/otlp/collector/logs/v1`
- `go.opentelemetry.io/proto/otlp/common/v1`
- `go.opentelemetry.io/proto/otlp/resource/v1`

## Estimated Effort

**2-3 hours** - Pattern is well-established from traces implementation

---

**Status:** Ready to implement
**Dependencies:** **Task 01: Storage Optimization** must be complete.
**Next:** **Task 03: Metrics Support**
