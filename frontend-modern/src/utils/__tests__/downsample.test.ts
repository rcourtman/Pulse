import { describe, it, expect } from 'vitest';
import { downsampleLTTB, calculateOptimalPoints, TimeSeriesPoint } from '../downsample';

describe('downsampleLTTB', () => {
  it('returns original data when length is less than or equal to target', () => {
    const data: TimeSeriesPoint[] = [
      { timestamp: 1, value: 10 },
      { timestamp: 2, value: 20 },
      { timestamp: 3, value: 30 },
    ];

    const result = downsampleLTTB(data, 5);
    expect(result).toEqual(data);
  });

  it('returns original data when target is less than 3', () => {
    const data: TimeSeriesPoint[] = [
      { timestamp: 1, value: 10 },
      { timestamp: 2, value: 20 },
      { timestamp: 3, value: 30 },
      { timestamp: 4, value: 40 },
    ];

    const result = downsampleLTTB(data, 2);
    expect(result).toEqual(data);
  });

  it('always keeps first and last points', () => {
    const data: TimeSeriesPoint[] = [];
    for (let i = 0; i < 100; i++) {
      data.push({ timestamp: i, value: Math.random() * 100 });
    }

    const result = downsampleLTTB(data, 10);

    expect(result[0]).toEqual(data[0]);
    expect(result[result.length - 1]).toEqual(data[data.length - 1]);
  });

  it('returns the correct number of points', () => {
    const data: TimeSeriesPoint[] = [];
    for (let i = 0; i < 1000; i++) {
      data.push({ timestamp: i * 1000, value: Math.sin(i / 10) * 50 + 50 });
    }

    const result = downsampleLTTB(data, 100);

    expect(result.length).toBe(100);
  });

  it('distributes output points evenly across time for non-uniform density data', () => {
    // Simulate tiered data: sparse first half (10 points over 6 days),
    // dense second half (200 points over 1 day).
    const data: TimeSeriesPoint[] = [];
    const dayMs = 24 * 60 * 60_000;
    const t0 = 0;

    // Sparse: 10 points over 6 days
    for (let i = 0; i < 10; i++) {
      data.push({ timestamp: t0 + i * ((6 * dayMs) / 10), value: 50 + i });
    }
    // Dense: 200 points over 1 day
    const denseStart = 6 * dayMs;
    for (let i = 0; i < 200; i++) {
      data.push({ timestamp: denseStart + i * (dayMs / 200), value: 50 + (i % 10) });
    }

    const result = downsampleLTTB(data, 20);

    // Split output into first-half (0-6d) and second-half (6-7d) by timestamp.
    const midpoint = 6 * dayMs;
    const firstHalf = result.filter((p) => p.timestamp < midpoint);
    const secondHalf = result.filter((p) => p.timestamp >= midpoint);

    // With temporal bucketing, ~6/7 of buckets cover the first 6 days and
    // ~1/7 cover the last day. The first half should get the majority.
    // With old index-based bucketing, the 200 dense points would dominate
    // and the first half would get only ~1-2 points.
    expect(firstHalf.length).toBeGreaterThanOrEqual(8);
    expect(secondHalf.length).toBeGreaterThanOrEqual(2);
  });

  it('preserves peaks and valleys', () => {
    // Create data with a clear peak at index 50
    const data: TimeSeriesPoint[] = [];
    for (let i = 0; i < 100; i++) {
      const value = i === 50 ? 100 : 10; // spike at index 50
      data.push({ timestamp: i * 1000, value });
    }

    const result = downsampleLTTB(data, 10);

    // The peak should be preserved
    const hasPeak = result.some((p) => p.value === 100);
    expect(hasPeak).toBe(true);
  });
});

describe('calculateOptimalPoints', () => {
  describe('sparkline mode', () => {
    it('returns ~1 point per 1.5 pixels', () => {
      const result = calculateOptimalPoints(120, 'sparkline');
      // 120px / 1.5 = 80 points
      expect(result).toBe(80);
    });

    it('clamps to minimum of 20 points', () => {
      const result = calculateOptimalPoints(10, 'sparkline');
      expect(result).toBe(20);
    });

    it('clamps to maximum of 100 points', () => {
      const result = calculateOptimalPoints(500, 'sparkline');
      expect(result).toBe(100);
    });
  });

  describe('history mode', () => {
    it('returns ~1 point per 2 pixels', () => {
      const result = calculateOptimalPoints(400, 'history');
      // 400px / 2 = 200 points
      expect(result).toBe(200);
    });

    it('clamps to minimum of 60 points', () => {
      const result = calculateOptimalPoints(50, 'history');
      expect(result).toBe(60);
    });

    it('clamps to maximum of 600 points', () => {
      const result = calculateOptimalPoints(2000, 'history');
      expect(result).toBe(600);
    });
  });
});
