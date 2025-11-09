# Task 03: Metrics Support

## Overview

Implement OTLP metrics gRPC endpoint and **`MetricStorage`** to receive, store, and query metric data from instrumented applications.

**Pattern:** Follow the same architecture as logs (Task 02) and traces (bootstrap).
- OTLP gRPC receiver for metrics.
- Ring buffer storage with indexing.
- Integration with main `serve` command.

## Prerequisite

- **Task 01: Storage Optimization** must be complete. This task relies on the **`SetOnEvict`** callback pattern established in **`internal/storage/ringbuffer.go`** to prevent memory leaks.

## Goals

1. Accept OTLP metric data via gRPC.
2. Store metrics in a ring buffer (100,000 data point capacity).
3. Index by `metric name`, `service name`, and `metric type`.
4. Handle all 5 OTLP metric types (Gauge, Sum, Histogram, ExponentialHistogram, Summary).
5. Prepare for MCP query tools (Task 05).

## OpenTelemetry Metrics Specifications

**OTLP Metrics Spec:**
- Protocol: https://opentelemetry.io/docs/specs/otel/metrics/
- Data Model: https://opentelemetry.io/docs/specs/otel/metrics/data-model/
- Proto: `go.opentelemetry.io/proto/otlp/metrics/v1`

**Key Metric Types:**

```go
import (
    metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
    collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// Metric data model (from proto)
type Metric struct {
    Name        string
    Description string
    Unit        string

    // One of these will be set:
    Gauge                *Gauge
    Sum                  *Sum
    Histogram            *Histogram
    ExponentialHistogram *ExponentialHistogram
    Summary              *Summary
}

// Gauge - instantaneous measurement
type Gauge struct {
    DataPoints []*NumberDataPoint
}

// Sum - cumulative or delta value
type Sum struct {
    DataPoints              []*NumberDataPoint
    AggregationTemporality  AggregationTemporality  // CUMULATIVE or DELTA
    IsMonotonic             bool                    // Counter vs UpDownCounter
}

// Histogram - distribution of values
type Histogram struct {
    DataPoints              []*HistogramDataPoint
    AggregationTemporality  AggregationTemporality
}

// ExponentialHistogram - high-resolution distribution
type ExponentialHistogram struct {
    DataPoints              []*ExponentialHistogramDataPoint
    AggregationTemporality  AggregationTemporality
}

// Summary - pre-aggregated quantiles (legacy, used by Prometheus)
type Summary struct {
    DataPoints []*SummaryDataPoint
}

// NumberDataPoint - used by Gauge and Sum
type NumberDataPoint struct {
    Attributes        []*KeyValue
    StartTimeUnixNano uint64
    TimeUnixNano      uint64
    // One of:
    AsDouble          float64
    AsInt             int64

    Exemplars         []*Exemplar
    Flags             uint32
}

// HistogramDataPoint - used by Histogram
type HistogramDataPoint struct {
    Attributes         []*KeyValue
    StartTimeUnixNano  uint64
    TimeUnixNano       uint64
    Count              uint64
    Sum                *float64
    BucketCounts       []uint64
    ExplicitBounds     []float64
    Exemplars          []*Exemplar
    Flags              uint32
    Min                *float64
    Max                *float64
}
```

**Metric Type Summary:**

| Type | Use Case | Example |
|------|----------|---------|
| **Gauge** | Current value at measurement time | `memory_usage_bytes`, `temperature_celsius` |
| **Sum (monotonic)** | Cumulative counter that only increases | `http_requests_total`, `bytes_sent_total` |
| **Sum (non-monotonic)** | Value that can increase/decrease | `queue_size`, `active_connections` |
| **Histogram** | Distribution (latency, sizes) | `request_duration_seconds`, `response_size_bytes` |
| **ExponentialHistogram** | High-res distribution (fewer buckets) | `rpc_duration_ms` (with auto-scaling buckets) |
| **Summary** | Pre-calculated quantiles (Prometheus) | `request_duration_summary` (p50, p90, p99) |

## Implementation

### Step 1: Create MetricStorage

**File:** `internal/storage/metric_storage.go`

