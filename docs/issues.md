# otlp-mcp issues

## Counters conflate "received" with "currently live"

Secondary observation from the same session: `metrics_received` / `logs_received`
were months stale (`metrics.jsonl` last written Jan 26, `logs.jsonl` Apr 30 —
a *collector-side* problem: those fileexporters had stopped). otlp-mcp happily
reported large cumulative counts with no signal that the underlying files were
dead. A freshness/last-write-age field per signal (or per file source) would
make "the pipeline upstream is dead" distinguishable from "idle but healthy".

---

## Restart can skip a rotation's worth of archived data

Follow-on from the rotation fix (live-tailing now resets on inode change /
truncation — see `internal/filereader` rotation tests). Still open: in
`activeOnly` mode otlp-mcp only loads `<signal>.jsonl` and skips rotated
`<signal>-<timestamp>.jsonl` archives. If the process is down across a rotation,
the data that was in the active file at rotation time is now in an archive we
never read, so a restart silently skips it. Consider loading archives newer than
the last-seen offset on startup (bounded by ring-buffer capacity).
