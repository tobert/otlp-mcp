package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tobert/otlp-mcp/internal/cli"
	cliframework "github.com/urfave/cli/v3"
)

// Set by goreleaser ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	fullVersion := fmt.Sprintf("%s (%s, %s)", version, commit, date)

	serveCmd := cli.ServeCommand()

	app := &cliframework.Command{
		Name:    "otlp-mcp",
		Usage:   "OTLP MCP server for AI agent observability",
		Version: fullVersion,
		// Default to serve when no subcommand provided
		Action: serveCmd.Action,
		Flags:  serveCmd.Flags,
		Commands: []*cliframework.Command{
			serveCmd,
			cli.DoctorCommand(fullVersion),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "❌ error: %v\n", err)
		os.Exit(1)
	}
}
