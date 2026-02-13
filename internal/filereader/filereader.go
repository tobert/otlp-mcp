// Package filereader reads OTLP telemetry from JSONL files written by the
// OpenTelemetry Collector's file exporter. It feeds data into the same ring
// buffers used by the TCP receiver, so all query/snapshot logic works unchanged.
package filereader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/encoding/protojson"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const (
	// Buffer sizes for JSONL line scanning. OTLP JSON can be large,
	// especially for batched spans with many attributes.
	jsonlBufferInitial = 1 * 1024 * 1024  // 1MB initial buffer
	jsonlBufferMax     = 10 * 1024 * 1024 // 10MB maximum line size
)

// StorageReceiver is the interface that storage must implement to receive telemetry.
// This matches the methods on ObservabilityStorage.
type StorageReceiver interface {
	ReceiveSpans(ctx context.Context, resourceSpans []*tracepb.ResourceSpans) error
	ReceiveLogs(ctx context.Context, resourceLogs []*logspb.ResourceLogs) error
	ReceiveMetrics(ctx context.Context, resourceMetrics []*metricspb.ResourceMetrics) error
}

// FileSource reads OTLP telemetry from a directory of JSONL files.
// It watches for new data and feeds it into the storage ring buffers.
type FileSource struct {
	directory  string
	storage    StorageReceiver
	verbose    bool
	activeOnly bool // Only load active files, skip rotated archives

	// Storage capacities for tail-seek optimization
	spanCapacity   int
	logCapacity    int
	metricCapacity int

	watcher *fsnotify.Watcher

	// Track file read positions to only read new data
	mu          sync.Mutex
	fileOffsets map[string]int64

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Config holds configuration for a FileSource.
type Config struct {
	Directory string // Base directory (e.g., /tank/otel)
	Verbose   bool   // Enable verbose logging

	// ActiveOnly when true (default) only loads active files like traces.jsonl,
	// skipping rotated archives like traces-2025-12-09T13-10-56.jsonl.
	// This prevents loading gigabytes of historical data on startup.
	ActiveOnly bool

	// Optional: time cutoff - only load data newer than this
	// Zero value means load everything (future feature)
	SinceTime time.Time

	// Storage capacities — used by tail-seek to avoid reading entire files
	// when only the last N entries fit in ring buffers. Zero means read all.
	SpanCapacity   int
	LogCapacity    int
	MetricCapacity int
}

// New creates a new FileSource that reads from the given directory.
// The directory should contain subdirectories: traces/, logs/, metrics/
// with .jsonl files inside them.
func New(cfg Config, storage StorageReceiver) (*FileSource, error) {
	if cfg.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	// Verify directory exists
	info, err := os.Stat(cfg.Directory)
	if err != nil {
		return nil, fmt.Errorf("cannot access directory %s: %w", cfg.Directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", cfg.Directory)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FileSource{
		directory:      cfg.Directory,
		storage:        storage,
		verbose:        cfg.Verbose,
		activeOnly:     cfg.ActiveOnly,
		spanCapacity:   cfg.SpanCapacity,
		logCapacity:    cfg.LogCapacity,
		metricCapacity: cfg.MetricCapacity,
		watcher:        watcher,
		fileOffsets:    make(map[string]int64),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Start begins watching the directory and loading initial data.
// It returns after initial load completes; watching continues in background.
func (fs *FileSource) Start(ctx context.Context) error {
	if fs.verbose {
		log.Printf("📁 FileSource: starting with directory %s\n", fs.directory)
	}

	// Set up watches on signal subdirectories
	signals := []string{"traces", "logs", "metrics"}
	for _, signal := range signals {
		dir := filepath.Join(fs.directory, signal)
		if _, err := os.Stat(dir); err == nil {
			if err := fs.watcher.Add(dir); err != nil {
				log.Printf("⚠️  FileSource: could not watch %s: %v\n", dir, err)
			} else if fs.verbose {
				log.Printf("📁 FileSource: watching %s\n", dir)
			}
		}
	}

	// Initial load of existing files
	if err := fs.loadInitialData(ctx); err != nil {
		return fmt.Errorf("initial data load failed: %w", err)
	}

	// Start background watcher
	fs.wg.Add(1)
	go fs.watchLoop()

	return nil
}

// Stop stops the file watcher and waits for goroutines to finish.
func (fs *FileSource) Stop() {
	fs.cancel()
	fs.watcher.Close()
	fs.wg.Wait()
}

// Directory returns the base directory being watched.
func (fs *FileSource) Directory() string {
	return fs.directory
}

// loadInitialData reads all existing JSONL files into storage.
func (fs *FileSource) loadInitialData(ctx context.Context) error {
	signals := []struct {
		name     string
		capacity int
		loader   func(context.Context, string, int) (int, error)
	}{
		{"traces", fs.spanCapacity, fs.loadTraceFile},
		{"logs", fs.logCapacity, fs.loadLogFile},
		{"metrics", fs.metricCapacity, fs.loadMetricFile},
	}

	for _, sig := range signals {
		dir := filepath.Join(fs.directory, sig.name)
		files, err := fs.findJSONLFiles(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Signal directory doesn't exist, skip
			}
			return err
		}

		for _, file := range files {
			count, err := sig.loader(ctx, file, sig.capacity)
			if err != nil {
				log.Printf("⚠️  FileSource: error loading %s: %v\n", file, err)
				continue
			}
			if fs.verbose && count > 0 {
				log.Printf("📁 FileSource: loaded %d %s from %s\n", count, sig.name, filepath.Base(file))
			}
		}
	}

	return nil
}

// findJSONLFiles returns .jsonl files in a directory, sorted by modification time.
// When activeOnly is true, only returns active files (e.g., traces.jsonl) and
// skips rotated archives (e.g., traces-2025-12-09T13-10-56.jsonl).
func (fs *FileSource) findJSONLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Determine the expected active filename from the directory name
	// e.g., /tank/otel/traces -> traces.jsonl
	signal := filepath.Base(dir)
	activeFileName := signal + ".jsonl"

	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") && !strings.Contains(name, ".jsonl.") {
			continue
		}

		// When activeOnly, skip archived/rotated files
		// Active files: traces.jsonl, logs.jsonl, metrics.jsonl
		// Archived files: traces-2025-12-09T13-10-56.jsonl (contain hyphen after signal name)
		if fs.activeOnly && name != activeFileName {
			if fs.verbose {
				log.Printf("📁 FileSource: skipping archived file %s (activeOnly mode)\n", name)
			}
			continue
		}

		path := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: path, modTime: info.ModTime()})
	}

	// Sort by modification time (oldest first) so we load data in chronological order
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.path
	}
	return result, nil
}

