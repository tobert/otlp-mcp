# otlp-mcp

**OTLP MCP Server** - Expose OpenTelemetry traces to AI agents via the Model Context Protocol

## Status

✅ **MVP Complete** - Bootstrap implementation finished and tested.

See `docs/plans/bootstrap/` for the implementation plan.

## Vision

Enable AI agents to observe and analyze telemetry from programs they execute in a tight feedback loop. The agent starts `otlp-mcp serve`, runs instrumented programs pointing to the OTLP endpoint, and queries trace data via MCP tools to debug and iterate.

## Architecture

```
Agent (stdio) ←→ MCP Server ←→ Ring Buffer ←→ OTLP gRPC Server ←→ Your Programs
```

**MVP Scope:**
- Single binary: `otlp-mcp serve`
- OTLP receiver: gRPC on localhost (ephemeral port)
- MCP server: stdio transport
- Storage: In-memory ring buffer for traces
- Localhost only, no authentication needed

## Quick Start

### Build

```bash
go build -o otlp-mcp ./cmd/otlp-mcp
```

### Configure in Claude Code

Add to your MCP settings (`~/.config/claude-code/mcp_settings.json`):

```json
{
  "mcpServers": {
    "otlp-mcp": {
      "command": "/home/atobey/src/otlp-mcp/otlp-mcp",
      "args": ["serve", "--verbose"]
    }
  }
}
```

**Important:** Use the absolute path to your built binary.

### Restart Claude Code

After adding the configuration, restart Claude Code to load the MCP server.

### Try It!

In Claude Code, ask:

```
Use the get_otlp_endpoint tool to find the OTLP address
```

You'll get back something like:
```json
{
  "endpoint": "localhost:54321",
  "protocol": "grpc"
}
```

Now run an instrumented program:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:54321 your-program
```

Then query traces:

```
Show me recent traces
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `get_otlp_endpoint` | Returns OTLP gRPC endpoint address |
| `get_recent_traces` | Returns N most recent spans (default: 100) |
| `get_trace_by_id` | Fetches all spans for a specific trace ID |
| `query_traces` | Filters by service name or span name |
| `get_stats` | Returns buffer statistics |
| `clear_traces` | Clears all stored traces |

## Example Workflow

```bash
# Terminal 1: Claude Code discovers endpoint
# "Use get_otlp_endpoint tool"
# Response: localhost:54321

# Terminal 2: Send test span (if you have otel-cli)
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:54321 \
  otel-cli span --service my-app --name "test-span"

# Claude Code: Query traces
# "Show me traces from my-app"
# "Get statistics on the trace buffer"
```

## Development

See [CLAUDE.md](CLAUDE.md) for:
- Jujutsu (jj) workflow
- Go 1.25+ code style
- Agent collaboration patterns

See [docs/plans/bootstrap/](docs/plans/bootstrap/) for:
- Task-by-task implementation plan
- Architecture diagrams
- Acceptance criteria

## License

MIT License - Copyright (c) 2025 Amy Tobey
