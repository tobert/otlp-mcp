package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/tobert/otlp-mcp/internal/storage"
)

//go:embed static/index.html
var staticFiles embed.FS

// Server serves the embedded web UI and WebSocket updates.
type Server struct {
	storage        *storage.ObservabilityStorage
	originPatterns []string // host patterns for websocket.AcceptOptions.OriginPatterns
}

// New creates a new web UI server.
// allowedOrigins are URI patterns like "http://localhost:*"; schemes are stripped
// for the websocket library which matches on host only.
func New(s *storage.ObservabilityStorage, allowedOrigins []string) *Server {
	patterns := buildOriginPatterns(allowedOrigins)
	return &Server{storage: s, originPatterns: patterns}
}

// buildOriginPatterns converts config-style origins (e.g. "http://localhost:*")
// to host-only patterns (e.g. "localhost:*") for coder/websocket.
// If no origins are provided, defaults to localhost-only.
func buildOriginPatterns(origins []string) []string {
	if len(origins) == 0 {
		return []string{"localhost:*", "127.0.0.1:*"}
	}
	patterns := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
		} else {
			// Already a bare host pattern or unparseable — pass through
			patterns = append(patterns, o)
		}
	}
	return patterns
}

// securityHeaders wraps a handler to set security response headers.
func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:")
		next(w, r)
	}
}

// RegisterRoutes attaches web UI routes to an existing ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ui/", securityHeaders(s.handleUI))
	mux.HandleFunc("GET /ui", securityHeaders(s.handleUIRedirect))
	mux.HandleFunc("GET /api/services", securityHeaders(s.handleServices))
	mux.HandleFunc("GET /api/status", securityHeaders(s.handleStatus))
	mux.HandleFunc("GET /api/query", securityHeaders(s.handleQuery))
	mux.HandleFunc("GET /ws", s.handleWebSocket)
}

// ListenAndServe starts a standalone HTTP server for the web UI.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// handleUIRedirect redirects /ui to /ui/ for consistent routing.
func (s *Server) handleUIRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
}

// handleUI serves the embedded index.html.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "UI not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleServices returns the list of known service names.
func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	services := s.storage.Services()
	writeJSON(w, services)
}

// statusResponse is the JSON shape for /api/status.
type statusResponse struct {
	Generation uint64  `json:"generation"`
	Spans      uint64  `json:"spans"`
	Logs       uint64  `json:"logs"`
	Metrics    uint64  `json:"metrics"`
	Uptime     float64 `json:"uptime_seconds"`
}

// handleStatus returns generation counter, signal counts, and uptime.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ac := s.storage.ActivityCache()
	writeJSON(w, statusResponse{
		Generation: ac.Generation(),
		Spans:      ac.SpansReceived(),
		Logs:       ac.LogsReceived(),
		Metrics:    ac.MetricsReceived(),
		Uptime:     ac.UptimeSeconds(),
	})
}

// handleQuery performs a filtered query against storage.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := storage.QueryFilter{
		ServiceName: q.Get("service"),
		LogSeverity: q.Get("severity"),
		SpanName:    q.Get("span_name"),
		TraceID:     q.Get("trace_id"),
		SpanStatus:  q.Get("span_status"),
	}

	if q.Get("errors_only") == "true" {
		filter.ErrorsOnly = true
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			filter.Limit = n
		}
	}

	result, err := s.storage.Query(filter)
	if err != nil {
		log.Printf("webui: query error: %v", err)
		http.Error(w, "invalid query parameters", http.StatusBadRequest)
		return
	}

	writeJSON(w, result)
}

// wsFilter is the client-sent filter message on the WebSocket.
type wsFilter struct {
	Service  string `json:"service"`
	Severity string `json:"severity"`
	Paused   bool   `json:"paused"`
}

// wsUpdate is the server-sent update message on the WebSocket.
type wsUpdate struct {
	Generation uint64            `json:"generation"`
	Counters   wsCounters        `json:"counters"`
	Traces     []wsSpanSummary   `json:"traces,omitempty"`
	Logs       []wsLogSummary    `json:"logs,omitempty"`
	Metrics    []wsMetricSummary `json:"metrics,omitempty"`
}

type wsCounters struct {
	Spans   uint64 `json:"spans"`
	Logs    uint64 `json:"logs"`
	Metrics uint64 `json:"metrics"`
}

type wsSpanSummary struct {
	Time         string  `json:"time"`
	TraceID      string  `json:"trace_id"`
	SpanID       string  `json:"span_id"`
	ParentSpanID string  `json:"parent_span_id,omitempty"`
	Service      string  `json:"service"`
	SpanName     string  `json:"span_name"`
	Kind         string  `json:"kind"`
	DurationMs   float64 `json:"duration_ms"`
	Status       string  `json:"status"`
	StartNs      uint64  `json:"start_ns"`
	EndNs        uint64  `json:"end_ns"`
}

type wsLogSummary struct {
	Time     string `json:"time"`
	Service  string `json:"service"`
	Severity string `json:"severity"`
	Body     string `json:"body"`
}

type wsMetricSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Service string `json:"service"`
	Value   string `json:"value"`
	Updated string `json:"updated"`
}

