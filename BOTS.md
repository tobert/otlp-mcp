# CLAUDE.md: Agent Development Guide

Guidance for agents working with this codebase.

## Project Overview

**otlp-mcp** is an MCP server that exposes OpenTelemetry telemetry (traces, logs, metrics) to agents. It enables real-time observability and debugging within agent conversations.

## Technology

- **Language**: Go 1.25+
- **Protocols**: OTLP (gRPC/HTTP), MCP
- **Source**: Refactored from [otel-cli](https://github.com/tobert/otel-cli)

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

## Code Style

- Readable, idiomatic Go
- Full descriptive names
- "Why" comments only
- Handle all errors with context (`%w`)
- Table-driven tests

## Git Workflow

Use standard git with feature branches and PRs:

```bash
git checkout -b feat/my-feature
# ... make changes ...
go test ./...
git add -A && git commit
git push -u origin feat/my-feature
gh pr create
```

**PR Requirements:**
- Test before PR (`go test ./...`)
- One logical change per PR
- Include `Co-Authored-By` for agent contributions

## Agent Attribution

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
