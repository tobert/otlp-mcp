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
├── cmd/otlp-mcp/           # Binary entry point (version injected via ldflags)
├── internal/
│   ├── cli/                # CLI and config
│   ├── otlpreceiver/       # OTLP gRPC receiver
│   ├── logsreceiver/       # Logs receiver
│   ├── metricsreceiver/    # Metrics receiver
│   ├── storage/            # Ring buffers + snapshots
│   ├── filereader/         # JSONL file source
│   └── mcpserver/          # MCP server + tools
├── release/                # GoReleaser Dockerfile (multi-arch)
├── systemd/                # Systemd user unit
└── test/                   # E2E tests
```

## Development Commands

```bash
go build -o otlp-mcp ./cmd/otlp-mcp  # Build
go test ./...                         # Test
go fmt ./...                          # Format
go vet ./...                          # Lint
make release-snapshot                 # Local goreleaser build (binaries + packages)
```

## Architecture

```
Agent ←→ MCP Server ←→ Storage ←→ OTLP Server ←→ Programs
                         ↑
                    File Sources (optional)
```

- **OTLP**: Listens on localhost, accepts traces/logs/metrics
- **Storage**: Ring buffers (10K spans, 50K logs, 100K metrics)

## MCP Tools

When otlp-mcp is connected, use these tools to observe telemetry:

```
# Get the OTLP endpoint for instrumented programs
get_otlp_endpoint() → {"endpoint": "127.0.0.1:4317", "protocol": "grpc"}

# Check buffer stats
get_stats() → span_count, log_count, metric_count, services

# Query telemetry with filters
query(service_name: "my-service")
query(errors_only: true)
query(min_duration_ns: 500000000)  # Slow spans > 500ms

# Snapshot workflow for before/after comparison
create_snapshot(name: "before-fix")
# ... run tests or make changes ...
create_snapshot(name: "after-fix")
get_snapshot_data(start_snapshot: "before-fix", end_snapshot: "after-fix")

# Load telemetry from otel-collector file exports
set_file_source(directory: "/tank/otel")
list_file_sources()
```

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
