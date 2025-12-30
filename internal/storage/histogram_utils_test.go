package storage

import (
	"math"
	"testing"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func TestComputeHistogramPercentiles(t *testing.T) {
	tests := []struct {
		name           string
		dataPoint      *metricspb.HistogramDataPoint
		expectNil      bool
		expectP50Range [2]float64 // [min, max] expected range
		expectP95Range [2]float64
		expectP99Range [2]float64
	}{
		{
			name:      "nil data point",
			dataPoint: nil,
			expectNil: true,
		},
		{
			name: "empty counts",
			dataPoint: &metricspb.HistogramDataPoint{
				Count:          0,
				ExplicitBounds: []float64{10, 50, 100},
				BucketCounts:   []uint64{},
			},
			expectNil: true,
		},
		{
			name: "zero total count",
			dataPoint: &metricspb.HistogramDataPoint{
				Count:          0,
				ExplicitBounds: []float64{10, 50, 100},
				BucketCounts:   []uint64{0, 0, 0, 0},
			},
			expectNil: true,
		},
		{
			name: "simple histogram - all in first bucket",
			dataPoint: &metricspb.HistogramDataPoint{
				Count:          100,
				ExplicitBounds: []float64{10, 50, 100},
				BucketCounts:   []uint64{100, 0, 0, 0}, // All values <= 10
			},
			expectNil:      false,
			expectP50Range: [2]float64{0, 10},
			expectP95Range: [2]float64{0, 10},
			expectP99Range: [2]float64{0, 10},
		},
		{
			name: "uniform distribution",
			dataPoint: &metricspb.HistogramDataPoint{
				Count:          100,
				ExplicitBounds: []float64{25, 50, 75, 100},
				BucketCounts:   []uint64{25, 25, 25, 25, 0}, // Even distribution
			},
			expectNil:      false,
			expectP50Range: [2]float64{25, 75}, // Should be around 50
			expectP95Range: [2]float64{75, 100},
			expectP99Range: [2]float64{75, 100},
		},
		{
			name: "realistic latency histogram",
			dataPoint: &metricspb.HistogramDataPoint{
				Count:          1000,
				ExplicitBounds: []float64{5, 10, 25, 50, 100, 250, 500, 1000}, // ms buckets
				BucketCounts:   []uint64{100, 300, 400, 150, 30, 15, 4, 1, 0},
			},
			expectNil:      false,
			expectP50Range: [2]float64{10, 25},  // Most values in 10-25ms bucket
			expectP95Range: [2]float64{50, 250}, // 95th percentile in higher buckets
			expectP99Range: [2]float64{100, 1000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeHistogramPercentiles(tt.dataPoint)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Check p50
			if p50, ok := result["p50"]; ok {
				if p50 < tt.expectP50Range[0] || p50 > tt.expectP50Range[1] {
					t.Errorf("p50 = %f, expected in range [%f, %f]", p50, tt.expectP50Range[0], tt.expectP50Range[1])
				}
			} else {
				t.Error("missing p50")
			}

			// Check p95
			if p95, ok := result["p95"]; ok {
				if p95 < tt.expectP95Range[0] || p95 > tt.expectP95Range[1] {
					t.Errorf("p95 = %f, expected in range [%f, %f]", p95, tt.expectP95Range[0], tt.expectP95Range[1])
				}
			} else {
				t.Error("missing p95")
			}

			// Check p99
			if p99, ok := result["p99"]; ok {
				if p99 < tt.expectP99Range[0] || p99 > tt.expectP99Range[1] {
					t.Errorf("p99 = %f, expected in range [%f, %f]", p99, tt.expectP99Range[0], tt.expectP99Range[1])
				}
			} else {
				t.Error("missing p99")
			}
		})
	}
}

func TestEstimatePercentileLinearInterpolation(t *testing.T) {
	// Test linear interpolation within a bucket
	bounds := []float64{100}
	counts := []uint64{100} // All 100 values in [0, 100]
	total := uint64(100)

	// p50 should be around 50 (linear interpolation)
	p50 := estimatePercentile(bounds, counts, total, 0.50)
	if math.Abs(p50-50) > 1 {
		t.Errorf("p50 = %f, expected ~50", p50)
	}

	// p90 should be around 90
	p90 := estimatePercentile(bounds, counts, total, 0.90)
	if math.Abs(p90-90) > 1 {
		t.Errorf("p90 = %f, expected ~90", p90)
	}
}

func TestComputeExponentialHistogramPercentiles(t *testing.T) {
	tests := []struct {
		name      string
		dataPoint *metricspb.ExponentialHistogramDataPoint
		expectNil bool
	}{
		{
			name:      "nil data point",
			dataPoint: nil,
			expectNil: true,
		},
		{
			name: "zero count",
			dataPoint: &metricspb.ExponentialHistogramDataPoint{
				Count: 0,
			},
			expectNil: true,
		},
		{
			name: "only zero bucket",
			dataPoint: &metricspb.ExponentialHistogramDataPoint{
				Count:         100,
				ZeroCount:     100,
				ZeroThreshold: 0.001,
				Scale:         2,
			},
			expectNil: false,
		},
		{
			name: "positive buckets",
			dataPoint: &metricspb.ExponentialHistogramDataPoint{
				Count: 100,
				Scale: 2,
				Positive: &metricspb.ExponentialHistogramDataPoint_Buckets{
					Offset:       0,
					BucketCounts: []uint64{25, 25, 25, 25},
				},
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeExponentialHistogramPercentiles(tt.dataPoint)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Just verify we got the expected keys
			for _, key := range []string{"p50", "p95", "p99"} {
				if _, ok := result[key]; !ok {
					t.Errorf("missing %s", key)
				}
			}
		})
	}
}

func TestPercentileOrderingProperty(t *testing.T) {
	// Property: p50 <= p95 <= p99 should always hold
	dataPoint := &metricspb.HistogramDataPoint{
		Count:          1000,
		ExplicitBounds: []float64{10, 25, 50, 100, 250, 500},
		BucketCounts:   []uint64{100, 200, 400, 200, 80, 15, 5},
	}

	result := ComputeHistogramPercentiles(dataPoint)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	p50 := result["p50"]
	p95 := result["p95"]
	p99 := result["p99"]

	if p50 > p95 {
		t.Errorf("p50 (%f) > p95 (%f), should be <=", p50, p95)
	}
	if p95 > p99 {
		t.Errorf("p95 (%f) > p99 (%f), should be <=", p95, p99)
	}
}

func TestOverflowBucket(t *testing.T) {
	// Test behavior when percentile falls in overflow bucket
	dataPoint := &metricspb.HistogramDataPoint{
		Count:          100,
		ExplicitBounds: []float64{10, 50},
		BucketCounts:   []uint64{5, 5, 90}, // 90% in overflow bucket (>50)
	}

	result := ComputeHistogramPercentiles(dataPoint)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// p99 should be at the last bound since we can't interpolate into infinity
	p99 := result["p99"]
	if p99 < 50 {
		t.Errorf("p99 = %f, expected >= 50 (last bound)", p99)
	}
}
