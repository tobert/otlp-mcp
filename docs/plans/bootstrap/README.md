# Bootstrap Plan: OTLP MCP Server MVP

This directory contains the complete implementation plan for building the MVP of otlp-mcp.

## Overview

Build a single-binary OTLP receiver + MCP server that enables AI agents to observe telemetry from programs they execute in a tight feedback loop.

## Task Files

Each numbered file is a self-contained task with:
- **Why**: Problem being solved
- **What**: Specific deliverables
- **Approach**: Technical decisions and code examples
- **Dependencies**: Prerequisites
- **Acceptance Criteria**: How to verify completion
- **Testing**: How to test the implementation
- **Status**: Track progress with jj

## Execution Order

```
01-project-setup.md
    ↓
02-cli-framework.md
    ↓
┌────────────────┴───────────────┐
│                                │
03-otlp-receiver.md    04-ring-buffer.md
│                                │
└────────────────┬───────────────┘
                 ↓
        05-mcp-server.md
                 ↓
        06-integration.md
                 ↓
        07-documentation.md
```

**Dependencies:**
- 01 → Everything (must be first)
- 02 → 06 (nice to have, not blocking)
- 03 → 06 (OTLP receiver needed for integration)
- 04 → 05, 06 (Storage needed for MCP and integration)
- 05 → 06 (MCP server needed for integration)

## Using These Plans with jj

### Starting a Task

```bash
# Review the task
cat docs/plans/bootstrap/01-project-setup.md

# Start a new jj change
jj new -m "feat: project setup and go module initialization"

# Work on the task, updating description as you go
jj describe

# When done, squash any fixups
jj squash
```

### Task Description Template

When you `jj describe`, use this format:

```
<type>: <summary>

Why: <from the task file>
Approach: <what you actually did, any deviations from plan>
Tried: <approaches that didn't work>
Context: <discoveries, gotchas, important notes>
Status: <complete | blocked: <reason> | next: <what's needed>>

Acceptance:
- [x] Criterion 1
- [x] Criterion 2
- [ ] Criterion 3 (blocked on X)

Co-authored-by: Claude <claude@anthropic.com>
```

### Tracking Progress

Mark status in each task file as you work:

```bash
# Update status in the task file
vim docs/plans/bootstrap/01-project-setup.md

# Change:
# Status: pending
# To:
# Status: complete ✓
```

## Success Criteria

MVP is complete when an agent can:

1. ✅ Start `otlp-mcp serve`
2. ✅ Call MCP tool to get OTLP endpoint
3. ✅ Run a program: `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:XXXXX my-program`
4. ✅ Query traces via MCP tools
5. ✅ Analyze trace data in conversation
6. ✅ Iterate quickly on program changes

## After MVP

Future enhancements (not in bootstrap plan):
- Logs and metrics support
- HTTP OTLP endpoint
- WebSocket MCP transport
- Trace export/import
- Advanced query language
- Sampling strategies

## Questions?

- **Technical decisions**: See task files for rationale
- **Architecture**: See `00-overview.md`
- **Contributing**: See `../../CLAUDE.md`
- **Agent workflow**: See `../../CLAUDE.md` sections 1-4

## Notes

- These are **living documents** - update as you learn
- **Document failures** in task status - saves future agents time
- **Keep context** - jj descriptions preserve reasoning across sessions
- **Be specific** - vague status like "almost done" doesn't help future you

Let's build this! 🚀
