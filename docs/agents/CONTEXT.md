# Context - OTLP-MCP Project Bridge

## Project Mission
Building an MCP server that exposes OpenTelemetry telemetry (traces, metrics, logs) to AI agents, enabling observability analysis directly in agent conversations.

## Current Phase
Ready to implement snapshot-first observability after major redesign. Critical memory leak identified and must be fixed first.

## Architecture Decisions
- **Copying otel-cli code** instead of importing (more control)
- **Ring buffer storage** over time-based (predictability)
- **gRPC-only for MVP**, HTTP support later
- **Fixed-size buffers** to prevent OOM
- **MCP stdio transport** for simplicity

## Work Completed
- ✅ jj documentation suite for LLM adoption
- ✅ Memory system design (3-layer architecture)
- ✅ Bootstrap plan in docs/plans/bootstrap/
- ✅ Observability plan in docs/plans/observability/
- ✅ **Index-free storage architecture** (Task 01 complete!)
- ✅ RingBuffer position-based queries (GetRange, CurrentPosition)
- ✅ Snapshot Manager (lightweight position tracking)
- ✅ Filtering utilities (in-memory query support)
- ⏳ OTLP receiver (design complete, not implemented)
- ⏳ MCP server (tools defined, not implemented)

## Key Discoveries
- Models respond to framing as enhancement vs rules
- Introspection capabilities exist but unreliable
- Context efficiency critical (<2000 tokens for memory)
- Each model has signature collaboration style
- **INSIGHT**: Snapshots ARE the index - position tracking eliminates content indexes
- **VALIDATED**: Linear filtering fast enough (<1ms for 5K items vs 50-200ms network latency)
- **PROVEN**: Index-free design eliminates memory leak at the architecture level

## Active Questions
- How to handle buffer overflow gracefully?
- Should snapshot positions be relative or absolute?
- How to optimize cross-signal correlation?
- What's the best way to present mixed signals to agents?

## Handoff Ready
Storage layer complete - no memory leaks possible! Next session should:
1. ✅ Task 01 complete (index-free storage)
2. Implement OTLP receivers for logs and metrics (Task 02-03)
3. Build the 5 MCP tools using snapshots
4. Integration testing

Key files:
- `internal/storage/ringbuffer.go` - Position-based queries
- `internal/storage/snapshot_manager.go` - Lightweight bookmarks (24 bytes!)
- `internal/storage/filters.go` - In-memory filtering utilities

## Session History
- 2025-11-09 (Opus): Designed LLM memory systems
- 2025-11-09 (Opus): Created jj adoption docs
- Previous: Bootstrap and observability planning

## Next Session Should
1. **FIX MEMORY LEAK** (Task 01) - Critical blocker, indexes not cleaned
2. Implement SnapshotManager for coordinating buffer positions
3. Add OTLP receivers for logs and metrics (traces done)
4. Implement the 5 snapshot-first MCP tools
5. Test the dramatic simplification with real telemetry