# Task 09: Integration & Testing

> ⚠️ **UPDATED FOR SNAPSHOT-FIRST APPROACH**
>
> Testing now focuses on the 5 snapshot-based tools instead of 26 signal-specific tools.
>
> **See [SNAPSHOT-FIRST-PLAN.md](./SNAPSHOT-FIRST-PLAN.md) for the new approach.**

## Overview

Comprehensive integration testing for the **snapshot-first observability system**, validating that all signals (traces, logs, metrics) work together through the 5 unified tools.

**Dependencies:** All previous tasks (01-08) must be complete

## Testing Strategy

### Test Levels

1. **Unit Tests** - Already covered in individual tasks
2. **Integration Tests** - End-to-end signal flows (this task)
3. **Multi-Signal Tests** - Cross-signal correlation
4. **Snapshot Workflow Tests** - Real agent workflows
5. **Performance Tests** - High volume scenarios
6. **Memory Tests** - Leak detection and optimization validation

---

## Integration Test Suite

### Test 1: All Signals End-to-End

Validate complete pipeline for traces, logs, and metrics.

**File:** `test/all_signals_e2e_test.go`

```go
package test

import (
    "context"
    "testing"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    collectortraces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
    collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"

    "otlp-mcp/internal/storage"
    "otlp-mcp/internal/otlpreceiver"
    "otlp-mcp/internal/logsreceiver"
    "otlp-mcp/internal/metricsreceiver"
)

func TestAllSignalsEndToEnd(t *testing.T) {
    // Create storages
    traceStorage := storage.NewTraceStorage(1000)
    logStorage := storage.NewLogStorage(1000)
    metricStorage := storage.NewMetricStorage(1000)

    // Create receivers
    traceReceiver, err := otlpreceiver.NewServer(
        otlpreceiver.Config{Host: "127.0.0.1", Port: 0},
        traceStorage,
    )
    if err != nil {
        t.Fatalf("failed to create trace receiver: %v", err)
    }

    logReceiver, err := logsreceiver.NewServer(
        logsreceiver.Config{Host: "127.0.0.1", Port: 0},
        logStorage,
    )
    if err != nil {
        t.Fatalf("failed to create log receiver: %v", err)
    }

    metricReceiver, err := metricsreceiver.NewServer(
        metricsreceiver.Config{Host: "127.0.0.1", Port: 0},
        metricStorage,
    )
    if err != nil {
        t.Fatalf("failed to create metric receiver: %v", err)
    }

    // Start receivers
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go traceReceiver.Start(ctx)
    go logReceiver.Start(ctx)
    go metricReceiver.Start(ctx)

    defer traceReceiver.Stop()
    defer logReceiver.Stop()
    defer metricReceiver.Stop()

    time.Sleep(100 * time.Millisecond)

    // Create clients
    traceConn, _ := grpc.NewClient(traceReceiver.Endpoint(),
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    defer traceConn.Close()

    logConn, _ := grpc.NewClient(logReceiver.Endpoint(),
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    defer logConn.Close()

    metricConn, _ := grpc.NewClient(metricReceiver.Endpoint(),
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    defer metricConn.Close()

    traceClient := collectortraces.NewTraceServiceClient(traceConn)
    logClient := collectorlogs.NewLogsServiceClient(logConn)
    metricClient := collectormetrics.NewMetricsServiceClient(metricConn)

    // Send test data with same trace_id and service
    traceID := generateTraceID()
    serviceName := "test-service"

    // 1. Send trace
    _, err = traceClient.Export(ctx, createTestTraceRequest(traceID, serviceName))
    if err != nil {
        t.Fatalf("failed to export trace: %v", err)
    }

    // 2. Send logs (with trace_id)
    _, err = logClient.Export(ctx, createTestLogRequest(traceID, serviceName))
    if err != nil {
        t.Fatalf("failed to export logs: %v", err)
    }

    // 3. Send metrics
    _, err = metricClient.Export(ctx, createTestMetricRequest(serviceName))
    if err != nil {
        t.Fatalf("failed to export metrics: %v", err)
    }

    time.Sleep(100 * time.Millisecond)

    // Verify all signals stored
    traceStats := traceStorage.Stats()
    logStats := logStorage.Stats()
    metricStats := metricStorage.Stats()

    if traceStats.SpanCount == 0 {
        t.Error("no traces stored")
    }
    if logStats.LogCount == 0 {
        t.Error("no logs stored")
    }
    if metricStats.MetricCount == 0 {
        t.Error("no metrics stored")
    }

    // Verify correlation
    logs := logStorage.GetLogsByTraceID(traceID)
    if len(logs) == 0 {
        t.Error("logs not correlated with trace")
    }

    t.Logf("✅ All signals working: %d traces, %d logs, %d metrics",
        traceStats.SpanCount, logStats.LogCount, metricStats.MetricCount)
}
```

