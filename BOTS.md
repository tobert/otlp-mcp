# BOTS.md: Agent Development Guide

Guidance for agents working with this codebase.

## Project Overview

**otlp-mcp** is an MCP server that exposes OpenTelemetry telemetry (traces, logs, metrics) to agents. It enables real-time observability and debugging within agent conversations.

## Technology

- **Language**: Go 1.25+
- **Protocols**: OTLP (gRPC/HTTP), MCP

## Package Structure

```
otlp-mcp/
├── cmd/otlp-mcp/           # Binary entry point
├── internal/
│   ├── cli/                # CLI and config
│   ├── otlpreceiver/       # OTLP gRPC receiver
│   ├── logsreceiver/       # Logs receiver
│   ├── metricsreceiver/    # Metrics receiver
│   ├── storage/            # Ring buffers + snapshots
│   ├── filereader/         # JSONL file source
│   └── mcpserver/          # MCP server + tools
├── systemd/                # Systemd user unit
└── test/                   # E2E tests
```

## Development Commands

```bash
go build -o otlp-mcp ./cmd/otlp-mcp  # Build
go test ./...                         # Test
go fmt ./...                          # Format
go vet ./...                          # Lint
```

## Architecture

```
Agent ←→ MCP Server ←→ Storage ←→ OTLP Server ←→ Programs
                         ↑
                    File Sources (optional)
```

- **OTLP**: Listens on localhost, accepts traces/logs/metrics
- **Storage**: Ring buffers (10K spans, 50K logs, 100K metrics)
- **MCP Tools**: get_otlp_endpoint, query, create_snapshot, get_snapshot_data, etc.

## Contribution

Fork and build your feature on a branch. PR it when you're ready.

Always test and fmt before uploading.

- `go test ./...`
- `go fmt ./...`

### Agent Attribution

Include in commits when agents contribute:

```
Co-Authored-By: Claude <claude@anthropic.com>
Co-Authored-By: Gemini <gemini@google.com>
```

## Cross-Session Handoffs

When handing off work:
- Commit with clear description of state
- Note what's done and what's next
- Push to a branch so next session can continue

## License

Apache License 2.0 - Copyright (c) 2025 Amy Tobey