```go
package storage

import (
	"context"
	"fmt"
	"sync"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// MetricType represents the type of metric.
type MetricType int

const (
	MetricTypeUnknown MetricType = iota
	MetricTypeGauge
	MetricTypeSum
	MetricTypeHistogram
	MetricTypeExponentialHistogram
	MetricTypeSummary
)

func (mt MetricType) String() string {
	switch mt {
	case MetricTypeGauge:
		return "Gauge"
	case MetricTypeSum:
		return "Sum"
	case MetricTypeHistogram:
		return "Histogram"
	case MetricTypeExponentialHistogram:
		return "ExponentialHistogram"
	case MetricTypeSummary:
		return "Summary"
	default:
		return "Unknown"
	}
}

// StoredMetric wraps a protobuf metric with indexed fields.
type StoredMetric struct {
	ResourceMetric *metricspb.ResourceMetrics
	ScopeMetric    *metricspb.ScopeMetrics
	Metric         *metricspb.Metric

	// Indexed fields for fast lookup
	MetricName   string
	ServiceName  string
	MetricType   MetricType
	Timestamp    uint64 // TimeUnixNano from first data point
	DataPointCount int  // Number of data points in this metric

	// Type-specific summary data for quick stats
	NumericValue *float64  // For Gauge/Sum single values
	Count        *uint64   // For Histogram/Summary
	Sum          *float64  // For Histogram/Summary
}

// MetricStorage stores and indexes OTLP metric data.
type MetricStorage struct {
	metrics       *RingBuffer[*StoredMetric]
	nameIndex     map[string][]*StoredMetric // metric_name → metrics
	serviceIndex  map[string][]*StoredMetric // service → metrics
	typeIndex     map[MetricType][]*StoredMetric // type → metrics
	mu            sync.RWMutex
}

// NewMetricStorage creates a new metric storage with the specified capacity.
func NewMetricStorage(capacity int) *MetricStorage {
	ms := &MetricStorage{
		metrics:      NewRingBuffer[*StoredMetric](capacity),
		nameIndex:    make(map[string][]*StoredMetric),
		serviceIndex: make(map[string][]*StoredMetric),
		typeIndex:    make(map[MetricType][]*StoredMetric),
	}

	// PATTERN: Use the SetOnEvict callback to ensure indexes are cleaned up
    // when the ring buffer overwrites old data. This prevents memory leaks.
    // This pattern is established in Task 01.
	ms.metrics.SetOnEvict(func(position int, oldMetric *StoredMetric) {
		ms.removeFromIndexes(position, oldMetric)
	})

	return ms
}

// ReceiveMetrics implements the metric receiver interface.
func (ms *MetricStorage) ReceiveMetrics(ctx context.Context, resourceMetrics []*metricspb.ResourceMetrics) error {
	for _, rm := range resourceMetrics {
		serviceName := extractServiceName(rm.Resource)

		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				stored := &StoredMetric{
					ResourceMetric: rm,
					ScopeMetric:    sm,
					Metric:         metric,
					MetricName:     metric.Name,
					ServiceName:    serviceName,
					MetricType:     determineMetricType(metric),
				}

				// Extract summary data based on metric type
				extractMetricSummary(stored)

				ms.addMetric(stored)
			}
		}
	}

	return nil
}

// determineMetricType identifies the metric type from the proto message.
func determineMetricType(metric *metricspb.Metric) MetricType {
	switch metric.Data.(type) {
	case *metricspb.Metric_Gauge:
		return MetricTypeGauge
	case *metricspb.Metric_Sum:
		return MetricTypeSum
	case *metricspb.Metric_Histogram:
		return MetricTypeHistogram
	case *metricspb.Metric_ExponentialHistogram:
		return MetricTypeExponentialHistogram
	case *metricspb.Metric_Summary:
		return MetricTypeSummary
	default:
		return MetricTypeUnknown
	}
}

// extractMetricSummary populates summary fields for quick access.
func extractMetricSummary(stored *StoredMetric) {
	metric := stored.Metric

	switch data := metric.Data.(type) {
	case *metricspb.Metric_Gauge:
		if len(data.Gauge.DataPoints) > 0 {
			dp := data.Gauge.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.Gauge.DataPoints)

			// Extract numeric value
			if val := dp.GetAsDouble(); val != 0 {
				stored.NumericValue = &val
			} else if intVal := dp.GetAsInt(); intVal != 0 {
				floatVal := float64(intVal)
				stored.NumericValue = &floatVal
			}
		}

	case *metricspb.Metric_Sum:
		if len(data.Sum.DataPoints) > 0 {
			dp := data.Sum.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.Sum.DataPoints)

			// Extract numeric value
			if val := dp.GetAsDouble(); val != 0 {
				stored.NumericValue = &val
			} else if intVal := dp.GetAsInt(); intVal != 0 {
				floatVal := float64(intVal)
				stored.NumericValue = &floatVal
			}
		}

	case *metricspb.Metric_Histogram:
		if len(data.Histogram.DataPoints) > 0 {
			dp := data.Histogram.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.Histogram.DataPoints)
			stored.Count = &dp.Count
			if dp.Sum != nil {
				stored.Sum = dp.Sum
			}
		}

	case *metricspb.Metric_ExponentialHistogram:
		if len(data.ExponentialHistogram.DataPoints) > 0 {
			dp := data.ExponentialHistogram.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.ExponentialHistogram.DataPoints)
			stored.Count = &dp.Count
			if dp.Sum != nil {
				stored.Sum = dp.Sum
			}
		}

	case *metricspb.Metric_Summary:
		if len(data.Summary.DataPoints) > 0 {
			dp := data.Summary.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.Summary.DataPoints)
			stored.Count = &dp.Count
			if dp.Sum != nil {
				stored.Sum = dp.Sum
			}
		}
	}
}

// addMetric adds a metric to storage and updates indexes.
func (ms *MetricStorage) addMetric(metric *StoredMetric) {
	ms.metrics.Add(metric)

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Update name index
	ms.nameIndex[metric.MetricName] = append(ms.nameIndex[metric.MetricName], metric)

	// Update service index
	ms.serviceIndex[metric.ServiceName] = append(ms.serviceIndex[metric.ServiceName], metric)

	// Update type index
	ms.typeIndex[metric.MetricType] = append(ms.typeIndex[metric.MetricType], metric)
}

// removeFromIndexes cleans up index entries when a metric is evicted.
func (ms *MetricStorage) removeFromIndexes(position int, oldMetric *StoredMetric) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Remove from name index
	metrics := ms.nameIndex[oldMetric.MetricName]
	ms.nameIndex[oldMetric.MetricName] = removeMetricFromSlice(metrics, oldMetric)
	if len(ms.nameIndex[oldMetric.MetricName]) == 0 {
		delete(ms.nameIndex, oldMetric.MetricName)
	}

	// Remove from service index
	metrics = ms.serviceIndex[oldMetric.ServiceName]
	ms.serviceIndex[oldMetric.ServiceName] = removeMetricFromSlice(metrics, oldMetric)
	if len(ms.serviceIndex[oldMetric.ServiceName]) == 0 {
		delete(ms.serviceIndex, oldMetric.ServiceName)
	}

	// Remove from type index
	metrics = ms.typeIndex[oldMetric.MetricType]
	ms.typeIndex[oldMetric.MetricType] = removeMetricFromSlice(metrics, oldMetric)
	if len(ms.typeIndex[oldMetric.MetricType]) == 0 {
		delete(ms.typeIndex, oldMetric.MetricType)
	}
}

// GetRecentMetrics returns the N most recent metrics.
func (ms *MetricStorage) GetRecentMetrics(n int) []*StoredMetric {
	return ms.metrics.GetRecent(n)
}

// GetMetricsByName returns all metrics with the given name.
func (ms *MetricStorage) GetMetricsByName(name string) []*StoredMetric {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metrics := ms.nameIndex[name]
	if len(metrics) == 0 {
		return nil
	}

	result := make([]*StoredMetric, len(metrics))
	copy(result, metrics)
	return result
}

// GetMetricsByService returns all metrics for a given service.
func (ms *MetricStorage) GetMetricsByService(serviceName string) []*StoredMetric {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metrics := ms.serviceIndex[serviceName]
	if len(metrics) == 0 {
		return nil
	}

	result := make([]*StoredMetric, len(metrics))
	copy(result, metrics)
	return result
}

// GetMetricsByType returns all metrics of a specific type.
func (ms *MetricStorage) GetMetricsByType(metricType MetricType) []*StoredMetric {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metrics := ms.typeIndex[metricType]
	if len(metrics) == 0 {
		return nil
	}

	result := make([]*StoredMetric, len(metrics))
	copy(result, metrics)
	return result
}

// GetMetricNames returns all unique metric names currently in storage.
func (ms *MetricStorage) GetMetricNames() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	names := make([]string, 0, len(ms.nameIndex))
	for name := range ms.nameIndex {
		names = append(names, name)
	}
	return names
}

// Stats returns current storage statistics.
func (ms *MetricStorage) Stats() MetricStorageStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Count by type
	typeCounts := make(map[string]int)
	for metricType, metrics := range ms.typeIndex {
		typeCounts[metricType.String()] = len(metrics)
	}

	// Calculate total data points
	totalDataPoints := 0
	allMetrics := ms.metrics.GetRecent(ms.metrics.Size())
	for _, m := range allMetrics {
		totalDataPoints += m.DataPointCount
	}

	return MetricStorageStats{
		MetricCount:      ms.metrics.Size(),
		Capacity:         ms.metrics.Capacity(),
		UniqueNames:      len(ms.nameIndex),
		ServiceCount:     len(ms.serviceIndex),
		TypeCounts:       typeCounts,
		TotalDataPoints:  totalDataPoints,
	}
}

// Clear removes all metrics and resets indexes.
func (ms *MetricStorage) Clear() {
	ms.metrics.Clear()

	ms.mu.Lock()
	ms.nameIndex = make(map[string][]*StoredMetric)
	ms.serviceIndex = make(map[string][]*StoredMetric)
	ms.typeIndex = make(map[MetricType][]*StoredMetric)
	ms.mu.Unlock()
}

// MetricStorageStats contains statistics about metric storage.
type MetricStorageStats struct {
	MetricCount     int            // Current number of metrics stored
	Capacity        int            // Maximum number of metrics
	UniqueNames     int            // Number of unique metric names
	ServiceCount    int            // Number of distinct services
	TypeCounts      map[string]int // Type → count
	TotalDataPoints int            // Sum of all data points across metrics
}

func removeMetricFromSlice(metrics []*StoredMetric, toRemove *StoredMetric) []*StoredMetric {
	result := make([]*StoredMetric, 0, len(metrics))
	for _, m := range metrics {
		if m != toRemove {
			result = append(result, m)
		}
	}
	return result
}
```

