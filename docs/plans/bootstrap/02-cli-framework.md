# Task 02: CLI Framework & Serve Command

## Why

Need a simple, modern CLI framework to handle the `serve` subcommand and configuration flags. We explicitly don't want Cobra - looking for something lighter and more modern.

## What

Implement:
- CLI framework (NOT cobra)
- `serve` subcommand
- Configuration flags for ring buffer sizes
- Clean help output
- Version command

## Approach

### Framework Selection

**Option 1: urfave/cli v3** (Recommended)
- Modern, actively maintained
- Clean API, good documentation
- Supports subcommands, flags, environment variables
- Reasonable size (~15KB)

**Option 2: peterbourgon/ff/v4**
- Minimal, functional style
- Excellent flag/env/config file layering
- Very small footprint

**Option 3: Standard library `flag`**
- Zero dependencies
- Simple subcommand handling with custom routing
- Most control, most boilerplate

**Recommendation: Start with urfave/cli v3** - good balance of features and simplicity.

### Command Structure

```
otlp-mcp - OTLP MCP server for AI agent observability

USAGE:
   otlp-mcp [global options] command [command options]

COMMANDS:
   serve      Start the OTLP receiver and MCP server
   version    Show version information
   help       Show help

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version

SERVE COMMAND:
   otlp-mcp serve [options]

   Starts an OTLP gRPC receiver on localhost:0 (ephemeral port) and an
   MCP server on stdio. The agent can query the OTLP endpoint and trace
   data via MCP tools.

OPTIONS:
   --trace-buffer-size value    Number of spans to buffer (default: 10000)
   --log-buffer-size value      Number of log records to buffer (default: 50000)
   --metric-buffer-size value   Number of metric points to buffer (default: 100000)
   --otlp-host value           OTLP server bind address (default: "127.0.0.1")
   --otlp-port value           OTLP server port, 0 for ephemeral (default: 0)
   --verbose, -v               Enable verbose logging
```

### Configuration Structure

```go
// internal/cli/config.go
package cli

type Config struct {
    // Buffer sizes
    TraceBufferSize  int
    LogBufferSize    int
    MetricBufferSize int

    // OTLP server
    OTLPHost string
    OTLPPort int

    // Logging
    Verbose bool
}

func DefaultConfig() *Config {
    return &Config{
        TraceBufferSize:  10_000,
        LogBufferSize:    50_000,
        MetricBufferSize: 100_000,
        OTLPHost:        "127.0.0.1",
        OTLPPort:        0, // ephemeral
        Verbose:         false,
    }
}
```

### Main Entry Point

```go
// cmd/otlp-mcp/main.go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/tobert/otlp-mcp/internal/cli"
    "github.com/urfave/cli/v3"
)

const version = "0.1.0-dev"

func main() {
    app := &cli.Command{
        Name:    "otlp-mcp",
        Usage:   "OTLP MCP server for AI agent observability",
        Version: version,
        Commands: []*cli.Command{
            cli.ServeCommand(),
        },
    }

    if err := app.Run(context.Background(), os.Args); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
```

### Serve Command Stub

```go
// internal/cli/serve.go
package cli

import (
    "fmt"

    "github.com/urfave/cli/v3"
)

func ServeCommand() *cli.Command {
    return &cli.Command{
        Name:  "serve",
        Usage: "Start the OTLP receiver and MCP server",
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
                Name:  "verbose",
                Usage: "Enable verbose logging",
            },
        },
        Action: runServe,
    }
}

func runServe(c *cli.Context) error {
    cfg := &Config{
        TraceBufferSize:  c.Int("trace-buffer-size"),
        LogBufferSize:    c.Int("log-buffer-size"),
        MetricBufferSize: c.Int("metric-buffer-size"),
        OTLPHost:        c.String("otlp-host"),
        OTLPPort:        c.Int("otlp-port"),
        Verbose:         c.Bool("verbose"),
    }

    fmt.Printf("Starting OTLP MCP server...\n")
    fmt.Printf("  Trace buffer: %d spans\n", cfg.TraceBufferSize)
    fmt.Printf("  OTLP endpoint: %s:%d\n", cfg.OTLPHost, cfg.OTLPPort)

    // Actual server startup will be implemented in tasks 03-06
    // For now, just validate configuration

    return fmt.Errorf("not yet implemented")
}
```

## Dependencies

- Task 01 (project-setup) must be complete

## Acceptance Criteria

- [ ] CLI framework dependency added to go.mod
- [ ] `otlp-mcp --help` shows usage
- [ ] `otlp-mcp --version` shows version
- [ ] `otlp-mcp serve --help` shows serve options
- [ ] `otlp-mcp serve` runs (even if it errors with "not implemented")
- [ ] All flags parse correctly
- [ ] Config struct properly populated from flags

## Testing

```bash
# Build
go build ./cmd/otlp-mcp

# Test help
./otlp-mcp --help
./otlp-mcp serve --help

# Test version
./otlp-mcp --version

# Test serve with custom flags
./otlp-mcp serve --trace-buffer-size 5000 --verbose

# Should show:
# Starting OTLP MCP server...
#   Trace buffer: 5000 spans
#   OTLP endpoint: 127.0.0.1:0
# error: not yet implemented
```

## Notes

- Keep it simple - we don't need complex config files for MVP
- All config via CLI flags is fine for now
- Version string should include git commit in future builds
- Consider adding `--debug` flag for development

## Status

Status: pending
Next: 03-otlp-receiver.md
