package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tobert/otlp-mcp/internal/logsreceiver"
	"github.com/tobert/otlp-mcp/internal/mcpserver"
	"github.com/tobert/otlp-mcp/internal/metricsreceiver"
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
			&cli.IntFlag{
				Name:  "trace-buffer-size",
				Usage: "Number of spans to buffer",
				Value: 10_000,
			},
			&cli.IntFlag{
				Name:  "log-buffer-size",
				Usage: "Number of log records to buffer",
				Value: 50_000,
			},
			&cli.IntFlag{
				Name:  "metric-buffer-size",
				Usage: "Number of metric points to buffer",
				Value: 100_000,
			},
			&cli.StringFlag{
				Name:  "otlp-host",
				Usage: "OTLP server bind address",
				Value: "127.0.0.1",
			},
			&cli.IntFlag{
				Name:  "otlp-port",
				Usage: "OTLP server port (0 for ephemeral)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose logging",
			},
		},
		Action: runServe,
	}
}

// runServe is the action handler for the serve command.
// It wires together all components: storage, OTLP receiver, and MCP server.
func runServe(cliCtx context.Context, cmd *cli.Command) error {
	cfg := &Config{
		TraceBufferSize:  cmd.Int("trace-buffer-size"),
		LogBufferSize:    cmd.Int("log-buffer-size"),
		MetricBufferSize: cmd.Int("metric-buffer-size"),
		OTLPHost:         cmd.String("otlp-host"),
		OTLPPort:         cmd.Int("otlp-port"),
		Verbose:          cmd.Bool("verbose"),
	}

	if cfg.Verbose {
		log.Println("🔧 Configuration:")
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

	// 2. Create and start all OTLP gRPC receivers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create trace receiver using unified storage
	traceServer, err := otlpreceiver.NewServer(
		otlpreceiver.Config{
			Host: cfg.OTLPHost,
			Port: cfg.OTLPPort,
		},
		obsStorage, // Implements ReceiveSpans
	)
	if err != nil {
		return fmt.Errorf("failed to create trace receiver: %w", err)
	}

	// Create logs receiver (ephemeral port) using unified storage
	logsServer, err := logsreceiver.NewServer(
		logsreceiver.Config{
			Host: cfg.OTLPHost,
			Port: 0, // ephemeral
		},
		obsStorage, // Implements ReceiveLogs
	)
	if err != nil {
		return fmt.Errorf("failed to create logs receiver: %w", err)
	}

	// Create metrics receiver (ephemeral port) using unified storage
	metricsServer, err := metricsreceiver.NewServer(
		metricsreceiver.Config{
			Host: cfg.OTLPHost,
			Port: 0, // ephemeral
		},
		obsStorage, // Implements ReceiveMetrics
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics receiver: %w", err)
	}

	// Start all receivers in background
	traceErrChan := make(chan error, 1)
	logsErrChan := make(chan error, 1)
	metricsErrChan := make(chan error, 1)

	go func() {
		traceErrChan <- traceServer.Start(ctx)
	}()

	go func() {
		logsErrChan <- logsServer.Start(ctx)
	}()

	go func() {
		metricsErrChan <- metricsServer.Start(ctx)
	}()

	// Get the actual endpoints (important for ephemeral ports)
	traceEndpoint := traceServer.Endpoint()
	logsEndpoint := logsServer.Endpoint()
	metricsEndpoint := metricsServer.Endpoint()

	log.Printf("🌐 OTLP gRPC receivers listening:\n")
	log.Printf("   Traces:  %s\n", traceEndpoint)
	log.Printf("   Logs:    %s\n", logsEndpoint)
	log.Printf("   Metrics: %s\n", metricsEndpoint)
	if cfg.Verbose {
		log.Printf("\n   Programs can send telemetry with:\n")
		log.Printf("   OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=%s\n", traceEndpoint)
		log.Printf("   OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=%s\n", logsEndpoint)
		log.Printf("   OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=%s\n", metricsEndpoint)
	}

	// 3. Create MCP server with unified storage and all endpoints
	mcpServer, err := mcpserver.NewServer(obsStorage, mcpserver.Endpoints{
		Traces:  traceEndpoint,
		Logs:    logsEndpoint,
		Metrics: metricsEndpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	if cfg.Verbose {
		log.Println("✅ MCP server created with 5 snapshot-first tools:")
		log.Println("   - get_otlp_endpoints (all three signal types)")
		log.Println("   - create_snapshot (bookmark buffer positions)")
		log.Println("   - query (multi-signal query with filters)")
		log.Println("   - get_snapshot_data (time-based query)")
		log.Println("   - manage_snapshots (list/delete/clear)")
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
		traceServer.Stop()
		logsServer.Stop()
		metricsServer.Stop()
	}()

	// 5. Run MCP server on stdio (blocks until stdin closes or context cancelled)
	log.Println("🎯 MCP server ready on stdio")
	log.Println("💡 Use MCP tools to query traces and get the OTLP endpoint")
	log.Println()

	if err := mcpServer.Run(ctx); err != nil {
		// Check if any OTLP receiver had an error
		select {
		case traceErr := <-traceErrChan:
			if traceErr != nil {
				return fmt.Errorf("trace receiver error: %w", traceErr)
			}
		case logsErr := <-logsErrChan:
			if logsErr != nil {
				return fmt.Errorf("logs receiver error: %w", logsErr)
			}
		case metricsErr := <-metricsErrChan:
			if metricsErr != nil {
				return fmt.Errorf("metrics receiver error: %w", metricsErr)
			}
		default:
		}

		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}
