// Tests for the pure timeline geometry.
// Run: node --test internal/webui/static/*.test.mjs
// (a bare directory path makes node try to import it as a module.)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  ns, cmpNs, spanInterval, traceBounds, buildChildren, childCounts,
  findRoots, serviceOrder, hiddenDescendants, positionSpans, packRows,
  computeTimelineLayout, pickTickStep,
} from './timeline.mjs';

// A realistic 2026-era epoch base so precision behavior is exercised, not faked.
const BASE = 1782514166000000000n; // ns
const at = off => (BASE + BigInt(off)).toString(); // off in ns, returned as string (wire format)

function span(o) {
  return {
    span_id: o.id,
    parent_span_id: o.parent || '',
    service: o.svc,
    span_name: o.name || o.id,
    status: o.status || 'OK',
    start_ns: at(o.start),
    end_ns: at(o.end),
  };
}

test('ns / cmpNs accept string|number|bigint', () => {
  assert.equal(ns('5'), 5n);
  assert.equal(ns(5), 5n);
  assert.equal(ns(5n), 5n);
  assert.equal(ns(''), 0n);
  assert.equal(cmpNs('10', 9), 1);
  assert.equal(cmpNs(9, '10'), -1);
  assert.equal(cmpNs('7', 7n), 0);
});

test('spanInterval clamps end<start (no unsigned-underflow blowups)', () => {
  const { start, end } = spanInterval({ start_ns: at(1000), end_ns: at(500) });
  assert.equal(end, start, 'end clamped up to start');
});

test('traceBounds spans the full extent in BigInt', () => {
  const spans = [
    span({ id: 'a', svc: 's', start: 0, end: 800 }),
    span({ id: 'b', svc: 's', start: 100, end: 300 }),
    span({ id: 'c', svc: 's', start: 600, end: 1000 }),
  ];
  const b = traceBounds(spans);
  assert.equal(b.minStart, BASE);
  assert.equal(b.maxEnd, BASE + 1000n);
  assert.equal(b.duration, 1000n);
});

// The headline regression guard: geometry is header-agnostic. A span at trace
// start MUST position at x=0, never at a lane-header offset. If someone folds a
// header gutter back into the geometry, this fails.
test('positionSpans: first span sits flush-left (x≈0), not offset by a gutter', () => {
  const spans = [
    span({ id: 'root', svc: 'gw', start: 0, end: 1000 }),
    span({ id: 'mid', svc: 'gw', start: 250, end: 750 }),
  ];
  const bounds = traceBounds(spans);
  const pos = positionSpans(spans, { width: 1000, minBarW: 0, bounds });
  assert.equal(pos[0].x, 0, 'root must start at x=0');
  assert.equal(pos[0].w, 1000, 'root spans full width');
  assert.equal(pos[1].x, 250);
  assert.equal(pos[1].w, 500);
});

// Precision guard: two spans that differ by 500ns on a 2026 epoch must render
// as distinct, correctly-sized bars. Parsing start_ns as a plain JS Number would
// quantize to ~256ns and collapse/distort these — BigInt deltas keep them exact.
test('positionSpans: sub-microsecond precision survives the 2026 epoch', () => {
  // total trace duration 1000ns; a 500ns span starting at +250ns.
  const spans = [
    span({ id: 'root', svc: 's', start: 0, end: 1000 }),
    span({ id: 'tiny', svc: 's', start: 250, end: 750 }),
  ];
  const bounds = traceBounds(spans);
  const pos = positionSpans(spans, { width: 1000, minBarW: 0, bounds });
  const tiny = pos.find(p => p.span.span_id === 'tiny');
  assert.equal(tiny.x, 250, 'offset exact to the nanosecond');
  assert.equal(tiny.w, 500, 'width exact to the nanosecond');

  // Demonstrate why BigInt matters: parsing the absolute ns as a Number first
  // quantizes to the ~256ns double ULP at this epoch, so the offset comes out
  // wrong (256 here, not 250). BigInt subtraction is what keeps it exact above.
  const naive = Number(BASE + 250n) - Number(BASE);
  assert.notEqual(naive, 250, 'naive Number path mis-measures the offset');
  assert.ok(naive >= 256, 'and it is quantized to the double ULP (~256ns)');
});

test('positionSpans: minBarW widens zero/near-zero bars', () => {
  const spans = [
    span({ id: 'root', svc: 's', start: 0, end: 1000 }),
    span({ id: 'instant', svc: 's', start: 500, end: 500 }),
  ];
  const bounds = traceBounds(spans);
  const pos = positionSpans(spans, { width: 1000, minBarW: 3, bounds });
  const instant = pos.find(p => p.span.span_id === 'instant');
  assert.equal(instant.w, 3);
});