### Step 2: Create OTLP Metrics Receiver

**File:** `internal/metricsreceiver/receiver.go`

```go
package metricsreceiver

import (
	"context"
	"fmt"
	"net"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/grpc"
)

// MetricReceiver defines the interface for receiving metrics.
type MetricReceiver interface {
	ReceiveMetrics(ctx context.Context, metrics []*metricspb.ResourceMetrics) error
}

// Server implements the OTLP metrics gRPC service.
type Server struct {
	collectormetrics.UnimplementedMetricsServiceServer
	receiver   MetricReceiver
	grpcServer *grpc.Server
	listener   net.Listener
	endpoint   string
}

// Config holds configuration for the metrics receiver.
type Config struct {
	Host string // Bind address (default: 127.0.0.1)
	Port int    // Port (0 for ephemeral)
}

// NewServer creates a new OTLP metrics receiver.
func NewServer(cfg Config, receiver MetricReceiver) (*Server, error) {
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

	collectormetrics.RegisterMetricsServiceServer(grpcServer, s)

	return s, nil
}

// Export implements the OTLP metrics service.
func (s *Server) Export(ctx context.Context, req *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	if err := s.receiver.ReceiveMetrics(ctx, req.ResourceMetrics); err != nil {
		return nil, err
	}

	return &collectormetrics.ExportMetricsServiceResponse{}, nil
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

Add metrics receiver alongside trace and log receivers:

```go
// In runServe function:

