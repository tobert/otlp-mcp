# otlp-mcp

Expose OTLP (OpenTelemetry Protocol) telemetry to AI agents via MCP (Model Context Protocol).

## Status

🚧 **In Development** - Bootstrap MVP implementation in progress.

See `docs/plans/bootstrap/` for the complete implementation plan.

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

## Quick Start (Post-MVP)

```bash
# Start the server
otlp-mcp serve

# Agent queries for OTLP endpoint via MCP
# Then runs program:
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:XXXXX my-program

# Agent queries traces via MCP tools
# Analyzes, debugs, iterates
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
