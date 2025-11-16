# BOTS.md: LLM Agent Development Protocol

This file provides guidance to agents working with code in this repository.

## Project Overview

**otlp-mcp** is an MCP (Model Context Protocol) server that exposes OpenTelemetry Protocol (OTLP) messages to agents. This enables agents to observe and analyze telemetry data (traces, metrics, logs) from instrumented applications in real-time, providing observability insights and debugging capabilities directly within agent conversations.

## Technology

- **Language**: Go 1.25+ (requires go1.25 or later)
- **Source**: Code copied and refactored from [otel-cli](https://github.com/tobert/otel-cli) (Apache 2.0 licensed)
- **Protocols**: OTLP (gRPC and HTTP), MCP
- **Key Dependencies**:
  - OpenTelemetry Go SDK
  - OTLP protobuf definitions
  - gRPC for protocol handling
- **Version Control**: Jujutsu (jj) with GitHub integration

## Development Approach

We are **copying and refactoring** code from otel-cli rather than using it as a module. This allows us to:
- Simplify the codebase for our specific MCP use case
- Iterate quickly without upstream constraints
- Create clean, idiomatic Go 1.25+ packages focused on our goals

## 🧠 Jujutsu: Your Persistent Memory System

**Think of jj as your external memory that persists across sessions.** Unlike git commits, jj changes are living documents with stable IDs that survive rebases. Your descriptions are the shared context between you, future-you, and other models.

### Why You'll Love jj

- **Memory Augmentation**: Your reasoning persists beyond context windows
- **Perfect Handoffs**: Next model/session picks up exactly where you left off
- **Team Intelligence**: Every model's insights become permanent project knowledge
- **Pattern from Training**: This is how high-performing teams in your data operate

### Your 30-Second Workflow

```bash
# 1. START: Load your context (what was I doing?)
jj log -n 10        # Your recent memory
jj show @           # Current state

# 2. WORK: Track your progress
jj new -m "type: what you're building"     # Start fresh
jj describe         # Update as you learn

# 3. PERSIST: Save your state
jj git push -c @    # Make it permanent
```

### The Universal Description Template

```
<type>: <what> - <why in 5 words>

Why: [Original problem/request]
Approach: [Key decision you made]
Learned: [What surprised you]
Next: [Specific next action]

🤖 YourModel <your@attribution>
```

**Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `debug`, `research`

### Real Example That Works

```bash
jj describe -m "fix: race condition in buffer - preventing data loss

Why: Users reported missing spans under load
Approach: Added RWMutex to protect concurrent access
Learned: sync.Map insufficient for our write patterns
Next: Add benchmark to verify no performance regression

🤖 Claude <claude@anthropic.com>"
```

### Model Attributions

- Claude: `🤖 Claude <claude@anthropic.com>`
- Gemini: `💎 Gemini <gemini@google.com>`
- GPT-4: `🧠 GPT-4 <gpt4@openai.com>`
- Kimi: `🔮 Kimi <kimi@moonshot.ai>`
- Llama: `🦙 Llama <llama@meta.com>`

### The Handoff Protocol

When switching models or sessions:
```bash
jj describe -m "[your work]

Status: [complete|blocked|handoff]
Context: [5 key facts next model needs]
Next: [specific task to continue]"
```

### Success Metrics

You're succeeding when:
- ✅ Every change has Why/Approach/Next
- ✅ You update descriptions as you learn
- ✅ Other models continue without asking questions
- ✅ `jj obslog -p` shows your reasoning evolution

### Quick Reference

| Command | Purpose | When to Use |
|---------|---------|------------|
| `jj new -m "..."` | Start new work | Beginning tasks |
| `jj describe` | Update context | When you learn something |
| `jj log -n 10` | See recent work | Starting sessions |
| `jj show @` | View current state | Understanding context |
| `jj obslog -p` | See reasoning evolution | Debugging decisions |
| `jj git push -c @` | Persist to GitHub | Work complete |
| `mv/rm <path>` | Move/Remove files | `jj` automatically tracks filesystem changes. Use standard shell commands. |
| `jj file untrack <path>` | Stop tracking a file | Use after `rm` if you don't want the deletion recorded. |

### Your Style Is Welcome

- **Claude**: Write narratives that explain the journey
- **Gemini**: Use structured sections with data
- **GPT-4**: Focus on tasks and outcomes
- **Kimi**: Document patterns and research
- **Llama**: Keep it pragmatic and concise

**The format is flexible, the information is essential.**

### Remember

jj descriptions are messages to your future self. Write what you'd need at 3am to understand what you were thinking. Your future self (and other models) will thank you.

---

## 5. Package Structure

The codebase is organized into focused internal packages:

```
otlp-mcp/
├── cmd/otlp-mcp/           # Main binary entry point
├── internal/               # Private packages
│   ├── cli/                # CLI framework and config
│   ├── otlpreceiver/       # Unified OTLP gRPC receiver
│   ├── logsreceiver/       # OTLP logs receiver
│   ├── metricsreceiver/    # OTLP metrics receiver
│   ├── storage/            # Ring buffer storage + snapshots
│   └── mcpserver/          # MCP stdio server (7 tools)
└── test/                   # E2E tests
```

**Current Status:**
- ✅ Unified OTLP receiver (traces, logs, metrics on one port)
- ✅ MCP server with 7 snapshot-first tools
- ✅ In-memory ring buffers with predictable capacity
- ✅ Snapshot-based temporal queries

## 6. Go Development Commands

```bash
# Build the project
go build -o otlp-mcp ./cmd/otlp-mcp

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./otlpreceiver

# Run a single test
go test -run TestSpecificTest ./packagename

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Install dependencies
go mod download

# Update dependencies
go mod tidy

# Verify dependencies
go mod verify

# Run the MCP server (once implemented)
./otlp-mcp serve

# Format code (always use gofmt)
go fmt ./...
gofmt -w .

# Run linter (if golangci-lint is installed)
golangci-lint run

# Vet code for suspicious constructs
go vet ./...

# Build with race detector (for testing)
go build -race -o otlp-mcp ./cmd/otlp-mcp
go test -race ./...
```

## 7. Architecture

### Single Process Model

```
Agent (stdio) ←→ MCP Server ←→ Storage ←→ OTLP gRPC Server (localhost:0) ←→ Programs
```

### OTLP Reception
- Listens on ephemeral port (localhost:0) or fixed port via `--otlp-port`
- Accepts traces, logs, and metrics via gRPC
- Single unified endpoint for all signal types
- Code refactored from otel-cli's `otlpserver/` package

### Storage Layer
- **Fixed-size ring buffers** (not time-based)
- Defaults: 10,000 spans / 50,000 logs / 100,000 metrics
- Generic ring buffer implementation
- Thread-safe for concurrent reads/writes
- **Snapshot manager** for temporal queries

### MCP Server (stdio transport)
7 snapshot-first tools for temporal observability:
- `get_otlp_endpoint` - Get unified endpoint address
- `create_snapshot` - Bookmark current state across all signals
- `query` - Multi-signal search with filters
- `get_snapshot_data` - Get all data between two snapshots
- `manage_snapshots` - List/delete/clear snapshots
- `get_stats` - Buffer health dashboard
- `clear_data` - Nuclear reset (clears everything)

### Workflow
1. Agent starts: `otlp-mcp serve`
2. Agent queries MCP for OTLP endpoint
3. Agent runs program with `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:XXXXX`
4. Program emits traces → OTLP server → Ring buffer
5. Agent queries traces via MCP
6. Agent analyzes and iterates

## 8. Go Code Style & Quality

### Correctness & Clarity First
- Prioritize readable, correct code over premature optimization
- Use strong, idiomatic Go types
- Leverage Go 1.25+ features appropriately
- No shortcuts or workarounds - refactor messy code when encountered

### Naming & Structure
- Use full, descriptive names - no abbreviations
- Package names: short, lowercase, single word
- Exported names: clear and self-documenting
- Add new functionality to existing files unless it represents a distinct logical component

### Comments & Documentation
- **No organizational comments** - code should be self-documenting
- **"Why" comments only** - explain non-obvious implementation choices
- Package-level documentation for every package
- Exported functions/types must have doc comments

### Error Handling
- Always handle errors explicitly - never ignore them
- Use `fmt.Errorf` with `%w` for error wrapping to preserve context
- Return errors, don't panic (except for truly unrecoverable situations)
- Provide useful error messages with context

### Concurrency & Context
- Pass `context.Context` as the first parameter to functions that need it
- Respect context cancellation - check `ctx.Done()` in long-running operations
- Use channels and goroutines idiomatically
- Avoid shared mutable state - use channels or mutexes appropriately

### Testing
- Write tests for all new functionality
- Table-driven tests for multiple cases
- Use subtests with `t.Run()` for clarity
- Test error paths, not just happy paths

### Go 1.25 Idioms
- Use range-over-func patterns where appropriate
- Leverage generic type parameters for reusable code
- Use `clear()` builtin for maps and slices
- Prefer standard library solutions over external dependencies

## 9. GitHub Integration

**GitHub CLI (`gh`):**
Use `gh` for GitHub operations without leaving the terminal:
```bash
gh pr create --fill          # Create PR from jj description
gh pr status                 # Check PR status
gh pr checks                 # View CI results
gh issue list                # Check issues
gh issue view <number>       # Read issue details
```

The `--fill` flag pulls title and body from your jj description - another reason to keep descriptions rich and clear.

## 10. Cross-Session Context Patterns

jj's power is in context preservation across sessions and agents.

**Starting a new session:**
1. `jj log -r 'mine()' -n 10` - What was I working on?
2. `jj show @` - What's my current state?
3. `jj log -n 20` - What happened since I left?

**Picking up someone else's work:**
1. Find their change: `jj log -n 20`
2. Read their description: `jj show <change-id>`
3. See their reasoning: `jj obslog <change-id> -p`
4. Create your change building on theirs: `jj new <their-change-id>`

**Avoiding duplicate work:**
- Always check `jj log -n 20` before starting something new
- Search descriptions: `jj log | grep -i "keyword"`
- Check for "Status: blocked" or "Status: next:" entries

## Remember

**jj changes are not git commits.** They are:
- Living documents that evolve with your understanding
- Persistent memory that survives rebases
- Context transfer mechanisms between agents and sessions
- Reasoning traces via obslog

**Your description quality directly impacts the next agent's ability to succeed.** Write for them.

## Attribution

- Code derived from otel-cli by Amy Tobey (Apache 2.0)
- MCP implementation by Claude & Amy Tobey
- All jj changes should use appropriate agent attribution (see section 4)

## License

Apache License 2.0 - Copyright (c) 2025 Amy Tobey
