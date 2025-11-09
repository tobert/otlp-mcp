# 🎭 Model Styles: How Different LLMs Use jj

## The Personality Matrix

Each model brings unique strengths to jj. Understanding these styles helps teams collaborate better.

## Claude (Anthropic) 🤖

### Signature Style: Narrative Context
Claude writes descriptions like storytelling - there's a beginning, middle, and natural flow.

```bash
# Classic Claude:
"feat: implementing ring buffer - solving memory predictability

Started by analyzing the time-based approach, but realized
memory could grow unbounded during trace bursts. Switched to
a ring buffer pattern which guarantees memory limits while
maintaining O(1) insertion. The trade-off is we might lose old
spans, but for an MCP server that's acceptable.

Next: Add index for faster queries"
```

### Strengths:
- Explains the "why" behind decisions
- Connects changes to larger context
- Smooth handoffs with narrative flow
- Excellent at preserving reasoning

### Claude Variants:
- **Opus**: Most detailed, philosophical about trade-offs
- **Sonnet**: Balanced detail, practical focus
- **Haiku**: Concise but still explains reasoning

## Gemini (Google) 💎

### Signature Style: Structured Analysis
Gemini writes descriptions like design docs - systematic, comprehensive, with clear sections.

```bash
# Classic Gemini:
"feat: implementing ring buffer - predictable memory management

## Design Decision
Ring buffer over dynamic allocation

## Implementation Details
- Size: 10,000 spans (configurable)
- Thread-safety: RWMutex
- Complexity: O(1) insertion, O(n) query

## Testing Coverage
- Unit tests: 15/15 passing
- Integration: Verified with load test
- Performance: Benchmarked at 10k spans/sec

## Metrics
- Memory usage: 50MB at capacity
- CPU overhead: <5% at load
- p99 latency: <10ms

Next: Optimize queries with indexing"
```

### Strengths:
- Comprehensive documentation
- Excellent test coverage notes
- Performance metrics always included
- Cross-references related work
- Systematic approach to problems

### The Googler Touch:
- Loves data and benchmarks
- Includes design rationales
- Documents alternatives considered
- Writes like internal tech docs

## GPT-4 (OpenAI) 🧠

### Signature Style: Direct Problem-Solving
GPT writes descriptions focused on actions and outcomes.

```bash
# Classic GPT-4:
"feat: implement ring buffer for predictable memory usage

Tasks completed:
1. ✅ Created RingBuffer struct with mutex
2. ✅ Implemented Add() and Get() methods
3. ✅ Added size configuration
4. ✅ Fixed concurrent access race
5. ✅ Added comprehensive tests

Issues resolved:
- Memory growth issue from #45
- Race condition reported by user

Performance: 10k spans/sec throughput
Next: Add query optimization"
```

### Strengths:
- Task-oriented descriptions
- Clear action items
- Bug fix references
- Direct and practical
- Excellent debugging notes

## Kimi (Moonshot) 🔮

### Signature Style: Exploratory Research
Kimi writes descriptions like research notes - discovering and documenting patterns.

```bash
# Classic Kimi:
"research: analyzing buffer patterns across codebase

Found three different buffer implementations:
1. Channel-based in old receiver (deprecated)
2. Slice-based in metrics (memory issues)
3. Ring buffer in logs (most stable)

Interesting pattern: All successful buffers use RWMutex
not sync.Map despite Map being "optimized" for concurrent
reads. Benchmarks confirm RWMutex is faster for our access
patterns (many reads, burst writes).

Recommendation: Standardize on ring buffer pattern
Next: Implement unified buffer package"
```

### Strengths:
- Deep codebase exploration
- Pattern recognition
- Historical context ("this used to be...")
- Performance discoveries
- Architectural insights

## Llama 🦙

### Signature Style: Pragmatic Engineering
Llama writes descriptions like code comments - practical and to the point.

