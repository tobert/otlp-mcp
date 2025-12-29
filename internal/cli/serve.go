package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tobert/otlp-mcp/internal/mcpserver"
	"github.com/tobert/otlp-mcp/internal/otlpreceiver"
	"github.com/tobert/otlp-mcp/internal/storage"
	"github.com/urfave/cli/v3"
)

// ServeCommand returns the CLI command definition for the 'serve' subcommand.
// This command starts both the OTLP gRPC receiver and the MCP stdio server.
func ServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the OTLP receiver and MCP server",
		Description: `Starts an OTLP gRPC receiver on localhost:0 (ephemeral port) and an
MCP server on stdio (default) or HTTP.

Transport modes:
  stdio  - MCP over stdin/stdout (default, for agent spawned processes)
  http   - MCP over Streamable HTTP at /mcp (for persistent services)

Examples:
  otlp-mcp serve                        # stdio mode (default)
  otlp-mcp serve --transport http       # HTTP mode on port 4380
  otlp-mcp serve --transport http --http-port 8080`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file (default: search for .otlp-mcp.json)",
			},
			&cli.IntFlag{
				Name:  "trace-buffer-size",
				Usage: "Number of spans to buffer (overrides config file)",
				Value: 0, // 0 means use config/default
			},
			&cli.IntFlag{
				Name:  "log-buffer-size",
				Usage: "Number of log records to buffer (overrides config file)",
				Value: 0, // 0 means use config/default
			},
			&cli.IntFlag{
				Name:  "metric-buffer-size",
				Usage: "Number of metric points to buffer (overrides config file)",
				Value: 0, // 0 means use config/default
			},
			&cli.StringFlag{
				Name:  "otlp-host",
				Usage: "OTLP server bind address (overrides config file)",
				Value: "",
			},
			&cli.IntFlag{
				Name:  "otlp-port",
				Usage: "OTLP server port, 0 for ephemeral (overrides config file)",
				Value: -1, // -1 means not set
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose logging (overrides config file)",
			},
			&cli.StringSliceFlag{
				Name:    "file-source",
				Aliases: []string{"f"},
				Usage:   "Directory to load OTLP JSONL files from (can be specified multiple times)",
			},
			// HTTP transport flags
			&cli.StringFlag{
				Name:  "transport",
				Usage: "MCP transport: 'stdio' (default) or 'http'",
				Value: "",
			},
			&cli.StringFlag{
				Name:  "http-host",
				Usage: "HTTP server bind address (when transport=http)",
				Value: "",
			},
			&cli.IntFlag{
				Name:  "http-port",
				Usage: "HTTP server port (when transport=http, default 4380)",
				Value: -1,
			},
			&cli.StringSliceFlag{
				Name:  "allowed-origin",
				Usage: "Allowed Origin headers for HTTP transport (can specify multiple)",
			},
			&cli.DurationFlag{
				Name:  "session-timeout",
				Usage: "Session idle timeout for HTTP transport (e.g., 30m)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "stateless",
				Usage: "Run HTTP transport in stateless mode (no session persistence)",
			},
			// Otel collector integration
			&cli.StringFlag{
				Name:  "otel-config",
				Usage: "Path to otel-collector config.yaml to auto-discover file sources (skips OTLP listener)",
			},
		},
		Action: runServe,
	}
}

