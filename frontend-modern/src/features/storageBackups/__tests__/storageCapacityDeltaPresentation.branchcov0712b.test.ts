import { describe, expect, it } from 'vitest';
import type { MetricPoint } from '@/api/charts';
import { computeStorageCapacityDeltaAnalysis } from '@/features/storageBackups/storageCapacityDeltaPresentation';

// Branch-coverage extension for `storageCapacityDeltaPresentation`.
//
// `averageMetricValues` and `averageMetricTimestamps` are module-private (not
// exported), so they can only be driven transitively through the public
// `computeStorageCapacityDeltaAnalysis`. Every happy-path test below exercises
// their non-empty (reduce + divide) arm. Their `length === 0 -> 0` early-return
// arm is unreachable from the public API (the start/end windows are always
// non-empty once `normalized.length >= 2`) and is called out in GLM_REPORT.md.
//
// `computeStorageCapacityDeltaAnalysis` owns the meaningful branches: the
// `< 2 -> null` guard, the `sampleWindowSize` ternary, the
// `!isFinite(delta) || !isFinite(durationMs) || durationMs <= 0 -> null` guard,
// and the `Math.abs(delta) < 1 ? 0 : delta` clamp.

const point = (timestamp: number, value: number): MetricPoint => ({ timestamp, value });

// ---------------------------------------------------------------------------
// computeStorageCapacityDeltaAnalysis: normalized.length < 2 -> null
// ---------------------------------------------------------------------------

describe('computeStorageCapacityDeltaAnalysis branch coverage — < 2 points guard', () => {
  it('returns null for an empty series', () => {
    expect(computeStorageCapacityDeltaAnalysis([])).toBeNull();
  });

  it('returns null for a single point', () => {
    expect(computeStorageCapacityDeltaAnalysis([point(1_000, 100)])).toBeNull();
  });

  it('returns null when every point is dropped by the finite filter (NaN timestamp/value)', () => {
    const malformed = [
      { timestamp: NaN, value: 100 },
      { timestamp: 1_000, value: NaN },
      { timestamp: Infinity, value: 50 },
    ] as unknown as MetricPoint[];
    // All three are non-finite in at least one axis -> normalized is empty.
    expect(computeStorageCapacityDeltaAnalysis(malformed)).toBeNull();
  });

  it('returns null when only one point survives the finite filter', () => {
    const mixed = [
      { timestamp: NaN, value: 100 },
      { timestamp: 1_000, value: 100 },
    ] as unknown as MetricPoint[];
    expect(computeStorageCapacityDeltaAnalysis(mixed)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// sampleWindowSize ternary: length < 4 ? 1 : max(2, floor(length * 0.25))
// ---------------------------------------------------------------------------

describe('computeStorageCapacityDeltaAnalysis branch coverage — sampleWindowSize ternary', () => {
  it('uses a window of 1 for exactly two points (no averaging across the window)', () => {
    expect(computeStorageCapacityDeltaAnalysis([point(1_000, 100), point(2_000, 150)])).toEqual({
      deltaBytes: 50,
      durationMs: 1_000,
      startTimestamp: 1_000,
      endTimestamp: 2_000,
    });
  });

  it('uses a window of 1 for three points (start vs end point only)', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([
        point(1_000, 100),
        point(2_000, 200),
        point(3_000, 300),
      ]),
    ).toEqual({
      deltaBytes: 200,
      durationMs: 2_000,
      startTimestamp: 1_000,
      endTimestamp: 3_000,
    });
  });

  it('uses a window of max(2, floor(4*0.25)=1) -> 2 for four points (averages both ends)', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([
        point(1_000, 100),
        point(2_000, 120),
        point(3_000, 180),
        point(4_000, 200),
      ]),
    ).toEqual({
      deltaBytes: 80, // (180+200)/2 - (100+120)/2 = 190 - 110
      durationMs: 2_000, // (3000+4000)/2 - (1000+2000)/2 = 3500 - 1500
      startTimestamp: 1_500,
      endTimestamp: 3_500,
    });
  });

  it('uses a window of max(2, floor(12*0.25)=3) -> 3 for twelve points', () => {
    const points: MetricPoint[] = [
      point(0, 100),
      point(1_000, 100),
      point(2_000, 100),
      // middle six points are outside both windows; values are irrelevant
      point(3_000, 0),
      point(4_000, 0),
      point(5_000, 0),
      point(6_000, 0),
      point(7_000, 0),
      point(8_000, 0),
      point(9_000, 400),
      point(10_000, 400),
      point(11_000, 400),
    ];
    expect(computeStorageCapacityDeltaAnalysis(points)).toEqual({
      deltaBytes: 300, // avg(last 3)=400 - avg(first 3)=100
      durationMs: 9_000, // avg(last 3 ts)=10000 - avg(first 3 ts)=1000
      startTimestamp: 1_000, // (0+1000+2000)/3
      endTimestamp: 10_000, // (9000+10000+11000)/3
    });
  });

  it('sorts an out-of-order series by timestamp before selecting the windows', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([
        point(4_000, 200),
        point(1_000, 100),
        point(3_000, 180),
        point(2_000, 120),
      ]),
    ).toEqual({
      deltaBytes: 80,
      durationMs: 2_000,
      startTimestamp: 1_500,
      endTimestamp: 3_500,
    });
  });
});