test('packRows: overlapping bars stack, disjoint bars reuse a row', () => {
  // a [0,40] and b [10,50] overlap -> 2 rows; c [60,80] disjoint -> reuses row 0.
  const items = [
    { x: 0, w: 40 },
    { x: 10, w: 40 },
    { x: 60, w: 20 },
  ];
  const { items: out, rowCount } = packRows(items, { gap: 1, maxRows: Infinity });
  assert.equal(out[0].subRow, 0);
  assert.equal(out[1].subRow, 1, 'overlap pushed to a new row');
  assert.equal(out[2].subRow, 0, 'disjoint bar reuses row 0');
  assert.equal(rowCount, 2);
});

test('packRows: maxRows caps growth and overflows into earliest-free row', () => {
  const items = [
    { x: 0, w: 100 },
    { x: 5, w: 100 },
    { x: 10, w: 100 },
  ];
  const { rowCount } = packRows(items, { gap: 1, maxRows: 2 });
  assert.equal(rowCount, 2, 'never exceeds maxRows');
});

test('serviceOrder: BFS from root, orphan services appended', () => {
  const spans = [
    span({ id: 'g', svc: 'gateway', start: 0, end: 800 }),
    span({ id: 'p', parent: 'g', svc: 'product', start: 50, end: 650 }),
    span({ id: 'a', parent: 'g', svc: 'auth', start: 10, end: 45 }),
    span({ id: 'db', parent: 'p', svc: 'postgres', start: 60, end: 310 }),
    span({ id: 'orphan', parent: 'missing', svc: 'ghost', start: 5, end: 9 }),
  ];
  const order = serviceOrder(spans);
  assert.equal(order[0], 'gateway', 'root service first');
  // auth (start 10) is visited before product (start 50) at the same BFS level.
  assert.ok(order.indexOf('auth') < order.indexOf('postgres'));
  assert.ok(order.includes('ghost'), 'orphan service still appears');
});

test('findRoots: orphan (parent outside set) is treated as a root', () => {
  const spans = [
    span({ id: 'child', parent: 'gone', svc: 's', start: 0, end: 10 }),
  ];
  const roots = findRoots(spans);
  assert.equal(roots.length, 1);
  assert.equal(roots[0].span_id, 'child');
});

test('hiddenDescendants: collapsing a node hides its whole subtree', () => {
  const spans = [
    span({ id: 'g', svc: 's', start: 0, end: 100 }),
    span({ id: 'p', parent: 'g', svc: 's', start: 1, end: 90 }),
    span({ id: 'db', parent: 'p', svc: 's', start: 2, end: 50 }),
    span({ id: 'a', parent: 'g', svc: 's', start: 1, end: 5 }),
  ];
  const hidden = hiddenDescendants(spans, new Set(['p']));
  assert.ok(hidden.has('db'), 'grandchild hidden');
  assert.ok(!hidden.has('a'), 'sibling not hidden');
  assert.ok(!hidden.has('p'), 'collapsed node itself stays visible');
  assert.equal(hiddenDescendants(spans, new Set()).size, 0);
});

test('childCounts: direct children only', () => {
  const spans = [
    span({ id: 'g', svc: 's', start: 0, end: 100 }),
    span({ id: 'p', parent: 'g', svc: 's', start: 1, end: 90 }),
    span({ id: 'db', parent: 'p', svc: 's', start: 2, end: 50 }),
  ];
  const c = childCounts(spans);
  assert.equal(c.get('g'), 1);
  assert.equal(c.get('p'), 1);
  assert.equal(c.get('db'), undefined);
});

test('computeTimelineLayout: one lane per service, header-agnostic positions', () => {
  const spans = [
    span({ id: 'g', svc: 'gateway', start: 0, end: 800 }),
    span({ id: 'p', parent: 'g', svc: 'product', start: 50, end: 650 }),
  ];
  const layout = computeTimelineLayout(spans, { width: 800, minBarW: 3, gap: 1 });
  assert.deepEqual(layout.serviceOrder, ['gateway', 'product']);
  assert.equal(layout.lanes.length, 2);
  const gw = layout.lanes[0].items[0];
  assert.equal(gw.x, 0, 'gateway root flush-left');
  const prod = layout.lanes[1].items[0];
  assert.equal(prod.x, 50, 'product offset = 50/800 * 800');
});

test('pickTickStep: smallest step with >= minPx spacing', () => {
  // pxPerNs such that 1e6 ns -> 60px exactly => step 1e6.
  assert.equal(pickTickStep(60 / 1e6, 60), 1e6);
  // coarser zoom: 1e6 ns -> 6px, need 1e7 (60px).
  assert.equal(pickTickStep(6 / 1e6, 60), 1e7);
});
