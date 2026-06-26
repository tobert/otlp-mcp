package filereader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestSource builds a FileSource pointed at dir without starting the watcher
// goroutine. storage is nil because these tests drive processFile with their own
// line handler and never touch the ring buffers.
func newTestSource(t *testing.T, dir string) *FileSource {
	t.Helper()
	fs, err := New(Config{Directory: dir, ActiveOnly: true}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { fs.watcher.Close() })
	return fs
}

// writeLines writes n newline-terminated lines to path, replacing any existing
// content. Each line is unique so partial reads are detectable.
func writeLines(t *testing.T, path string, n int, prefix string) {
	t.Helper()
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "%s-%05d-%s\n", prefix, i, strings.Repeat("x", 40))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func countLines(t *testing.T, fs *FileSource, path string) int {
	t.Helper()
	n, err := fs.processFile(context.Background(), path, 0, func([]byte) error { return nil })
	if err != nil {
		t.Fatalf("processFile %s: %v", path, err)
	}
	return n
}

// TestProcessFileRotation reproduces the collector-rotation bug: the active file
// is renamed out and a fresh, smaller file is created at the same path (a new
// inode). The new file's contents must be read even though it is smaller than
// the previous end-of-file offset.
func TestProcessFileRotation(t *testing.T) {
	dir := t.TempDir()
	fs := newTestSource(t, dir)
	path := filepath.Join(dir, "traces.jsonl")

	writeLines(t, path, 100, "old")
	if got := countLines(t, fs, path); got != 100 {
		t.Fatalf("first read: got %d, want 100", got)
	}
	if got := countLines(t, fs, path); got != 0 {
		t.Fatalf("re-read with no new data: got %d, want 0", got)
	}

	// Rotate: remove the active file and create a fresh, smaller one. On most
	// filesystems this yields a new inode; the file is also smaller than the
	// 100-line offset we stored, so a path-keyed offset would skip everything.
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	writeLines(t, path, 10, "new")

	if got := countLines(t, fs, path); got != 10 {
		t.Fatalf("after rotation: got %d, want 10 (stale offset skipped the fresh file)", got)
	}
}

// TestProcessFileTruncation covers in-place truncation: same inode, but the file
// shrinks below the stored offset. We must reset to the start rather than seek
// past the (now shorter) end.
func TestProcessFileTruncation(t *testing.T) {
	dir := t.TempDir()
	fs := newTestSource(t, dir)
	path := filepath.Join(dir, "traces.jsonl")

	writeLines(t, path, 100, "old")
	if got := countLines(t, fs, path); got != 100 {
		t.Fatalf("first read: got %d, want 100", got)
	}

	// Truncate in place (O_TRUNC keeps the inode) and write fewer lines.
	writeLines(t, path, 10, "new")

	if got := countLines(t, fs, path); got != 10 {
		t.Fatalf("after truncation: got %d, want 10", got)
	}
}

// TestProcessFileAppend confirms the normal case still works: appended lines are
// read incrementally and only the new ones surface.
func TestProcessFileAppend(t *testing.T) {
	dir := t.TempDir()
	fs := newTestSource(t, dir)
	path := filepath.Join(dir, "traces.jsonl")

	writeLines(t, path, 10, "a")
	if got := countLines(t, fs, path); got != 10 {
		t.Fatalf("first read: got %d, want 10", got)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	for i := range 5 {
		fmt.Fprintf(f, "b-%05d-%s\n", i, strings.Repeat("x", 40))
	}
	f.Close()

	if got := countLines(t, fs, path); got != 5 {
		t.Fatalf("after append: got %d, want 5", got)
	}
}
