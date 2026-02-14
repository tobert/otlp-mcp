package viz

import (
	"fmt"
	"sort"
	"strings"
)

const (
	maxSpansPerTrace = 50
	maxTraces        = 5
	maxInputSpans    = 500 // cap input to avoid sorting huge slices
	defaultBarWidth  = 20
)

// Waterfall renders an ASCII trace waterfall for the given spans.
// Width controls the total line width; 0 uses a sensible default (80).
func Waterfall(spans []SpanInfo, width int) string {
	if len(spans) == 0 {
		return ""
	}
	if width <= 0 {
		width = 80
	}

	// Cap input to avoid expensive sorting on huge result sets
	if len(spans) > maxInputSpans {
		spans = spans[:maxInputSpans]
	}

	// Group spans by TraceID
	byTrace := make(map[string][]SpanInfo)
	var traceOrder []string
	for _, s := range spans {
		if _, seen := byTrace[s.TraceID]; !seen {
			traceOrder = append(traceOrder, s.TraceID)
		}
		byTrace[s.TraceID] = append(byTrace[s.TraceID], s)
	}

	// Sort traces by earliest start time
	sort.Slice(traceOrder, func(i, j int) bool {
		return earliestStart(byTrace[traceOrder[i]]) < earliestStart(byTrace[traceOrder[j]])
	})

	// Cap number of traces
	overflow := 0
	if len(traceOrder) > maxTraces {
		overflow = len(traceOrder) - maxTraces
		traceOrder = traceOrder[:maxTraces]
	}

	var b strings.Builder
	for i, tid := range traceOrder {
		if i > 0 {
			b.WriteByte('\n')
		}
		renderTrace(&b, tid, byTrace[tid], width)
	}

	if overflow > 0 {
		fmt.Fprintf(&b, "\n... +%d more traces\n", overflow)
	}

	return b.String()
}

func renderTrace(b *strings.Builder, traceID string, spans []SpanInfo, width int) {
	// Sort by start time
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].StartNano < spans[j].StartNano
	})

	// Find time bounds, clamping end to max(end, start) to handle bad data
	minStart := spans[0].StartNano
	maxEnd := minStart
	for _, s := range spans {
		end := max(s.EndNano, s.StartNano)
		if end > maxEnd {
			maxEnd = end
		}
	}
	totalDur := maxEnd - minStart

	// Build tree
	tree := buildTree(spans)

	// Header
	shortID := traceID
	if len(shortID) > 6 {
		shortID = shortID[:6]
	}
	durStr := formatDuration(totalDur)
	fmt.Fprintf(b, "Trace %s (%d spans, %s)\n", shortID, len(spans), durStr)

	// Cap spans rendered
	spanOverflow := 0
	if len(tree.order) > maxSpansPerTrace {
		spanOverflow = len(tree.order) - maxSpansPerTrace
		tree.order = tree.order[:maxSpansPerTrace]
	}

	// Pass 1: Find max length of duration + error suffix for alignment
	maxDurErrLen := 0
	for _, entry := range tree.order {
		durStr := formatDuration(max(entry.span.EndNano, entry.span.StartNano) - entry.span.StartNano)
		errLen := 0
		if entry.span.StatusCode == "STATUS_CODE_ERROR" || entry.span.StatusCode == "ERROR" {
			errLen = 7 // " !! ERR"
		}
		if len(durStr)+errLen > maxDurErrLen {
			maxDurErrLen = len(durStr) + errLen
		}
	}

	// Pass 2: Render each span
	for _, entry := range tree.order {
		renderSpanRow(b, entry, minStart, totalDur, width, maxDurErrLen)
	}

	if spanOverflow > 0 {
		fmt.Fprintf(b, "  ... +%d more spans\n", spanOverflow)
	}
}

type treeEntry struct {
	span   SpanInfo
	depth  int
	isLast []bool // at each depth level, whether this node is the last child
}

type spanTree struct {
	order []treeEntry
}

func buildTree(spans []SpanInfo) spanTree {
	if len(spans) == 0 {
		return spanTree{}
	}

	// Index by SpanID
	byID := make(map[string]SpanInfo, len(spans))
	children := make(map[string][]string) // parentID -> child spanIDs
	var rootIDs []string

	for _, s := range spans {
		byID[s.SpanID] = s
		if s.ParentID == "" || s.ParentID == "0000000000000000" {
			rootIDs = append(rootIDs, s.SpanID)
		} else {
			children[s.ParentID] = append(children[s.ParentID], s.SpanID)
		}
	}

	// If no root found, use earliest span as root
	if len(rootIDs) == 0 {
		rootIDs = []string{spans[0].SpanID}
	}

	// Also find orphaned spans whose parent isn't in our set
	inSet := make(map[string]bool, len(spans))
	for _, s := range spans {
		inSet[s.SpanID] = true
	}
	for _, s := range spans {
		if s.ParentID != "" && s.ParentID != "0000000000000000" && !inSet[s.ParentID] {
			// Orphan - treat as root
			rootIDs = append(rootIDs, s.SpanID)
		}
	}

	// Deduplicate rootIDs
	seen := make(map[string]bool)
	var uniqueRoots []string
	for _, id := range rootIDs {
		if !seen[id] {
			seen[id] = true
			uniqueRoots = append(uniqueRoots, id)
		}
	}
	rootIDs = uniqueRoots

	// Sort roots by start time
	sort.Slice(rootIDs, func(i, j int) bool {
		return byID[rootIDs[i]].StartNano < byID[rootIDs[j]].StartNano
	})

	// DFS walk
	var result []treeEntry
	for ri, rootID := range rootIDs {
		isLastRoot := ri == len(rootIDs)-1
		walkTree(&result, byID, children, rootID, 0, []bool{isLastRoot})
	}

	return spanTree{order: result}
}

