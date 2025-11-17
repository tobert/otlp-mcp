package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
MCP server on stdio. The agent can query the OTLP endpoint and trace
data via MCP tools.`,
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
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging (overrides config file)",
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

	// 2. Create and start unified OTLP gRPC receiver (all signals on one port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create unified receiver for all signal types
	otlpServer, err := otlpreceiver.NewUnifiedServer(
		otlpreceiver.Config{
			Host: cfg.OTLPHost,
			Port: cfg.OTLPPort,
		},
		obsStorage, // Implements ReceiveSpans, ReceiveLogs, ReceiveMetrics
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP receiver: %w", err)
	}

	// Start receiver in background
	otlpErrChan := make(chan error, 1)
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

	// 3. Create MCP server with unified storage and receiver
	mcpServer, err := mcpserver.NewServer(obsStorage, otlpServer)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	if cfg.Verbose {
		log.Println("✅ MCP server created with 9 snapshot-first tools:")
		log.Println("   - get_otlp_endpoint (get primary endpoint)")
		log.Println("   - add_otlp_port (add listening ports on-demand)")
		log.Println("   - remove_otlp_port (remove ports when done)")
		log.Println("   - create_snapshot (bookmark buffer positions)")
		log.Println("   - query (multi-signal query with filters)")
		log.Println("   - get_snapshot_data (time-based query)")
		log.Println("   - manage_snapshots (list/delete/clear)")
		log.Println("   - get_stats (buffer health dashboard)")
		log.Println("   - clear_data (nuclear reset)")
	}

	// 4. Setup graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		if cfg.Verbose {
			log.Printf("📡 Received signal %v, initiating graceful shutdown...\n", sig)
		}
		cancel()
		otlpServer.Stop()
	}()

	// 5. Run MCP server on stdio (blocks until stdin closes or context cancelled)
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

	return nil
}
