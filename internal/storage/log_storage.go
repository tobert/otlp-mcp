package storage

import (
	"context"
	"fmt"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// StoredLog wraps a protobuf log record with extracted fields for filtering.
type StoredLog struct {
	ResourceLog *logspb.ResourceLogs
	ScopeLog    *logspb.ScopeLogs
	LogRecord   *logspb.LogRecord

	// Extracted fields for in-memory filtering
	TraceID     string
	SpanID      string
	ServiceName string
	Severity    string
	SeverityNum int32
	Body        string
	Timestamp   uint64
}

// LogStorage stores OTLP log records without content indexes.
// Queries use position-based ranges with in-memory filtering.
type LogStorage struct {
	logs *RingBuffer[*StoredLog]
}

// NewLogStorage creates a new log storage with the specified capacity.
func NewLogStorage(capacity int) *LogStorage {
	return &LogStorage{
		logs: NewRingBuffer[*StoredLog](capacity),
	}
}

// ReceiveLogs stores received log records.
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

				ls.logs.Add(stored)
			}
		}
	}

	return nil
}

// GetRecentLogs returns the N most recent logs.
func (ls *LogStorage) GetRecentLogs(n int) []*StoredLog {
	return ls.logs.GetRecent(n)
}

// GetAllLogs returns all stored logs in chronological order.
func (ls *LogStorage) GetAllLogs() []*StoredLog {
	return ls.logs.GetAll()
}

// GetLogsByTraceID returns all currently stored logs for a given trace ID.
// This performs an in-memory scan.
func (ls *LogStorage) GetLogsByTraceID(traceID string) []*StoredLog {
	all := ls.logs.GetAll()
	var result []*StoredLog

	for _, log := range all {
		if log.TraceID == traceID {
			result = append(result, log)
		}
	}

	return result
}

// GetLogsBySeverity returns all logs matching a severity level.
// This performs an in-memory scan.
func (ls *LogStorage) GetLogsBySeverity(severity string) []*StoredLog {
	all := ls.logs.GetAll()
	var result []*StoredLog

	for _, log := range all {
		if log.Severity == severity {
			result = append(result, log)
		}
	}

	return result
}

// GetLogsByService returns all logs for a given service.
// This performs an in-memory scan.
func (ls *LogStorage) GetLogsByService(serviceName string) []*StoredLog {
	all := ls.logs.GetAll()
	var result []*StoredLog

	for _, log := range all {
		if log.ServiceName == serviceName {
			result = append(result, log)
		}
	}

	return result
}

// GetRange returns logs between start and end positions (inclusive).
// Positions are absolute and represent the logical sequence of logs added.
func (ls *LogStorage) GetRange(start, end int) []*StoredLog {
	return ls.logs.GetRange(start, end)
}

// CurrentPosition returns the current write position.
// Used by snapshots to bookmark a point in time.
func (ls *LogStorage) CurrentPosition() int {
	return ls.logs.CurrentPosition()
}

// Stats returns current storage statistics.
func (ls *LogStorage) Stats() LogStorageStats {
	all := ls.logs.GetAll()

	// Count unique traces and severities by scanning
	traceIDs := make(map[string]struct{})
	serviceNames := make(map[string]struct{})
	severities := make(map[string]int)

	for _, log := range all {
		if log.TraceID != "" {
			traceIDs[log.TraceID] = struct{}{}
		}
		serviceNames[log.ServiceName] = struct{}{}
		severities[log.Severity]++
	}

	return LogStorageStats{
		LogCount:     ls.logs.Size(),
		Capacity:     ls.logs.Capacity(),
		TraceCount:   len(traceIDs),
		ServiceCount: len(serviceNames),
		Severities:   severities,
	}
}

// Clear removes all logs.
func (ls *LogStorage) Clear() {
	ls.logs.Clear()
}

// LogStorageStats contains statistics about log storage.
type LogStorageStats struct {
	LogCount     int
	Capacity     int
	TraceCount   int
	ServiceCount int
	Severities   map[string]int
}

// extractLogBody extracts the string body from an AnyValue.
func extractLogBody(body *commonpb.AnyValue) string {
	if body == nil {
		return ""
	}

	if sv := body.GetStringValue(); sv != "" {
		return sv
	}

	// For structured logs, convert to string representation
	return fmt.Sprintf("%v", body)
}
