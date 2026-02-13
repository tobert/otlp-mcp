package viz

import (
	"strings"
	"testing"
)

func TestWaterfall_Alignment(t *testing.T) {
	spans := []SpanInfo{
		{TraceID: "align1", SpanID: "root", ServiceName: "svc", SpanName: "root", StartNano: 0, EndNano: 60_000_000_000}, // 60.0s (5 chars)
		{TraceID: "align1", SpanID: "c1", ParentID: "root", ServiceName: "svc", SpanName: "c1", StartNano: 100_000_000, EndNano: 100_001_000}, // 1µs (3 chars)
	}
	result := Waterfall(spans, 80)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	
	// line 0: Trace align1 (2 spans, 60.0s)
	// line 1:  svc.root ... [###...] 60.0s
	// line 2:  └─ svc.c1 ... [.#...] 1µs
	
	idx1 := strings.Index(lines[1], "[")
	idx2 := strings.Index(lines[2], "[")
	
	if idx1 != idx2 {
		t.Errorf("mismatched alignment: line 1 '[' at %d, line 2 '[' at %d", idx1, idx2)
		t.Logf("Result:\n%s", result)
	}
}
