# Task 07: Documentation & Polish

## Why

Complete the MVP with user-facing documentation, examples, and polish for a great developer experience.

## What

Create:
- Updated README.md with quickstart guide
- Example workflows showing agent usage
- Troubleshooting guide
- Performance characteristics documentation

## Approach

### README.md Structure

```markdown
# otlp-mcp

OTLP MCP server for AI agent observability

## What is this?

otlp-mcp enables AI agents to observe and analyze telemetry from programs they execute. It runs an OTLP (OpenTelemetry Protocol) receiver and exposes the telemetry data via MCP (Model Context Protocol), creating a tight feedback loop for agent-driven development and debugging.

## Quick Start

### Installation

```bash
go install github.com/tobert/otlp-mcp/cmd/otlp-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/tobert/otlp-mcp.git
cd otlp-mcp
go build ./cmd/otlp-mcp
```

### Usage

Start the server:

```bash
otlp-mcp serve
```

This starts:
- OTLP gRPC receiver on localhost (ephemeral port)
- MCP server on stdio

The agent can then:
1. Query the OTLP endpoint via MCP
2. Run programs with telemetry pointing to that endpoint
3. Analyze traces via MCP tools

## Architecture

```
Agent ←→ MCP (stdio) ←→ otlp-mcp ←→ OTLP (gRPC) ←→ Your Program
```

## MCP Tools

- `get_otlp_endpoint` - Get the OTLP receiver endpoint
- `get_recent_traces` - Retrieve recent trace spans
- `get_trace_by_id` - Fetch a specific trace
- `query_traces` - Filter traces by service, name, etc.
- `get_stats` - Get buffer statistics
- `clear_traces` - Clear the trace buffer

## Configuration

```bash
otlp-mcp serve [options]

Options:
  --trace-buffer-size    Number of spans to buffer (default: 10000)
  --otlp-host           OTLP bind address (default: 127.0.0.1)
  --otlp-port           OTLP port, 0 for ephemeral (default: 0)
  --verbose             Enable verbose logging
```

## Example Workflow

See [docs/examples/](docs/examples/) for complete workflows.

## Development

See [CLAUDE.md](CLAUDE.md) for development guidelines and jj workflow.

## License

MIT License - see [LICENSE](LICENSE)
```

### Example Workflows

Create `docs/examples/debugging-with-traces.md`:

```markdown
# Example: Debugging a Program with Traces

This example shows how an agent can use otlp-mcp to debug a program by examining its traces.

## Scenario

You're debugging a web service that occasionally returns 500 errors. You want to understand what's happening.

## Agent Workflow

### 1. Start otlp-mcp

```bash
otlp-mcp serve
```

### 2. Get OTLP endpoint (via MCP)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_otlp_endpoint"
  }
}
```

Response:
```json
{
  "endpoint": "localhost:54321",
  "protocol": "grpc"
}
```

### 3. Run the program with OTLP enabled

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:54321 \
OTEL_SERVICE_NAME=my-web-service \
  ./my-web-service
```

### 4. Generate some traffic

```bash
# Make requests, some will fail
for i in {1..100}; do
  curl http://localhost:8080/api/endpoint
done
```

### 5. Query for error traces (via MCP)

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "query_traces",
    "arguments": {
      "service_name": "my-web-service"
    }
  }
}
```

### 6. Analyze the traces

The agent examines the traces and finds:
- Most requests complete in 50-100ms
- Failed requests all show a span named "database_query" with 5+ second duration
- Error spans have attribute `error.type=timeout`

### 7. Root cause identified

The agent concludes: database query timeouts are causing the 500 errors. Suggests:
- Increase database connection pool
- Add query timeout handling
- Consider caching frequently accessed data

## Benefits

- **Tight feedback loop**: No need to ship logs/traces to external systems
- **Context preserved**: All telemetry available to agent in conversation
- **Fast iteration**: Agent can run, observe, and adjust quickly
```

### Troubleshooting Guide

Create `docs/troubleshooting.md`:

```markdown
# Troubleshooting

## MCP server not responding

**Problem**: Agent can't communicate with MCP server

**Solutions**:
- Ensure server was started with `otlp-mcp serve`
- Check that agent is using stdio transport
- Verify JSON-RPC 2.0 message format
- Run with `--verbose` to see debug output

## OTLP traces not arriving

**Problem**: Program sends traces but they don't appear in queries

**Solutions**:
- Verify endpoint matches: use `get_otlp_endpoint` tool
- Check that program is configured for gRPC (not HTTP)
- Ensure OTEL_EXPORTER_OTLP_ENDPOINT is set correctly
- Check for network firewalls (shouldn't be an issue on localhost)
- Run with `--verbose` to see incoming spans

## Buffer full / traces missing

**Problem**: Old traces are missing

**Explanation**: Ring buffer has limited capacity. Oldest traces are evicted when buffer fills.

**Solutions**:
- Increase buffer size: `--trace-buffer-size 50000`
- Clear buffer between test runs: use `clear_traces` tool
- Query sooner after running tests

## Performance issues

**Problem**: Server is slow or using too much memory

**Solutions**:
- Reduce buffer sizes
- Clear buffer more frequently
- Check for very large span attributes
```

### Performance Documentation

Create `docs/performance.md`:

```markdown
# Performance Characteristics

## Memory Usage

### Trace Buffer

Default capacity: 10,000 spans

Estimated memory per span:
- Span protobuf: ~500 bytes - 2 KB (varies with attributes)
- Index overhead: ~100 bytes
- Total: ~0.6 - 2.5 KB per span

**Total estimated memory**: 6-25 MB for default buffer

### Scaling

Buffer size is configurable. For reference:

| Buffer Size | Estimated Memory |
|------------|------------------|
| 1,000      | 0.6-2.5 MB      |
| 10,000     | 6-25 MB         |
| 100,000    | 60-250 MB       |
| 1,000,000  | 600 MB - 2.5 GB |

## Latency

### OTLP Ingestion

- gRPC overhead: <1ms on localhost
- Buffer insertion: O(1), typically <10µs
- Index update: O(1), typically <50µs

**Total**: <2ms for span ingestion

### MCP Queries

- `get_recent_traces(100)`: <5ms
- `get_trace_by_id`: O(1) lookup, <1ms
- `query_traces` by service: O(n) scan, 10-100ms for 10k spans

## Concurrency

All operations are thread-safe:
- Concurrent OTLP ingestion: supported
- Concurrent MCP queries: supported
- Reads don't block writes

## Recommendations

- Keep buffer size reasonable for your session length
- Query specific traces by ID when possible (faster than scanning)
- Clear buffer between major test runs
```

## Dependencies

- Tasks 01-06 should be complete (or nearly complete)

## Acceptance Criteria

- [ ] README.md is clear and complete
- [ ] At least one example workflow documented
- [ ] Troubleshooting guide covers common issues
- [ ] Performance characteristics documented
- [ ] All examples tested and work

## Notes

- Documentation should be living - update as we learn
- Examples should be copy-pasteable and work
- Keep it concise - developers want to get started quickly
- Link to CLAUDE.md for contributor guidelines

## Status

Status: pending
Depends: 01-06
Next: MVP complete! 🎉
