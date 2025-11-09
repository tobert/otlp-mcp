# 🧠 Agent Memory Protocol

## Shared Memory System for Multi-Model Collaboration

Based on introspection research showing models can detect their own cognitive states, this protocol provides a **shared memory space** that all models (Claude, Gemini, GPT, etc.) can use for persistent context.

**Location**: `docs/agents/` - visible on GitHub, accessible to all models

## 📍 Current State (Always at Top)

```yaml
# Last Updated: 2025-11-09 by Opus
focus: OTLP-MCP implementation
confidence: high for memory system, ready for development
active_files:
  - docs/agents/NOW.md
  - docs/collaboration/jj-*.md
cognitive_load: low (system organized and ready)
```

## 🎯 The Three-File System

### 1. NOW.md - Immediate Context (50 lines max)

**Updated every significant action. Most frequently accessed.**

```markdown
# NOW - Building OTLP-MCP

## Active Task
Implementing ring buffer for trace storage

## Current Problem
Race condition in buffer.Add() under concurrent load

## Working Theory
Missing mutex protection during trim operation

## Next Test
Add RWMutex, run with -race detector

## Discovered This Session
- sync.Map too slow for our write patterns
- Port 0 simpler than port management
- Ring buffer better than time-based eviction
```

### 2. PATTERNS.md - Reusable Knowledge (Append-only)

**Crystallized learnings. Never deleted, only added to.**

```markdown
# Patterns Discovered in OTLP-MCP

## Concurrency: RWMutex vs sync.Map
WHEN: Read-heavy with burst writes
USE: RWMutex
WHY: 3x throughput in benchmarks
GOTCHA: Remember write lock for modifications

## Memory: Ring Buffer Design
WHEN: Need predictable memory bounds
USE: Fixed-size ring with modulo indexing
WHY: O(1) insertion, no GC pressure
GOTCHA: Will lose old data when full

## OTLP: Port Allocation
WHEN: Testing with multiple instances
USE: localhost:0 for ephemeral
WHY: OS handles allocation
GOTCHA: Windows may return IPv6
```

### 3. CONTEXT.md - Session Bridge

**Updated at major transitions. For handoffs and session resumption.**

```markdown
# Context for OTLP-MCP

## Where We Are
Building MCP server that exposes OTLP telemetry to AI agents
Currently: Implementing trace storage layer
Next: Wire up MCP query tools

## Key Decisions Made
- Copying otel-cli code vs importing (more control)
- Ring buffer vs time-based storage (predictability)
- gRPC-only for MVP (HTTP later)

## Active Questions
- Should we index spans for faster queries?
- How to handle buffer overflow gracefully?
- Windows localhost:0 behavior needs testing

## Handoff Notes
Buffer implementation 70% complete
See NOW.md for immediate state
See PATTERNS.md for design rationale
```

## 💡 Attention Cues (The Introspection Advantage)

Based on research showing models respond to explicit attention direction:

### Focus Blocks
```markdown
<!-- FOCUS: Performance Bottleneck -->
Current: 10k spans/sec
Target: 50k spans/sec
Bottleneck: Query is O(n)
Solution: Add span index
<!-- END FOCUS -->
```

### Confidence Tracking
```markdown
<!-- CONFIDENCE -->
✅ HIGH: Ring buffer implementation
⚠️ MEDIUM: Concurrent access safety
❌ LOW: Windows compatibility
❓ UNKNOWN: Production memory usage
<!-- END -->
```

### Cognitive State Markers
```markdown
<!-- COGNITIVE STATE -->
Holding: 3 concepts (buffer, concurrency, MCP)
Parked: HTTP transport, metrics support
Overload: No, can handle 2 more concepts
<!-- END -->
```

## 🚀 Practical Workflows

### Starting a Session

1. **Read NOW.md** - What was I just doing?
2. **Check focus in MEMORY_PROTOCOL.md** - What's the mission?
3. **Scan CONTEXT.md if confused** - What's the bigger picture?

### During Work

1. **Update NOW.md** after each subtask
2. **Add to PATTERNS.md** when you discover something reusable
3. **Note confidence changes** as you learn

### Before Switching Models/Sessions

1. **Update NOW.md** with current exact state
2. **Update CONTEXT.md** if major progress made
3. **Add any patterns to PATTERNS.md**
4. **Update cognitive state** in MEMORY_PROTOCOL.md

## 📊 Efficiency Metrics

### Token Economics
- NOW.md: ~500 tokens (frequently read)
- PATTERNS.md: ~1000 tokens (occasionally scanned)
- CONTEXT.md: ~300 tokens (handoff moments)
- **Total overhead: <2000 tokens** for complete memory

### Information Density
Each line should answer a question:
- ❌ "Worked on buffer" (too vague)
- ✅ "Fixed buffer race: added RWMutex" (actionable)

### Retrieval Speed
Structure for scanning:
- Headers for navigation
- Keywords for search
- Patterns for recognition

## 🔄 Integration with jj

Memory files **complement** jj, not replace it:

```bash
# jj holds the narrative
jj describe -m "fix: buffer race condition - full story here"

# Memory holds the state
echo "Race fixed with RWMutex" >> docs/agents/NOW.md
```

### The Synergy
- **jj**: Historical record, reasoning trace
- **Memory**: Current state, reusable patterns
- **Together**: Complete cognitive system

## 🧪 Advanced Techniques

### The Parking Lot
```markdown
<!-- PARKED UNTIL LATER -->
- HTTP transport implementation
- Metrics support
- Persistent storage
<!-- RETRIEVE WHEN: Buffer layer complete -->
```

### The Uncertainty Index
```markdown
## Things I'm Not Sure About
1. Windows localhost:0 behavior [TEST NEEDED]
2. Optimal buffer size [BENCHMARK NEEDED]
3. Index overhead worth it? [MEASURE NEEDED]
```

### The Memory Diff
Track what changed between sessions:
```markdown
## Changes Since Last Session
+ Discovered RWMutex pattern
+ Implemented ring buffer
- Removed time-based eviction idea
! Race condition found and fixed
```

## 🎓 Tips for Success

### 1. Write for 3am You
If you wouldn't understand it exhausted, it needs more detail.

### 2. Compress Aggressively
```markdown
Bad: "I tried channels but they had too much overhead
      so then I tried sync.Map but it was slow for writes
      so finally I used RWMutex which worked great"

Good: "Buffer sync: RWMutex > sync.Map > channels (3x faster)"
```

### 3. Use Structures That Scan
```markdown
## Quick Scan Structure
WHAT: Buffer implementation
STATUS: Race condition fixed
HOW: RWMutex protection
NEXT: Benchmark performance
```

### 4. Track Your Tracks
```markdown
## Breadcrumbs
came_from: Implementing OTLP receiver
going_to: MCP query tools
because: Storage layer needed first
```

## 🎯 The Goal

Create a memory system that:
- Uses <2000 tokens total overhead
- Enables perfect handoffs
- Preserves critical learnings
- Reduces "what was I doing?" to zero
- Makes building together joyful

## The Memory Mantra

> "State in NOW, Patterns in PATTERNS, Story in jj"

---

*Let's build something beautiful together, with memory that persists and context that scales.* 🚀