// handleWebSocket upgrades to WebSocket and streams real-time updates.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.originPatterns,
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)

	ctx := r.Context()

	// Subscribe to storage notifications
	notifyCh, unsubscribe := s.storage.ActivityCache().Subscribe()
	defer unsubscribe()

	// Track positions for delta reads — back up to include recent history on connect
	const backfillTraces, backfillLogs, backfillMetrics = 50, 100, 50
	lastTracePos := max(0, s.storage.Traces().CurrentPosition()-backfillTraces)
	lastLogPos := max(0, s.storage.Logs().CurrentPosition()-backfillLogs)
	lastMetricPos := max(0, s.storage.Metrics().CurrentPosition()-backfillMetrics)

	// Current filter (initially empty = show all)
	var filter wsFilter

	// Read filter messages from client in a goroutine
	filterCh := make(chan wsFilter, 4)
	go func() {
		defer close(filterCh)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var f wsFilter
			if json.Unmarshal(data, &f) == nil {
				select {
				case filterCh <- f:
				default:
				}
			}
		}
	}()

	// Send initial status immediately
	s.sendWSUpdate(ctx, conn, &lastTracePos, &lastLogPos, &lastMetricPos, filter)

	// Keepalive ticker (send status even with no data changes, so client knows we're alive)
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "server shutting down")
			return

		case f, ok := <-filterCh:
			if !ok {
				// Client disconnected
				return
			}
			filter = f

		case <-notifyCh:
			if filter.Paused {
				continue
			}
			s.sendWSUpdate(ctx, conn, &lastTracePos, &lastLogPos, &lastMetricPos, filter)

		case <-keepalive.C:
			if filter.Paused {
				continue
			}
			s.sendWSUpdate(ctx, conn, &lastTracePos, &lastLogPos, &lastMetricPos, filter)
		}
	}
}

// sendWSUpdate reads deltas from ring buffers and sends a JSON update over WebSocket.
func (s *Server) sendWSUpdate(ctx context.Context, conn *websocket.Conn,
	lastTracePos, lastLogPos, lastMetricPos *int, filter wsFilter) {

	ac := s.storage.ActivityCache()

	curTracePos := s.storage.Traces().CurrentPosition()
	curLogPos := s.storage.Logs().CurrentPosition()
	curMetricPos := s.storage.Metrics().CurrentPosition()

	update := wsUpdate{
		Generation: ac.Generation(),
		Counters: wsCounters{
			Spans:   ac.SpansReceived(),
			Logs:    ac.LogsReceived(),
			Metrics: ac.MetricsReceived(),
		},
	}

	// Get trace deltas
	if curTracePos > *lastTracePos {
		spans := s.storage.Traces().GetRange(*lastTracePos, curTracePos-1)
		for _, span := range spans {
			if span.Span == nil {
				continue
			}
			if filter.Service != "" && span.ServiceName != filter.Service {
				continue
			}
			durationNs := span.Span.EndTimeUnixNano - span.Span.StartTimeUnixNano
			status := "UNSET"
			if span.Span.Status != nil {
				switch span.Span.Status.Code {
				case 1:
					status = "OK"
				case 2:
					status = "ERROR"
				}
			}
			parentSpanID := ""
			if len(span.Span.ParentSpanId) > 0 {
				parentSpanID = fmt.Sprintf("%x", span.Span.ParentSpanId)
			}
			kind := strings.TrimPrefix(span.Span.Kind.String(), "SPAN_KIND_")

			update.Traces = append(update.Traces, wsSpanSummary{
				Time:         formatNanoTime(span.Span.StartTimeUnixNano),
				TraceID:      span.TraceID,
				SpanID:       span.SpanID,
				ParentSpanID: parentSpanID,
				Service:      span.ServiceName,
				SpanName:     span.SpanName,
				Kind:         kind,
				DurationMs:   float64(durationNs) / 1e6,
				Status:       status,
				StartNs:      span.Span.StartTimeUnixNano,
				EndNs:        span.Span.EndTimeUnixNano,
			})
		}
		*lastTracePos = curTracePos
	}

	// Get log deltas
	if curLogPos > *lastLogPos {
		logs := s.storage.Logs().GetRange(*lastLogPos, curLogPos-1)
		for _, l := range logs {
			if filter.Service != "" && l.ServiceName != filter.Service {
				continue
			}
			if filter.Severity != "" && l.Severity != filter.Severity {
				continue
			}
			body := l.Body
			if utf8.RuneCountInString(body) > 500 {
				body = string([]rune(body)[:500]) + "..."
			}
			update.Logs = append(update.Logs, wsLogSummary{
				Time:     formatNanoTime(l.Timestamp),
				Service:  l.ServiceName,
				Severity: l.Severity,
				Body:     body,
			})
		}
		*lastLogPos = curLogPos
	}

	// Get metric deltas
	if curMetricPos > *lastMetricPos {
		metrics := s.storage.Metrics().GetRange(*lastMetricPos, curMetricPos-1)
		for _, m := range metrics {
			if filter.Service != "" && m.ServiceName != filter.Service {
				continue
			}
			valStr := ""
			if m.NumericValue != nil {
				valStr = fmt.Sprintf("%.4g", *m.NumericValue)
			} else if m.Count != nil {
				valStr = fmt.Sprintf("count=%d", *m.Count)
				if m.Sum != nil {
					valStr += fmt.Sprintf(" sum=%.4g", *m.Sum)
				}
			}
			update.Metrics = append(update.Metrics, wsMetricSummary{
				Name:    m.MetricName,
				Type:    m.MetricType.String(),
				Service: m.ServiceName,
				Value:   valStr,
				Updated: formatNanoTime(m.Timestamp),
			})
		}
		*lastMetricPos = curMetricPos
	}

	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("webui: failed to marshal update: %v", err)
		return
	}

	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
		// Connection closed; the main loop will handle cleanup.
		return
	}
}

// formatNanoTime converts unix nanoseconds to a human-readable time string.
func formatNanoTime(nanos uint64) string {
	if nanos == 0 {
		return ""
	}
	t := time.Unix(0, int64(nanos))
	return t.Format("15:04:05.000")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "")
	if err := enc.Encode(v); err != nil {
		log.Printf("webui: failed to write JSON: %v", err)
	}
}
