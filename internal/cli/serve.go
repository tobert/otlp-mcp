package cli

import (
	"context"
	"fmt"

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
// It extracts configuration from CLI flags and starts the server.
// The actual server implementation will be added in tasks 03-06.
func runServe(ctx context.Context, cmd *cli.Command) error {
	cfg := &Config{
		TraceBufferSize:  cmd.Int("trace-buffer-size"),
		LogBufferSize:    cmd.Int("log-buffer-size"),
		MetricBufferSize: cmd.Int("metric-buffer-size"),
		OTLPHost:         cmd.String("otlp-host"),
		OTLPPort:         cmd.Int("otlp-port"),
		Verbose:          cmd.Bool("verbose"),
	}

	if cfg.Verbose {
		fmt.Println("🔧 Configuration:")
		fmt.Printf("  Trace buffer: %d spans\n", cfg.TraceBufferSize)
		fmt.Printf("  Log buffer: %d records\n", cfg.LogBufferSize)
		fmt.Printf("  Metric buffer: %d points\n", cfg.MetricBufferSize)
		fmt.Printf("  OTLP endpoint: %s:%d\n", cfg.OTLPHost, cfg.OTLPPort)
		fmt.Println()
	}

	fmt.Println("🚀 Starting OTLP MCP server...")
	fmt.Printf("  Trace buffer: %d spans\n", cfg.TraceBufferSize)
	fmt.Printf("  OTLP endpoint: %s:%d\n", cfg.OTLPHost, cfg.OTLPPort)

	// Actual server startup will be implemented in tasks 03-06
	// For now, just validate configuration and return

	return fmt.Errorf("not yet implemented - server components pending (tasks 03-06)")
}