// runServe is the action handler for the serve command.
// It wires together all components: storage, OTLP receiver, and MCP server.
func runServe(cliCtx context.Context, cmd *cli.Command) error {
	// Load effective config from files
	configPath := cmd.String("config")
	cfg, err := LoadEffectiveConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply CLI flag overrides (highest precedence)
	if traceSize := cmd.Int("trace-buffer-size"); traceSize > 0 {
		cfg.TraceBufferSize = traceSize
	}
	if logSize := cmd.Int("log-buffer-size"); logSize > 0 {
		cfg.LogBufferSize = logSize
	}
	if metricSize := cmd.Int("metric-buffer-size"); metricSize > 0 {
		cfg.MetricBufferSize = metricSize
	}
	if host := cmd.String("otlp-host"); host != "" {
		cfg.OTLPHost = host
	}
	if port := cmd.Int("otlp-port"); port >= 0 { // 0 is valid (ephemeral), -1 means not set
		cfg.OTLPPort = port
	}
	if cmd.IsSet("verbose") { // Only override if explicitly set
		cfg.Verbose = cmd.Bool("verbose")
	}

	// Apply HTTP transport flag overrides
	if transport := cmd.String("transport"); transport != "" {
		cfg.Transport = transport
	}
	if httpHost := cmd.String("http-host"); httpHost != "" {
		cfg.HTTPHost = httpHost
	}
	if httpPort := cmd.Int("http-port"); httpPort > 0 {
		cfg.HTTPPort = httpPort
	}
	if origins := cmd.StringSlice("allowed-origin"); len(origins) > 0 {
		cfg.AllowedOrigins = origins
	}
	if timeout := cmd.Duration("session-timeout"); timeout > 0 {
		cfg.SessionTimeout = timeout.String()
	}
	if cmd.IsSet("stateless") {
		cfg.Stateless = cmd.Bool("stateless")
	}

	if cfg.Verbose {
		log.Println("🔧 Configuration:")
		if configPath != "" {
			log.Printf("  Config file: %s\n", configPath)
		} else if projectPath, err := FindProjectConfig(); err == nil {
			log.Printf("  Config file: %s (project)\n", projectPath)
		} else if globalPath := GlobalConfigPath(); globalPath != "" {
			if _, err := os.Stat(globalPath); err == nil {
				log.Printf("  Config file: %s (global)\n", globalPath)
			}
		}
		log.Printf("  Trace buffer: %d spans\n", cfg.TraceBufferSize)
		log.Printf("  Log buffer: %d records\n", cfg.LogBufferSize)
		log.Printf("  Metric buffer: %d points\n", cfg.MetricBufferSize)
		log.Printf("  OTLP bind: %s:%d\n", cfg.OTLPHost, cfg.OTLPPort)
		log.Println()
	}

	// 1. Create unified observability storage with configured buffer sizes
	obsStorage := storage.NewObservabilityStorage(
		cfg.TraceBufferSize,
		cfg.LogBufferSize,
		cfg.MetricBufferSize,
	)

	if cfg.Verbose {
		log.Printf("✅ Created observability storage:\n")
		log.Printf("   Trace buffer:  %d spans\n", cfg.TraceBufferSize)
		log.Printf("   Log buffer:    %d records\n", cfg.LogBufferSize)
		log.Printf("   Metric buffer: %d points\n", cfg.MetricBufferSize)
	}

	// Check if we're using otel-config mode (file sources only, no OTLP listener by default)
	otelConfigPath := cmd.String("otel-config")
	useOtelConfig := otelConfigPath != ""

	// 2. Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Create and optionally start unified OTLP gRPC receiver
	var otlpServer *otlpreceiver.UnifiedServer
	otlpErrChan := make(chan error, 1)

	if useOtelConfig {
		// When using otel-config, create receiver but don't start it
		// Agent can use add_otlp_port if they need live OTLP ingestion
		var err error
		otlpServer, err = otlpreceiver.NewUnifiedServer(
			otlpreceiver.Config{
				Host: cfg.OTLPHost,
				Port: cfg.OTLPPort,
			},
			obsStorage,
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP receiver: %w", err)
		}
		log.Println("📁 Using file sources from otel-collector config (OTLP listener disabled)")
		log.Println("   💡 Use add_otlp_port tool to enable live OTLP ingestion if needed")
	} else {
		// Normal mode: create and start OTLP receiver
		var err error
		otlpServer, err = otlpreceiver.NewUnifiedServer(
			otlpreceiver.Config{
				Host: cfg.OTLPHost,
				Port: cfg.OTLPPort,
			},
			obsStorage,
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP receiver: %w", err)
		}

		// Start receiver in background
		go func() {
			otlpErrChan <- otlpServer.Start(ctx)
		}()

		// Get the actual endpoint (important for ephemeral ports)
		endpoint := otlpServer.Endpoint()

		log.Printf("🌐 OTLP gRPC receiver listening on: %s\n", endpoint)
		log.Printf("   📡 Accepting: traces, logs, and metrics\n")
		if cfg.Verbose {
			log.Printf("\n   Programs can send all telemetry with:\n")
			log.Printf("   OTEL_EXPORTER_OTLP_ENDPOINT=%s\n", endpoint)
			log.Printf("\n   Or per-signal (all use same endpoint):\n")
			log.Printf("   OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=%s\n", endpoint)
			log.Printf("   OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=%s\n", endpoint)
			log.Printf("   OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=%s\n", endpoint)
		}
	}

	// 4. Create MCP server with unified storage and receiver
	mcpServer, err := mcpserver.NewServer(obsStorage, otlpServer, mcpserver.ServerOptions{
		Verbose: cfg.Verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	if cfg.Verbose {
		log.Println("✅ MCP server created with 12 snapshot-first tools:")
		log.Println("   - get_otlp_endpoint (get primary endpoint)")
		log.Println("   - add_otlp_port (add listening ports on-demand)")
		log.Println("   - remove_otlp_port (remove ports when done)")
		log.Println("   - create_snapshot (bookmark buffer positions)")
		log.Println("   - query (multi-signal query with filters)")
		log.Println("   - get_snapshot_data (time-based query)")
		log.Println("   - manage_snapshots (list/delete/clear)")
		log.Println("   - get_stats (buffer health dashboard)")
		log.Println("   - clear_data (nuclear reset)")
		log.Println("   - set_file_source (load from filesystem)")
		log.Println("   - remove_file_source (stop watching)")
		log.Println("   - list_file_sources (show active sources)")
	}

	// 4. Load file sources from CLI flags (activeOnly=true to skip archives)
	fileSources := cmd.StringSlice("file-source")
	for _, dir := range fileSources {
		if err := mcpServer.AddFileSource(ctx, dir, true); err != nil {
			log.Printf("⚠️  Failed to load file source %s: %v\n", dir, err)
		} else {
			log.Printf("📁 Loaded file source: %s\n", dir)
		}
	}

	// 5. Load file sources from otel-collector config (activeOnly=true to skip archives)
	if useOtelConfig {
		dirs, err := ParseOtelConfig(otelConfigPath)
		if err != nil {
			return fmt.Errorf("failed to parse otel config %s: %w", otelConfigPath, err)
		}
		for _, dir := range dirs {
			if err := mcpServer.AddFileSource(ctx, dir, true); err != nil {
				log.Printf("⚠️  Failed to add file source %s: %v\n", dir, err)
			} else {
				log.Printf("📁 Auto-discovered file source: %s\n", dir)
			}
		}
	}

	// 6. Setup graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		if cfg.Verbose {
			log.Printf("📡 Received signal %v, initiating graceful shutdown...\n", sig)
		}
		cancel()
		if !useOtelConfig && otlpServer != nil {
			otlpServer.Stop()
		}
	}()

	// 7. Run MCP server on selected transport
	switch cfg.Transport {
	case "http":
		// Warn if binding to non-localhost address (security risk)
		if cfg.HTTPHost != "127.0.0.1" && cfg.HTTPHost != "::1" && cfg.HTTPHost != "localhost" {
			log.Printf("⚠️  WARNING: Binding to %s - this server has NO AUTHENTICATION!\n", cfg.HTTPHost)
			log.Println("⚠️  Only bind to localhost (127.0.0.1) unless you understand the security implications.")
		}
		log.Printf("🌐 MCP server starting on http://%s:%d/mcp\n", cfg.HTTPHost, cfg.HTTPPort)
		log.Println("💡 Use MCP tools to query traces and get the OTLP endpoint")
		log.Println("💡 If programs need a specific port, use add_otlp_port to listen on it")
		log.Println()

		if err := runHTTPTransport(ctx, cfg, mcpServer, otlpErrChan); err != nil {
			return err
		}

	case "stdio", "":
		log.Println("🎯 MCP server ready on stdio")
		log.Println("💡 Use MCP tools to query traces and get the OTLP endpoint")
		log.Println("💡 If programs need a specific port, use add_otlp_port to listen on it")
		log.Println()

		if err := mcpServer.Run(ctx); err != nil {
			// Check if OTLP receiver had an error
			select {
			case otlpErr := <-otlpErrChan:
				if otlpErr != nil {
					return fmt.Errorf("OTLP receiver error: %w", otlpErr)
				}
			default:
			}
			return fmt.Errorf("MCP server error: %w", err)
		}

	default:
		return fmt.Errorf("unknown transport: %s (use 'stdio' or 'http')", cfg.Transport)
	}

	return nil
}

