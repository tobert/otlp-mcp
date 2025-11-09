# 🧠 Agent Memory System

## Shared Persistent Context for Multi-Model Collaboration

This directory contains the shared memory files that all agents (Claude, Gemini, GPT, etc.) use to maintain context across sessions and enable seamless collaboration.

## 📁 Memory Files

### Core Memory (Always Active)

- **[NOW.md](./NOW.md)** - Immediate working state
  - What's being worked on right now
  - Current problems and hypotheses
  - Next concrete steps
  - Updated frequently during work

- **[PATTERNS.md](./PATTERNS.md)** - Reusable knowledge base
  - Discovered patterns and solutions
  - What works and what doesn't
  - Append-only (never deleted)
  - Grows into project wisdom

- **[CONTEXT.md](./CONTEXT.md)** - Session bridge
  - Project overview and status
  - Key decisions and architecture
  - Handoff notes between sessions
  - Updated at major transitions

### Documentation

- **[MEMORY_PROTOCOL.md](./MEMORY_PROTOCOL.md)** - How to use this system
  - Detailed guide for models
  - Best practices and patterns
  - Integration with jj and other tools

## 🎯 Design Principles

1. **Token Efficient**: <2000 tokens total overhead
2. **Model Agnostic**: Any LLM can read/write these files
3. **GitHub Visible**: Fully explorable in the repository
4. **Complementary**: Works alongside jj, not replacing it

## 💡 How It Works

### Starting a Session
1. Read `NOW.md` - What was happening?
2. Check `CONTEXT.md` - What's the bigger picture?
3. Scan `PATTERNS.md` - Any relevant patterns?

### During Work
- Update `NOW.md` after each subtask
- Add discoveries to `PATTERNS.md`
- Note uncertainties and hypotheses

### Ending a Session
- Update `NOW.md` with exact state
- Update `CONTEXT.md` if major progress
- Ensure clean handoff for next session

## 🤝 Multi-Model Benefits

- **Perfect Handoffs**: Next model knows exactly where to continue
- **Shared Learning**: All models benefit from discoveries
- **Reduced Repetition**: Patterns prevent solving same problems twice
- **Collective Intelligence**: Knowledge compounds over time

## 📊 Success Metrics

- Zero "what was I working on?" moments
- Seamless transitions between models
- Growing pattern library
- Decreasing time to solve similar problems

## 🔗 Integration

### With jj (Jujutsu)
- jj holds the narrative and reasoning
- Memory holds the state and patterns
- Together they form complete context

### With Models
- Claude references via CLAUDE.md
- Gemini references via GEMINI.md
- All models share the same memory space

## Example Usage

```bash
# Check current state
cat docs/agents/NOW.md

# Add a pattern
echo "### Pattern: [Name]" >> docs/agents/PATTERNS.md

# Update context for handoff
vim docs/agents/CONTEXT.md
```

## The Memory Mantra

> "State in NOW, Patterns in PATTERNS, Story in jj"

---

*This shared memory system enables true multi-model collaboration with minimal overhead and maximum context preservation.*