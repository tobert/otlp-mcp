# otlp-mcp

**OpenTelemetry observability for coding agents** - Capture and analyze traces, logs, and metrics from programs your agent executes.

## Quick Start

```bash
# Install
go install github.com/tobert/otlp-mcp/cmd/otlp-mcp@latest

# Add to Claude Code
claude mcp add otlp-mcp $(go env GOPATH)/bin/otlp-mcp

# Verify (ask your agent)
"What is the OTLP endpoint address?"
```

## What It Does

```
Agent ←→ MCP Server ←→ Ring Buffer ←→ OTLP Server ←→ Your Programs
```

1. Agent calls `get_otlp_endpoint` to get the listening address
2. Run programs with `OTEL_EXPORTER_OTLP_ENDPOINT=<endpoint>`
3. Query telemetry with MCP tools: `query`, `get_stats`, `create_snapshot`

## MCP Tools

| Tool | Purpose |
|------|---------|
| `get_otlp_endpoint` | Get the OTLP endpoint for programs |
| `query` | Search traces/logs/metrics with filters |
| `create_snapshot` | Bookmark a point in time |
| `get_snapshot_data` | Compare before/after snapshots |
| `get_stats` | Check buffer usage |
| `add_otlp_port` | Listen on additional ports |
| `set_file_source` | Load from otel-collector file exports |

## Example: Before/After Comparison

```
You: Create a snapshot called "before"
Agent: [creates snapshot]

You: Run the tests
Agent: OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go test ./...

You: Create a snapshot called "after"
Agent: [creates snapshot]

You: What changed?
Agent: [compares snapshots - shows new errors, timing changes, etc.]
```

## Configuration

**Stable port** (for watch workflows):

```json
// .otlp-mcp.json in project root
{
  "otlp_port": 4317
}
```

**CLI flags:**
- `--otlp-port <port>` - Fixed port instead of ephemeral
- `--verbose` - Detailed logging
- `--config <path>` - Config file path

## Security

**Local development only.** No authentication, no encryption. Bind to localhost only.

## Troubleshooting

**No traces?** Check the endpoint matches what `get_otlp_endpoint` returns.

**Connection refused?** Ensure you're using `127.0.0.1`, not a remote address.

**Agent restarted?** Use `add_otlp_port` to listen on the port your program expects.

## Contributing

PRs welcome! Agent-assisted contributions encouraged - include `Co-Authored-By` for attribution.

See [CLAUDE.md](CLAUDE.md) for development guidelines.

## License

Apache License 2.0 - Copyright (c) 2025 Amy Tobey