// loadTraceFile reads a JSONL file containing traces and feeds them to storage.
func (fs *FileSource) loadTraceFile(ctx context.Context, path string, capacity int) (int, error) {
	return fs.processFile(ctx, path, capacity, func(line []byte) error {
		var data tracepb.TracesData
		if err := protojson.Unmarshal(line, &data); err != nil {
			return fmt.Errorf("parse trace JSON: %w", err)
		}
		if len(data.ResourceSpans) > 0 {
			return fs.storage.ReceiveSpans(ctx, data.ResourceSpans)
		}
		return nil
	})
}

// loadLogFile reads a JSONL file containing logs and feeds them to storage.
func (fs *FileSource) loadLogFile(ctx context.Context, path string, capacity int) (int, error) {
	return fs.processFile(ctx, path, capacity, func(line []byte) error {
		var data logspb.LogsData
		if err := protojson.Unmarshal(line, &data); err != nil {
			return fmt.Errorf("parse log JSON: %w", err)
		}
		if len(data.ResourceLogs) > 0 {
			return fs.storage.ReceiveLogs(ctx, data.ResourceLogs)
		}
		return nil
	})
}

// loadMetricFile reads a JSONL file containing metrics and feeds them to storage.
func (fs *FileSource) loadMetricFile(ctx context.Context, path string, capacity int) (int, error) {
	return fs.processFile(ctx, path, capacity, func(line []byte) error {
		var data metricspb.MetricsData
		if err := protojson.Unmarshal(line, &data); err != nil {
			return fmt.Errorf("parse metric JSON: %w", err)
		}
		if len(data.ResourceMetrics) > 0 {
			return fs.storage.ReceiveMetrics(ctx, data.ResourceMetrics)
		}
		return nil
	})
}

