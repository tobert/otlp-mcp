# otlp-mcp issues

## Tailer stops following the active file across collector rotation (stale-offset)

**Observed 2026-05-29.** The kaijutsu otelcol-contrib collector writes
signals to `/tank/otel/{traces,logs,metrics}` via fileexporter, rotating the
active `<signal>.jsonl` to `<signal>-<timestamp>.jsonl` and creating a fresh
`<signal>.jsonl`. otlp-mcp watches those dirs and tails the active file.

Symptom: `traces.jsonl` was live and growing (last span ~now, file ~99.5 MB),
yet `status` reported `spans_received` frozen at 562566 and `query` returned
nothing newer than the **previous rotation** (May 25). Live idle traffic
(the app's 5 s drift poll) was invisible. A process restart cleared it: on
startup otlp-mcp re-read the active files to EOF and resumed following appends,
and `spans_received` began climbing again.

**Leading hypothesis (contributing factors, not a single root cause):**
offset is keyed by path and not reset when the file shrinks or its inode
changes. The fileexporter rotates by renaming the active file out and creating
a new empty one at the same path. The stored offset still held the *previous*
cycle's end-of-file (~104 MB, the size at the May 25 rotation). The new
`traces.jsonl` only had to grow back past that stale offset before any line
looked "new" — it had reached ~99.5 MB (still under ~104 MB), so nothing was
emitted for ~3 days. Restart re-stat'd from 0 and the problem vanished.

This is the classic `tail -F` vs `tail -f` distinction: we need to detect
rotation/truncation (inode change or size decrease) and reset the read offset
to 0, rather than trusting a monotonic path→offset map.

**Suggested fix:**
- Track `(inode, size)` per watched file, not just path→offset.
- On inode change → treat as a new file, read from 0.
- On size < stored offset (truncation/fresh file at same path) → reset offset to 0.
- Consider also reading rotated `<signal>-*.jsonl` files created while we were
  down, so a restart doesn't silently skip a rotation's worth of data.

**Repro sketch:** point a file source at a dir; write N bytes to `f.jsonl`;
`mv f.jsonl f.1.jsonl`; create new `f.jsonl`; append M < N bytes. Expect the M
bytes to surface; currently they don't until the file exceeds N.

---

## Counters conflate "received" with "currently live"

Secondary observation from the same session: `metrics_received` / `logs_received`
were months stale (`metrics.jsonl` last written Jan 26, `logs.jsonl` Apr 30 —
a *collector-side* problem: those fileexporters had stopped). otlp-mcp happily
reported large cumulative counts with no signal that the underlying files were
dead. A freshness/last-write-age field per signal (or per file source) would
make "the pipeline upstream is dead" distinguishable from "idle but healthy".
