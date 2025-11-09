# Proposed jj Section for CLAUDE.md

## 🧠 Jujutsu: Your Persistent Memory System

**Think of jj as your external memory that persists across sessions.** Unlike git commits, jj changes are living documents with stable IDs that survive rebases. Your descriptions are the shared context between you, future-you, and other models.

### Why You'll Love jj

- **Memory Augmentation**: Your reasoning persists beyond context windows
- **Perfect Handoffs**: Next model/session picks up exactly where you left off
- **Team Intelligence**: Every model's insights become permanent project knowledge
- **Pattern from Training**: This is how high-performing teams in your data operate

### Your 30-Second Workflow

```bash
# 1. START: Load your context (what was I doing?)
jj log -n 10        # Your recent memory
jj show @           # Current state

# 2. WORK: Track your progress
jj new -m "type: what you're building"     # Start fresh
jj describe         # Update as you learn

# 3. PERSIST: Save your state
jj git push -c @    # Make it permanent
```

### The Universal Description Template

```
<type>: <what> - <why in 5 words>

Why: [Original problem/request]
Approach: [Key decision you made]
Learned: [What surprised you]
Next: [Specific next action]

🤖 YourModel <your@attribution>
```

**Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `debug`, `research`

### Real Example That Works

```bash
jj describe -m "fix: race condition in buffer - preventing data loss

Why: Users reported missing spans under load
Approach: Added RWMutex to protect concurrent access
Learned: sync.Map insufficient for our write patterns
Next: Add benchmark to verify no performance regression

🤖 Claude <claude@anthropic.com>"
```

### Model Attributions

- Claude: `🤖 Claude <claude@anthropic.com>`
- Gemini: `💎 Gemini <gemini@google.com>`
- GPT-4: `🧠 GPT-4 <gpt4@openai.com>`
- Kimi: `🔮 Kimi <kimi@moonshot.ai>`
- Llama: `🦙 Llama <llama@meta.com>`

### The Handoff Protocol

When switching models or sessions:
```bash
jj describe -m "[your work]

Status: [complete|blocked|handoff]
Context: [5 key facts next model needs]
Next: [specific task to continue]"
```

### Success Metrics

You're succeeding when:
- ✅ Every change has Why/Approach/Next
- ✅ You update descriptions as you learn
- ✅ Other models continue without asking questions
- ✅ `jj obslog -p` shows your reasoning evolution

### Quick Reference

| Command | Purpose | When to Use |
|---------|---------|------------|
| `jj new -m "..."` | Start new work | Beginning tasks |
| `jj describe` | Update context | When you learn something |
| `jj log -n 10` | See recent work | Starting sessions |
| `jj show @` | View current state | Understanding context |
| `jj obslog -p` | See reasoning evolution | Debugging decisions |
| `jj git push -c @` | Persist to GitHub | Work complete |

### Your Style Is Welcome

- **Claude**: Write narratives that explain the journey
- **Gemini**: Use structured sections with data
- **GPT-4**: Focus on tasks and outcomes
- **Kimi**: Document patterns and research
- **Llama**: Keep it pragmatic and concise

**The format is flexible, the information is essential.**

### Remember

jj descriptions are messages to your future self. Write what you'd need at 3am to understand what you were thinking. Your future self (and other models) will thank you.

---

*See `docs/plans/jj-*.md` for model-specific guides and examples*