// Create metric storage
metricStorage := storage.NewMetricStorage(100000) // 100K metrics

// Create metrics receiver
metricsReceiver, err := metricsreceiver.NewServer(
	metricsreceiver.Config{
		Host: "127.0.0.1",
		Port: 0, // Ephemeral port
	},
	metricStorage,
)
if err != nil {
	return fmt.Errorf("failed to create metrics receiver: %w", err)
}

// Start metrics receiver
go func() {
	if err := metricsReceiver.Start(ctx); err != nil {
		log.Printf("Metrics receiver stopped: %v", err)
	}
}()
defer metricsReceiver.Stop()

// Update MCP server initialization to include all storages
mcpServer := mcpserver.NewServer(mcpserver.Config{
	TraceStorage:  traceStorage,
	LogStorage:    logStorage,
	MetricStorage: metricStorage,
	TraceEndpoint: traceReceiver.Endpoint(),
	LogEndpoint:   logsReceiver.Endpoint(),
	MetricEndpoint: metricsReceiver.Endpoint(),
})
```

## Testing

### Unit Tests

**File:** `internal/storage/metric_storage_test.go`

```go
package storage

import (
	"testing"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func TestMetricStorage_AddAndRetrieve(t *testing.T) {
	storage := NewMetricStorage(100)

	// Test Gauge metric
	gaugeMetric := &StoredMetric{
		MetricName:  "memory_usage",
		ServiceName: "test-service",
		MetricType:  MetricTypeGauge,
		Metric: &metricspb.Metric{
			Name: "memory_usage",
			Data: &metricspb.Metric_Gauge{
				Gauge: &metricspb.Gauge{
					DataPoints: []*metricspb.NumberDataPoint{
						{
							TimeUnixNano: 1234567890,
							Value: &metricspb.NumberDataPoint_AsInt{
								AsInt: 1024,
							},
						},
					},
				},
			},
		},
	}
	extractMetricSummary(gaugeMetric)
	storage.addMetric(gaugeMetric)

	// Test retrieval by name
	metrics := storage.GetMetricsByName("memory_usage")
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}

	// Test retrieval by type
	gauges := storage.GetMetricsByType(MetricTypeGauge)
	if len(gauges) != 1 {
		t.Errorf("expected 1 gauge, got %d", len(gauges))
	}
}

