package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tobert/otlp-mcp/internal/cli"
	cliframework "github.com/urfave/cli/v3"
)

const version = "0.3.0"

func main() {
	serveCmd := cli.ServeCommand()

	app := &cliframework.Command{
		Name:    "otlp-mcp",
		Usage:   "OTLP MCP server for AI agent observability",
		Version: version,
		// Default to serve when no subcommand provided
		Action: serveCmd.Action,
		Flags:  serveCmd.Flags,
		Commands: []*cliframework.Command{
			serveCmd,
			cli.DoctorCommand(version),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "❌ error: %v\n", err)
		os.Exit(1)
	}
}
