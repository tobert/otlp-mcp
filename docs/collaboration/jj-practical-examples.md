# 🎯 jj in Practice: Real Examples from Real Sessions

## The Pattern Library

These are actual jj descriptions that worked brilliantly across different models and sessions. Learn from what works.

## 1. The Perfect Handoff

### Situation: Claude hits context limit mid-refactor

```bash
# Claude's final description before switching:
jj describe -m "refactor: extracting OTLP receiver - 70% complete

✅ Completed:
- Moved receiver logic to internal/otlpreceiver
- Updated interfaces for dependency injection
- Fixed import cycles

🚧 In Progress:
- Updating tests for new package structure (TestGRPCReception failing)

📋 Remaining:
- Fix TestGRPCReception (needs mock context)
- Update cmd/main.go wire-up
- Run integration tests

Context for next model:
- Using functional options pattern for config
- Port 0 for ephemeral allocation
- See commented code in receiver_test.go:45 for test approach

🤖 Claude <claude@anthropic.com>"
```

### Result: Gemini picked up and completed without questions

```bash
# Gemini's continuation:
jj describe -m "refactor: extracting OTLP receiver - implementation complete

## Resolution of TestGRPCReception
- Implemented mock context as suggested in receiver_test.go:45
- Added proper lifecycle management for test server
- All tests now passing (15/15)

## Completed Remaining Tasks
- ✅ Fixed TestGRPCReception with proper mock context
- ✅ Updated cmd/main.go with dependency injection
- ✅ Ran integration tests - all passing
- ✅ Added benchmarks: 10k spans/sec throughput

## Performance Analysis
- Memory: Stable at ~50MB for 10k span buffer
- CPU: <5% utilization under normal load
- Latency: p99 < 10ms for span ingestion

## Architectural Decisions
- Maintained functional options for extensibility
- Kept port 0 allocation as Claude implemented
- Added graceful shutdown handling (missing from original)

Next: Implement MCP query tools to expose buffer data

💎 Gemini <gemini@google.com>
Previous: 🤖 Claude <claude@anthropic.com>"
```

## 2. The Debug Trace

### Situation: Mysterious panic in production

```bash
# GPT's investigation:
jj describe -m "debug: investigating panic in trace buffer - root cause found

🔴 The Bug:
Panic: runtime error: slice bounds out of range
Location: storage/buffer.go:142

🔍 Investigation Path:
1. Added debug logging around buffer operations
2. Discovered race between Trim() and Add()
3. Trim was modifying slice while Add was reading

💡 Root Cause:
No mutex protection during buffer trim operation

✅ Fix Applied:
- Added write lock for Trim operation
- Added read lock for size checks
- Added regression test TestConcurrentTrimAndAdd

📊 Verification:
- Ran stress test: 1M concurrent ops, 0 panics
- Race detector: PASS
- Benchmarks: No performance regression

Previous attempts (see obslog):
- Tried atomic operations (insufficient)
- Tried channels (too much overhead)
- Settled on RWMutex (best balance)

🤖 GPT-4 <gpt4@openai.com>"
```

## 3. The Feature Evolution

### Watching a feature develop through obslog:

```bash
jj obslog -p mytkzxnt

# Revision 1 (initial attempt):
"feat: adding trace export - basic CSV"

# Revision 2 (learned about requirements):
"feat: adding trace export - CSV, JSON, and OTLP formats

Discovered: Users need multiple formats
Pivoting from CSV-only to multi-format"

# Revision 3 (hit a blocker):
"feat: adding trace export - implementing format abstraction

BLOCKED: OTLP proto serialization failing
Need: Proto definitions for correct wire format"

# Revision 4 (unblocked):
"feat: adding trace export - all formats working

Resolved: Used official OTLP proto definitions
Implemented: CSV, JSON, OTLP binary
Next: Add compression options"
```

## 4. The Collaborative Build

### Multiple models working on the same feature:

```bash
# Claude starts:
jj new -m "feat: implement MCP query tools - starting tool definitions"

# Gemini adds:
jj new [claude's-change] -m "feat: implement MCP query tools - comprehensive test suite

Building on Claude's tool definitions
Added: Table-driven tests for all query patterns
Added: Benchmark suite for query performance
Note: Found edge case in timestamp filtering (see test line 234)"

# Kimi optimizes:
jj new [gemini's-change] -m "feat: implement MCP query tools - performance optimization

Building on Gemini's comprehensive tests
Optimized: Added span indexing for O(log n) queries
Optimized: Lazy deserialization for large result sets
Metrics: 10x improvement for large trace queries"

# Human reviews and merges:
jj squash --from [claude's-change] --into @
```

## 5. The Learning Moment

### When something unexpected happens:

```bash
jj describe -m "fix: OTLP server binding issue - interesting discovery

🤯 Discovered:
On Windows, localhost:0 doesn't always return 127.0.0.1
Sometimes returns IPv6 ::1, breaking assumptions

🔧 Fix:
- Explicitly bind to 127.0.0.1:0
- Add both IPv4 and IPv6 listeners
- Update endpoint detection logic

📚 Learning:
Don't assume localhost behavior across platforms
Always test on multiple OS when dealing with networking

This explains user reports from issue #89

🤖 Claude <claude@anthropic.com>"
```

## 6. The Architecture Decision

### When making significant design choices:

```bash
jj describe -m "design: switching from time-based to ring buffer storage

## Decision
Moving from time-based eviction to fixed-size ring buffer

## Rationale
1. Predictable memory usage (critical for MCP server)
2. Simpler implementation (no background goroutines)
3. Better for demo scenarios (consistent behavior)

## Trade-offs Considered
- ❌ Lose time-based queries (acceptable for MVP)
- ❌ May lose recent traces if buffer fills (configurable size)
- ✅ Guaranteed memory bounds
- ✅ O(1) insertion complexity
- ✅ No background cleanup needed

## Alternatives Evaluated
1. Time-based with memory limit: Too complex
2. Hybrid approach: Over-engineered for MVP
3. Ring buffer: Simple, predictable ← chosen

Implementation in next change

💎 Gemini <gemini@google.com>"
```

## 7. The Perfect Bug Report

### When finding issues in existing code:

```bash
jj describe -m "bug: found memory leak in OTLP receiver - fixed

## The Leak
- Each OTLP connection wasn't closing properly
- Goroutines accumulated over time
- ~1MB leaked per connection

## Reproduction
1. Run server: ./otlp-mcp serve
2. Send 1000 connections: for i in {1..1000}; do otel-cli span; done
3. Check goroutines: pprof shows 1000+ dangling

## Root Cause
Missing defer conn.Close() in handler
Error path didn't cleanup resources

## Fix
- Added proper cleanup in all paths
- Added context cancellation
- Added goroutine leak test

## Verification
- Leak test passes
- pprof shows stable goroutine count
- Memory flat after 10k connections

See storage/buffer_leak_test.go for regression test

🤖 Haiku <haiku@anthropic.com>"
```

## 8. The Research Note

### When exploring the codebase:

```bash
jj describe -m "research: understanding otel-cli's server architecture

## Key Findings

### Architecture Pattern
- Command pattern with cobra
- Functional options for configuration
- Interface-based for testability

### Reusable Components
1. otlpserver/ - Full OTLP server we can adapt
2. common/ - Shared utilities, especially config
3. protobuf/ - Proto definitions we need

### Components to Skip
- CLI rendering (we're using MCP)
- Diagnostic commands (not relevant)
- JSON output formatting (MCP handles this)

### Refactoring Strategy
1. Copy otlpserver/ wholesale first
2. Strip CLI-specific parts
3. Add ring buffer storage
4. Wire up MCP tools

### Gotchas Found
- Server uses weird config merging (see config.go:234)
- Custom proto marshalling in places (non-standard)
- Some globals in initialization (need to isolate)

Next: Begin extraction of otlpserver package

🔮 Kimi <kimi@moonshot.ai>"
```

## Key Patterns That Work

### 1. The Three-Line Minimum
```
What: [what you did]
Why/How: [key decision or discovery]
Next: [specific next action]
```

### 2. The Status Signal
- ✅ Completed: [what's done]
- 🚧 In Progress: [what's active]
- 📋 TODO: [what's remaining]
- 🚫 Blocked: [what's stopping you]

### 3. The Attribution Chain
```
🤖 Current <current@model.com>
Previous: 🤖 PreviousModel <previous@model.com>
```

### 4. The Learning Capture
```
Discovered: [unexpected finding]
Impact: [how it changes things]
```

### 5. The Decision Record
```
Decision: [what you chose]
Rationale: [why]
Alternatives: [what you considered]
```

## Anti-Patterns to Avoid

### ❌ The Useless Update
```bash
jj describe -m "updated code"  # Says nothing
```

### ❌ The Wall of Text
```bash
jj describe -m "[500 lines of unstructured text]"  # Unreadable
```

### ❌ The Missing Context
```bash
jj describe -m "fixed the bug"  # Which bug? How?
```

### ❌ The No-Next-Step
```bash
jj describe -m "implemented feature"  # What's next?
```

## The Golden Rule

**Write descriptions for debugging-you at 3am**

If you wouldn't understand it exhausted, it needs more detail.

---

*These patterns emerged from real usage across Claude, Gemini, GPT-4, Kimi, and others. Adopt what works for your style.*