func walkTree(result *[]treeEntry, byID map[string]SpanInfo, children map[string][]string, spanID string, depth int, isLast []bool) {
	s, ok := byID[spanID]
	if !ok {
		return
	}
	*result = append(*result, treeEntry{span: s, depth: depth, isLast: isLast})

	kids := children[spanID]
	// Sort children by start time
	sort.Slice(kids, func(i, j int) bool {
		return byID[kids[i]].StartNano < byID[kids[j]].StartNano
	})

	for ci, childID := range kids {
		childIsLast := append(append([]bool{}, isLast...), ci == len(kids)-1)
		walkTree(result, byID, children, childID, depth+1, childIsLast)
	}
}

func renderSpanRow(b *strings.Builder, entry treeEntry, minStart, totalDur uint64, width int, maxDurErrLen int) {
	barWidth := defaultBarWidth

	// Build tree prefix, tracking display width separately from byte length.
	// Tree-drawing characters (│, ├─, └─) are multi-byte UTF-8 but each
	// occupies a single display column.
	var prefix strings.Builder
	prefixCols := 0

	prefix.WriteString(" ")
	prefixCols++
	for d := 0; d < entry.depth; d++ {
		if d < len(entry.isLast)-1 {
			if entry.isLast[d] {
				prefix.WriteString("  ")
			} else {
				prefix.WriteString("│ ")
			}
			prefixCols += 2
		}
	}
	if entry.depth > 0 {
		if len(entry.isLast) > 0 && entry.isLast[len(entry.isLast)-1] {
			prefix.WriteString("└─ ")
		} else {
			prefix.WriteString("├─ ")
		}
		prefixCols += 3
	}

	prefixStr := prefix.String()

	// Build label: service.spanName
	label := entry.span.ServiceName + "." + entry.span.SpanName

	// Error suffix
	errSuffix := ""
	if entry.span.StatusCode == "STATUS_CODE_ERROR" || entry.span.StatusCode == "ERROR" {
		errSuffix = " !! ERR"
	}

	// Guard against bad data where end < start
	spanStart := entry.span.StartNano
	spanEnd := max(entry.span.EndNano, spanStart)

	// Duration
	dur := spanEnd - spanStart
	durStr := formatDuration(dur)

	// Calculate label budget using display columns, not byte lengths.
	// Layout: prefix + label + " [" + bar + "] " + durErr
	fixedCols := prefixCols + 2 + barWidth + 2 + maxDurErrLen
	labelBudget := max(width-fixedCols, 8)
	if len(label) > labelBudget {
		label = label[:labelBudget-1] + "…"
	}

	// Pad label to budget
	paddedLabel := label + strings.Repeat(" ", max(0, labelBudget-len(label)))

	// Build timing bar
	bar := buildBar(spanStart, spanEnd, minStart, totalDur, barWidth)

	// Pad duration area for consistent right edge
	durErrStr := durStr + errSuffix
	paddedDurErr := durErrStr + strings.Repeat(" ", max(0, maxDurErrLen-len(durErrStr)))

	fmt.Fprintf(b, "%s%s [%s] %s\n", prefixStr, paddedLabel, bar, paddedDurErr)
}

func buildBar(startNano, endNano, minStart, totalDur uint64, barWidth int) string {
	if totalDur == 0 {
		// All spans are zero-duration or same time
		return strings.Repeat("#", barWidth)
	}

	startPos := int((startNano - minStart) * uint64(barWidth) / totalDur)
	endPos := int((endNano - minStart) * uint64(barWidth) / totalDur)

	if startPos >= barWidth {
		startPos = barWidth - 1
	}
	if endPos > barWidth {
		endPos = barWidth
	}
	// Ensure at least 1 char active
	endPos = max(endPos, startPos+1)
	endPos = min(endPos, barWidth)

	bar := make([]byte, barWidth)
	for i := range bar {
		if i >= startPos && i < endPos {
			bar[i] = '#'
		} else {
			bar[i] = '.'
		}
	}
	return string(bar)
}

func earliestStart(spans []SpanInfo) uint64 {
	if len(spans) == 0 {
		return 0
	}
	min := spans[0].StartNano
	for _, s := range spans[1:] {
		if s.StartNano < min {
			min = s.StartNano
		}
	}
	return min
}

func formatDuration(nanos uint64) string {
	if nanos == 0 {
		return "0ns"
	}
	us := float64(nanos) / 1000
	if us < 1000 {
		return fmt.Sprintf("%.0fµs", us)
	}
	ms := us / 1000
	if ms < 1000 {
		return fmt.Sprintf("%.0fms", ms)
	}
	s := ms / 1000
	return fmt.Sprintf("%.1fs", s)
}