---

### Test 2: Snapshot Workflow

Test the complete snapshot workflow from creation to analysis.

**File:** `test/snapshot_workflow_test.go`

```go
package test

import (
    "testing"
    "time"

    "otlp-mcp/internal/storage"
)

func TestSnapshotWorkflow(t *testing.T) {
    // Setup storages
    traceStorage := storage.NewTraceStorage(1000)
    logStorage := storage.NewLogStorage(1000)
    metricStorage := storage.NewMetricStorage(1000)
    snapshotManager := storage.NewSnapshotManager()

    // Phase 1: Baseline
    addTestSpans(traceStorage, 10, "baseline-service")
    addTestLogs(logStorage, 20, "baseline-service", "")
    addTestMetrics(metricStorage, 30, "baseline-service")

    // Create "before" snapshot
    snapshot1 := snapshotManager.Create(
        "before-test",
        traceStorage.CurrentPosition(),
        logStorage.CurrentPosition(),
        metricStorage.CurrentPosition(),
    )

    if snapshot1 == nil {
        t.Fatal("failed to create snapshot")
    }

    t.Logf("Created snapshot 'before-test' at positions: traces=%d, logs=%d, metrics=%d",
        snapshot1.TracePos, snapshot1.LogPos, snapshot1.MetricPos)

    time.Sleep(10 * time.Millisecond)

    // Phase 2: Test run
    addTestSpans(traceStorage, 5, "test-service")
    addTestLogs(logStorage, 10, "test-service", "")
    addTestMetrics(metricStorage, 15, "test-service")

    // Create "after" snapshot
    snapshot2 := snapshotManager.Create(
        "after-test",
        traceStorage.CurrentPosition(),
        logStorage.CurrentPosition(),
        metricStorage.CurrentPosition(),
    )

    t.Logf("Created snapshot 'after-test' at positions: traces=%d, logs=%d, metrics=%d",
        snapshot2.TracePos, snapshot2.LogPos, snapshot2.MetricPos)

    // Verify snapshots exist
    snapshots := snapshotManager.List()
    if len(snapshots) != 2 {
        t.Errorf("expected 2 snapshots, got %d", len(snapshots))
    }

    // Get data between snapshots
    traces := traceStorage.GetRange(snapshot1.TracePos, snapshot2.TracePos)
    logs := logStorage.GetRange(snapshot1.LogPos, snapshot2.LogPos)
    metrics := metricStorage.GetRange(snapshot1.MetricPos, snapshot2.MetricPos)

    if len(traces) != 5 {
        t.Errorf("expected 5 traces from test run, got %d", len(traces))
    }
    if len(logs) != 10 {
        t.Errorf("expected 10 logs from test run, got %d", len(logs))
    }
    if len(metrics) != 15 {
        t.Errorf("expected 15 metrics from test run, got %d", len(metrics))
    }

    // Delete snapshot
    deleted := snapshotManager.Delete("before-test")
    if !deleted {
        t.Error("failed to delete snapshot")
    }

    t.Log("✅ Snapshot workflow complete")
}
```