// ---------------------------------------------------------------------------
// delta sign + Math.abs(delta) < 1 ? 0 : delta clamp
// ---------------------------------------------------------------------------

describe('computeStorageCapacityDeltaAnalysis branch coverage — delta sign and clamp', () => {
  it('preserves a negative delta (used capacity shrank)', () => {
    expect(computeStorageCapacityDeltaAnalysis([point(1_000, 200), point(2_000, 100)])).toEqual({
      deltaBytes: -100,
      durationMs: 1_000,
      startTimestamp: 1_000,
      endTimestamp: 2_000,
    });
  });

  it('clamps a sub-byte positive delta (abs < 1) to exactly 0', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([point(1_000, 100.0), point(2_000, 100.4)]),
    ).toEqual({
      deltaBytes: 0, // raw delta 0.4 -> clamped
      durationMs: 1_000,
      startTimestamp: 1_000,
      endTimestamp: 2_000,
    });
  });

  it('clamps a sub-byte negative delta (abs < 1) to exactly 0', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([point(1_000, 100.0), point(2_000, 99.6)]),
    ).toEqual({
      deltaBytes: 0, // raw delta -0.4 -> abs < 1 -> clamped
      durationMs: 1_000,
      startTimestamp: 1_000,
      endTimestamp: 2_000,
    });
  });
});

// ---------------------------------------------------------------------------
// !isFinite(delta) || !isFinite(durationMs) || durationMs <= 0 -> null
// ---------------------------------------------------------------------------

describe('computeStorageCapacityDeltaAnalysis branch coverage — finite/duration guard', () => {
  it('returns null when start and end timestamps are equal (durationMs === 0, <= 0 arm)', () => {
    expect(
      computeStorageCapacityDeltaAnalysis([point(1_000, 100), point(1_000, 200)]),
    ).toBeNull();
  });

  it('returns null when end timestamp precedes start after sorting only equal timestamps collapse (durationMs 0)', () => {
    // Three identical timestamps -> startTimestamp === endTimestamp -> durationMs 0.
    expect(
      computeStorageCapacityDeltaAnalysis([
        point(5_000, 100),
        point(5_000, 150),
        point(5_000, 200),
      ]),
    ).toBeNull();
  });

  it('returns null when the value averages overflow to Infinity (delta becomes NaN) via MAX_VALUE sums', () => {
    // Number.MAX_VALUE is finite and passes the normalizer's filter, but
    // MAX_VALUE + MAX_VALUE = Infinity -> Infinity / 2 = Infinity for both
    // windows -> delta = Infinity - Infinity = NaN -> !isFinite(delta) arm.
    const points: MetricPoint[] = [
      point(1_000, Number.MAX_VALUE),
      point(2_000, Number.MAX_VALUE),
      point(3_000, Number.MAX_VALUE),
      point(4_000, Number.MAX_VALUE),
    ];
    expect(computeStorageCapacityDeltaAnalysis(points)).toBeNull();
  });

  it('returns null when the timestamp averages overflow to Infinity (durationMs becomes NaN)', () => {
    // Finite values keep delta finite, but MAX_VALUE timestamps overflow the
    // timestamp averages -> durationMs = Infinity - Infinity = NaN ->
    // !isFinite(durationMs) arm (distinct from the delta arm above).
    const points: MetricPoint[] = [
      point(Number.MAX_VALUE, 100),
      point(Number.MAX_VALUE, 120),
      point(Number.MAX_VALUE, 180),
      point(Number.MAX_VALUE, 200),
    ];
    expect(computeStorageCapacityDeltaAnalysis(points)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// averageMetricValues / averageMetricTimestamps (private) — transitive coverage
// ---------------------------------------------------------------------------

describe('averageMetricValues / averageMetricTimestamps (transitive, private helpers)', () => {
  it('averages the window values and timestamps (non-empty reduce arm) for a 4-point series', () => {
    // Exercises both helpers' `points.length > 0 -> reduce/divide` arm: the
    // emitted startTimestamp/endTimestamp are the arithmetic means of the
    // first/last windows, proving the timestamp averaging ran, and deltaBytes
    // is the difference of the value averages, proving the value averaging ran.
    expect(
      computeStorageCapacityDeltaAnalysis([
        point(1_000, 100),
        point(2_000, 120),
        point(3_000, 180),
        point(4_000, 200),
      ]),
    ).toEqual({
      deltaBytes: 80,
      durationMs: 2_000,
      startTimestamp: 1_500,
      endTimestamp: 3_500,
    });
  });
});
