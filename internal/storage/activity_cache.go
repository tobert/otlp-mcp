package storage

import (
	"sync"
	"sync/atomic"
	"time"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// ActivityCache provides fast access to telemetry data for frequent polling.
// All counters use atomic operations for lock-free reads.
type ActivityCache struct {
	// Monotonic counters (never reset, wrap at uint64 max)
	spansReceived   atomic.Uint64
	logsReceived    atomic.Uint64
	metricsReceived atomic.Uint64

	// Generation counter for change detection
	// Incremented on any telemetry receipt
	generation atomic.Uint64

	// Recent errors ring buffer (separate from main log storage)
	recentErrors *RingBuffer[*ErrorEntry]

	// Recent traces tracking - deduplicated by service:rootSpan
	// This keeps one entry per unique service+rootSpan combo, showing the most recent
	recentTracesMu   sync.RWMutex
	recentTraces     map[string]*TraceEntry // key: "service:rootSpan"
	traceIDToKey     map[string]string      // traceID -> "service:rootSpan" for updates
	traceInsertOrder []string               // keys in insertion order for LRU eviction

	// Metric peek cache - keyed by metric name
	metricPeekMu   sync.RWMutex
	metricPeekData map[string]*MetricPeek

	// Subscriber notification for real-time streaming (e.g. WebSocket)
	subscriberMu     sync.Mutex
	subscribers      map[uint64]chan struct{}
	nextSubscriberID uint64

	// Start time for uptime calculation
	startTime time.Time
}

// ErrorEntry captures minimal error info for activity tracking.
type ErrorEntry struct {
	TraceID   string
	SpanID    string
	Service   string
	SpanName  string
	ErrorMsg  string
	Timestamp uint64 // Unix nano
}

// TraceEntry captures trace-level info for activity tracking.
type TraceEntry struct {
	TraceID    string
	Service    string
	RootSpan   string
	Status     string // OK, ERROR, UNSET
	DurationMs float64
	ErrorMsg   string // If failed
	Timestamp  uint64 // Start time
	SpanCount  int
	HasRoot    bool // True if we've seen the actual root span
}

// MetricPeek holds the current value(s) of a metric for quick access.
type MetricPeek struct {
	Name        string
	Type        MetricType
	LastUpdated uint64

	// For Gauge/Sum
	Value *float64

	// For Histogram
	Count       *uint64
	Sum         *float64
	Min         *float64
	Max         *float64
	Percentiles map[string]float64 // p50, p95, p99
}

const (
	// DefaultRecentErrorsCapacity is the number of recent errors to track.
	DefaultRecentErrorsCapacity = 100

	// DefaultRecentTracesCapacity is the number of recent traces to track.
	DefaultRecentTracesCapacity = 50
)

// NewActivityCache creates a new activity cache.
func NewActivityCache() *ActivityCache {
	return &ActivityCache{
		recentErrors:     NewRingBuffer[*ErrorEntry](DefaultRecentErrorsCapacity),
		recentTraces:     make(map[string]*TraceEntry),
		traceIDToKey:     make(map[string]string),
		traceInsertOrder: make([]string, 0, DefaultRecentTracesCapacity),
		metricPeekData:   make(map[string]*MetricPeek),
		subscribers:      make(map[uint64]chan struct{}),
		startTime:        time.Now(),
	}
}

// Subscribe returns a notification channel and an unsubscribe function.
// The channel receives a signal (non-blocking) whenever new telemetry arrives.
// The channel is buffered with capacity 1 to coalesce rapid updates.
func (h *ActivityCache) Subscribe() (<-chan struct{}, func()) {
	h.subscriberMu.Lock()
	defer h.subscriberMu.Unlock()

	id := h.nextSubscriberID
	h.nextSubscriberID++

	ch := make(chan struct{}, 1)
	h.subscribers[id] = ch

	unsubscribe := func() {
		h.subscriberMu.Lock()
		defer h.subscriberMu.Unlock()
		delete(h.subscribers, id)
	}

	return ch, unsubscribe
}

// notifySubscribers sends a non-blocking signal to all subscriber channels.
func (h *ActivityCache) notifySubscribers() {
	h.subscriberMu.Lock()
	defer h.subscriberMu.Unlock()

	for _, ch := range h.subscribers {
		select {
		case ch <- struct{}{}:
		default:
			// Channel already has a pending notification; skip to coalesce.
		}
	}
}

// RecordSpan records a span for activity tracking.
// Called from ObservabilityStorage when spans are received.
func (h *ActivityCache) RecordSpan(span *StoredSpan) {
	h.spansReceived.Add(1)
	h.generation.Add(1)

	// Check for errors
	if span.Span.Status != nil && span.Span.Status.Code == tracepb.Status_STATUS_CODE_ERROR {
		errorMsg := span.Span.Status.Message
		h.recentErrors.Add(&ErrorEntry{
			TraceID:   span.TraceID,
			SpanID:    span.SpanID,
			Service:   span.ServiceName,
			SpanName:  span.SpanName,
			ErrorMsg:  errorMsg,
			Timestamp: span.Span.StartTimeUnixNano,
		})
	}

	// Track trace-level info
	h.updateTraceEntry(span)

	h.notifySubscribers()
}

// updateTraceEntry updates or creates a trace entry.
// Deduplicates by service:rootSpan - if we see a new trace with the same
// service and root span name, we replace the old entry with the new one.
func (h *ActivityCache) updateTraceEntry(span *StoredSpan) {
	h.recentTracesMu.Lock()
	defer h.recentTracesMu.Unlock()

	traceID := span.TraceID
	isRoot := len(span.Span.ParentSpanId) == 0

	// Calculate status
	status := "UNSET"
	var errorMsg string
	if span.Span.Status != nil {
		switch span.Span.Status.Code {
		case tracepb.Status_STATUS_CODE_OK:
			status = "OK"
		case tracepb.Status_STATUS_CODE_ERROR:
			status = "ERROR"
			errorMsg = span.Span.Status.Message
		}
	}

	// Calculate duration
	durationNs := span.Span.EndTimeUnixNano - span.Span.StartTimeUnixNano
	durationMs := float64(durationNs) / 1_000_000.0

	// Check if this trace ID already has an entry (continuing an existing trace)
	if existingKey, exists := h.traceIDToKey[traceID]; exists {
		if entry, ok := h.recentTraces[existingKey]; ok {
			entry.SpanCount++
			// Update to root span if this is one and we haven't seen root yet
			if isRoot && !entry.HasRoot {
				// Received root span - re-key entry from child span name to root span name
				newKey := span.ServiceName + ":" + span.SpanName
				if newKey != existingKey {
					// Move entry to new key
					delete(h.recentTraces, existingKey)
					h.recentTraces[newKey] = entry
					h.traceIDToKey[traceID] = newKey
					// Update insert order
					h.updateInsertOrder(existingKey, newKey)
				}
				entry.RootSpan = span.SpanName
				entry.HasRoot = true
				entry.DurationMs = durationMs
				entry.Timestamp = span.Span.StartTimeUnixNano
			}
			// Propagate error status
			if status == "ERROR" {
				entry.Status = "ERROR"
				entry.ErrorMsg = errorMsg
			}
			return
		}
	}

	// New trace - create entry
	spanName := span.SpanName
	key := span.ServiceName + ":" + spanName

	// Check if we already have an entry with this service:spanName
	// If so, replace it (this is the deduplication)
	if existingEntry, exists := h.recentTraces[key]; exists {
		// Remove old trace ID mapping
		delete(h.traceIDToKey, existingEntry.TraceID)
	}

	entry := &TraceEntry{
		TraceID:    traceID,
		Service:    span.ServiceName,
		RootSpan:   spanName,
		Status:     status,
		DurationMs: durationMs,
		ErrorMsg:   errorMsg,
		Timestamp:  span.Span.StartTimeUnixNano,
		SpanCount:  1,
		HasRoot:    isRoot,
	}

	h.recentTraces[key] = entry
	h.traceIDToKey[traceID] = key

	// Update insert order (move to end if exists, add if new)
	h.updateInsertOrder("", key)

	// Evict oldest if over capacity
	h.evictOldestTraces()
}

// updateInsertOrder updates the insertion order tracking.
// If oldKey is non-empty, removes it. Adds newKey to the end.
func (h *ActivityCache) updateInsertOrder(oldKey, newKey string) {
	// Remove old key if present
	if oldKey != "" {
		for i, k := range h.traceInsertOrder {
			if k == oldKey {
				h.traceInsertOrder = append(h.traceInsertOrder[:i], h.traceInsertOrder[i+1:]...)
				break
			}
		}
	}

	// Remove newKey if it exists (we'll re-add at end)
	for i, k := range h.traceInsertOrder {
		if k == newKey {
			h.traceInsertOrder = append(h.traceInsertOrder[:i], h.traceInsertOrder[i+1:]...)
			break
		}
	}

	// Add to end
	h.traceInsertOrder = append(h.traceInsertOrder, newKey)
}

// evictOldestTraces removes oldest entries if over capacity.
func (h *ActivityCache) evictOldestTraces() {
	for len(h.traceInsertOrder) > DefaultRecentTracesCapacity {
		oldestKey := h.traceInsertOrder[0]
		h.traceInsertOrder = h.traceInsertOrder[1:]

		if entry, exists := h.recentTraces[oldestKey]; exists {
			delete(h.traceIDToKey, entry.TraceID)
			delete(h.recentTraces, oldestKey)
		}
	}
}

// RecordLog records a log entry for activity tracking.
func (h *ActivityCache) RecordLog() {
	h.logsReceived.Add(1)
	h.generation.Add(1)
	h.notifySubscribers()
}

// RecordMetric records a metric for activity tracking.
func (h *ActivityCache) RecordMetric(stored *StoredMetric) {
	h.metricsReceived.Add(1)
	h.generation.Add(1)

	// Update metric peek cache
	h.updateMetricPeek(stored)

	h.notifySubscribers()
}

// updateMetricPeek updates the peek cache for a metric.
func (h *ActivityCache) updateMetricPeek(stored *StoredMetric) {
	h.metricPeekMu.Lock()
	defer h.metricPeekMu.Unlock()

	peek := &MetricPeek{
		Name:        stored.MetricName,
		Type:        stored.MetricType,
		LastUpdated: stored.Timestamp,
		Value:       stored.NumericValue,
		Count:       stored.Count,
		Sum:         stored.Sum,
	}

	// Extract histogram percentiles if available
	if stored.MetricType == MetricTypeHistogram {
		if hist, ok := stored.Metric.Data.(*metricspb.Metric_Histogram); ok {
			if len(hist.Histogram.DataPoints) > 0 {
				dp := hist.Histogram.DataPoints[0]
				peek.Min = dp.Min
				peek.Max = dp.Max
				peek.Percentiles = ComputeHistogramPercentiles(dp)
			}
		}
	}

	h.metricPeekData[stored.MetricName] = peek
}

// SpansReceived returns the total number of spans received.
func (h *ActivityCache) SpansReceived() uint64 {
	return h.spansReceived.Load()
}

// LogsReceived returns the total number of logs received.
func (h *ActivityCache) LogsReceived() uint64 {
	return h.logsReceived.Load()
}

// MetricsReceived returns the total number of metrics received.
func (h *ActivityCache) MetricsReceived() uint64 {
	return h.metricsReceived.Load()
}

// Generation returns the current generation counter.
func (h *ActivityCache) Generation() uint64 {
	return h.generation.Load()
}

// RecentErrorCount returns the number of recent errors tracked.
func (h *ActivityCache) RecentErrorCount() int {
	return h.recentErrors.Size()
}

// UptimeSeconds returns the uptime in seconds.
func (h *ActivityCache) UptimeSeconds() float64 {
	return time.Since(h.startTime).Seconds()
}

// RecentErrors returns the N most recent errors.
func (h *ActivityCache) RecentErrors(n int) []*ErrorEntry {
	return h.recentErrors.GetRecent(n)
}

// RecentTraces returns the N most recent traces.
func (h *ActivityCache) RecentTraces(n int) []*TraceEntry {
	h.recentTracesMu.RLock()
	defer h.recentTracesMu.RUnlock()

	// Return the most recent n entries (from the end of insert order)
	count := len(h.traceInsertOrder)
	if n > count {
		n = count
	}
	if n == 0 {
		return nil
	}

	result := make([]*TraceEntry, n)
	for i := 0; i < n; i++ {
		key := h.traceInsertOrder[count-n+i]
		if entry, exists := h.recentTraces[key]; exists {
			// Return a copy to avoid data races
			entryCopy := *entry
			result[i] = &entryCopy
		}
	}

	return result
}

// PeekMetrics returns the current values for the specified metric names.
// Returns only metrics that exist in the cache. Limit to MaxMetricPeek names.
func (h *ActivityCache) PeekMetrics(names []string) []*MetricPeek {
	if len(names) == 0 {
		return nil
	}

	h.metricPeekMu.RLock()
	defer h.metricPeekMu.RUnlock()

	result := make([]*MetricPeek, 0, len(names))
	for _, name := range names {
		if peek, exists := h.metricPeekData[name]; exists {
			// Return a copy to avoid data races
			peekCopy := *peek
			if peek.Percentiles != nil {
				peekCopy.Percentiles = make(map[string]float64, len(peek.Percentiles))
				for k, v := range peek.Percentiles {
					peekCopy.Percentiles[k] = v
				}
			}
			result = append(result, &peekCopy)
		}
	}

	return result
}

// Clear resets all activity cache data.
func (h *ActivityCache) Clear() {
	h.spansReceived.Store(0)
	h.logsReceived.Store(0)
	h.metricsReceived.Store(0)
	h.generation.Store(0)
	h.recentErrors.Clear()

	h.recentTracesMu.Lock()
	h.recentTraces = make(map[string]*TraceEntry)
	h.traceIDToKey = make(map[string]string)
	h.traceInsertOrder = make([]string, 0, DefaultRecentTracesCapacity)
	h.recentTracesMu.Unlock()

	h.metricPeekMu.Lock()
	h.metricPeekData = make(map[string]*MetricPeek)
	h.metricPeekMu.Unlock()

	h.startTime = time.Now()
}
