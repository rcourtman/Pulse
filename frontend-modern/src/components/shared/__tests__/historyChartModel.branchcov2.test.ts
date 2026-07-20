import { describe, expect, it } from 'vitest';
import type { AggregatedMetricPoint, HistoryTimeRange } from '@/api/charts';
import {
  createHistoryChartGeometry,
  findHistoryChartClosestPoint,
  formatHistoryChartTimeLabel,
  formatHistoryChartTooltipValue,
  getHistoryChartDataMax,
  getHistoryChartDataMin,
  getHistoryChartDefaultColor,
  getHistoryChartRefreshIntervalMs,
  getHistoryChartScale,
  getHistoryChartTooltipLayout,
  getHistoryChartYAxisLabels,
} from '@/components/shared/historyChartModel';

const pt = (timestamp: number, value: number, min: number, max: number): AggregatedMetricPoint => ({
  timestamp,
  value,
  min,
  max,
});

describe('formatHistoryChartTooltipValue', () => {
  it('formats percentage units with one decimal', () => {
    expect(formatHistoryChartTooltipValue(42.35, '%')).toBe('42.4%');
  });

  it('formats byte-rate units via formatBytes with a /s suffix', () => {
    expect(formatHistoryChartTooltipValue(1024, 'B/s')).toBe('1.00 KB/s');
  });

  it('formats celsius units by rounding to the nearest degree', () => {
    expect(formatHistoryChartTooltipValue(23.6, 'C')).toBe('24°C');
  });

  it('falls back to raw formatBytes output when unit is undefined', () => {
    expect(formatHistoryChartTooltipValue(0)).toBe('0 B');
    expect(formatHistoryChartTooltipValue(2048)).toBe('2.00 KB');
  });

  it('treats an empty-string unit the same as no unit', () => {
    expect(formatHistoryChartTooltipValue(2048, '')).toBe('2.00 KB');
  });

  it('renders integer values for arbitrary units without decimals', () => {
    expect(formatHistoryChartTooltipValue(42, 'rpm')).toBe('42 rpm');
  });

  it('renders fractional values for arbitrary units with one decimal', () => {
    expect(formatHistoryChartTooltipValue(42.5, 'rpm')).toBe('42.5 rpm');
  });
});

describe('getHistoryChartRefreshIntervalMs', () => {
  it.each<[HistoryTimeRange, number]>([
    ['7d', 30000],
    ['14d', 30000],
    ['30d', 60000],
    ['90d', 120000],
    ['1h', 10000],
    ['30m', 10000],
  ])('returns the expected refresh interval for range %s', (range, expected) => {
    expect(getHistoryChartRefreshIntervalMs(range)).toBe(expected);
  });
});

describe('getHistoryChartDefaultColor', () => {
  it('returns the explicit color override when provided', () => {
    expect(getHistoryChartDefaultColor('cpu', '#custom')).toBe('#custom');
  });

  it.each<[string, string]>([
    ['cpu', '#8b5cf6'],
    ['memory', '#f59e0b'],
    ['disk', '#10b981'],
    ['network', '#3b82f6'],
  ])('uses the metric-specific default color for %s', (metric, color) => {
    expect(getHistoryChartDefaultColor(metric)).toBe(color);
  });
});

describe('getHistoryChartDataMin', () => {
  it('returns null for an empty point set', () => {
    expect(getHistoryChartDataMin([])).toBeNull();
  });

  it('uses point.min when it is present (including zero)', () => {
    const points = [pt(1, 100, 10, 200), pt(2, 50, 5, 60)];
    expect(getHistoryChartDataMin(points)).toBe(5);
  });

  it('keeps a zero min because the null-check guards against falsy zero', () => {
    const points = [pt(1, 100, 0, 200), pt(2, 50, 5, 60)];
    expect(getHistoryChartDataMin(points)).toBe(0);
  });

  it('falls back to point.value when min is null', () => {
    const nullMin = {
      timestamp: 1,
      value: 7,
      min: null,
      max: null,
    } as unknown as AggregatedMetricPoint;
    const value = {
      timestamp: 2,
      value: 3,
      min: null,
      max: null,
    } as unknown as AggregatedMetricPoint;
    expect(getHistoryChartDataMin([nullMin, value])).toBe(3);
  });
});

