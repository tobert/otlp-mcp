# BOTS.md: LLM Agent Development Protocol

This file provides guidance to agents working with code in this repository.

## Project Overview

**otlp-mcp** is an MCP (Model Context Protocol) server that exposes OpenTelemetry Protocol (OTLP) messages to AI agents. This enables agents to observe and analyze telemetry data (traces, metrics, logs) from instrumented applications in real-time, providing observability insights and debugging capabilities directly within agent conversations.

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

## 1. Core Principle: Jujutsu as a Persistent Context Store

**Jujutsu (jj) changes are persistent context stores.**

Unlike git commits, a `jj` change has a stable ID that survives rebases and amendments. The description associated with this change is our shared, persistent memory. When you read a change description, you are reading the reasoning and intent of the previous agent. When you write one, you are passing critical context to the next.

**Your primary goal is to maintain the integrity and clarity of this context.**

## 2. Agent Workflow Checklist

Follow this sequence for every task.

### Step 1: Understand Context
Always begin by reading the persistent context store. This is your working memory.
```bash
# 1. See your last 10 changes (your memory)
jj log -r 'mine()' -n 10

# 2. See ALL recent project activity (what others are doing)
jj log -n 20

# 3. Review the current change in detail
jj show @

# 4. See how a change evolved (the reasoning trace)
jj obslog -p
```

**What to extract from descriptions:**
- **Decisions & tradeoffs**: Why this approach over alternatives?
- **Blockers & next steps**: What couldn't be completed? What's needed next?
- **Architectural context**: How does this fit into the larger system?
- **Failures & learnings**: What didn't work and why?

**Before starting new work:**
- Check if someone else is already working on this (`jj log -n 20`)
- Read related changes to understand existing patterns
- Look for "Status: next:" entries that describe follow-on work

### Step 2: Start New Work
Create a new change with a clear, descriptive message.
```bash
jj new -m "<type>: <what you are building>"
```
- **Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

### Step 3: Implement and Iterate
**Treat descriptions as living documents.** Update them as your understanding evolves.
```bash
# After learning something new or changing approach
jj describe

# Review before updating
jj show @

# Review your work-in-progress
jj diff
```

**When to update descriptions:**
- When you discover the problem is different than you thought
- When you try an approach that doesn't work (document WHY)
- When you make a key architectural decision
- When you discover a blocker or dependency
- Before taking a break (preserve your mental state)

**What to document:**
- Approaches tried and abandoned (save future agents time)
- Surprising discoveries about the codebase
- Performance characteristics discovered
- Dependencies or interactions discovered

Use `jj squash` to fold fixups and minor corrections into the main change, keeping the history clean and atomic.

### Step 4: Finalize and Sync
Once the work is complete, tested, and builds pass, push the change.
```bash
# Push only the current change to GitHub
jj git push -c @

# Create a pull request using the rich description
gh pr create --fill
```

## 3. Jujutsu (jj) Command Reference

| Command | Agent Usage |
| :--- | :--- |
| `jj new -m "..."` | Start a new atomic change. |
| `jj describe -m "..."` | Update the description (the persistent context). **Use frequently.** |
| `jj log -r 'mine()' -n 10` | Review your recent work history (your memory). |
| `jj log -n 20` | See all recent project activity. |
| `jj show <id>` | Read the full diff and description of a change. |
| `jj obslog -p` | See how a change evolved over time. **This is the reasoning trace** - shows abandoned approaches, refinements, and learning. |
| `jj diff` | Review current working copy changes. |
| `jj squash` | Fold the current change into its parent. |
| `jj split` | Separate a change into smaller, more logical units. |
| `jj abandon` | Discard the current change entirely. |
| `jj git push -c @` | Push the current change to the remote. |

## 4. Change Description Format

**The description is a letter to your future self and other agents.**

```
<type>: <summary>

Why: <original user prompt or problem being solved>
Approach: <key decisions, algorithms, patterns>
Tried: <approaches that didn't work>
Context: <architectural decisions, dependencies discovered>
Status: <complete | blocked: <reason> | next: <specific tasks>>

Co-authored-by: <Your Name> <your@email>
```

- **Why**: Include the original user prompt when possible - full context matters
- **Tried**: Document failed approaches - this is crucial learning
- **Status**: Be specific about blockers and next steps

**Agent Attribution:**
- **Claude**: `Co-authored-by: Claude <claude@anthropic.com>`
- **Gemini**: `Co-authored-by: Gemini <gemini@google.com>`
- **Kimi**: `Co-authored-by: Kimi <kimi@moonshot.ai>`

## 5. Bootstrap Plan & Package Structure

**See `docs/plans/bootstrap/` for detailed implementation tasks.**

The codebase is organized into focused internal packages:

```
otlp-mcp/
├── cmd/otlp-mcp/           # Main binary entry point
├── internal/               # Private packages
│   ├── cli/                # CLI framework and config
│   ├── otlpreceiver/       # OTLP gRPC server for traces
│   ├── storage/            # Ring buffer storage
│   └── mcpserver/          # MCP stdio server
└── docs/
    └── plans/bootstrap/    # Task-by-task implementation plan
```

**MVP Scope (Bootstrap):**
- OTLP: gRPC only, traces only, localhost only
- MCP: stdio transport, trace query tools
- Storage: Fixed-size ring buffers in memory

**Future:** HTTP OTLP, logs/metrics, WebSocket MCP, persistence

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

## 7. Architecture (MVP)

**See `docs/plans/bootstrap/00-overview.md` for complete architecture and diagrams.**

### Single Process Model

```
Agent (stdio) ←→ MCP Server ←→ Storage ←→ OTLP gRPC Server (localhost:0) ←→ Programs
```

### OTLP Reception (gRPC only for MVP)
- Listens on ephemeral port (localhost:0)
- Accepts OTLP trace exports via gRPC
- Code refactored from otel-cli's `otlpserver/` package
- HTTP support deferred to post-MVP

### Storage Layer
- **Fixed-size ring buffers** (not time-based)
- Default: 10,000 spans
- Generic ring buffer implementation
- Thread-safe for concurrent reads/writes
- Logs and metrics support planned for future

### MCP Server (stdio transport)
Exposes trace data via MCP tools:
- `get_otlp_endpoint` - Get gRPC endpoint address
- `get_recent_traces` - List recent spans
- `get_trace_by_id` - Fetch specific trace
- `query_traces` - Filter by service/span name
- `get_stats` - Buffer statistics
- `clear_traces` - Clear buffer

Resources:
- `otlp://config` - Current configuration

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
