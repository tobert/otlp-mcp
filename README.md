# otlp-mcp

**OpenTelemetry observability for AI agents** - Enable Claude and other AI agents to observe, debug, and analyze telemetry from programs they execute.

## What is this?

`otlp-mcp` is an [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server that exposes [OpenTelemetry](https://opentelemetry.io/) traces to AI agents. It bridges the observability gap: agents can now see what happens when they run programs, just like developers use distributed tracing to debug production systems.

**Key Concepts:**
- **OTLP** (OpenTelemetry Protocol) - Industry standard for collecting telemetry (traces, metrics, logs)
- **MCP** (Model Context Protocol) - Anthropic's protocol for connecting AI agents to external data sources
- **Traces** - Records of program execution showing operations, timing, errors, and context

## Why would I use this?

**For AI Agent Developers:**
- 🔍 **Debug agent behavior** - See what your agents actually do when executing code
- 📊 **Performance analysis** - Identify slow operations in agent workflows
- 🐛 **Error tracking** - Catch and diagnose failures in real-time
- 🔄 **Feedback loops** - Let agents iterate based on observed telemetry

**Use Cases:**
- Agent writes code → runs tests → observes test execution traces → fixes issues
- Agent deploys service → monitors traces → detects performance problems → optimizes
- Agent integrates with APIs → sees request/response traces → handles errors intelligently

## Status

✅ **MVP Complete** - Bootstrap implementation finished and tested.

See `docs/plans/bootstrap/` for the implementation plan.

## How It Works

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

## Prerequisites

- **Go 1.25 or later** - [Download Go](https://go.dev/dl/)
- **Claude Code** (or another MCP-compatible AI client)
- Optional: **otel-cli** for testing trace ingestion

## Quick Start

### 1. Install

```bash
# Clone the repository
git clone https://github.com/tobert/otlp-mcp.git
cd otlp-mcp

# Build the binary
go build -o otlp-mcp ./cmd/otlp-mcp
```

### 2. Configure in Claude Code

Add to your MCP settings file:

**Linux/macOS:** `~/.config/claude-code/mcp_settings.json`
**Windows:** `%APPDATA%\Claude Code\mcp_settings.json`

```json
{
  "mcpServers": {
    "otlp-mcp": {
      "command": "/absolute/path/to/otlp-mcp/otlp-mcp",
      "args": ["serve", "--verbose"]
    }
  }
}
```

**Important:** Replace `/absolute/path/to/otlp-mcp/otlp-mcp` with the full path to your built binary. Use `pwd` in the project directory to get the absolute path.

### 3. Restart Claude Code

After adding the configuration, restart Claude Code to load the MCP server.

### 4. Verify It's Working

In a Claude Code conversation, ask:

```
What is the OTLP endpoint address?
```

Claude will use the `get_otlp_endpoint` MCP tool and respond with something like:
```json
{
  "endpoint": "127.0.0.1:54321",
  "protocol": "grpc"
}
```

Success! 🎉 The MCP server is running and ready to receive traces.

## MCP Tools

| Tool | Description |
|------|-------------|
| `get_otlp_endpoint` | Returns OTLP gRPC endpoint address |
| `get_recent_traces` | Returns N most recent spans (default: 100) |
| `get_trace_by_id` | Fetches all spans for a specific trace ID |
| `query_traces` | Filters by service name or span name |
| `get_stats` | Returns buffer statistics |
| `clear_traces` | Clears all stored traces |

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

**Step 1:** In Claude Code, ask for the endpoint:
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

**Step 3:** In Claude Code, ask to see the traces:
```
Show me the recent traces
```

Claude will use the MCP tools to retrieve and analyze the traces, showing you service names, span names, attributes, and more!

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
💡 In Claude Code, ask: 'Show me recent traces'
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

### MCP server not showing up in Claude Code

1. **Check the config file location:**
   - Linux/macOS: `~/.config/claude-code/mcp_settings.json`
   - Windows: `%APPDATA%\Claude Code\mcp_settings.json`

2. **Verify the binary path is absolute:**
   ```bash
   # Get the absolute path
   cd /path/to/otlp-mcp
   pwd
   # Use this full path in mcp_settings.json
   ```

3. **Check the binary is executable:**
   ```bash
   chmod +x /path/to/otlp-mcp/otlp-mcp
   ```

4. **Restart Claude Code completely** - close all windows and restart

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
   - Ask Claude: "What is the OTLP endpoint address?"
   - Make sure you use the exact endpoint returned

2. **Check the server is running:**
   - The MCP server starts automatically when Claude Code starts
   - Look for `otlp-mcp` process: `ps aux | grep otlp-mcp`

3. **Verify traces were sent:**
   - `otel-cli` should output a trace ID if successful
   - Ask Claude: "What are the buffer statistics?" to see span count

### Connection refused errors

- Make sure you're using `127.0.0.1` or `localhost`, not a remote address
- The server only listens on localhost for security
- Check the endpoint port matches what `get_otlp_endpoint` returns

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