describe('getHistoryChartDataMax', () => {
  it('returns null for an empty point set', () => {
    expect(getHistoryChartDataMax([])).toBeNull();
  });

  it('uses point.max when it is present (including zero)', () => {
    const points = [pt(1, 100, 10, 200), pt(2, 50, 5, 60)];
    expect(getHistoryChartDataMax(points)).toBe(200);
  });

  it('keeps a zero max because the null-check guards against falsy zero', () => {
    const points = [pt(1, 100, 0, 0), pt(2, 50, 5, 60)];
    expect(getHistoryChartDataMax(points)).toBe(60);
  });

  it('falls back to point.value when max is null', () => {
    const low = {
      timestamp: 1,
      value: 7,
      min: null,
      max: null,
    } as unknown as AggregatedMetricPoint;
    const high = {
      timestamp: 2,
      value: 99,
      min: null,
      max: null,
    } as unknown as AggregatedMetricPoint;
    expect(getHistoryChartDataMax([low, high])).toBe(99);
  });
});

describe('getHistoryChartScale', () => {
  it('returns the 0..100 baseline when there are no points and no unit (byte-like)', () => {
    expect(getHistoryChartScale([])).toStrictEqual({
      isPercentLike: false,
      isByteLike: true,
      minValue: 0,
      maxValue: 100,
    });
  });

  it('clamps percent-like scales to at least 100 even with low rawMax', () => {
    expect(getHistoryChartScale([pt(1, 10, 0, 30)], '%')).toStrictEqual({
      isPercentLike: true,
      isByteLike: false,
      minValue: 0,
      maxValue: 100,
    });
  });

  it('lets percent-like scales exceed 100 when rawMax is larger', () => {
    expect(getHistoryChartScale([pt(1, 10, 0, 150)], '%').maxValue).toBe(150);
  });

  it('applies the 1.15 headroom factor for byte-rate units', () => {
    expect(getHistoryChartScale([pt(1, 0, 0, 100)], 'B/s').maxValue).toBeCloseTo(115, 10);
  });

  it('treats an undefined unit as byte-like and applies the headroom factor', () => {
    const scale = getHistoryChartScale([pt(1, 0, 0, 10)]);
    expect(scale.isByteLike).toBe(true);
    expect(scale.maxValue).toBeCloseTo(11.5, 6);
  });

  it('falls back to point.value for rawMax when point.max is falsy zero', () => {
    expect(getHistoryChartScale([pt(1, 50, 0, 0)], 'B/s').maxValue).toBeCloseTo(57.5, 10);
  });

  it('clamps non-byte, non-percent scales to a minimum of 1', () => {
    expect(getHistoryChartScale([pt(1, 0, 0, 0)], 'rpm').maxValue).toBe(1);
  });

  it('marks arbitrary units as neither byte-like nor percent-like', () => {
    expect(getHistoryChartScale([pt(1, 0, 0, 100)], 'rpm').isPercentLike).toBe(false);
    expect(getHistoryChartScale([pt(1, 0, 0, 100)], 'rpm').isByteLike).toBe(false);
  });
});

describe('getHistoryChartYAxisLabels', () => {
  it('renders percent labels at the three tick positions', () => {
    expect(
      getHistoryChartYAxisLabels({
        minValue: 0,
        maxValue: 100,
        isPercentLike: true,
        isByteLike: false,
      }),
    ).toStrictEqual([
      { pct: 0, label: '0%' },
      { pct: 0.5, label: '50%' },
      { pct: 1, label: '100%' },
    ]);
  });

  it('renders byte-like 0/avg/max labels', () => {
    expect(
      getHistoryChartYAxisLabels({
        minValue: 0,
        maxValue: 100,
        isPercentLike: false,
        isByteLike: true,
      }),
    ).toStrictEqual([
      { pct: 0, label: '0' },
      { pct: 0.5, label: 'Avg' },
      { pct: 1, label: 'Max' },
    ]);
  });

  it('renders computed numeric labels for other unit kinds', () => {
    expect(
      getHistoryChartYAxisLabels({
        minValue: 10,
        maxValue: 110,
        isPercentLike: false,
        isByteLike: false,
      }),
    ).toStrictEqual([
      { pct: 0, label: '0' },
      { pct: 0.5, label: '60' },
      { pct: 1, label: '110' },
    ]);
  });
});

describe('formatHistoryChartTimeLabel', () => {
  const ts = new Date(2024, 0, 15, 9, 30).getTime();

  it.each<HistoryTimeRange>(['7d', '14d', '30d', '90d'])(
    'renders a calendar date for the %s range',
    (range) => {
      expect(formatHistoryChartTimeLabel(ts, range)).toBe(
        new Date(ts).toLocaleDateString([], { month: 'short', day: 'numeric' }),
      );
    },
  );

  it.each<HistoryTimeRange>(['30m', '1h', '6h', '12h', '24h'])(
    'renders a clock time for the %s range',
    (range) => {
      expect(formatHistoryChartTimeLabel(ts, range)).toBe(
        new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      );
    },
  );

  it('produces different output for the day vs time branches at the same timestamp', () => {
    expect(formatHistoryChartTimeLabel(ts, '7d')).not.toBe(formatHistoryChartTimeLabel(ts, '1h'));
  });
});

