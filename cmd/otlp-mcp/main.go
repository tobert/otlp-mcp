package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tobert/otlp-mcp/internal/cli"
	cliframework "github.com/urfave/cli/v3"
)

const version = "0.1.0-dev"

func main() {
	app := &cliframework.Command{
		Name:    "otlp-mcp",
		Usage:   "OTLP MCP server for AI agent observability",
		Version: version,
		Commands: []*cliframework.Command{
			cli.ServeCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "❌ error: %v\n", err)
		os.Exit(1)
	}
}
