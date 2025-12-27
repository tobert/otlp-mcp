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
// directories from file exporter paths. It looks for exporters with names
// starting with "file/" and returns the parent directories of their paths.
func ParseOtelConfig(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read otel config: %w", err)
	}

	var config OtelCollectorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse otel config: %w", err)
	}

	// Collect unique directories from file exporters
	dirSet := make(map[string]struct{})
	for name, exporter := range config.Exporters {
		if strings.HasPrefix(name, "file/") && exporter.Path != "" {
			dir := filepath.Dir(exporter.Path)
			dirSet[dir] = struct{}{}
		}
	}

	// Convert to slice
	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}

	return dirs, nil
}
