package viz

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWaterfall_DumpVisual(t *testing.T) {
	width := 100
	sep := strings.Repeat("─", width)
	ruler := func() string {
		tens := strings.Builder{}
		ones := strings.Builder{}
		for i := 0; i < width; i++ {
			if i%10 == 0 {
				tens.WriteString(fmt.Sprintf("%d", (i/10)%10))
			} else {
				tens.WriteString(" ")
			}
			ones.WriteString(fmt.Sprintf("%d", i%10))
		}
		return tens.String() + "\n" + ones.String()
	}

	cases := []struct {
		name  string
		spans []SpanInfo
	}{
		{
			name: "Parent/Child varied durations",
			spans: []SpanInfo{
				{TraceID: "t1", SpanID: "root", ServiceName: "api-gateway", SpanName: "GET /users", StartNano: 0, EndNano: 500_000_000},
				{TraceID: "t1", SpanID: "c1", ParentID: "root", ServiceName: "user-service", SpanName: "fetchUsers", StartNano: 10_000_000, EndNano: 300_000_000},
				{TraceID: "t1", SpanID: "c2", ParentID: "root", ServiceName: "cache", SpanName: "get", StartNano: 5_000_000, EndNano: 8_000_000},
				{TraceID: "t1", SpanID: "gc1", ParentID: "c1", ServiceName: "postgres", SpanName: "SELECT * FROM users", StartNano: 50_000_000, EndNano: 250_000_000},
			},
		},
		{
			name: "Mixed duration scales (µs to s)",
			spans: []SpanInfo{
				{TraceID: "t2", SpanID: "root", ServiceName: "svc", SpanName: "root", StartNano: 0, EndNano: 60_000_000_000},
				{TraceID: "t2", SpanID: "c1", ParentID: "root", ServiceName: "svc", SpanName: "fast-child", StartNano: 100_000_000, EndNano: 100_001_000},
				{TraceID: "t2", SpanID: "c2", ParentID: "root", ServiceName: "svc", SpanName: "medium-child", StartNano: 200_000_000, EndNano: 700_000_000},
				{TraceID: "t2", SpanID: "c3", ParentID: "root", ServiceName: "svc", SpanName: "slow-child", StartNano: 1_000_000_000, EndNano: 45_000_000_000},
			},
		},
		{
			name: "With errors",
			spans: []SpanInfo{
				{TraceID: "t3", SpanID: "root", ServiceName: "api", SpanName: "POST /order", StartNano: 0, EndNano: 2_000_000_000},
				{TraceID: "t3", SpanID: "c1", ParentID: "root", ServiceName: "payment", SpanName: "charge", StartNano: 100_000_000, EndNano: 1_800_000_000, StatusCode: "ERROR"},
				{TraceID: "t3", SpanID: "c2", ParentID: "root", ServiceName: "inventory", SpanName: "reserve", StartNano: 50_000_000, EndNano: 90_000_000},
			},
		},
		{
			name: "Deep nesting (8 levels)",
			spans: func() []SpanInfo {
				s := make([]SpanInfo, 8)
				for i := range s {
					parent := ""
					if i > 0 {
						parent = fmt.Sprintf("s%d", i-1)
					}
					s[i] = SpanInfo{
						TraceID: "t4", SpanID: fmt.Sprintf("s%d", i), ParentID: parent,
						ServiceName: "svc", SpanName: fmt.Sprintf("level-%d", i),
						StartNano: uint64(i * 100_000_000), EndNano: uint64((8 - i) * 100_000_000),
					}
				}
				return s
			}(),
		},
		{
			name: "Long names + short names",
			spans: []SpanInfo{
				{TraceID: "t5", SpanID: "root", ServiceName: "my-very-long-service-name", SpanName: "GET /api/v1/users/search", StartNano: 0, EndNano: 1_500_000_000},
				{TraceID: "t5", SpanID: "c1", ParentID: "root", ServiceName: "x", SpanName: "y", StartNano: 10_000_000, EndNano: 20_000_000},
				{TraceID: "t5", SpanID: "c2", ParentID: "root", ServiceName: "another-long-name-service", SpanName: "POST /api/v2/process/batch", StartNano: 100_000_000, EndNano: 1_400_000_000},
			},
		},
		{
			name: "Siblings with errors and without",
			spans: []SpanInfo{
				{TraceID: "t6", SpanID: "root", ServiceName: "gateway", SpanName: "request", StartNano: 0, EndNano: 5_000_000_000},
				{TraceID: "t6", SpanID: "c1", ParentID: "root", ServiceName: "auth", SpanName: "validate", StartNano: 10_000_000, EndNano: 50_000_000},
				{TraceID: "t6", SpanID: "c2", ParentID: "root", ServiceName: "backend", SpanName: "process", StartNano: 100_000_000, EndNano: 4_500_000_000, StatusCode: "STATUS_CODE_ERROR"},
				{TraceID: "t6", SpanID: "c3", ParentID: "root", ServiceName: "cache", SpanName: "invalidate", StartNano: 4_600_000_000, EndNano: 4_700_000_000},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Waterfall(tc.spans, width)
			t.Logf("\n%s\n%s\n%s\n%s", sep, ruler(), sep, result)

			// Verify all span lines have '[' at the same display column.
			// Must count runes, not bytes, because tree connectors (│├└─)
			// are multi-byte UTF-8 but single display column.
			lines := strings.Split(strings.TrimSpace(result), "\n")
			bracketCol := -1
			for _, line := range lines {
				col := displayCol(line, '[')
				if col < 0 {
					continue // header or overflow line
				}
				if bracketCol < 0 {
					bracketCol = col
				} else if col != bracketCol {
					t.Errorf("bracket misalignment: expected display col %d, got %d in line: %s", bracketCol, col, line)
				}
			}
		})
	}
}

// displayCol returns the display column (0-indexed) of the first occurrence
// of target in s, counting each rune as one column. Returns -1 if not found.
func displayCol(s string, target rune) int {
	col := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == target {
			return col
		}
		col++
		i += size
	}
	return -1
}
