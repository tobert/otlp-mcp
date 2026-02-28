package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the runtime configuration for the OTLP MCP server.
// It can be populated from CLI flags, config files, or both.
type Config struct {
	// Comment field for user documentation (ignored by the application)
	Comment string `json:"comment,omitempty"`

	// Buffer sizes for different signal types (direct JSON mapping to CLI flags)
	TraceBufferSize  int `json:"trace_buffer_size,omitempty"`
	LogBufferSize    int `json:"log_buffer_size,omitempty"`
	MetricBufferSize int `json:"metric_buffer_size,omitempty"`

	// OTLP server configuration
	OTLPHost string `json:"otlp_host,omitempty"`
	OTLPPort int    `json:"otlp_port,omitempty"`

	// MCP transport configuration
	Transport      string   `json:"transport,omitempty"`       // "stdio" (default) or "http"
	HTTPHost       string   `json:"http_host,omitempty"`       // HTTP server bind address
	HTTPPort       int      `json:"http_port,omitempty"`       // HTTP server port
	AllowedOrigins []string `json:"allowed_origins,omitempty"` // Allowed Origin headers for CORS
	SessionTimeout string   `json:"session_timeout,omitempty"` // Session idle timeout (e.g., "30m")
	Stateless      bool     `json:"stateless,omitempty"`       // Run HTTP transport in stateless mode

	// SSH transport configuration
	SSHHost           string `json:"ssh_host,omitempty"`            // SSH server bind address
	SSHPort           int    `json:"ssh_port,omitempty"`            // SSH server port (default 2222)
	SSHHostKeyFile    string `json:"ssh_host_key_file,omitempty"`   // Path to host key (generated if missing)
	SSHAuthorizedKeys string `json:"ssh_authorized_keys,omitempty"` // Path to authorized keys file

	// Web UI configuration
	WebUIPort int    `json:"webui_port,omitempty"` // 0 = use same port as HTTP (default)
	WebUIHost string `json:"webui_host,omitempty"` // default: 127.0.0.1

	// Logging configuration
	Verbose bool `json:"verbose,omitempty"`
}

// DefaultConfig returns a Config with sensible default values.
// These defaults match the MVP requirements:
// - 10,000 spans for traces
// - 50,000 log records (future)
// - 100,000 metric points (future)
// - Localhost binding on ephemeral port
// - stdio transport (or http on port 4380)
func DefaultConfig() *Config {
	return &Config{
		TraceBufferSize:  10_000,
		LogBufferSize:    50_000,
		MetricBufferSize: 100_000,
		OTLPHost:         "127.0.0.1",
		OTLPPort:         0, // 0 means ephemeral port assignment
		Transport:        "stdio",
		HTTPHost:         "127.0.0.1",
		HTTPPort:         4380,
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		SessionTimeout:   "30m",
		Stateless:        false,
		SSHHost:          "0.0.0.0",
		SSHPort:          2222,
		WebUIPort:        0,
		WebUIHost:        "127.0.0.1",
		Verbose:          false,
	}
}

// LoadConfigFromFile loads configuration from a JSON file at the given path.
// It returns an error if the file cannot be read or parsed.
func LoadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return &config, nil
}

// FindProjectConfig searches for a .otlp-mcp.json config file.
// It starts in the current directory and walks up looking for the file,
// stopping when it finds a .git directory (project root) or reaches root.
func FindProjectConfig() (string, error) {
	// Start from current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Walk up directory tree
	for {
		configPath := filepath.Join(dir, ".otlp-mcp.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Check if we're at a git repo root (stop here even if no config)
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			// We're at repo root but no config found
			break
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	// No project config found
	return "", os.ErrNotExist
}

// GlobalConfigPath returns the path to the global config file.
// This is ~/.config/otlp-mcp/config.json
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "otlp-mcp", "config.json")
}

// MergeConfigs merges two configs with the overlay taking precedence.
// Fields in overlay override corresponding fields in base.
// Returns a new Config with the merged values.
func MergeConfigs(base, overlay *Config) *Config {
	if base == nil {
		base = &Config{}
	}
	if overlay == nil {
		return base
	}

	// Create a copy of base
	merged := *base

	// Override with overlay values if set
	if overlay.OTLPHost != "" {
		merged.OTLPHost = overlay.OTLPHost
	}
	if overlay.OTLPPort != 0 {
		merged.OTLPPort = overlay.OTLPPort
	}
	if overlay.Verbose {
		merged.Verbose = overlay.Verbose
	}

	// Merge buffer sizes
	if overlay.TraceBufferSize > 0 {
		merged.TraceBufferSize = overlay.TraceBufferSize
	}
	if overlay.LogBufferSize > 0 {
		merged.LogBufferSize = overlay.LogBufferSize
	}
	if overlay.MetricBufferSize > 0 {
		merged.MetricBufferSize = overlay.MetricBufferSize
	}

	// Merge HTTP transport settings
	if overlay.Transport != "" {
		merged.Transport = overlay.Transport
	}
	if overlay.HTTPHost != "" {
		merged.HTTPHost = overlay.HTTPHost
	}
	if overlay.HTTPPort > 0 {
		merged.HTTPPort = overlay.HTTPPort
	}
	if len(overlay.AllowedOrigins) > 0 {
		merged.AllowedOrigins = overlay.AllowedOrigins
	}
	if overlay.SessionTimeout != "" {
		merged.SessionTimeout = overlay.SessionTimeout
	}
	if overlay.Stateless {
		merged.Stateless = overlay.Stateless
	}

	// Merge SSH transport settings
	if overlay.SSHHost != "" {
		merged.SSHHost = overlay.SSHHost
	}
	if overlay.SSHPort > 0 {
		merged.SSHPort = overlay.SSHPort
	}
	if overlay.SSHHostKeyFile != "" {
		merged.SSHHostKeyFile = overlay.SSHHostKeyFile
	}
	if overlay.SSHAuthorizedKeys != "" {
		merged.SSHAuthorizedKeys = overlay.SSHAuthorizedKeys
	}

	// Merge Web UI settings
	if overlay.WebUIPort > 0 {
		merged.WebUIPort = overlay.WebUIPort
	}
	if overlay.WebUIHost != "" {
		merged.WebUIHost = overlay.WebUIHost
	}

	return &merged
}

// LoadEffectiveConfig loads the effective configuration by merging:
// 1. Built-in defaults
// 2. Global config file (if exists)
// 3. Project config file (if exists)
// 4. Explicit config file (if specified via configPath)
// Later sources override earlier ones.
func LoadEffectiveConfig(configPath string) (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Layer 2: Global config (if exists)
	globalPath := GlobalConfigPath()
	if globalPath != "" {
		if globalCfg, err := LoadConfigFromFile(globalPath); err == nil {
			config = MergeConfigs(config, globalCfg)
		}
		// Ignore errors for global config (it's optional)
	}

	// Layer 3: Project config (if exists and no explicit path)
	if configPath == "" {
		if projectPath, err := FindProjectConfig(); err == nil {
			projectCfg, err := LoadConfigFromFile(projectPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load project config: %w", err)
			}
			config = MergeConfigs(config, projectCfg)
		}
		// Ignore not found error for project config (it's optional)
	} else {
		// Explicit config file specified
		explicitCfg, err := LoadConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		config = MergeConfigs(config, explicitCfg)
	}

	return config, nil
}
