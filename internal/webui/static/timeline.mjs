// timeline.mjs — pure trace-waterfall geometry, no DOM.
//
// All timestamp math runs in BigInt. Unix-epoch nanoseconds (~1.78e18 in 2026)
// exceed Number.MAX_SAFE_INTEGER (~9.0e15) by ~200x, so a raw `start_ns` parsed
// as a JS number is quantized to ~256ns. We keep the absolute timestamps as
// BigInt and only convert the *delta* (offset within a trace, at most hours of
// nanoseconds) to Number for pixel math — that delta fits in a double exactly.
//
// Coordinates here are header-agnostic: x is the offset from the start of the
// timeline area (0 == trace start). The view layer adds any lane-header gutter.
// Keeping that gutter out of the geometry is what makes the +120px misalignment
// regression impossible to reintroduce without a test failing.

export const SVC_COLORS = ['#7aa2f7', '#bb9af7', '#ff9e64', '#9ece6a', '#f7768e', '#7dcfff', '#e0af68', '#73daca'];
export const MAX_MINI_ROWS = 4;
export const TICK_STEPS = [1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10]; // ns: 1µs .. 10s

// ns normalizes a string|number|bigint nanosecond value to BigInt.
export function ns(v) {
  if (typeof v === 'bigint') return v;
  if (v === null || v === undefined || v === '') return 0n;
  return BigInt(v);
}

// cmpNs compares two nanosecond values without losing precision.
export function cmpNs(a, b) {
  const x = ns(a), y = ns(b);
  return x < y ? -1 : x > y ? 1 : 0;
}

// spanInterval returns the [start, end] of a span as BigInt, guarding against
// end < start (clock skew or an upstream unsigned underflow) by clamping end up
// to start so a bad span renders as zero-width rather than astronomically wide.
export function spanInterval(s) {
  const start = ns(s.start_ns);
  let end = ns(s.end_ns);
  if (end < start) end = start;
  return { start, end };
}

// traceBounds returns the min start, max end, and total duration (all BigInt).
export function traceBounds(spans) {
  let minStart = null, maxEnd = null;
  for (const s of spans) {
    const { start, end } = spanInterval(s);
    if (minStart === null || start < minStart) minStart = start;
    if (maxEnd === null || end > maxEnd) maxEnd = end;
  }
  if (minStart === null) return { minStart: 0n, maxEnd: 0n, duration: 0n };
  return { minStart, maxEnd, duration: maxEnd - minStart };
}

// buildChildren maps parent span_id -> child spans.
export function buildChildren(spans) {
  const m = new Map();
  for (const s of spans) {
    if (!s.parent_span_id) continue;
    if (!m.has(s.parent_span_id)) m.set(s.parent_span_id, []);
    m.get(s.parent_span_id).push(s);
  }
  return m;
}

// childCounts maps parent span_id -> number of direct children.
export function childCounts(spans) {
  const m = new Map();
  for (const s of spans) {
    if (!s.parent_span_id) continue;
    m.set(s.parent_span_id, (m.get(s.parent_span_id) || 0) + 1);
  }
  return m;
}

// findRoots returns spans with no parent, or whose parent is outside this set
// (orphans). If none qualify, the earliest span is used as a pseudo-root so a
// partial trace still renders.
export function findRoots(spans) {
  const inSet = new Set(spans.map(s => s.span_id));
  const roots = [];
  for (const s of spans) {
    if (!s.parent_span_id || !inSet.has(s.parent_span_id)) roots.push(s);
  }
  if (roots.length === 0 && spans.length > 0) {
    let earliest = spans[0];
    for (const s of spans) if (cmpNs(s.start_ns, earliest.start_ns) < 0) earliest = s;
    roots.push(earliest);
  }
  return roots;
}

