import { describe, expect, it } from 'vitest';

import {
  applyAlertHistoryWindow,
  buildAlertAxisTicks,
  buildAlertHistoryParams,
  buildAlertTrends,
  filterAlertHistoryItems,
  getIncidentRowKey,
  groupAlertHistoryItems,
  MS_PER_HOUR,
  type AlertHistoryRange,
  type AlertTrendSeries,
  type HistoryItem,
} from '../alertHistoryModel';

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
// getIncidentRowKey (zero-hit)
// ---------------------------------------------------------------------------

describe('getIncidentRowKey', () => {
  it('joins id and startTime with ::', () => {
    expect(
      getIncidentRowKey(makeItem({ id: 'a1', startTime: '2026-03-22T09:00:00.000Z' })),
    ).toBe('a1::2026-03-22T09:00:00.000Z');
  });

  it('produces distinct keys for same id with different startTime', () => {
    const a = makeItem({ id: 'x', startTime: '2026-03-22T09:00:00.000Z' });
    const b = makeItem({ id: 'x', startTime: '2026-03-22T10:00:00.000Z' });
    expect(getIncidentRowKey(a)).not.toBe(getIncidentRowKey(b));
  });
});

// ---------------------------------------------------------------------------
// buildAlertHistoryParams – default switch branch + default now param
// ---------------------------------------------------------------------------

describe('buildAlertHistoryParams', () => {
  it('falls back to limit 1000 with no startTime for an unrecognised range', () => {
    const result = buildAlertHistoryParams(
      'bogus' as AlertHistoryRange,
      Date.UTC(2026, 2, 22),
    );
    expect(result).toEqual({ limit: 1000 });
    expect(result.startTime).toBeUndefined();
  });

  it('uses Date.now() when now is omitted', () => {
    const before = Date.now();
    const result = buildAlertHistoryParams('24h');
    const after = Date.now();
    const startMs = new Date(result.startTime!).getTime();
    expect(startMs).toBeGreaterThanOrEqual(before - 24 * MS_PER_HOUR);
    expect(startMs).toBeLessThanOrEqual(after - 24 * MS_PER_HOUR);
  });
});

// ---------------------------------------------------------------------------
// filterAlertHistoryItems
// ---------------------------------------------------------------------------

