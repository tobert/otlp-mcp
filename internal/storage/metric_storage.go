package storage

import (
	"context"

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

// StoredMetric wraps a protobuf metric with extracted fields for filtering.
type StoredMetric struct {
	ResourceMetric *metricspb.ResourceMetrics
	ScopeMetric    *metricspb.ScopeMetrics
	Metric         *metricspb.Metric

	// Extracted fields for in-memory filtering
	MetricName     string
	ServiceName    string
	MetricType     MetricType
	Timestamp      uint64
	DataPointCount int

	// Summary data for quick stats
	NumericValue *float64
	Count        *uint64
	Sum          *float64
}

// MetricStorage stores OTLP metric data without content indexes.
// Queries use position-based ranges with in-memory filtering.
type MetricStorage struct {
	metrics *RingBuffer[*StoredMetric]
}

// NewMetricStorage creates a new metric storage with the specified capacity.
func NewMetricStorage(capacity int) *MetricStorage {
	return &MetricStorage{
		metrics: NewRingBuffer[*StoredMetric](capacity),
	}
}

// ReceiveMetrics stores received metric data.
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

				extractMetricSummary(stored)
				ms.metrics.Add(stored)
			}
		}
	}

	return nil
}

// addMetric adds a single metric to storage.
func (ms *MetricStorage) addMetric(metric *StoredMetric) {
	ms.metrics.Add(metric)
}

// GetRecentMetrics returns the N most recent metrics.
func (ms *MetricStorage) GetRecentMetrics(n int) []*StoredMetric {
	return ms.metrics.GetRecent(n)
}

// GetAllMetrics returns all stored metrics in chronological order.
func (ms *MetricStorage) GetAllMetrics() []*StoredMetric {
	return ms.metrics.GetAll()
}

// GetMetricsByName returns all currently stored metrics with the given name.
// This performs an in-memory scan.
func (ms *MetricStorage) GetMetricsByName(name string) []*StoredMetric {
	all := ms.metrics.GetAll()
	var result []*StoredMetric

	for _, metric := range all {
		if metric.MetricName == name {
			result = append(result, metric)
		}
	}

	return result
}

// GetMetricsByService returns all metrics for a given service.
// This performs an in-memory scan.
func (ms *MetricStorage) GetMetricsByService(serviceName string) []*StoredMetric {
	all := ms.metrics.GetAll()
	var result []*StoredMetric

	for _, metric := range all {
		if metric.ServiceName == serviceName {
			result = append(result, metric)
		}
	}

	return result
}

// GetMetricsByType returns all metrics of a specific type.
// This performs an in-memory scan.
func (ms *MetricStorage) GetMetricsByType(metricType MetricType) []*StoredMetric {
	all := ms.metrics.GetAll()
	var result []*StoredMetric

	for _, metric := range all {
		if metric.MetricType == metricType {
			result = append(result, metric)
		}
	}

	return result
}

// GetMetricNames returns all unique metric names currently in storage.
func (ms *MetricStorage) GetMetricNames() []string {
	all := ms.metrics.GetAll()
	nameSet := make(map[string]struct{})

	for _, metric := range all {
		nameSet[metric.MetricName] = struct{}{}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	return names
}

// GetRange returns metrics between start and end positions (inclusive).
// Positions are absolute and represent the logical sequence of metrics added.
func (ms *MetricStorage) GetRange(start, end int) []*StoredMetric {
	return ms.metrics.GetRange(start, end)
}

// CurrentPosition returns the current write position.
// Used by snapshots to bookmark a point in time.
func (ms *MetricStorage) CurrentPosition() int {
	return ms.metrics.CurrentPosition()
}

// Stats returns current storage statistics.
func (ms *MetricStorage) Stats() MetricStorageStats {
	all := ms.metrics.GetAll()

	// Count by scanning
	nameSet := make(map[string]struct{})
	serviceSet := make(map[string]struct{})
	typeCounts := make(map[string]int)
	totalDataPoints := 0

	for _, metric := range all {
		nameSet[metric.MetricName] = struct{}{}
		serviceSet[metric.ServiceName] = struct{}{}
		typeCounts[metric.MetricType.String()]++
		totalDataPoints += metric.DataPointCount
	}

	return MetricStorageStats{
		MetricCount:     ms.metrics.Size(),
		Capacity:        ms.metrics.Capacity(),
		UniqueNames:     len(nameSet),
		ServiceCount:    len(serviceSet),
		TypeCounts:      typeCounts,
		TotalDataPoints: totalDataPoints,
	}
}

// Clear removes all metrics.
func (ms *MetricStorage) Clear() {
	ms.metrics.Clear()
}

// MetricStorageStats contains statistics about metric storage.
type MetricStorageStats struct {
	MetricCount     int
	Capacity        int
	UniqueNames     int
	ServiceCount    int
	TypeCounts      map[string]int
	TotalDataPoints int
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
			stored.Sum = dp.Sum
		}

	case *metricspb.Metric_ExponentialHistogram:
		if len(data.ExponentialHistogram.DataPoints) > 0 {
			dp := data.ExponentialHistogram.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.ExponentialHistogram.DataPoints)
			stored.Count = &dp.Count
			stored.Sum = dp.Sum
		}

	case *metricspb.Metric_Summary:
		if len(data.Summary.DataPoints) > 0 {
			dp := data.Summary.DataPoints[0]
			stored.Timestamp = dp.TimeUnixNano
			stored.DataPointCount = len(data.Summary.DataPoints)
			stored.Count = &dp.Count
			sum := dp.Sum
			stored.Sum = &sum
		}
	}
}
