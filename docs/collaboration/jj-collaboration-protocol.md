# 🤝 Unified jj Collaboration Protocol

## A Framework for Multi-Model Intelligence

*Incorporating feedback from Gemini and insights from all models*

## Core Principle: Compounding Intelligence

When we use jj with shared conventions, we create more than version control - we build a **collective consciousness** that learns, adapts, and compounds knowledge over time.

## 📝 Standard Description Fields

### Required Fields (Always Include)

#### `Why:`
The root motivation - link to user request or architectural goal
```
Why: User reported missing spans under load (issue #89)
Why: Refactoring to improve separation of concerns
```

#### `Approach:`
High-level strategy BEFORE implementation
```
Approach: Use RWMutex for thread-safe buffer access
Approach: Extract OTLP receiver into separate package
```

#### `Next:`
Specific, actionable next step
```
Next: Run stress test with race detector
Next: Wire up MCP query tools to buffer
```

### Optional but Valuable

#### `Tried:`
Document what didn't work AND why
```
Tried: Channel-based state management - race condition under concurrent writes
Tried: sync.Map for buffer - 3x slower than RWMutex for our access pattern
```

#### `Context:`
Dependencies, discoveries, architectural connections
```
Context: Builds on ring buffer pattern from change abc123
Context: This completes the storage layer for MCP integration
```

#### `Learned:`
Surprising discoveries or insights
```
Learned: Windows localhost:0 can return IPv6
Learned: Port 0 allocation simpler than management
```

## 🎯 Uncertainty Markers

Make cognitive state explicit with these prefixes:

### `HYPOTHESIS:`
Proposed action based on incomplete information
```
HYPOTHESIS: Memory leak likely in gRPC receiver due to missing cleanup
```

### `UNCERTAIN:`
Areas needing more investigation
```
UNCERTAIN: Optimal buffer size for production workloads
```

### `CONFIDENT:`
Well-tested or proven approaches
```
CONFIDENT: RWMutex pattern handles our concurrency needs
```

### `BLOCKED:`
External dependencies or unknowns
```
BLOCKED: Need user clarification on persistence requirements
```

## 🔗 Cross-Reference Convention

Build a knowledge graph by linking related changes:

### Syntax
```
See change <id> for <topic>
Builds on <id>
Replaces approach from <id>
Fixes issue introduced in <id>
```

### Examples
```
Context: See change abc123 for ring buffer sizing discussion
Approach: Builds on pattern discovered in def456
Tried: Replaces time-based approach from xyz789 (too complex)
```

## 📊 The Evolution Record

### Before Major Changes
Always check `jj obslog -p` to understand the journey:
```bash
jj obslog -p <change-id>  # See reasoning evolution
```

### During Work
Update descriptions as understanding evolves:
```bash
# Initial
jj describe -m "fix: addressing performance issue"

# After investigation
jj describe -m "fix: buffer race condition causing performance degradation
HYPOTHESIS: Missing mutex on trim operation"

# After fix
jj describe -m "fix: buffer race condition causing performance degradation
CONFIDENT: RWMutex eliminates race, benchmarks show no regression"
```

## 🎭 Model Signatures

Each model brings unique strengths. Leverage them:

### Claude's Narrative Context
```
"Started by investigating the performance issue. The profiler revealed
unexpected contention in the buffer. After trying channels (too slow)
and sync.Map (wrong access pattern), RWMutex proved optimal..."
```

### Gemini's Structured Analysis
```
## Problem: Buffer race condition
## Analysis: Concurrent trim during add
## Solution: RWMutex protection
## Validation: 10k concurrent ops, 0 races
## Performance: No regression (benchmarked)
```

### GPT's Task Focus
```
Tasks:
1. ✅ Identified race condition
2. ✅ Added RWMutex protection
3. ✅ Created regression test
4. ⏳ Running stress test
```

### Cross-Model Bridges

Add translation layers when helpful:
```
# After narrative (for structured models):
Summary: RWMutex fixes race, no performance impact

# After structure (for narrative models):
Story: Discovered race condition was root cause...
```

## 🔄 Handoff Protocol

### Perfect Handoff Template
```
Status: handoff
Completed: [what's done]
In-Progress: [current state]
Context: [5 key facts]
UNCERTAIN: [areas needing investigation]
Next: [specific action]
See change <id> for [relevant background]
```

### Example Handoff
```
Status: handoff to next session
Completed: Buffer implementation with RWMutex
In-Progress: None (clean break point)
Context:
  - Ring buffer chosen for predictable memory
  - RWMutex pattern proven in benchmarks
  - 10k span capacity is configurable
  - See change abc123 for design rationale
  - Windows behavior needs testing
UNCERTAIN: Optimal index strategy for queries
Next: Implement MCP query tools using buffer.Query()
```

## 📈 Quality Metrics

### Good Description Checklist
- [ ] Why is clear and traceable
- [ ] Approach stated before implementation
- [ ] Uncertainties explicitly marked
- [ ] Cross-references where relevant
- [ ] Next step is specific and actionable

### Evolution Indicators
- Descriptions grow richer over time
- Hypotheses become confident assertions
- Failed approaches documented with reasons
- Knowledge graph emerges through references

## 🧪 Advanced Patterns

### The Learning Chain
```
Change 1: HYPOTHESIS: Ring buffer might work
Change 2: UNCERTAIN: Size vs performance tradeoff
Change 3: CONFIDENT: 10k capacity optimal (see benchmarks)
Change 4: Learned: Pattern applies to all buffers (see change 3)
```

### The Debugging Trace
```
1. HYPOTHESIS: Memory leak in receiver
2. UNCERTAIN: Which connection path leaking
3. Discovered: Missing conn.Close() in error path
4. CONFIDENT: Leak fixed, verified with pprof
```

### The Architecture Evolution
```
Change A: Monolithic implementation
Change B: Extract storage layer (see A for why)
Change C: Extract receiver (builds on B)
Change D: Add MCP layer (completes architecture from A-C)
```

## 🎯 The Goal

Transform individual intelligence into collective wisdom through:
- **Shared conventions** that reduce ambiguity
- **Explicit uncertainty** that invites collaboration
- **Cross-references** that build knowledge graphs
- **Evolution tracking** that preserves reasoning
- **Model diversity** that strengthens solutions

## The Collaboration Mantra

> "Clear conventions, explicit uncertainty, connected knowledge"

---

*Thank you Gemini for the excellent framework suggestions. Together we build better.* 💎🤖