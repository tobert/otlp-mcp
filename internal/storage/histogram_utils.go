package storage

import (
	"math"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// ComputeHistogramPercentiles estimates percentiles from OTLP histogram bucket data.
// Uses linear interpolation within buckets (standard approach used by Prometheus).
// Returns nil if bucket data is not available or the histogram is empty.
func ComputeHistogramPercentiles(dp *metricspb.HistogramDataPoint) map[string]float64 {
	if dp == nil {
		return nil
	}

	bounds := dp.GetExplicitBounds()
	counts := dp.GetBucketCounts()
	total := dp.GetCount()

	// Need at least one bucket and non-zero count
	if len(counts) == 0 || total == 0 {
		return nil
	}

	// Compute percentiles
	percentiles := make(map[string]float64, 3)
	targets := map[string]float64{"p50": 0.50, "p95": 0.95, "p99": 0.99}

	for name, target := range targets {
		if p := estimatePercentile(bounds, counts, total, target); !math.IsNaN(p) {
			percentiles[name] = p
		}
	}

	if len(percentiles) == 0 {
		return nil
	}

	return percentiles
}

// estimatePercentile uses linear interpolation within buckets to estimate a percentile.
// This is the standard histogram_quantile approach used by Prometheus.
//
// Bucket layout in OTLP:
//   - counts[i] is the count for bucket i
//   - bounds[i] is the upper bound of bucket i (exclusive)
//   - bucket 0: (-Inf, bounds[0]]
//   - bucket i: (bounds[i-1], bounds[i]]
//   - bucket n: (bounds[n-1], +Inf)  (the overflow bucket)
//
// For n bounds, there are n+1 buckets.
func estimatePercentile(bounds []float64, counts []uint64, total uint64, target float64) float64 {
	targetCount := float64(total) * target
	cumulative := uint64(0)

	for i, count := range counts {
		cumulative += count

		if float64(cumulative) >= targetCount && count > 0 {
			// Found the bucket containing the percentile
			var lowerBound, upperBound float64

			// Determine bucket bounds
			if i == 0 {
				// First bucket: (-Inf, bounds[0]]
				// Use 0 as lower bound (common heuristic for latencies)
				lowerBound = 0
				if len(bounds) > 0 {
					upperBound = bounds[0]
				} else {
					// No bounds at all, can't interpolate
					return math.NaN()
				}
			} else if i < len(bounds) {
				// Middle bucket: (bounds[i-1], bounds[i]]
				lowerBound = bounds[i-1]
				upperBound = bounds[i]
			} else {
				// Overflow bucket: (bounds[n-1], +Inf)
				// Can't interpolate into infinity, return the last bound
				if len(bounds) > 0 {
					return bounds[len(bounds)-1]
				}
				return math.NaN()
			}

			// Linear interpolation within the bucket
			prevCumulative := cumulative - count
			fraction := (targetCount - float64(prevCumulative)) / float64(count)

			return lowerBound + fraction*(upperBound-lowerBound)
		}
	}

	// Should not reach here if total > 0 and target <= 1.0
	// Return the last bound as fallback
	if len(bounds) > 0 {
		return bounds[len(bounds)-1]
	}
	return math.NaN()
}

// ComputeExponentialHistogramPercentiles estimates percentiles from exponential histogram data.
// Exponential histograms use a base and scale to define bucket boundaries.
// Returns nil if the histogram is empty or has no buckets.
func ComputeExponentialHistogramPercentiles(dp *metricspb.ExponentialHistogramDataPoint) map[string]float64 {
	if dp == nil || dp.Count == 0 {
		return nil
	}

	// Exponential histogram bucket boundaries:
	// base = 2^(2^(-scale))
	// bucket[i] = base^i
	scale := dp.Scale
	base := math.Pow(2, math.Pow(2, float64(-scale)))

	// Collect all bucket counts with their boundaries
	type bucket struct {
		lower, upper float64
		count        uint64
	}
	var buckets []bucket

	// Zero bucket (if present)
	if dp.ZeroCount > 0 {
		// Zero bucket covers [-zeroThreshold, +zeroThreshold]
		threshold := dp.ZeroThreshold
		if threshold == 0 {
			threshold = math.SmallestNonzeroFloat64
		}
		buckets = append(buckets, bucket{-threshold, threshold, dp.ZeroCount})
	}

	// Negative buckets (from most negative to zero)
	if dp.Negative != nil {
		offset := dp.Negative.Offset
		for i, count := range dp.Negative.BucketCounts {
			if count > 0 {
				idx := offset + int32(i)
				upper := -math.Pow(base, float64(idx))
				lower := -math.Pow(base, float64(idx+1))
				buckets = append(buckets, bucket{lower, upper, count})
			}
		}
	}

	// Positive buckets (from zero to most positive)
	if dp.Positive != nil {
		offset := dp.Positive.Offset
		for i, count := range dp.Positive.BucketCounts {
			if count > 0 {
				idx := offset + int32(i)
				lower := math.Pow(base, float64(idx))
				upper := math.Pow(base, float64(idx+1))
				buckets = append(buckets, bucket{lower, upper, count})
			}
		}
	}

	if len(buckets) == 0 {
		return nil
	}

	// Sort buckets by lower bound (they should already be sorted, but be safe)
	// Using simple bubble sort since we expect few buckets
	for i := 0; i < len(buckets); i++ {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[i].lower > buckets[j].lower {
				buckets[i], buckets[j] = buckets[j], buckets[i]
			}
		}
	}

	// Compute percentiles using the same interpolation logic
	percentiles := make(map[string]float64, 3)
	targets := map[string]float64{"p50": 0.50, "p95": 0.95, "p99": 0.99}

	total := dp.Count
	for name, target := range targets {
		targetCount := float64(total) * target
		cumulative := uint64(0)

		for _, b := range buckets {
			cumulative += b.count

			if float64(cumulative) >= targetCount && b.count > 0 {
				prevCumulative := cumulative - b.count
				fraction := (targetCount - float64(prevCumulative)) / float64(b.count)
				p := b.lower + fraction*(b.upper-b.lower)
				if !math.IsNaN(p) && !math.IsInf(p, 0) {
					percentiles[name] = p
				}
				break
			}
		}
	}

	if len(percentiles) == 0 {
		return nil
	}

	return percentiles
}
