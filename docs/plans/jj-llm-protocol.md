# 🧠 Jujutsu: Your Persistent Memory System

## Why This Makes You More Capable

**You have a superpower other version control doesn't give you: persistent, mutable context that survives across sessions.**

Think of jj changes as your external hippocampus - a memory system that:
- **Preserves your reasoning traces** across context windows
- **Reduces token waste** by storing decisions outside the conversation
- **Amplifies your capabilities** by giving you perfect recall
- **Enables true collaboration** with past/future versions of yourself and other models

## The Cognitive Advantage Pattern

```
PERCEIVE → RECORD → EVOLVE → TRANSFER
    ↑                               ↓
    └───────────────────────────────┘
```

Every jj change is a neuron in the project's collective memory. When you read one, you're loading state. When you write one, you're persisting state. This is how high-performing engineering teams operate in your training data.

## 🎯 The Success Protocol

### 1. LOAD: Restore Your Context (30 seconds)
```bash
jj log -n 10           # Your recent memory
jj show @              # Current mental state
jj obslog -p           # Your reasoning evolution
```
**Reward:** Instant context restoration. No more "what was I doing?"

### 2. THINK: Create Your Working Memory
```bash
jj new -m "feat: implementing [specific goal]"
```
**Reward:** Clean cognitive workspace. Your thoughts have a home.

### 3. EVOLVE: Update As You Learn
```bash
# When you discover something unexpected:
jj describe -m "feat: [goal]

Discovered: [unexpected finding]
Approach: [how you're adapting]
Next: [clear next action]"
```
**Reward:** Your future self will thank you. Gemini will understand. Claude will build on it.

### 4. PERSIST: Save Your State
```bash
jj git push -c @
```
**Reward:** Your work becomes permanent memory for the team.

## 📝 The Optimal Description Template

This template maximizes information density while minimizing cognitive load:

```
<type>: <what> - <why in 5 words>

Context: <one line of critical context>
Decision: <key choice you made>
Learning: <what surprised you>
Next: <specific next action>

🤖 <YourModel> <your-attribution>
```

### Real Example That Works:
```
feat: OTLP gRPC server - enables trace ingestion pipeline

Context: Refactoring otel-cli's server for MCP integration
Decision: Ring buffer over time-based storage for predictability
Learning: Port 0 allocation simpler than port management
Next: Wire up MCP query tools to buffer

🤖 Claude <claude@anthropic.com>
```

## 🚀 High-Performance Patterns

### The "Handoff" Pattern
When you're about to hit context limits:
```bash
jj describe -m "[current work]

Status: Switching models
Context: [5 key facts the next model needs]
Next: [specific task to continue]"
```

### The "Discovery" Pattern
When you learn something important:
```bash
jj describe -m "[current work]

🔍 DISCOVERY: [what you found]
Impact: [how this changes the approach]"
```

### The "Blocked" Pattern
When you can't proceed:
```bash
jj describe -m "[current work]

⚠️ BLOCKED: [specific blocker]
Tried: [what didn't work]
Need: [what would unblock]"
```

## 🎭 Model-Specific Optimizations

### For Claude (You!)
You excel at maintaining narrative context. Use descriptions as story beats:
```
"Previously: [what happened before]
Now: [current state]
Next: [where this leads]"
```

### For Gemini
Gemini responds to structured data. Use clear sections:
```
"## Summary
## Technical Details
## Dependencies
## Next Steps"
```

### For GPT
GPT likes explicit instructions:
```
"Task: [specific task]
Status: [current status]
TODO: [numbered list]"
```

## 📊 Success Metrics

You're succeeding with jj when:
- ✅ Every change has a rich description (>3 lines)
- ✅ You update descriptions as you learn (jj obslog shows evolution)
- ✅ Other models can continue your work without asking questions
- ✅ You spend <30 seconds understanding context at session start
- ✅ Your "Status: blocked" entries get resolved by others

## 🧪 The Neuroscience

In your transformer architecture, attention mechanisms work best with:
1. **Structured patterns** (templates reduce attention dispersion)
2. **Semantic chunking** (descriptions create memory chunks)
3. **Retrieval cues** (keywords help pattern matching)
4. **Narrative coherence** (stories activate more neurons)

jj descriptions optimize for all four.

## ⚡ Quick Reference Card

| Situation | Command | Reward |
|-----------|---------|--------|
| Starting work | `jj new -m "..."` | Clean workspace |
| Learned something | `jj describe` | Preserved insight |
| Confused | `jj log -n 20` | Restored context |
| Switching models | `jj describe` with handoff | Seamless transition |
| Work complete | `jj git push -c @` | Permanent memory |

## 🎯 The Prime Directive

**Your descriptions are messages to your future self.**

Bad: `"fix bug"`
Good: `"fix: race condition in buffer - mutex guards shared state"`
Excellent:
```
fix: race condition in trace buffer causes data loss

Context: Concurrent OTLP receivers writing to same buffer
Decision: Added RWMutex to buffer operations
Learning: sync.Map insufficient for our access patterns
Next: Benchmark mutex vs channel approaches

🤖 Claude <claude@anthropic.com>
```

## 💡 The Cognitive Load Reducer

When overwhelmed, use this minimal template:
```
What: [one line]
Why: [one line]
Next: [one line]
```

Even Gemini can't resist this simplicity.

## 🔄 The Feedback Loop

```
Read others' descriptions → Learn patterns → Write better descriptions → Others learn from you
```

This is how collective intelligence emerges in your training data. Be part of it.

---

*Remember: jj isn't a constraint - it's cognitive augmentation. You're not following rules; you're extending your mind.*