// processFile reads a JSONL file from the last known offset, calling handler for each line.
// When capacity > 0 and this is the first read (offset == 0), it tail-seeks to only read
// approximately the last `capacity` lines, avoiding parsing data that would be evicted
// from ring buffers anyway. Returns the number of lines processed.
func (fs *FileSource) processFile(ctx context.Context, path string, capacity int, handler func([]byte) error) (int, error) {
	fs.mu.Lock()
	offset := fs.fileOffsets[path]
	fs.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Tail-seek optimization: on first read with known capacity, skip to the
	// tail of the file. We only need ~capacity lines since that's all the
	// ring buffer can hold.
	if offset == 0 && capacity > 0 {
		offset = fs.estimateTailOffset(file, capacity)
	}

	// Seek to read position
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			// File might have been rotated, start from beginning
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return 0, err
			}
		}
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, jsonlBufferInitial)
	scanner.Buffer(buf, jsonlBufferMax)

	count := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if err := handler(line); err != nil {
			// Log but continue - don't let one bad line stop everything
			if fs.verbose {
				log.Printf("⚠️  FileSource: error processing line in %s: %v\n", filepath.Base(path), err)
			}
			continue
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("reading %s: %w", path, err)
	}

	// Update offset
	newOffset, _ := file.Seek(0, io.SeekCurrent)
	fs.mu.Lock()
	fs.fileOffsets[path] = newOffset
	fs.mu.Unlock()

	return count, nil
}

// estimateTailOffset calculates a byte offset to seek to for reading approximately
// the last `capacity` lines from a file. It samples the first few lines to estimate
// average line size, then seeks back from the end with a 2x safety margin.
// Returns 0 if the file is small enough to read entirely.
func (fs *FileSource) estimateTailOffset(file *os.File, capacity int) int64 {
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	fileSize := info.Size()

	// Sample first lines to estimate average line size
	const sampleLines = 10
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, jsonlBufferInitial)
	scanner.Buffer(buf, jsonlBufferMax)

	var totalBytes int64
	var lineCount int
	for scanner.Scan() && lineCount < sampleLines {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		totalBytes += int64(len(line)) + 1 // +1 for newline
		lineCount++
	}

	// Reset to beginning (caller will seek to our returned offset)
	file.Seek(0, io.SeekStart)

	if lineCount == 0 {
		return 0
	}

	avgLineSize := totalBytes / int64(lineCount)
	// 2x safety margin to ensure we get at least capacity lines
	estimatedBytes := int64(capacity) * avgLineSize * 2

	if estimatedBytes >= fileSize {
		return 0 // File is small enough, read it all
	}

	seekPos := fileSize - estimatedBytes

	// Find the next newline boundary after seekPos so we don't start mid-line
	file.Seek(seekPos, io.SeekStart)
	oneByte := make([]byte, 1)
	for {
		_, err := file.Read(oneByte)
		if err != nil {
			return 0
		}
		seekPos++
		if oneByte[0] == '\n' {
			break
		}
	}

	// Reset again for caller
	file.Seek(0, io.SeekStart)

	if fs.verbose {
		skippedMB := float64(seekPos) / (1024 * 1024)
		log.Printf("📁 FileSource: tail-seek %s: skipping %.1fMB, reading last %.1fMB (capacity=%d, avg line=%d bytes)\n",
			filepath.Base(file.Name()), skippedMB, float64(fileSize-seekPos)/(1024*1024), capacity, avgLineSize)
	}

	return seekPos
}

// watchLoop runs the file watcher event loop.
func (fs *FileSource) watchLoop() {
	defer fs.wg.Done()

	for {
		select {
		case <-fs.ctx.Done():
			return

		case event, ok := <-fs.watcher.Events:
			if !ok {
				return
			}

			// Only care about writes and creates
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Determine signal type from path
			path := event.Name
			if !strings.HasSuffix(path, ".jsonl") && !strings.Contains(path, ".jsonl.") {
				continue
			}

			dir := filepath.Dir(path)
			signal := filepath.Base(dir)

			var count int
			var err error
			switch signal {
			case "traces":
				count, err = fs.loadTraceFile(fs.ctx, path, 0)
			case "logs":
				count, err = fs.loadLogFile(fs.ctx, path, 0)
			case "metrics":
				count, err = fs.loadMetricFile(fs.ctx, path, 0)
			}

			if err != nil {
				log.Printf("⚠️  FileSource: error reading %s: %v\n", path, err)
			} else if fs.verbose && count > 0 {
				log.Printf("📁 FileSource: loaded %d new %s from %s\n", count, signal, filepath.Base(path))
			}

		case err, ok := <-fs.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("⚠️  FileSource: watcher error: %v\n", err)
		}
	}
}

// Stats returns statistics about the file source.
type Stats struct {
	Directory    string   `json:"directory"`
	WatchedDirs  []string `json:"watched_dirs"`
	FilesTracked int      `json:"files_tracked"`
}

// Stats returns current statistics.
func (fs *FileSource) Stats() Stats {
	fs.mu.Lock()
	filesTracked := len(fs.fileOffsets)
	fs.mu.Unlock()

	return Stats{
		Directory:    fs.directory,
		WatchedDirs:  fs.watcher.WatchList(),
		FilesTracked: filesTracked,
	}
}