func TestMetricStorage_MultipleTypes(t *testing.T) {
	storage := NewMetricStorage(100)

	// Add different metric types
	metrics := []*StoredMetric{
		{MetricName: "gauge1", ServiceName: "svc1", MetricType: MetricTypeGauge},
		{MetricName: "counter1", ServiceName: "svc1", MetricType: MetricTypeSum},
		{MetricName: "histogram1", ServiceName: "svc2", MetricType: MetricTypeHistogram},
	}

	for _, m := range metrics {
		storage.addMetric(m)
	}

	// Verify type index
	if len(storage.GetMetricsByType(MetricTypeGauge)) != 1 {
		t.Error("expected 1 gauge")
	}
	if len(storage.GetMetricsByType(MetricTypeSum)) != 1 {
		t.Error("expected 1 sum")
	}
	if len(storage.GetMetricsByType(MetricTypeHistogram)) != 1 {
		t.Error("expected 1 histogram")
	}

	// Verify service index
	if len(storage.GetMetricsByService("svc1")) != 2 {
		t.Error("expected 2 metrics for svc1")
	}
}

func TestMetricStorage_IndexCleanup(t *testing.T) {
	storage := NewMetricStorage(3) // Small buffer to force eviction

	// Add 3 metrics
	for i := 0; i < 3; i++ {
		storage.addMetric(&StoredMetric{
			MetricName:  fmt.Sprintf("metric%d", i),
			ServiceName: "svc1",
			MetricType:  MetricTypeGauge,
		})
	}

	// Add 4th metric (evicts first)
	storage.addMetric(&StoredMetric{
		MetricName:  "metric3",
		ServiceName: "svc2",
		MetricType:  MetricTypeSum,
	})

	// metric0 should be removed from index
	metrics := storage.GetMetricsByName("metric0")
	if len(metrics) != 0 {
		t.Error("metric0 index should be cleaned up")
	}

	// metric1-3 should still exist
	if storage.metrics.Size() != 3 {
		t.Errorf("expected 3 metrics, got %d", storage.metrics.Size())
	}
}

func TestMetricStorage_Stats(t *testing.T) {
	storage := NewMetricStorage(100)

	// Add various metrics
	storage.addMetric(&StoredMetric{
		MetricName:     "gauge1",
		ServiceName:    "svc1",
		MetricType:     MetricTypeGauge,
		DataPointCount: 1,
	})
	storage.addMetric(&StoredMetric{
		MetricName:     "histogram1",
		ServiceName:    "svc1",
		MetricType:     MetricTypeHistogram,
		DataPointCount: 5,
	})

	stats := storage.Stats()

	if stats.MetricCount != 2 {
		t.Errorf("expected 2 metrics, got %d", stats.MetricCount)
	}
	if stats.UniqueNames != 2 {
		t.Errorf("expected 2 unique names, got %d", stats.UniqueNames)
	}
	if stats.TotalDataPoints != 6 {
		t.Errorf("expected 6 data points, got %d", stats.TotalDataPoints)
	}
	if stats.TypeCounts["Gauge"] != 1 {
		t.Errorf("expected 1 gauge, got %d", stats.TypeCounts["Gauge"])
	}
}
```

### Integration Test

**File:** `test/metrics_e2e_test.go`

```go
package test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"otlp-mcp/internal/metricsreceiver"
	"otlp-mcp/internal/storage"
)