---

### Test 3: MCP Tools Integration

Test all MCP tools with real data.

**File:** `test/mcp_tools_integration_test.go`

```go
package test

import (
    "context"
    "testing"

    "otlp-mcp/internal/mcpserver"
    "otlp-mcp/internal/storage"
)

func TestMCPToolsIntegration(t *testing.T) {
    // Setup
    traceStorage := storage.NewTraceStorage(1000)
    logStorage := storage.NewLogStorage(1000)
    metricStorage := storage.NewMetricStorage(1000)
    snapshotManager := storage.NewSnapshotManager()

    // Populate with test data
    traceID := generateTraceID()
    service := "api-service"

    addTestSpansWithTraceID(traceStorage, 3, service, traceID)
    addTestLogsWithTraceID(logStorage, 5, service, traceID)
    addTestMetrics(metricStorage, 10, service)

    // Create MCP server
    server, err := mcpserver.NewServer(mcpserver.Config{
        TraceStorage:    traceStorage,
        LogStorage:      logStorage,
        MetricStorage:   metricStorage,
        SnapshotManager: snapshotManager,
        TraceEndpoint:   "localhost:1234",
        LogEndpoint:     "localhost:1235",
        MetricEndpoint:  "localhost:1236",
    })
    if err != nil {
        t.Fatalf("failed to create MCP server: %v", err)
    }

    ctx := context.Background()

    // Test log tools
    t.Run("get_recent_logs", func(t *testing.T) {
        result, err := server.CallTool(ctx, "get_recent_logs", map[string]interface{}{
            "limit": 10,
        })
        if err != nil {
            t.Fatalf("tool failed: %v", err)
        }

        logs, ok := result.(map[string]interface{})["logs"]
        if !ok || len(logs.([]interface{})) == 0 {
            t.Error("no logs returned")
        }
    })

    t.Run("get_logs_for_trace", func(t *testing.T) {
        result, err := server.CallTool(ctx, "get_logs_for_trace", map[string]interface{}{
            "trace_id": traceID,
            "include_spans": true,
        })
        if err != nil {
            t.Fatalf("tool failed: %v", err)
        }

        data := result.(map[string]interface{})
        logs := data["logs"].([]interface{})
        spans := data["spans"].([]interface{})

        if len(logs) != 5 {
            t.Errorf("expected 5 logs, got %d", len(logs))
        }
        if len(spans) != 3 {
            t.Errorf("expected 3 spans, got %d", len(spans))
        }
    })

    t.Run("get_timeline", func(t *testing.T) {
        result, err := server.CallTool(ctx, "get_timeline", map[string]interface{}{
            "service_name": service,
        })
        if err != nil {
            t.Fatalf("tool failed: %v", err)
        }

        data := result.(map[string]interface{})
        timeline := data["timeline"].([]interface{})
        counts := data["counts"].(map[string]interface{})

        if len(timeline) == 0 {
            t.Error("timeline is empty")
        }

        if counts["spans"].(int) != 3 {
            t.Errorf("expected 3 spans in timeline, got %d", counts["spans"])
        }
    })

    t.Run("snapshot_tools", func(t *testing.T) {
        // Create snapshot
        result, err := server.CallTool(ctx, "create_snapshot", map[string]interface{}{
            "name": "test-snapshot",
        })
        if err != nil {
            t.Fatalf("create_snapshot failed: %v", err)
        }

        // List snapshots
        result, err = server.CallTool(ctx, "list_snapshots", map[string]interface{}{})
        if err != nil {
            t.Fatalf("list_snapshots failed: %v", err)
        }

        data := result.(map[string]interface{})
        if data["count"].(int) != 1 {
            t.Error("snapshot not created")
        }

        // Delete snapshot
        result, err = server.CallTool(ctx, "delete_snapshot", map[string]interface{}{
            "name": "test-snapshot",
        })
        if err != nil {
            t.Fatalf("delete_snapshot failed: %v", err)
        }
    })

    t.Log("✅ All MCP tools working")
}
```

