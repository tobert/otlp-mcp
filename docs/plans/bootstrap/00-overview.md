# Bootstrap Plan Overview

## Vision

Enable AI agents to observe and analyze telemetry from programs they execute in a tight feedback loop.

## Workflow

```
┌─────────────────────────────────────────────────────────────┐
│ Agent starts: otlp-mcp serve                                │
│                                                              │
│ ┌─────────────────────────┐                                 │
│ │  otlp-mcp process       │                                 │
│ │                         │                                 │
│ │  ┌──────────────────┐   │                                 │
│ │  │ MCP Server       │   │  stdio ◄──────┐                │
│ │  │ (stdio)          │◄──┼────────────────┼─── Agent       │
│ │  └──────────────────┘   │                │                │
│ │                         │                │                │
│ │  ┌──────────────────┐   │                │                │
│ │  │ OTLP gRPC Server │   │                │                │
│ │  │ (localhost:XXXX) │   │  localhost:54321 (ephemeral)   │
│ │  └──────────────────┘   │                │                │
│ │           ▲             │                │                │
│ │           │             │                │                │
│ │  ┌────────┴─────────┐   │                │                │
│ │  │ Ring Buffers     │   │                │                │
│ │  │ - Traces         │   │                │                │
│ │  │ - Logs (future)  │   │                │                │
│ │  │ - Metrics (fut.) │   │                │                │
│ │  └──────────────────┘   │                │                │
│ └─────────────────────────┘                │                │
│                                            │                │
│ 1. Agent: get_otlp_endpoint() ─────────────┘                │
│    Returns: "localhost:54321"                               │
│                                                              │
│ 2. Agent runs: OTEL_EXPORTER_OTLP_ENDPOINT=localhost:54321 \│
│               my-program --args                             │
│                                                              │
│ 3. Program emits traces ──────────► OTLP Server ──► Buffer  │
│                                                              │
│ 4. Agent: query_traces(service="my-program")                │
│    Returns: [trace data...]                                 │
│                                                              │
│ 5. Agent analyzes, iterates, debugs                         │
└─────────────────────────────────────────────────────────────┘
```

## Architecture Decisions

### Single Binary, Multiple Modes
- Command: `otlp-mcp serve`
- MCP server: stdio transport
- OTLP server: gRPC on localhost:0 (ephemeral port)
- Both run in same process

### CLI Framework
- **NOT cobra** - looking for something modern and simple
- Options: `ff`, `urfave/cli` v3, `kong`, or even just `flag` package
- Preference: Minimal dependencies, clear API

### OTLP Protocol
- **gRPC only** for MVP (port auto-assigned via localhost:0)
- HTTP support deferred to later
- Traces first, logs and metrics in follow-on tasks

### Storage Strategy
- **Fixed-size ring buffers** per signal type
- Not time-based (prompts can sit for days)
- Configurable via CLI flags
- Defaults:
  - Traces: 10,000 spans
  - Logs: 50,000 entries (future)
  - Metrics: 100,000 points (future)

### MCP Interface

**Tools:**
- `get_otlp_endpoint` - Returns current OTLP gRPC endpoint
- `get_recent_traces` - List recent trace spans
- `get_trace_by_id` - Fetch specific trace by trace ID
- `query_traces` - Filter traces by service, attributes, status
- `clear_traces` - Clear the trace buffer
- `get_stats` - Get buffer stats (size, capacity, oldest/newest)

**Resources:**
- `otlp://config` - Current OTLP server configuration
- `otlp://traces` - All traces in buffer (may be large!)

### Localhost Only
- All network binding on 127.0.0.1
- No TLS needed
- No authentication needed
- Simplifies security model

## Implementation Tasks

Each numbered file in this directory is a self-contained task:

1. **01-project-setup.md** - Initialize Go module, dependencies, directory structure
2. **02-cli-framework.md** - Choose and implement CLI framework, basic serve command
3. **03-otlp-receiver.md** - Copy and refactor OTLP gRPC server from otel-cli
4. **04-ring-buffer.md** - Implement generic ring buffer storage
5. **05-mcp-server.md** - Implement MCP stdio server with trace tools
6. **06-integration.md** - Wire OTLP + MCP together, test end-to-end
7. **07-documentation.md** - README, examples, troubleshooting, polish

## Success Criteria

MVP is complete when:

1. ✅ Agent can start `otlp-mcp serve`
2. ✅ Agent can call `get_otlp_endpoint` and receive `localhost:XXXXX`
3. ✅ Agent can run program with `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:XXXXX`
4. ✅ Program's traces arrive at OTLP server and are stored
5. ✅ Agent can query traces via MCP tools
6. ✅ Agent can analyze trace data in conversation
7. ✅ Ring buffer properly manages memory with configurable limits

## Non-Goals for MVP

- HTTP/protobuf OTLP support (gRPC only)
- TLS/authentication (localhost only)
- Persistence to disk (memory only)
- Time-based retention (ring buffer only)
- Metrics and logs (traces only)
- Multiple MCP clients (stdio is 1:1)
- Remote connections (localhost only)

## Future Enhancements

After MVP:
- Logs and metrics support
- HTTP OTLP endpoint
- WebSocket MCP transport (multi-client)
- Export/import traces to files
- Trace visualization tools via MCP
- Query language for complex filters
- Sampling strategies
