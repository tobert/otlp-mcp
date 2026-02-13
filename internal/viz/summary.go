package viz

import (
	"fmt"
	"strings"
)

// StatsOverview renders buffer fill-level bars.
func StatsOverview(stats BufferStats) string {
	var b strings.Builder

	b.WriteString("Buffer Health\n")
	writeBar(&b, "Traces", stats.SpanCount, stats.SpanCapacity)
	writeBar(&b, "Logs", stats.LogCount, stats.LogCapacity)
	writeBar(&b, "Metrics", stats.MetricCount, stats.MetricCapacity)
	fmt.Fprintf(&b, "  Snapshots: %d\n", stats.SnapshotCount)

	return b.String()
}

func writeBar(b *strings.Builder, label string, count, capacity int) {
	barWidth := 20
	filled := 0
	if capacity > 0 {
		filled = count * barWidth / capacity
	}
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("#", filled) + strings.Repeat(".", barWidth-filled)

	// Pad label to 8 chars for alignment
	paddedLabel := fmt.Sprintf("%-8s", label)
	fmt.Fprintf(b, "  %s [%s]  %s / %s\n", paddedLabel, bar, formatCount(count), formatCount(capacity))
}

// ServiceSummary renders a horizontal bar chart of services.
// Width controls total line width; 0 uses default (80).
func ServiceSummary(services []ServiceStats, width int) string {
	if len(services) == 0 {
		return ""
	}
	if width <= 0 {
		width = 80
	}

	totalSpans := 0
	for _, s := range services {
		totalSpans += s.SpanCount
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Services (%d active, %d spans)\n", len(services), totalSpans)

	// Find max span count for bar scaling
	maxCount := 0
	for _, s := range services {
		if s.SpanCount > maxCount {
			maxCount = s.SpanCount
		}
	}

	// Find longest name for alignment
	maxNameLen := 0
	for _, s := range services {
		if len(s.Name) > maxNameLen {
			maxNameLen = len(s.Name)
		}
	}
	if maxNameLen > 20 {
		maxNameLen = 20
	}

	barBudget := 20

	for _, s := range services {
		name := s.Name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}
		paddedName := fmt.Sprintf("%-*s", maxNameLen, name)

		barLen := 0
		if maxCount > 0 {
			barLen = s.SpanCount * barBudget / maxCount
		}
		if barLen < 1 && s.SpanCount > 0 {
			barLen = 1
		}
		bar := strings.Repeat("#", barLen)
		barPad := strings.Repeat(" ", barBudget-barLen)

		errStr := ""
		if s.ErrorCount > 0 {
			errStr = fmt.Sprintf(" (%d errors)", s.ErrorCount)
		}

		fmt.Fprintf(&b, "  %s  %s%s  %d spans%s\n", paddedName, bar, barPad, s.SpanCount, errStr)
	}

	return b.String()
}

func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n%1_000_000)/1000, n%1000)
}
