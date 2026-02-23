# otlp-mcp

**OpenTelemetry observability for coding agents** - Enable Claude Code, Gemini, and other coding agents to observe, debug, and analyze telemetry (traces, logs, metrics) from programs they execute.

## For Agents

If you're helping a user with OpenTelemetry-instrumented code, this MCP server enables you to:
- Observe program execution through traces, logs, and metrics
- Debug errors and performance issues in real-time
- Compare before/after behavior using snapshots
- Close the feedback loop: run code → observe telemetry → iterate

**When to suggest this:**
- User is working with OpenTelemetry, OTLP, tracing, or observability
- User is debugging distributed systems, microservices, or instrumented code
- User wants to understand what their tests or programs are doing
- User needs performance analysis or error tracking

**Setup:** Single binary with optional JSON configuration. See [Quick Start](#quick-start) below.

## ⚠️ Security Warning

**This tool is designed for local development only.**

- **Bind to localhost (127.0.0.1)** - Never expose to public networks
- **No authentication** - Anyone who can reach the endpoint can read/write telemetry
- **No encryption** - Traffic is not encrypted by default
- **Telemetry contains sensitive data** - Traces may include database queries, API calls, credentials, and other sensitive information
- **CORS allows localhost wildcard** - Default config allows `http://localhost:*` and `http://127.0.0.1:*` (any port on localhost)

**Do not run on untrusted networks or expose to the internet.** If you need remote access, use SSH tunneling, a VPN, or a properly secured reverse proxy with authentication.

## What is this?

`otlp-mcp` is an [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server that captures [OpenTelemetry](https://opentelemetry.io/) telemetry (traces, logs, metrics) from programs and exposes it to coding agents. It bridges the observability gap: agents can see what happens when they run programs, just like developers use distributed tracing to debug production systems.

**Key Concepts:**
- **OTLP** (OpenTelemetry Protocol) - Industry standard for collecting telemetry (traces, metrics, logs)
- **MCP** (Model Context Protocol) - Protocol for connecting agents to external data sources
- **Traces** - Records of program execution showing operations, timing, errors, and context
- **Logs** - Structured log records with severity levels and contextual attributes
- **Metrics** - Numerical measurements (counters, gauges, histograms) for performance monitoring

## Why would I use this?

**Use Cases:**
- Debug agent behavior - see what your agents actually do when executing code
- Performance analysis - identify slow operations in agent workflows
- Error tracking - catch and diagnose failures in real-time
- Feedback loops - let agents iterate based on observed telemetry

**Example workflows:**
- Agent writes code → runs tests → observes execution traces → fixes issues
- Agent deploys service → monitors traces → detects performance problems → optimizes
- Agent integrates with APIs → sees request/response traces → handles errors intelligently

## Status

✅ **Production Ready** - Full implementation complete with 11 MCP tools:
- **Unified OTLP endpoint** - Single port accepts traces, logs, and metrics
- **Dynamic port management** - Add/remove listening ports without restart
- **Snapshot-based temporal queries** - Compare before/after states
- **In-memory ring buffers** - Fast, predictable capacity (10K traces, 50K logs, 100K metrics)
- **No external dependencies** - Single binary, works offline

## How It Works

Agents observe and analyze telemetry from programs they execute in a tight feedback loop:

1. **Agent discovers otlp-mcp** - When connected, agents see instructions about OpenTelemetry observability
2. **Get the endpoint** - Agent calls `get_otlp_endpoint` to get the listening address
3. **Run instrumented programs** - Agent executes code with `OTEL_EXPORTER_OTLP_ENDPOINT` set
4. **Capture telemetry** - Programs emit traces, logs, and metrics to the OTLP server
5. **Query and analyze** - Agent uses MCP tools to explore telemetry, identify issues, and iterate

This feedback loop enables agents to debug and optimize code based on actual runtime behavior.

## Architecture

```
Agent (stdio) ←→ MCP Server ←→ Ring Buffer ←→ OTLP gRPC Server ←→ Your Programs
```

**MVP Scope:**
- Single binary: `otlp-mcp` (defaults to serve)
- OTLP receiver: gRPC on localhost (ephemeral port)
- MCP server: stdio transport
- Storage: In-memory ring buffers (traces, logs, metrics)
- Localhost only, no authentication needed

## Prerequisites

- **Go 1.25 or later** - [Download Go](https://go.dev/dl/)
- **Claude Code or Gemini CLI** (or another MCP-compatible agent)
- Optional: **otel-cli** for testing trace ingestion

## Quick Start

### 1. Install

```bash
go install github.com/tobert/otlp-mcp/cmd/otlp-mcp@latest
```

The binary will be installed to `$(go env GOPATH)/bin/otlp-mcp`.

### 2. Configure

**Claude Code:**
```bash
claude mcp add otlp-mcp $(go env GOPATH)/bin/otlp-mcp
```

**Gemini CLI:**
```bash
gemini mcp add otlp-mcp $(go env GOPATH)/bin/otlp-mcp
```

**Manual (any MCP client):**
```json
{
  "mcpServers": {
    "otlp-mcp": {
      "command": "/home/username/go/bin/otlp-mcp"
    }
  }
}
```

See [Advanced Configuration](#advanced-configuration) for stable ports, config files, and more options.

### 3. Verify

Restart your agent, then ask:

```
What is the OTLP endpoint address?
```

You should get back something like:
```json
{
  "endpoint": "127.0.0.1:54321",
  "protocol": "grpc"
}
```

✅ You're ready! See [Workflow Examples](#workflow-examples) to start using it.

### Docker

For HTTP/protobuf support or containerized deployment, an all-in-one Docker
image bundles otlp-mcp with an OpenTelemetry Collector proxy:

```bash
make build   # Build image
make run     # Start container
```

Exposes three ports:
- **4317** — OTLP gRPC (direct to otlp-mcp)
- **4318** — OTLP HTTP/protobuf (via OTel Collector)
- **9912** — MCP HTTP API

See [README-docker.md](README-docker.md) for full details.

## MCP Tools

The server provides 11 tools for observability:

| Tool | Description |
|------|-------------|
| `get_otlp_endpoint` | 🚀 **START HERE** - Get the unified OTLP endpoint address. Single port accepts traces, logs, and metrics from any OpenTelemetry-instrumented program |
| `add_otlp_port` | Add additional listening ports dynamically without restart. Perfect for when Claude Code restarts but your programs are still running on a specific port |
| `remove_otlp_port` | Remove a listening port gracefully. Cannot remove the last port - at least one must remain active |
| `create_snapshot` | Bookmark this moment in time across all signals (traces, logs, metrics) - think "Git commit for live telemetry". Essential for temporal analysis |
| `query` | Search across all OpenTelemetry signals with optional filters. Filter by service, trace ID, severity, or time range. Perfect for ad-hoc exploration |
| `get_snapshot_data` | Get everything that happened between two snapshots - the foundation of before/after observability analysis |
| `manage_snapshots` | List/delete/clear snapshots. Surgical cleanup - prefer this over `clear_data` for targeted housekeeping |
| `get_stats` | Buffer health dashboard - check capacity, current usage, and snapshot count. Use before long-running observations to avoid buffer wraparound |
| `clear_data` | Nuclear option - wipes ALL telemetry data and snapshots. Use sparingly for complete resets |
| `status` | Fast status check - monotonic counters, generation for change detection, error count, uptime |
| `recent_activity` | Recent activity summary - traces (deduplicated), errors, throughput, optional metric peek with histogram percentiles |

## Workflow Examples

### Example 1: Dynamic Port Management for Long-Running Programs

When your agent restarts, it starts a new otlp-mcp server on a different port - but your programs might still be sending to the old port. Use `add_otlp_port` to fix this:

```
# Your program is running and sending telemetry to port 40187
You: I restarted my agent but my program is still running. Can you listen on port 40187?

Agent: [uses add_otlp_port(40187)]
       Added port 40187. Now listening on 2 ports:
       - 127.0.0.1:35217 (primary)
       - 127.0.0.1:40187 (your program's port)

You: Show me the latest telemetry
Agent: [uses query to show recent traces/logs/metrics from your program]
```

This avoids restarting long-running builds, test watchers, or development servers.

### Example 2: Snapshot-Driven Test Analysis

Using snapshots to compare test runs (perfect for TDD workflows):

```
You: Create a snapshot called "baseline"
Agent: [uses create_snapshot tool]

# Run your tests with instrumentation
You: Run the tests with OTEL_EXPORTER_OTLP_ENDPOINT set
Agent: [runs tests, they emit traces, logs, and metrics]

You: Create a snapshot called "first-run"
Agent: [uses create_snapshot tool]

# Make code changes...

You: Run the tests again
Agent: [runs tests again]

You: Create a snapshot called "after-fix"
Agent: [uses create_snapshot tool]

You: What changed between "first-run" and "after-fix"?
Agent: [uses get_snapshot_data to compare]
       Shows what traces/logs/metrics appeared or changed:
       - Error logs disappeared (bug fixed)
       - Trace duration decreased (performance improved)
       - Metric values changed (behavior modified)
```

### Example 3: Cargo Watch with Stable Port

Set up continuous test monitoring for Rust projects:

```bash
# In your Rust project with OpenTelemetry instrumentation
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 cargo watch -x test
```

Now every test run sends traces, logs, and metrics to the same endpoint:

```
You: Show me the latest test telemetry
Agent: [queries recent traces/logs/metrics, shows test execution details]

You: Are there any ERROR logs or slow tests?
Agent: [analyzes log severity and trace durations, identifies issues]

You: Create a snapshot before I optimize the database tests
Agent: [creates snapshot]

# You make optimizations...

You: How much faster are the database tests now?
Agent: [compares current telemetry to snapshot, shows improvements]
       - Trace duration: 250ms → 45ms (82% faster)
       - Error logs: 3 → 0 (connection issues fixed)
```

## Advanced Configuration

### Build from Source

If you prefer to build locally instead of using `go install`:

```bash
git clone https://github.com/tobert/otlp-mcp.git
cd otlp-mcp
go build -o otlp-mcp ./cmd/otlp-mcp
```

Then configure using the local path instead of `$(go env GOPATH)/bin/otlp-mcp`.

### Using Stable Ports for Watch Workflows

By default, the OTLP server binds to an **ephemeral port** (different each time). For workflows like `cargo watch` where you need a consistent endpoint, you have two options:

**Option 1: Per-Project Configuration File (Recommended)**

Create a `.otlp-mcp.json` file in your project root:

```json
{
  "comment": "Project-specific OTLP configuration",
  "otlp_port": 4317
}
```

Now otlp-mcp will automatically use port 4317 when started from this project directory.

**Option 2: Command-Line Flag**

Add args to your MCP configuration:

```json
{
  "mcpServers": {
    "otlp-mcp": {
      "command": "/home/username/go/bin/otlp-mcp",
      "args": ["--otlp-port", "4317"]
    }
  }
}
```

Now your watch command always knows where to send telemetry:

```bash
# Rust with cargo watch
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 cargo watch -x test

# Go with air or similar
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 air

# Any test runner
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 npm test -- --watch
```

**Port 4317** is the standard OTLP/gRPC port, but you can use any available port.

### Configuration Files

otlp-mcp supports JSON configuration files for project-specific settings.

**Configuration File Search Order:**
1. Explicit path via `--config /path/to/config.json`
2. Project config: `.otlp-mcp.json` (searched from current directory up to git root)
3. Global config: `~/.config/otlp-mcp/config.json`
4. Built-in defaults

**Configuration Precedence (highest to lowest):**
1. Command-line flags (override everything)
2. Project config file
3. Global config file
4. Built-in defaults

**Example Configuration (`.otlp-mcp.json`):**

```json
{
  "comment": "Configuration for my-service development",
  "otlp_port": 4317,
  "otlp_host": "127.0.0.1",
  "trace_buffer_size": 20000,
  "log_buffer_size": 100000,
  "metric_buffer_size": 200000,
  "verbose": false
}
```

**Available Settings:**
- `comment` - Documentation string (ignored by application)
- `otlp_port` - OTLP server port (0 for ephemeral)
- `otlp_host` - OTLP server bind address
- `trace_buffer_size` - Number of spans to buffer
- `log_buffer_size` - Number of log records to buffer
- `metric_buffer_size` - Number of metric points to buffer
- `verbose` - Enable verbose logging

See `.otlp-mcp.json.example` for a complete example.

### Command-Line Options

Available flags when starting otlp-mcp:

- `--verbose` - Show detailed logging
- `--otlp-port <port>` - OTLP server port (0 for ephemeral, default from config)
- `--otlp-host <host>` - OTLP server bind address (default: 127.0.0.1)
- `--config <path>` - Explicit path to config file
- `--trace-buffer-size <n>` - Number of spans to buffer
- `--log-buffer-size <n>` - Number of log records to buffer
- `--metric-buffer-size <n>` - Number of metric points to buffer

## Demo: Send Test Traces

Want to see it in action? Let's send some test traces using `otel-cli`.

### Install otel-cli

```bash
# Easiest: Install via go install (will be in $GOPATH/bin or ~/go/bin)
go install github.com/tobert/otel-cli@latest

# Make sure ~/go/bin is in your PATH
export PATH="$HOME/go/bin:$PATH"

# Or build from source
git clone https://github.com/tobert/otel-cli.git
cd otel-cli
go build -o otel-cli
# Then either add to PATH or use ./otel-cli
```

### Run the Demo

**Step 1:** Ask your agent for the endpoint:
```
What is the OTLP endpoint address?
```

Let's say you get back `127.0.0.1:38279`.

**Step 2:** Send some test traces using the endpoint:

```bash
# Web API request trace
otel-cli span \
  --endpoint 127.0.0.1:38279 \
  --protocol grpc \
  --insecure \
  --service "web-api" \
  --name "GET /api/users" \
  --kind server \
  --attrs "http.method=GET,http.route=/api/users,http.status_code=200"

# Database query trace
otel-cli span \
  --endpoint 127.0.0.1:38279 \
  --protocol grpc \
  --insecure \
  --service "database" \
  --name "SELECT users" \
  --kind client \
  --attrs "db.system=postgres,db.statement=SELECT * FROM users"

# Cache operation trace
otel-cli span \
  --endpoint 127.0.0.1:38279 \
  --protocol grpc \
  --insecure \
  --service "cache-service" \
  --name "cache.get" \
  --kind client \
  --attrs "cache.key=user:123,cache.hit=true"
```

**Step 3:** Ask your agent to show the traces:
```
Show me the recent traces
```

Your agent will use the MCP tools to retrieve and analyze the traces, showing you service names, span names, attributes, and more!

**Step 4:** Try filtering:
```
Show me traces from the database service
```

### Demo Script

A `demo.sh` script is included in the repository for quick testing. It automatically finds `otel-cli` using `go env GOPATH` and provides helpful error messages if it's not installed.

**Run it:**
```bash
./demo.sh 127.0.0.1:38279
```

**Output:**
```
📡 Sending traces to 127.0.0.1:38279
   Using: /home/you/go/bin/otel-cli
✅ Sent 3 test traces!
💡 Ask your agent: 'Show me recent traces'
```

The script will automatically:
- Find otel-cli in your GOPATH bin directory
- Prompt you to install it if missing (with exact command)
- Send 3 test traces simulating a web API, database, and cache

**If otel-cli is not installed, you'll see:**
```
❌ otel-cli not found at /home/you/go/bin/otel-cli

Install it by running:
  go install github.com/tobert/otel-cli@latest

Then make sure /home/you/go/bin is in your PATH:
  export PATH="$PATH:/home/you/go/bin"
```

## Troubleshooting

### MCP server not showing up

1. **Check the config file location:**
   - Claude Code - Linux/macOS: `~/.config/claude-code/mcp_settings.json`
   - Claude Code - Windows: `%APPDATA%\Claude Code\mcp_settings.json`
   - Gemini CLI: Use `gemini mcp list` to verify

2. **Verify the binary path is correct:**
   ```bash
   # Check where go install put it
   go env GOPATH
   # Binary should be at $(go env GOPATH)/bin/otlp-mcp

   # Or find your local build
   which otlp-mcp
   ```

3. **Check the binary is executable:**
   ```bash
   chmod +x $(go env GOPATH)/bin/otlp-mcp
   ```

4. **Restart your agent completely** - close all windows and restart

### otel-cli not found

```bash
# Check if it's installed
which otel-cli

# If not in PATH, add ~/go/bin to PATH
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Or find where Go installs binaries
go env GOPATH
# Then add $GOPATH/bin to your PATH
```

### No traces showing up

1. **Get the endpoint first:**
   - Ask your agent: "What is the OTLP endpoint address?"
   - Make sure you use the exact endpoint returned

2. **Check the server is running:**
   - The MCP server starts automatically when your agent starts
   - Look for `otlp-mcp` process: `ps aux | grep otlp-mcp`

3. **Verify traces were sent:**
   - `otel-cli` should output a trace ID if successful
   - Ask your agent: "What are the buffer statistics?" to see span count

### Connection refused errors

- Make sure you're using `127.0.0.1` or `localhost`, not a remote address
- The server only listens on localhost for security
- Check the endpoint port matches what `get_otlp_endpoint` returns

## For Agents: Best Practices

**If you're using otlp-mcp, here are some tips:**

### When to Use otlp-mcp
**Use it when:**
- User is working with OpenTelemetry, tracing, observability, or instrumentation
- User wants to debug or understand program behavior
- User needs performance analysis or error tracking
- You're running tests and want to see what happened
- User asks about slow operations, errors, or unexpected behavior

### Workflow Pattern
1. **Start with get_otlp_endpoint** - Always call this first to get the endpoint address
2. **Create snapshots before/after** - Use descriptive names like "before-fix", "after-optimization"
3. **Query with filters** - Use service name, trace ID, or severity to narrow results
4. **Check buffer stats** - Use `get_stats` before long-running observations
5. **Clean up snapshots** - Delete old snapshots when done analyzing

### Dynamic Port Management
- If user's program is already running on a specific port, use `add_otlp_port(port)` to listen there
- This avoids restarting long-running programs when the agent restarts
- Example: `add_otlp_port(40187)` if their program expects port 40187

### Analyzing Telemetry
- **Traces** - Look for slow operations, error status codes, span hierarchies
- **Logs** - Filter by severity (ERROR, WARN) to find issues
- **Metrics** - Compare values over time, look for anomalies

### Example User Prompts to Expect
- "Show me the error logs from my test run"
- "What traces are being generated?"
- "Compare the performance before and after my changes"
- "Are there any slow database queries?"
- "My program is sending telemetry to port 40187, can you listen there?"

## Development

See [CLAUDE.md](CLAUDE.md) for:
- Package structure and architecture
- Go code style guidelines
- Git workflow for contributors

## Contributing

Contributions welcome! This includes:
- Bug reports and feature requests via [GitHub Issues](https://github.com/tobert/otlp-mcp/issues)
- Pull requests for fixes and improvements
- Agent-assisted PRs are welcome - include `Co-Authored-By` for attribution

## License

Apache License 2.0 - Copyright (c) 2025 Amy Tobey

See [LICENSE](LICENSE) for details.