// runHTTPTransport starts the MCP server using Streamable HTTP transport.
// It creates an HTTP server with origin validation and graceful shutdown.
func runHTTPTransport(ctx context.Context, cfg *Config, mcpServer *mcpserver.Server, otlpErrChan chan error) error {
	// Parse session timeout
	sessionTimeout, err := time.ParseDuration(cfg.SessionTimeout)
	if err != nil {
		log.Printf("⚠️  Invalid session timeout %q, using default 30m: %v", cfg.SessionTimeout, err)
		sessionTimeout = 30 * time.Minute
	}

	// Create StreamableHTTPHandler from SDK
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer.MCPServer()
		},
		&mcp.StreamableHTTPOptions{
			Stateless:      cfg.Stateless,
			SessionTimeout: sessionTimeout,
		},
	)

	// Wrap with origin validation middleware
	mux := http.NewServeMux()
	mux.Handle("/mcp", originValidationMiddleware(cfg.AllowedOrigins, handler))
	mux.Handle("/mcp/", originValidationMiddleware(cfg.AllowedOrigins, handler))

	// Create HTTP server with proper timeouts
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for shutdown signal or errors
	select {
	case <-ctx.Done():
		// Graceful shutdown
		mcpServer.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)

	case err := <-serverErr:
		return fmt.Errorf("HTTP server error: %w", err)

	case err := <-otlpErrChan:
		if err != nil {
			return fmt.Errorf("OTLP receiver error: %w", err)
		}
		return nil
	}
}

// originValidationMiddleware validates Origin headers and sets CORS headers.
// It supports wildcard patterns like "http://localhost:*".
func originValidationMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// No Origin header - allow (same-origin or non-browser client)
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check against allowed origins
		if !isOriginAllowed(origin, allowedOrigins) {
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}

		// Set CORS headers for allowed origins
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id, Last-Event-Id")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if the origin matches any allowed pattern.
// Supports wildcards: "http://localhost:*" matches "http://localhost:8080".
func isOriginAllowed(origin string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchOriginPattern(origin, pattern) {
			return true
		}
	}
	return false
}

// matchOriginPattern matches an origin against a pattern with wildcard support.
// Supports "*" as a wildcard for the port portion only.
func matchOriginPattern(origin, pattern string) bool {
	// Exact match
	if origin == pattern {
		return true
	}

	// Wildcard match (e.g., "http://localhost:*")
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(origin, prefix) {
			// Verify remaining part is a valid port number (1-65535)
			portStr := strings.TrimPrefix(origin, prefix)
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return false
			}
			return port >= 1 && port <= 65535
		}
	}

	return false
}
