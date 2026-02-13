package viz

import (
	"fmt"
	"strings"
)

// RecentTraces renders a compact table of recent traces.
func RecentTraces(traces []ActivityTrace) string {
	if len(traces) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Recent Traces (%d)\n", len(traces))

	for _, t := range traces {
		shortID := t.TraceID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		status := statusIcon(t.Status)
		durStr := fmt.Sprintf("%.0fms", t.DurationMs)

		label := t.Service + "/" + t.RootSpan
		if len(label) > 40 {
			label = label[:39] + "…"
		}

		fmt.Fprintf(&b, "  %s %s  %-40s  %8s\n", status, shortID, label, durStr)
	}

	return b.String()
}

// RecentErrors renders a compact table of recent errors.
func RecentErrors(errors []ActivityError) string {
	if len(errors) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Recent Errors (%d)\n", len(errors))

	for _, e := range errors {
		shortID := e.TraceID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		label := e.Service + "/" + e.SpanName
		if len(label) > 30 {
			label = label[:29] + "…"
		}

		msg := e.ErrorMsg
		if len(msg) > 40 {
			msg = msg[:39] + "…"
		}

		fmt.Fprintf(&b, "  ✗ %s  %-30s  %s\n", shortID, label, msg)
	}

	return b.String()
}

func statusIcon(status string) string {
	switch status {
	case "ERROR", "STATUS_CODE_ERROR":
		return "✗"
	case "OK", "STATUS_CODE_OK":
		return "✓"
	default:
		return "·"
	}
}
