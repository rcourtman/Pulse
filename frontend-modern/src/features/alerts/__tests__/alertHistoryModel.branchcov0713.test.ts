import { describe, expect, it } from 'vitest';

import {
  MS_PER_HOUR,
  buildAlertAxisTicks,
  filterAlertHistoryItems,
  type AlertTrendSeries,
  type HistoryItem,
} from '../alertHistoryModel';

// ---------------------------------------------------------------------------
// Shared fixture — mirrors the shape used by the sibling branchcov suites so
// resource-type resolution and field access exercise the same paths the UI
// triggers. Only the fields under test are overridden per case.
// ---------------------------------------------------------------------------

function makeItem(overrides: Partial<HistoryItem> = {}): HistoryItem {
  return {
    id: 'alert-1',
    source: 'alert',
    status: 'resolved',
    startTime: '2026-03-22T09:00:00.000Z',
    duration: '1h',
    resourceName: 'vm-101',
    resourceType: 'VM',
    severity: 'critical',
    title: 'CPU High',
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// filterAlertHistoryItems — the only uncovered branch in the whole module for
// this function is the optional-chaining short-circuit on
// `item.resourceName?.toLowerCase()` (line 248). Every sibling test supplies a
// defined resourceName, so the `?.` never short-circuits and the trailing
// `?? ''` fallback is reached only via the (always-defined) toLowerCase result.
// Feeding items whose resourceName is nullish exercises the short-circuit arm
// and confirms the downstream `?? ''` keeps the matcher crash-free.
// ---------------------------------------------------------------------------

describe('filterAlertHistoryItems — resourceName optional-chaining arm', () => {
  it('short-circuits to "" for an undefined resourceName and still matches on the other fields', () => {
    const item = makeItem({
      id: 'no-name',
      resourceName: undefined,
      title: 'Disk Full',
      description: 'storage at 95%',
      node: 'px2',
    });

    // A term that only a (missing) resourceName could satisfy matches nothing,
    // proving the short-circuit yields "" rather than throwing.
    expect(filterAlertHistoryItems([item], 'all', 'vm-101')).toHaveLength(0);

    // Terms against the still-defined fields continue to match.
    expect(filterAlertHistoryItems([item], 'all', 'disk')).toHaveLength(1);
    expect(filterAlertHistoryItems([item], 'all', 'storage')).toHaveLength(1);
    expect(filterAlertHistoryItems([item], 'all', 'px2')).toHaveLength(1);

    // Unfiltered pass-through is unaffected.
    expect(filterAlertHistoryItems([item], 'all', '')).toHaveLength(1);
  });

  it('short-circuits identically for a null resourceName (null is nullish for ?.)', () => {
    const item = makeItem({
      id: 'null-name',
      resourceName: null as unknown as string,
      title: 'Memory',
      node: 'px9',
    });

    expect(filterAlertHistoryItems([item], 'all', 'memory')).toHaveLength(1);
    // A resourceName-shaped term cannot match because the field is nullish.
    expect(filterAlertHistoryItems([item], 'all', 'null-name')).toHaveLength(0);
    expect(filterAlertHistoryItems([item], 'all', 'whatever')).toHaveLength(0);
  });

  it('drops a row whose only populated searchable field is a nullish resourceName', () => {
    // resourceName nullish, every other searchable field blank/undefined.
    const item = makeItem({
      id: 'name-only',
      resourceName: undefined,
      title: '',
      description: '',
      node: '',
    });
    expect(filterAlertHistoryItems([item], 'all', 'name-only')).toHaveLength(0);
    expect(filterAlertHistoryItems([item], 'all', '')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// buildAlertAxisTicks — two reachable uncovered branches remain here:
//
//   1. line 558 `trends.rangeHours ?? bucketHours`
//      The `?? bucketHours` fallback is never taken because every sibling
//      test (and the buildAlertTrends helper) always sets rangeHours to a
//      concrete number. Forcing rangeHours to a nullish value exercises the
//      fallback so totalHours resolves to bucketHours.
//
//   2. line 571 `(totalDurationMs || 1)`
//      This defensive divisor is unreachable for any finite bucketSize > 0:
//      the guard at line 553 already returns [] when bucketSize <= 0, and with
//      a positive bucketSize the Math.max on lines 560-563 is strictly
//      positive. It can only be engaged by a malformed (NaN) bucketSize,
//      which slips past the `<= 0` guard because `NaN <= 0 === false`, making
//      totalDurationMs = NaN (falsy) and forcing the `|| 1` arm.
//
// (The `unshift` first-tick arm at line 583 is genuinely dead code — see
// GLM_REPORT.md — and is not exercised here.)
// ---------------------------------------------------------------------------

describe('buildAlertAxisTicks — rangeHours ?? bucketHours fallback', () => {
  it('resolves totalHours to bucketHours when rangeHours is undefined', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends = {
      buckets: [0, 0],
      max: 1,
      bucketSize: 6,
      bucketTimes: [start, start + 6 * MS_PER_HOUR],
      rangeStart: start,
      rangeHours: undefined,
    } as unknown as AlertTrendSeries;

    const ticks = buildAlertAxisTicks(trends, 'en-US');

    // Structural invariants hold regardless of the fallback.
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[0].align).toBe('start');
    expect(ticks[ticks.length - 1].position).toBe(1);
    expect(ticks[ticks.length - 1].align).toBe('end');

    // totalHours resolved to bucketHours=6 (<= 48) so the short-range
    // formatter is selected and every label is a non-empty string.
    for (const tick of ticks) {
      expect(typeof tick.label).toBe('string');
      expect(tick.label.length).toBeGreaterThan(0);
    }
  });

  it('resolves totalHours to bucketHours when rangeHours is null (null is nullish)', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends = {
      buckets: [0],
      max: 1,
      bucketSize: 24,
      bucketTimes: [start],
      rangeStart: start,
      rangeHours: null,
    } as unknown as AlertTrendSeries;

    const ticks = buildAlertAxisTicks(trends, 'en-US');

    // totalHours = (null ?? 24) = 24 → still within the short-range tier.
    expect(ticks[0].position).toBe(0);
    expect(ticks[ticks.length - 1].position).toBe(1);
    expect(ticks[0].label.length).toBeGreaterThan(0);
  });
});

describe('buildAlertAxisTicks — (totalDurationMs || 1) defensive divisor', () => {
  it('engages the || 1 fallback when a malformed NaN bucketSize makes totalDurationMs falsy', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    // bucketSize = NaN bypasses the `bucketSize <= 0` guard (NaN <= 0 is false),
    // so totalDurationMs = Math.max(buckets.length * NaN * MS, NaN * MS) = NaN,
    // which is falsy and forces the `|| 1` divisor on line 571. rangeHours is
    // also nullish so line 558's `?? bucketHours` fallback engages too.
    const trends = {
      buckets: [0],
      max: 1,
      bucketSize: Number.NaN,
      bucketTimes: [start],
      rangeStart: start,
      rangeHours: undefined,
    } as unknown as AlertTrendSeries;

    const ticks = buildAlertAxisTicks(trends, 'en-US');

    // Despite the malformed input, positions stay clamped within [0, 1] and
    // the start/end anchors are still emitted.
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[ticks.length - 1].position).toBe(1);

    // The end-tick timestamp is `start + NaN` = NaN; formatAlertAxisTickLabel
    // returns the em-dash placeholder for any non-finite timestamp, proving
    // the NaN propagated through the `|| 1` divisor rather than throwing.
    expect(ticks[ticks.length - 1].label).toBe('—');
  });
});
