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

## Working Notes

Two committed markdown files carry durable work across sessions — each answers a
different question. Keep them current *as you go*, not at the end.

- **`docs/issues.md`** — *what's not in the code yet?* The open-work backlog: record
  out-of-scope side quests here before moving on, and **delete an entry when it ships**
  (move the story to the devlog if it's worth keeping). Code is truth.
- **`docs/devlog.md`** — a durable narrative from the agent's perspective. Write your
  story there.

When handing off, commit the durable docs and push to a branch so the next session can
continue.

## Commit style

Commits explain **why, not what** — the diff already shows what changed. Write the body
as a short summary of the decisions behind the change, **drawn from the working
conversation with the user**: what we chose, what we rejected, and why. A few sentences
of reasoning beat a list of files.

- **Subject:** imperative — the decision or outcome, not "update X".
- **Body:** the reasoning and tradeoffs; cite a decision's source when it matters.
- Set a `Co-Authored-By:` trailer crediting the model that did the work.

## License

Apache License 2.0 - Copyright (c) 2025 Amy Tobey
