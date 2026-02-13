package viz

// SpanInfo is the input type for trace waterfall rendering.
// Decoupled from storage types so viz is a pure rendering package.
type SpanInfo struct {
	TraceID     string
	SpanID      string
	ParentID    string // Empty = root span
	ServiceName string
	SpanName    string
	StartNano   uint64
	EndNano     uint64
	StatusCode  string // "OK", "ERROR", "UNSET"
}

// ServiceStats describes one service for the service summary bar chart.
type ServiceStats struct {
	Name       string
	SpanCount  int
	ErrorCount int
}

// BufferStats describes buffer fill levels for the stats overview.
type BufferStats struct {
	SpanCount      int
	SpanCapacity   int
	LogCount       int
	LogCapacity    int
	MetricCount    int
	MetricCapacity int
	SnapshotCount  int
}

// ActivityTrace describes one recent trace for the activity table.
type ActivityTrace struct {
	TraceID    string
	Service    string
	RootSpan   string
	Status     string
	DurationMs float64
	ErrorMsg   string
}

// ActivityError describes one recent error for the activity table.
type ActivityError struct {
	TraceID   string
	Service   string
	SpanName  string
	ErrorMsg  string
	Timestamp uint64
}