describe('filterAlertHistoryItems', () => {
  const items: HistoryItem[] = [
    makeItem({
      id: '1',
      severity: 'critical',
      resourceName: 'vm-101',
      title: 'CPU High',
      description: 'cpu 95%',
      node: 'px1',
    }),
    makeItem({
      id: '2',
      severity: 'warning',
      resourceName: 'db-1',
      title: 'Disk Full',
      description: 'disk 90%',
      node: 'px2',
    }),
    makeItem({
      id: '3',
      severity: 'critical',
      resourceName: 'web-1',
      title: 'Memory',
      description: 'mem high',
    }),
  ];

  it('returns all items for severity "all" with empty search', () => {
    expect(filterAlertHistoryItems(items, 'all', '')).toHaveLength(3);
  });

  it('treats whitespace-only search as no search', () => {
    expect(filterAlertHistoryItems(items, 'all', '   \t  ')).toHaveLength(3);
  });

  it('filters to warning severity only', () => {
    const result = filterAlertHistoryItems(items, 'warning', '');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('2');
  });

  it('filters to critical severity only', () => {
    const result = filterAlertHistoryItems(items, 'critical', '');
    expect(result.map((i) => i.id)).toEqual(['1', '3']);
  });

  it('matches case-insensitively on resourceName', () => {
    const result = filterAlertHistoryItems(items, 'all', 'VM-101');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });

  it('matches on title', () => {
    const result = filterAlertHistoryItems(items, 'all', 'disk');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('2');
  });

  it('matches on description', () => {
    const result = filterAlertHistoryItems(items, 'all', '95%');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });

  it('matches on node', () => {
    const result = filterAlertHistoryItems(items, 'all', 'px2');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('2');
  });

  it('combines severity and search filters', () => {
    const result = filterAlertHistoryItems(items, 'critical', 'vm');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });

  it('returns empty array when nothing matches the search', () => {
    expect(filterAlertHistoryItems(items, 'all', 'nonexistent')).toHaveLength(0);
  });

  it('handles items with undefined optional fields without throwing', () => {
    const item = makeItem({ id: 'u', description: undefined, node: undefined });
    expect(filterAlertHistoryItems([item], 'all', '')).toHaveLength(1);
    expect(filterAlertHistoryItems([item], 'all', 'zzz')).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// buildAlertTrends – 7d, 30d, all-empty, all-with-data, window edges
// ---------------------------------------------------------------------------

describe('buildAlertTrends', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('uses 6-hour buckets for 7d range', () => {
    const trends = buildAlertTrends([], '7d', now);
    expect(trends.bucketSize).toBe(6);
    expect(trends.buckets).toHaveLength(28);
    expect(trends.rangeHours).toBe(168);
  });

  it('uses 24-hour buckets for 30d range', () => {
    const trends = buildAlertTrends([], '30d', now);
    expect(trends.bucketSize).toBe(24);
    expect(trends.buckets).toHaveLength(30);
    expect(trends.rangeHours).toBe(720);
  });

  it('defaults to 24-hour single bucket for "all" range with no alerts', () => {
    const trends = buildAlertTrends([], 'all', now);
    expect(trends.bucketSize).toBe(24);
    expect(trends.buckets).toHaveLength(1);
    expect(trends.buckets[0]).toBe(0);
    expect(trends.max).toBe(1);
    expect(trends.rangeHours).toBe(24);
  });

  it('computes bucket size from earliest alert for "all" range with data', () => {
    const alerts = [
      makeItem({ startTime: new Date(now - 50 * MS_PER_HOUR).toISOString() }),
      makeItem({ startTime: new Date(now - 10 * MS_PER_HOUR).toISOString() }),
    ];
    const trends = buildAlertTrends(alerts, 'all', now);
    expect(trends.bucketSize).toBe(2);
    expect(trends.buckets).toHaveLength(25);
    expect(trends.rangeHours).toBe(50);
  });

  it('snaps bucket size up to the next nice value', () => {
    const alerts = [makeItem({ startTime: new Date(now - 200 * MS_PER_HOUR).toISOString() })];
    const trends = buildAlertTrends(alerts, 'all', now);
    // rawBucketSize = ceil(200/30) = 7, next nice >= 7 is 12
    expect(trends.bucketSize).toBe(12);
  });

  it('clamps alert at window end into the last bucket', () => {
    const alerts = [makeItem({ startTime: new Date(now).toISOString() })];
    const trends = buildAlertTrends(alerts, '24h', now);
    expect(trends.buckets[trends.buckets.length - 1]).toBe(1);
  });

  it('ignores alerts before the trend window', () => {
    const alerts = [makeItem({ startTime: new Date(now - 25 * MS_PER_HOUR).toISOString() })];
    const trends = buildAlertTrends(alerts, '24h', now);
    expect(trends.buckets.reduce((s, v) => s + v, 0)).toBe(0);
  });

  it('ignores alerts after now', () => {
    const alerts = [makeItem({ startTime: new Date(now + 5 * MS_PER_HOUR).toISOString() })];
    const trends = buildAlertTrends(alerts, '24h', now);
    expect(trends.buckets.reduce((s, v) => s + v, 0)).toBe(0);
  });

  it('reports max of at least 1 for empty buckets', () => {
    const trends = buildAlertTrends([], '24h', now);
    expect(trends.max).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// applyAlertHistoryWindow – selectedBar, all-range, cutoffs, sort
// ---------------------------------------------------------------------------

describe('applyAlertHistoryWindow', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('filters to the selected bucket window', () => {
    const trends = buildAlertTrends([], '24h', now);
    const bucketIndex = 23;
    const bucketStart = trends.bucketTimes[bucketIndex];
    const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;

    const items: HistoryItem[] = [
      makeItem({ id: 'inside', startTime: new Date(bucketStart + 1000).toISOString() }),
      makeItem({ id: 'before', startTime: new Date(bucketStart - MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'at-end', startTime: new Date(bucketEnd).toISOString() }),
    ];

    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: '24h',
      selectedBarIndex: bucketIndex,
      trends,
      now,
    });

    expect(result.map((i) => i.id)).toEqual(['inside']);
  });

  it('does not apply time cutoff for "all" range with no selected bar', () => {
    const trends = buildAlertTrends([], 'all', now);
    const items: HistoryItem[] = [
      makeItem({ id: 'recent', startTime: new Date(now - MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'ancient', startTime: new Date(now - 365 * 24 * MS_PER_HOUR).toISOString() }),
    ];

    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: 'all',
      selectedBarIndex: null,
      trends,
      now,
    });

    expect(result).toHaveLength(2);
  });

  it('applies 7d cutoff when no bar is selected', () => {
    const trends = buildAlertTrends([], '7d', now);
    const items: HistoryItem[] = [
      makeItem({ id: 'within', startTime: new Date(now - 3 * 24 * MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'beyond', startTime: new Date(now - 10 * 24 * MS_PER_HOUR).toISOString() }),
    ];

    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: '7d',
      selectedBarIndex: null,
      trends,
      now,
    });

    expect(result.map((i) => i.id)).toEqual(['within']);
  });

  it('sorts items by startTime descending', () => {
    const trends = buildAlertTrends([], 'all', now);
    const items: HistoryItem[] = [
      makeItem({ id: 'oldest', startTime: new Date(now - 100 * MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'newest', startTime: new Date(now - 1 * MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'middle', startTime: new Date(now - 50 * MS_PER_HOUR).toISOString() }),
    ];

    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: 'all',
      selectedBarIndex: null,
      trends,
      now,
    });

    expect(result.map((i) => i.id)).toEqual(['newest', 'middle', 'oldest']);
  });
});

// ---------------------------------------------------------------------------
// groupAlertHistoryItems
// ---------------------------------------------------------------------------

describe('groupAlertHistoryItems', () => {
  it('returns empty array for empty input', () => {
    expect(groupAlertHistoryItems([])).toHaveLength(0);
  });

  it('groups same-day alerts into one group', () => {
    const items = [
      makeItem({ id: 'a', startTime: '2026-01-15T12:00:00.000Z' }),
      makeItem({ id: 'b', startTime: '2026-01-15T14:00:00.000Z' }),
    ];
    const groups = groupAlertHistoryItems(items);
    expect(groups).toHaveLength(1);
    expect(groups[0].alerts).toHaveLength(2);
  });

  it('sorts groups descending by date', () => {
    const items = [
      makeItem({ id: 'old', startTime: '2026-01-15T12:00:00.000Z' }),
      makeItem({ id: 'new', startTime: '2026-06-15T12:00:00.000Z' }),
    ];
    const groups = groupAlertHistoryItems(items);
    expect(groups).toHaveLength(2);
    expect(groups[0].date.getTime()).toBeGreaterThan(groups[1].date.getTime());
  });

  it('labels today and yesterday relative to the current date', () => {
    const realNow = new Date();
    const todayMidnight = new Date(
      realNow.getFullYear(),
      realNow.getMonth(),
      realNow.getDate(),
    );
    const yesterdayMidnight = new Date(todayMidnight);
    yesterdayMidnight.setDate(yesterdayMidnight.getDate() - 1);

    const items = [
      makeItem({
        id: 'today',
        startTime: new Date(todayMidnight.getTime() + 3600000).toISOString(),
      }),
      makeItem({
        id: 'yesterday',
        startTime: new Date(yesterdayMidnight.getTime() + 3600000).toISOString(),
      }),
    ];
    const groups = groupAlertHistoryItems(items);
    expect(groups).toHaveLength(2);
    expect(groups[0].label).toMatch(/^Today /);
    expect(groups[1].label).toMatch(/^Yesterday /);
  });

  it('uses absolute date label for non-relative dates', () => {
    const items = [makeItem({ startTime: '2026-01-15T12:00:00.000Z' })];
    const groups = groupAlertHistoryItems(items);
    expect(groups[0].label).not.toMatch(/^Today/);
    expect(groups[0].label).not.toMatch(/^Yesterday/);
    expect(groups[0].label).toMatch(/January/);
  });

  it('includes weekday and year in fullLabel', () => {
    const items = [makeItem({ startTime: '2026-01-15T12:00:00.000Z' })];
    const groups = groupAlertHistoryItems(items);
    expect(groups[0].fullLabel).toMatch(/2026/);
    expect(groups[0].fullLabel).toMatch(/January/);
  });
});

// ---------------------------------------------------------------------------
// buildAlertAxisTicks
// ---------------------------------------------------------------------------

describe('buildAlertAxisTicks', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('returns empty array when bucketTimes is empty', () => {
    const trends: AlertTrendSeries = {
      buckets: [],
      max: 0,
      bucketSize: 1,
      bucketTimes: [],
      rangeStart: now,
      rangeHours: 0,
    };
    expect(buildAlertAxisTicks(trends, 'en-US')).toEqual([]);
  });

  it('returns empty array when bucketSize is zero', () => {
    const trends: AlertTrendSeries = {
      buckets: [1],
      max: 1,
      bucketSize: 0,
      bucketTimes: [now],
      rangeStart: now,
      rangeHours: 1,
    };
    expect(buildAlertAxisTicks(trends, 'en-US')).toEqual([]);
  });

  it('places first tick at position 0 and last tick at position 1', () => {
    const trends = buildAlertTrends([], '24h', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[0].align).toBe('start');
    expect(ticks[ticks.length - 1].position).toBe(1);
    expect(ticks[ticks.length - 1].align).toBe('end');
  });

  it('assigns center align to all intermediate ticks', () => {
    const trends = buildAlertTrends([], '30d', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    for (let i = 1; i < ticks.length - 1; i++) {
      expect(ticks[i].align).toBe('center');
    }
  });

  it('produces non-empty string labels for every tick', () => {
    const trends = buildAlertTrends([], '24h', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    for (const tick of ticks) {
      expect(typeof tick.label).toBe('string');
      expect(tick.label.length).toBeGreaterThan(0);
    }
  });

  it('replaces the last tick when loop already reaches position 1', () => {
    // Craft trends so bucketTimes has 2 entries but buckets.length is 1.
    // totalDurationMs = 1h, loop hits indices 0 and 1,
    // index 1 maps to position 1h/1h = 1.0 → replace branch.
    const trends: AlertTrendSeries = {
      buckets: [0],
      max: 1,
      bucketSize: 1,
      bucketTimes: [now, now + MS_PER_HOUR],
      rangeStart: now,
      rangeHours: 1,
    };
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    expect(ticks).toHaveLength(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[1].position).toBe(1);
  });

  it('limits tick count to a reasonable maximum', () => {
    const trends = buildAlertTrends([], '30d', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    expect(ticks.length).toBeLessThanOrEqual(7);
    expect(ticks.length).toBeGreaterThanOrEqual(2);
  });
});
