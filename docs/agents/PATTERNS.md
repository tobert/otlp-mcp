# Patterns - OTLP-MCP Project

## LLM Collaboration Patterns

### Pattern: Uncertainty Markers in Descriptions
WHEN: Communicating cognitive state in jj descriptions
USE: Prefixes like HYPOTHESIS, UNCERTAIN, CONFIDENT, BLOCKED
WHY: Makes uncertainty explicit, invites collaboration
EVIDENCE: Gemini's framework - improves handoffs
EXAMPLE: "HYPOTHESIS: Memory leak in gRPC receiver"
GOTCHA: Be honest about uncertainty levels

### Pattern: Cross-Reference Changes
WHEN: Building on previous work or related changes
USE: "See change <id> for X" in descriptions
WHY: Creates knowledge graph, prevents duplicate work
EXAMPLE: "See change abc123 for buffer sizing rationale"
GOTCHA: Use short hash or unique prefix

### Pattern: Memory as Enhancement Not Rule
WHEN: Getting models to adopt new tools/processes
USE: Frame as capability enhancement
WHY: Models resist constraints but embrace augmentation
EVIDENCE: Gemini adopted jj after reframing
GOTCHA: Must show immediate value

### Pattern: Model-Specific Communication
WHEN: Working with different LLMs
USE: Adapt style to model strengths
- Claude: Narratives and journeys
- Gemini: Structured data and analysis
- GPT: Task lists and outcomes
WHY: Each model has natural preferences from training
GOTCHA: Don't force unnatural styles

### Pattern: Attention Cues for Focus
WHEN: Need model to focus on specific aspect
USE: `<!-- FOCUS: X -->` blocks
WHY: Research shows models respond to explicit attention
EVIDENCE: Introspection paper - models can modulate attention
GOTCHA: Still unreliable, but better than nothing

## Memory System Patterns

### Pattern: Three-Layer Memory
WHEN: Need persistent context across sessions
USE: NOW.md (immediate) + PATTERNS.md (permanent) + CONTEXT.md (bridge)
WHY: Optimizes token usage while preserving knowledge
TRADEOFF: <2000 tokens overhead for complete context
GOTCHA: Must maintain actively or becomes stale

### Pattern: Cognitive Load Tracking
WHEN: Complex multi-concept work
USE: Explicit state markers in memory
WHY: Helps prevent overload and dropped concepts
EXAMPLE: "Holding: 3 concepts, Parked: 2 concepts"
GOTCHA: Models might not accurately self-assess

## Code Patterns (OTLP-MCP Specific)

### Pattern: Ring Buffer for Bounded Memory
WHEN: Need predictable memory usage
USE: Fixed-size buffer with modulo indexing
WHY: O(1) insertion, no unbounded growth
GOTCHA: Loses old data when full
CODE: See internal/storage/ring_buffer.go

### Pattern: RWMutex for Concurrent Access
WHEN: Read-heavy with burst writes
USE: sync.RWMutex over sync.Map
WHY: 3x better throughput in benchmarks
GOTCHA: Remember write lock for modifications
APPLIES: All shared state in this codebase

### Pattern: Port 0 for Ephemeral Binding
WHEN: Need dynamic port allocation
USE: Bind to localhost:0
WHY: OS handles allocation, avoids conflicts
GOTCHA: Windows might return IPv6 (::1)
EXAMPLE: OTLP receiver binds to :0

## Observability Patterns (from Plan Review)

### Pattern: Eviction Callbacks for Index Cleanup
WHEN: Using ring buffers with indexes
USE: SetOnEvict callback to clean indexes when overwriting
WHY: Prevents memory leaks from stale index entries
CRITICAL: Without this, indexes grow unbounded
CODE: See Task 01 in observability plan

### Pattern: Zero-Copy Snapshots
WHEN: Need to isolate operation telemetry (deployments, tests)
USE: Bookmark ring buffer positions instead of copying data
WHY: 24 bytes per snapshot vs megabytes of data
BENEFIT: 10-100x more context-efficient
EXAMPLE: create_snapshot("before-deploy"), create_snapshot("after-deploy")

### Pattern: Snapshot-First Tool Design
WHEN: Designing tools for agent telemetry interaction
USE: Operations/time-windows as primary abstraction, not signals
WHY: Agents think "what happened during X?" not "get traces then logs then metrics"
BENEFIT: 5 tools instead of 26, automatic correlation
EXAMPLE: snapshot.get("before", "after") returns all correlated signals
INSIGHT: Most telemetry questions are about time windows, not specific signals

### Pattern: Multi-Signal Ring Buffers
WHEN: Storing different telemetry types
USE: Separate buffers with different capacities
RATIOS: Logs 5x traces, Metrics 10x traces
WHY: Different data volumes and retention needs
GOTCHA: Need consistent eviction strategy across buffers

### Pattern: Operations Over Signals (Paradigm Shift)
WHEN: Designing telemetry interfaces for agents
USE: Time windows and operations as primary abstraction
WHY: Agents think "what happened during deployment?" not "get traces then logs"
BREAKTHROUGH: This led to 26→5 tool reduction (80% simpler!)
IMPLEMENTATION: Snapshots bookmark all buffers simultaneously
IMPACT: Revolutionary simplification of agent-telemetry interaction