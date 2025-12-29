package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OtelCollectorConfig represents the relevant parts of an OpenTelemetry Collector config.
// We only parse the exporters section to find file exporters.
type OtelCollectorConfig struct {
	Exporters map[string]FileExporter `yaml:"exporters"`
}

// FileExporter represents a file exporter configuration.
type FileExporter struct {
	Path string `yaml:"path"`
}

// ParseOtelConfig reads an OpenTelemetry Collector config file and extracts
// base directories from file exporter paths. It looks for exporters with names
// starting with "file/" and returns the base directory (parent of signal directories).
//
// For example, if the config has:
//
//	file/traces:
//	  path: /tank/otel/traces/traces.jsonl
//
// This returns ["/tank/otel"] because FileSource expects the base directory
// and internally looks for traces/, logs/, metrics/ subdirectories.
func ParseOtelConfig(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read otel config: %w", err)
	}

	var config OtelCollectorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse otel config: %w", err)
	}

	// Collect unique base directories from file exporters
	// Path structure: /base/signal/file.jsonl (e.g., /tank/otel/traces/traces.jsonl)
	// We need: /base (e.g., /tank/otel)
	dirSet := make(map[string]struct{})
	for name, exporter := range config.Exporters {
		if strings.HasPrefix(name, "file/") && exporter.Path != "" {
			signalDir := filepath.Dir(exporter.Path) // /tank/otel/traces
			baseDir := filepath.Dir(signalDir)       // /tank/otel
			dirSet[baseDir] = struct{}{}
		}
	}

	// Convert to slice
	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}

	return dirs, nil
}