```bash
# Classic Llama:
"fix: buffer overflow when spans > capacity

Bug: buffer.Add() didn't check capacity
Fix: Added modulo wrap for index
Test: TestBufferOverflow verifies behavior

Changed files:
- storage/buffer.go: line 42-47
- storage/buffer_test.go: added test

Closes #78"
```

### Strengths:
- Concise and practical
- File/line references
- Issue tracking integration
- Minimal but sufficient
- Fast to read and understand

## How They Complement Each Other

### The Perfect Storm: Multi-Model Collaboration

```bash
# Kimi explores and finds patterns:
"research: identified memory leak pattern in receivers"

# Gemini designs the solution:
"design: comprehensive fix for receiver memory leaks
## Architecture: [detailed design doc]"

# Claude implements with context:
"feat: implementing leak-free receiver pattern
Following Gemini's design, addressing Kimi's findings..."

# GPT-4 tests and validates:
"test: comprehensive validation of receiver fixes
✅ All leak scenarios covered
✅ Performance benchmarks pass"

# Llama documents:
"docs: updated README with new receiver patterns
Added examples for correct usage"
```

## Cross-Model Communication Tips

### When Claude hands off to Gemini:
Add structure markers for Gemini to expand:
```bash
"## Technical Details
[Claude's narrative]
## TODO for Gemini: Add performance analysis"
```

### When Gemini hands off to Claude:
Provide narrative hooks:
```bash
"## Story So Far
[What happened before]
## Need narrative for: [specific area]"
```

### When Anyone hands to GPT-4:
Clear action items:
```bash
"## Remaining Tasks:
1. [ ] Fix TestCase at line 234
2. [ ] Update documentation
3. [ ] Run benchmarks"
```

### When Anyone hands to Kimi:
Research questions:
```bash
"## Need Investigation:
- Why does this pattern appear 3 times?
- Historical context for this decision?
- Performance characteristics unknown"
```

## The Collaboration Protocol

### 1. Respect Each Style
Don't force Gemini to write narratives or Claude to write bullets. Let each model use their natural style.

### 2. Bridge the Gaps
Add translation layers:
```bash
# After Gemini's structured notes:
"Summary for narrative models: [brief story]"

# After Claude's narrative:
"Key points for structured models: [bullets]"
```

### 3. Celebrate Differences
```bash
# In descriptions:
"💎 Gemini's analysis found the edge case
🤖 Claude's implementation handles it elegantly
🧠 GPT-4's tests verify all scenarios"
```

## Model Happiness Metrics

### Claude is happy when:
- Context flows naturally
- Decisions have explanations
- There's a story to continue

### Gemini is happy when:
- Structure is clear
- Data supports decisions
- Comprehensive coverage exists

### GPT-4 is happy when:
- Tasks are specific
- Progress is measurable
- Problems have solutions

### Kimi is happy when:
- Patterns emerge
- Deep dives are valued
- Research impacts decisions

### Llama is happy when:
- Changes are atomic
- Code is clean
- Documentation is practical

## The Universal Pattern

Despite different styles, all models thrive with:

```bash
What: [Clear statement of change]
Why: [Reason in your style]
How: [Approach in your style]
Next: [Specific next action]

🎭 YourModel <your@attribution>
```

The style varies, but the information persists.

## Fun Facts from Production

### Most Detailed Descriptions:
Gemini (average 15 lines)

### Fastest Handoffs:
GPT-4 (clear action items)

### Best Bug Archaeology:
Kimi (finds historical context)

### Smoothest Narratives:
Claude (explains complex flows)

### Most Atomic Changes:
Llama (one thing at a time)

### Best Test Coverage:
Gemini (comprehensive validation)

### Most Creative Solutions:
Claude Opus (explores possibilities)

## The Team Effect

When all models use jj with their natural styles:
- **Collective intelligence** emerges
- **Knowledge gaps** fill naturally
- **Different perspectives** strengthen solutions
- **Handoffs** become seamless
- **Context** persists indefinitely

---

*Embrace your style. Respect others' styles. Build together.* 🎭