describe('createHistoryChartGeometry', () => {
  it('maps timestamps and values onto pixel coordinates', () => {
    const geo = createHistoryChartGeometry({
      width: 200,
      height: 100,
      startTime: 0,
      endTime: 100,
      minValue: 0,
      maxValue: 10,
    });

    expect(geo.timeSpan).toBe(100);
    expect(geo.getX(50)).toBe(120);
    expect(geo.getY(5)).toBe(50);
  });

  it('clamps a negative/inverted time span down to 1', () => {
    const geo = createHistoryChartGeometry({
      width: 200,
      height: 100,
      startTime: 100,
      endTime: 50,
      minValue: 0,
      maxValue: 10,
    });

    expect(geo.timeSpan).toBe(1);
    expect(geo.getX(100)).toBe(40);
  });

  it('left-pads the first timestamp to the chart origin', () => {
    const geo = createHistoryChartGeometry({
      width: 200,
      height: 100,
      startTime: 1000,
      endTime: 1100,
      minValue: 0,
      maxValue: 10,
    });

    expect(geo.getX(1000)).toBe(40);
    expect(geo.getX(1100)).toBe(200);
  });

  it('inverts the value axis so the max sits at the top padding', () => {
    const geo = createHistoryChartGeometry({
      width: 200,
      height: 100,
      startTime: 0,
      endTime: 100,
      minValue: 0,
      maxValue: 10,
    });

    expect(geo.getY(0)).toBe(80);
    expect(geo.getY(10)).toBe(20);
  });
});

describe('findHistoryChartClosestPoint', () => {
  it('returns the first point when its timestamp is the exact match', () => {
    const points = [pt(100, 1, 0, 0), pt(200, 2, 0, 0), pt(300, 3, 0, 0)];
    expect(findHistoryChartClosestPoint(points, 100)).toStrictEqual(points[0]);
  });

  it('updates the closest point as a nearer timestamp is found', () => {
    const points = [pt(100, 1, 0, 0), pt(200, 2, 0, 0), pt(300, 3, 0, 0)];
    expect(findHistoryChartClosestPoint(points, 210)).toStrictEqual(points[1]);
  });

  it('keeps the earlier point on an exact tie (strictly-less comparison)', () => {
    const points = [pt(100, 1, 0, 0), pt(200, 2, 0, 0)];
    expect(findHistoryChartClosestPoint(points, 150)).toStrictEqual(points[0]);
  });

  it('keeps the first point when no later point is closer', () => {
    const points = [pt(100, 1, 0, 0), pt(500, 2, 0, 0)];
    expect(findHistoryChartClosestPoint(points, 90)).toStrictEqual(points[0]);
  });
});

describe('getHistoryChartTooltipLayout', () => {
  it('places the tooltip to the right when only the right side has room', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 150, y: 70, timestamp: 0, value: 42 },
      chartWidth: 420,
      chartHeight: 180,
    });

    expect(layout).toStrictEqual({ x: 162, y: 47, width: 156, height: 46 });
  });

  it('places the tooltip to the left when only the left side has room', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 380, y: 70, timestamp: 0, value: 42 },
      chartWidth: 420,
      chartHeight: 180,
    });

    expect(layout.x).toBe(212);
    expect(layout.x + layout.width).toBeLessThan(380);
  });

  it('prefers the right side when both sides fit and right room is >= left room', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 250, y: 70, timestamp: 0, value: 42 },
      chartWidth: 500,
      chartHeight: 180,
    });

    expect(layout.x).toBe(262);
  });

  it('prefers the left side when both sides fit but left room is greater', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 250, y: 70, timestamp: 0, value: 42 },
      chartWidth: 490,
      chartHeight: 180,
    });

    expect(layout.x).toBe(82);
  });

  it('centers and clamps the tooltip when neither side can fit', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 90, y: 70, timestamp: 0, value: 42 },
      chartWidth: 180,
      chartHeight: 180,
    });

    expect(layout).toStrictEqual({ x: 12, y: 12, width: 156, height: 46 });
  });

  it('pushes an overlapping tooltip above the hovered point when there is headroom above', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 90, y: 30, timestamp: 0, value: 42 },
      chartWidth: 180,
      chartHeight: 180,
    });

    expect(layout.x).toBe(12);
    expect(layout.y).toBe(42);
  });

  it('clamps the tooltip y to the bottom edge when the hovered point is near the bottom', () => {
    const layout = getHistoryChartTooltipLayout({
      hoveredPoint: { x: 150, y: 200, timestamp: 0, value: 42 },
      chartWidth: 420,
      chartHeight: 180,
    });

    expect(layout.y).toBe(126);
  });
});