---

### Test 4: High Volume Stress Test

Test performance with high data volumes.

**File:** `test/high_volume_test.go`

```go
package test

import (
    "testing"
    "time"

    "otlp-mcp/internal/storage"
)

func TestHighVolumeStress(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping stress test in short mode")
    }

    // Large buffers
    traceStorage := storage.NewTraceStorage(10000)
    logStorage := storage.NewLogStorage(50000)
    metricStorage := storage.NewMetricStorage(100000)

    t.Run("fill_buffers", func(t *testing.T) {
        start := time.Now()

        // Add traces
        for i := 0; i < 10000; i++ {
            addTestSpans(traceStorage, 1, "service-1")
        }

        // Add logs
        for i := 0; i < 50000; i++ {
            addTestLogs(logStorage, 1, "service-1", "")
        }

        // Add metrics
        for i := 0; i < 100000; i++ {
            addTestMetrics(metricStorage, 1, "service-1")
        }

        duration := time.Since(start)
        t.Logf("Filled buffers in %v", duration)

        // Verify
        if traceStorage.Stats().SpanCount != 10000 {
            t.Error("trace buffer not filled correctly")
        }
        if logStorage.Stats().LogCount != 50000 {
            t.Error("log buffer not filled correctly")
        }
        if metricStorage.Stats().MetricCount != 100000 {
            t.Error("metric buffer not filled correctly")
        }
    })

    t.Run("query_performance", func(t *testing.T) {
        start := time.Now()

        // Query operations
        _ = traceStorage.GetRecentSpans(1000)
        _ = logStorage.GetRecentLogs(1000)
        _ = metricStorage.GetRecentMetrics(1000)

        duration := time.Since(start)
        t.Logf("Query operations took %v", duration)

        if duration > 100*time.Millisecond {
            t.Errorf("queries too slow: %v", duration)
        }
    })

    t.Run("index_integrity", func(t *testing.T) {
        // Verify indexes are clean (no memory leak)
        traceStats := traceStorage.Stats()
        logStats := logStorage.Stats()
        metricStats := metricStorage.Stats()

        t.Logf("Trace index: %d services, %d traces",
            traceStats.ServiceCount, traceStats.TraceCount)
        t.Logf("Log index: %d services, %d traces, %d severities",
            logStats.ServiceCount, logStats.TraceCount, len(logStats.Severities))
        t.Logf("Metric index: %d services, %d names",
            metricStats.ServiceCount, metricStats.UniqueNames)

        // Indexes should not exceed buffer size significantly
        if traceStats.TraceCount > 20000 {
            t.Error("trace index not cleaned up properly")
        }
    })

    t.Log("✅ High volume stress test passed")
}
```

---

### Test 5: Memory Leak Verification

Validate that index cleanup prevents memory leaks.

**File:** `test/memory_leak_test.go`

```go
package test

import (
    "runtime"
    "testing"

    "otlp-mcp/internal/storage"
)

func TestMemoryLeakPrevention(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping memory test in short mode")
    }

    // Small buffer to force wraparound
    storage := storage.NewLogStorage(100)

    runtime.GC()
    var m1 runtime.MemStats
    runtime.ReadMemStats(&m1)

    // Add 10x buffer capacity (force wraparound 10 times)
    for i := 0; i < 1000; i++ {
        addTestLogs(storage, 1, "service-1", "")
    }

    runtime.GC()
    var m2 runtime.MemStats
    runtime.ReadMemStats(&m2)

    allocDiff := m2.Alloc - m1.Alloc
    t.Logf("Memory used: %d bytes", allocDiff)

    // Verify buffer size
    stats := storage.Stats()
    if stats.LogCount != 100 {
        t.Errorf("buffer size incorrect: expected 100, got %d", stats.LogCount)
    }

    // Verify index cleanup (should not have 1000 entries)
    if stats.ServiceCount > 10 {
        t.Errorf("index not cleaned up: %d services", stats.ServiceCount)
    }

    // Memory should be bounded
    if allocDiff > 10*1024*1024 { // 10 MB limit for 100 items
        t.Errorf("potential memory leak: %d bytes allocated", allocDiff)
    }

    t.Log("✅ No memory leaks detected")
}
```