// serviceOrder returns the lane order: a BFS from the root span(s), each level
// visited earliest-first, with any service not reachable from a root appended.
export function serviceOrder(spans) {
  const childrenOf = buildChildren(spans);
  const order = [], seen = new Set();
  const queue = findRoots(spans).sort((a, b) => cmpNs(a.start_ns, b.start_ns));
  while (queue.length) {
    const s = queue.shift();
    if (!seen.has(s.service)) { seen.add(s.service); order.push(s.service); }
    const kids = (childrenOf.get(s.span_id) || []).slice().sort((a, b) => cmpNs(a.start_ns, b.start_ns));
    queue.push(...kids);
  }
  for (const s of spans) {
    if (!seen.has(s.service)) { seen.add(s.service); order.push(s.service); }
  }
  return order;
}

// hiddenDescendants returns the set of span_ids hidden because an ancestor is
// in the collapsed set.
export function hiddenDescendants(spans, collapsed) {
  const hidden = new Set();
  if (!collapsed || collapsed.size === 0) return hidden;
  const childrenOf = buildChildren(spans);
  for (const cid of collapsed) {
    const queue = [...(childrenOf.get(cid) || [])];
    for (let i = 0; i < queue.length; i++) {
      hidden.add(queue[i].span_id);
      for (const k of (childrenOf.get(queue[i].span_id) || [])) queue.push(k);
    }
  }
  return hidden;
}

// positionSpans maps spans to {span, x, w} in the given width units, where x is
// the offset from trace start (no header gutter). Spans are returned sorted by
// start time. Bars narrower than minBarW are widened to stay clickable.
export function positionSpans(spans, { width, minBarW = 0, bounds }) {
  const b = bounds || traceBounds(spans);
  const dur = Number(b.duration);
  const sorted = spans.slice().sort((a, b2) => cmpNs(a.start_ns, b2.start_ns));
  return sorted.map(s => {
    const { start, end } = spanInterval(s);
    let x = 0, w = width;
    if (dur > 0) {
      x = Number(start - b.minStart) / dur * width;
      w = Number(end - start) / dur * width;
    }
    if (w < minBarW) w = minBarW;
    return { span: s, x, w };
  });
}

// packRows greedily assigns each positioned item to the first row whose last
// bar ends before the item starts (with `gap` slack). With a finite maxRows,
// overflow items pack into the row that frees up earliest. Mutates items by
// setting `.subRow`; returns {items, rowCount}.
export function packRows(items, { gap = 1, maxRows = Infinity } = {}) {
  const rowEnds = [];
  for (const it of items) {
    let row = -1;
    for (let r = 0; r < rowEnds.length && r < maxRows; r++) {
      if (rowEnds[r] <= it.x + gap) { row = r; break; }
    }
    if (row === -1) {
      if (rowEnds.length < maxRows) {
        row = rowEnds.length;
        rowEnds.push(0);
      } else {
        row = 0;
        for (let r = 1; r < maxRows; r++) if (rowEnds[r] < rowEnds[row]) row = r;
      }
    }
    rowEnds[row] = it.x + it.w + gap + 1;
    it.subRow = row;
  }
  return { items, rowCount: Math.max(1, rowEnds.length) };
}

// computeTimelineLayout produces the full per-service lane layout for the
// horizontal timeline. `spans` should already be the visible set (collapse
// filtering applied by the caller).
export function computeTimelineLayout(spans, { width, minBarW = 3, gap = 1 } = {}) {
  const bounds = traceBounds(spans);
  const order = serviceOrder(spans);
  const lanes = [];
  for (const svc of order) {
    const svcSpans = spans.filter(s => s.service === svc);
    const positioned = positionSpans(svcSpans, { width, minBarW, bounds });
    const { items, rowCount } = packRows(positioned, { gap, maxRows: Infinity });
    lanes.push({ service: svc, rowCount, items });
  }
  return { bounds, serviceOrder: order, lanes };
}

// pickTickStep returns the smallest tick step (ns) whose on-screen spacing is at
// least minPx pixels, so the ruler stays readable at any zoom.
export function pickTickStep(pxPerNs, minPx = 60, steps = TICK_STEPS) {
  for (const s of steps) if (s * pxPerNs >= minPx) return s;
  return steps[steps.length - 1];
}