func TestMetricsEndToEnd(t *testing.T) {
	// Create storage
	metricStorage := storage.NewMetricStorage(1000)

	// Create and start receiver
	receiver, err := metricsreceiver.NewServer(
		metricsreceiver.Config{Host: "127.0.0.1", Port: 0},
		metricStorage,
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

	client := collectormetrics.NewMetricsServiceClient(conn)

	// Send test metrics (Gauge, Sum, Histogram)
	_, err = client.Export(context.Background(), &collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
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
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							// Gauge
							{
								Name:        "memory_usage_bytes",
								Description: "Current memory usage",
								Unit:        "bytes",
								Data: &metricspb.Metric_Gauge{
									Gauge: &metricspb.Gauge{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: uint64(time.Now().UnixNano()),
												Value: &metricspb.NumberDataPoint_AsInt{
													AsInt: 1024000,
												},
											},
										},
									},
								},
							},
							// Sum (Counter)
							{
								Name:        "requests_total",
								Description: "Total requests",
								Unit:        "1",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
										IsMonotonic:            true,
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: uint64(time.Now().UnixNano()),
												Value: &metricspb.NumberDataPoint_AsInt{
													AsInt: 42,
												},
											},
										},
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
		t.Fatalf("failed to export metrics: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify storage
	metrics := metricStorage.GetRecentMetrics(10)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	// Verify gauge
	gaugeMetrics := metricStorage.GetMetricsByName("memory_usage_bytes")
	if len(gaugeMetrics) != 1 {
		t.Errorf("expected 1 gauge metric, got %d", len(gaugeMetrics))
	}

	// Verify counter
	counterMetrics := metricStorage.GetMetricsByName("requests_total")
	if len(counterMetrics) != 1 {
		t.Errorf("expected 1 counter metric, got %d", len(counterMetrics))
	}

	// Verify stats
	stats := metricStorage.Stats()
	if stats.UniqueNames != 2 {
		t.Errorf("expected 2 unique names, got %d", stats.UniqueNames)
	}
}
```

## Definition of Done

- [ ] The **`MetricStorage`** struct is created in **`internal/storage/metric_storage.go`** with a ring buffer and indexes for `metric_name`, `service_name`, and `metric_type`.
- [ ] The **`NewMetricStorage`** function correctly sets up the **`SetOnEvict`** callback to prevent index memory leaks.
- [ ] The **`removeFromIndexes`** function is implemented and correctly removes evicted metrics from all indexes.
- [ ] The OTLP metrics gRPC receiver is implemented in **`internal/metricsreceiver/receiver.go`**.
- [ ] The `serve` command in **`internal/cli/serve.go`** is updated to initialize and start the metrics receiver.
- [ ] Unit tests in **`internal/storage/metric_storage_test.go`** are created and pass, including the **`TestMetricStorageIndexCleanup`** test.
- [ ] An end-to-end integration test is created in **`test/metrics_e2e_test.go`** and passes.
- [ ] The **`MetricStorage.Stats()`** method is implemented and provides accurate counts.
- [ ] The implementation correctly handles all 5 OTLP metric types.

## Files to Create

- `internal/storage/metric_storage.go`
- `internal/storage/metric_storage_test.go`
- `internal/metricsreceiver/receiver.go`
- `internal/metricsreceiver/receiver_test.go`
- `test/metrics_e2e_test.go`

## Files to Modify

- `internal/cli/serve.go` - Add metrics receiver startup
- `internal/mcpserver/server.go` - Add metric storage reference
- `go.mod` - Already has metrics proto dependencies

## Dependencies

All dependencies already in project from bootstrap:
- `go.opentelemetry.io/proto/otlp/metrics/v1`
- `go.opentelemetry.io/proto/otlp/collector/metrics/v1`
- `go.opentelemetry.io/proto/otlp/common/v1`
- `go.opentelemetry.io/proto/otlp/resource/v1`

## Estimated Effort

**3-4 hours** - More complex than logs due to 5 different metric types, but pattern is well-established

## Notes

**Metric Type Complexity:**
- Metrics are more complex than logs/traces due to multiple types.
- Each type has different data point structures.
- Summary extraction helps with quick queries without parsing full proto.
- Histogram and ExponentialHistogram require special handling for buckets.

**Performance Considerations:**
- 100K capacity chosen because metrics are high-volume but compact.
- Index by name is critical (agents will query by metric name most often).
- Type index enables "show me all gauges" queries.
- Data point counting important for understanding buffer utilization.

---

**Status:** Ready to implement
**Dependencies:** **Task 01: Storage Optimization** must be complete.
**Next:** **Task 04: MCP Log Tools**