---

## Test Helper Functions

**File:** `test/helpers.go`

```go
package test

import (
    "crypto/rand"
    "encoding/hex"
    "time"

    "otlp-mcp/internal/storage"
)

func generateTraceID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func generateSpanID() string {
    b := make([]byte, 8)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func addTestSpans(ts *storage.TraceStorage, count int, serviceName string) {
    for i := 0; i < count; i++ {
        span := &storage.Span{
            TraceID:     generateTraceID(),
            SpanID:      generateSpanID(),
            ServiceName: serviceName,
            Name:        "test-span",
            StartTimeUnixNano: uint64(time.Now().UnixNano()),
            DurationNanos: 1000000,
        }
        ts.AddSpan(span)
    }
}

func addTestSpansWithTraceID(ts *storage.TraceStorage, count int, serviceName, traceID string) {
    for i := 0; i < count; i++ {
        span := &storage.Span{
            TraceID:     traceID,
            SpanID:      generateSpanID(),
            ServiceName: serviceName,
            Name:        "test-span",
            StartTimeUnixNano: uint64(time.Now().UnixNano()),
            DurationNanos: 1000000,
        }
        ts.AddSpan(span)
    }
}

func addTestLogs(ls *storage.LogStorage, count int, serviceName, traceID string) {
    for i := 0; i < count; i++ {
        log := &storage.StoredLog{
            TraceID:     traceID,
            ServiceName: serviceName,
            Severity:    "INFO",
            Body:        "Test log message",
            Timestamp:   uint64(time.Now().UnixNano()),
        }
        ls.addLog(log)
    }
}

func addTestLogsWithTraceID(ls *storage.LogStorage, count int, serviceName, traceID string) {
    addTestLogs(ls, count, serviceName, traceID)
}

func addTestMetrics(ms *storage.MetricStorage, count int, serviceName string) {
    for i := 0; i < count; i++ {
        value := float64(i * 100)
        metric := &storage.StoredMetric{
            MetricName:  "test_metric",
            ServiceName: serviceName,
            MetricType:  storage.MetricTypeGauge,
            Timestamp:   uint64(time.Now().UnixNano()),
            NumericValue: &value,
        }
        ms.addMetric(metric)
    }
}
```

---

## Running Tests

```bash
# Run all tests
go test ./test/... -v

# Run only fast tests
go test ./test/... -short

# Run specific test
go test ./test -run TestAllSignalsEndToEnd

# Run with coverage
go test ./test/... -cover

# Stress tests
go test ./test -run TestHighVolume

# Memory tests
go test ./test -run TestMemoryLeak
```

---

## Acceptance Criteria

- [ ] All signals end-to-end test passing
- [ ] Snapshot workflow test passing
- [ ] All MCP tools tested with integration tests
- [ ] High volume stress test passing (10K traces, 50K logs, 100K metrics)
- [ ] Memory leak test passing (index cleanup verified)
- [ ] Performance benchmarks met (queries < 100ms)
- [ ] Index integrity maintained under load
- [ ] Buffer wraparound working correctly
- [ ] Correlation working across all signals
- [ ] Error handling tested for edge cases

## Success Metrics

- **Throughput**: Handle 1000+ spans/sec, 5000+ logs/sec, 10000+ metrics/sec
- **Latency**: Query response < 100ms for 1000 items
- **Memory**: < 100 MB for full buffers (10K+50K+100K items)
- **Index cleanup**: No unbounded growth over 10x buffer wraparound
- **Tool reliability**: 100% success rate for all MCP tools

---

**Status:** Ready to implement
**Dependencies:** All tasks 01-08 complete
**Next:** Task 10 (Documentation)
