package cli

// Config holds the runtime configuration for the OTLP MCP server.
// It is populated from CLI flags when the serve command runs.
type Config struct {
	// Buffer sizes for different signal types
	TraceBufferSize  int
	LogBufferSize    int
	MetricBufferSize int

	// OTLP server configuration
	OTLPHost string
	OTLPPort int

	// Logging configuration
	Verbose bool
}

// DefaultConfig returns a Config with sensible default values.
// These defaults match the MVP requirements:
// - 10,000 spans for traces
// - 50,000 log records (future)
// - 100,000 metric points (future)
// - Localhost binding on ephemeral port
func DefaultConfig() *Config {
	return &Config{
		TraceBufferSize:  10_000,
		LogBufferSize:    50_000,
		MetricBufferSize: 100_000,
		OTLPHost:         "127.0.0.1",
		OTLPPort:         0, // 0 means ephemeral port assignment
		Verbose:          false,
	}
}
