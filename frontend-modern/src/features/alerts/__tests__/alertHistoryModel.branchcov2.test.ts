import { describe, expect, it } from 'vitest';

import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

import {
  MS_PER_HOUR,
  applyAlertHistoryWindow,
  buildAlertHistoryItems,
  buildAlertHistoryParams,
  buildAlertRangeSummary,
  buildAlertAxisTicks,
  buildAlertTrends,
  filterAlertHistoryItems,
  formatAlertAxisTickLabel,
  formatAlertBucketRange,
  formatAlertHistoryDuration,
  formatAlertHistoryGroupLabel,
  getAlertBucketDurationLabel,
  getAlertHistoryDaySuffix,
  getIncidentRowKey,
  groupAlertHistoryItems,
  resolveAlertHistoryResourceType,
  type AlertHistoryRange,
  type AlertTrendSeries,
  type HistoryItem,
} from '../alertHistoryModel';

// ---------------------------------------------------------------------------
// Shared fixtures — keep these minimal but real-shaped so the production
// code paths (resource-type resolution, severity filtering, etc.) exercise
// the same branches the UI triggers.
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

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'critical',
    resourceId: 'resource-1',
    resourceName: 'vm-101',
    node: 'px1',
    instance: 'cpu',
    message: 'CPU high',
    value: 90,
    threshold: 80,
    startTime: '2026-03-22T09:00:00.000Z',
    acknowledged: false,
    ...overrides,
  } as Alert;
}

function makeResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'resource-1',
    type: 'vm',
    name: 'vm-101',
    displayName: 'vm-101',
    ...overrides,
  } as unknown as Resource;
}

// ---------------------------------------------------------------------------
// buildAlertHistoryParams — exact limit/startTime for each switch arm.
// ---------------------------------------------------------------------------

describe('buildAlertHistoryParams', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('emits limit 2000 and a startTime exactly 24h before now for "24h"', () => {
    expect(buildAlertHistoryParams('24h', now)).toStrictEqual({
      limit: 2000,
      startTime: new Date(now - 24 * MS_PER_HOUR).toISOString(),
    });
  });

  it('emits limit 10000 and a startTime exactly 7d before now for "7d"', () => {
    expect(buildAlertHistoryParams('7d', now)).toStrictEqual({
      limit: 10000,
      startTime: new Date(now - 7 * 24 * MS_PER_HOUR).toISOString(),
    });
  });

  it('emits limit 10000 and a startTime exactly 30d before now for "30d"', () => {
    expect(buildAlertHistoryParams('30d', now)).toStrictEqual({
      limit: 10000,
      startTime: new Date(now - 30 * 24 * MS_PER_HOUR).toISOString(),
    });
  });

  it('emits limit 0 and no startTime for "all"', () => {
    expect(buildAlertHistoryParams('all', now)).toStrictEqual({ limit: 0 });
  });

  it('falls back to limit 1000 with no startTime for an unrecognised range', () => {
    expect(buildAlertHistoryParams('bogus' as AlertHistoryRange, now)).toStrictEqual({
      limit: 1000,
    });
  });
});

// ---------------------------------------------------------------------------
// formatAlertHistoryDuration — negative-duration guard, now-fallback, and
// the three magnitude arms at their boundaries.
// ---------------------------------------------------------------------------

describe('formatAlertHistoryDuration', () => {
  it('returns "0m" when end is before start (negative duration)', () => {
    expect(formatAlertHistoryDuration('2026-03-22T10:00:00.000Z', '2026-03-22T09:00:00.000Z')).toBe(
      '0m',
    );
  });

  it('uses the provided `now` when endTime is omitted', () => {
    const start = '2026-03-22T09:00:00.000Z';
    const now = Date.UTC(2026, 2, 22, 9, 30, 0);
    expect(formatAlertHistoryDuration(start, undefined, now)).toBe('30m');
  });

  it('returns "0m" for a zero-length duration (start === end)', () => {
    const start = '2026-03-22T09:00:00.000Z';
    expect(formatAlertHistoryDuration(start, start)).toBe('0m');
  });

  it('formats the minute-only arm with leading minutes under one hour', () => {
    expect(formatAlertHistoryDuration('2026-03-22T09:00:00.000Z', '2026-03-22T09:05:00.000Z')).toBe(
      '5m',
    );
  });

  it('renders the residual minutes even when hours divide evenly', () => {
    // exactly 1 hour → minutes=60, hours=1, days=0 → "1h 0m"
    expect(formatAlertHistoryDuration('2026-03-22T09:00:00.000Z', '2026-03-22T10:00:00.000Z')).toBe(
      '1h 0m',
    );
  });

  it('renders the residual hours even when days divide evenly', () => {
    // exactly 1 day → minutes=1440, hours=24, days=1 → "1d 0h"
    expect(formatAlertHistoryDuration('2026-03-22T09:00:00.000Z', '2026-03-23T09:00:00.000Z')).toBe(
      '1d 0h',
    );
  });
});

// ---------------------------------------------------------------------------
// formatAlertBucketRange — same-day, cross-day, and cross-year (which forces
// the `year: 'numeric'` option on the start formatter).
// ---------------------------------------------------------------------------

describe('formatAlertBucketRange', () => {
  it('uses the en-dash separator and a single date for an intra-day bucket', () => {
    const start = Date.UTC(2026, 2, 22, 12, 0, 0);
    const end = Date.UTC(2026, 2, 22, 13, 0, 0);
    const label = formatAlertBucketRange(start, end, 'en-US');
    expect(label).toContain('\u2013'); // –
    expect(label).not.toContain('\u2192'); // →
    expect(label).toContain('Mar 22');
    expect(label.startsWith('Mar 22,')).toBe(true);
  });

  it('uses the arrow separator and both dates when the bucket spans midnight', () => {
    const start = Date.UTC(2026, 2, 22, 22, 0, 0);
    const end = Date.UTC(2026, 2, 23, 4, 0, 0);
    const label = formatAlertBucketRange(start, end, 'en-US');
    expect(label).toContain('\u2192'); // →
    expect(label).not.toContain('\u2013'); // –
    expect(label).toContain('Mar 22');
    expect(label).toContain('Mar 23');
  });

  it('adds a year to the start segment when start and end fall in different years', () => {
    const start = Date.UTC(2025, 11, 31, 22, 0, 0);
    const end = Date.UTC(2026, 0, 1, 2, 0, 0);
    const label = formatAlertBucketRange(start, end, 'en-US');
    expect(label).toContain('2025');
    expect(label).toContain('Dec 31');
    expect(label).toContain('Jan 1');
    expect(label).toContain('2026');
    // Cross-day so the arrow separator is used.
    expect(label).toContain('\u2192');
  });
});

// ---------------------------------------------------------------------------
// resolveAlertHistoryResourceType — every early-return and fallback arm.
// ---------------------------------------------------------------------------

describe('resolveAlertHistoryResourceType', () => {
  it('returns the metadata resourceType when it is a non-empty string', () => {
    expect(
      resolveAlertHistoryResourceType({
        resourceName: 'vm-101',
        metadata: { resourceType: 'Custom' },
        resourceId: 'resource-1',
        getResource: () => makeResource({ type: 'vm' }),
        allResources: [makeResource({ type: 'vm' })],
      }),
    ).toBe('Custom');
  });

  it('falls through when metadata.resourceType is only whitespace', () => {
    const result = resolveAlertHistoryResourceType({
      resourceName: 'vm-101',
      metadata: { resourceType: '   ' },
      resourceId: 'resource-1',
      getResource: () => makeResource({ type: 'vm' }),
      allResources: [],
    });
    expect(result).toBe('VM');
  });

  it('falls through when metadata.resourceType is not a string (number)', () => {
    const result = resolveAlertHistoryResourceType({
      resourceName: 'vm-101',
      metadata: { resourceType: 42 },
      resourceId: 'resource-1',
      getResource: () => makeResource({ type: 'vm' }),
      allResources: [],
    });
    expect(result).toBe('VM');
  });

  it('falls through when metadata is undefined', () => {
    const result = resolveAlertHistoryResourceType({
      resourceName: 'vm-101',
      metadata: undefined,
      resourceId: 'resource-1',
      getResource: () => makeResource({ type: 'vm' }),
      allResources: [],
    });
    expect(result).toBe('VM');
  });

  it('resolves via getResource when resourceId is provided and the lookup hits', () => {
    expect(
      resolveAlertHistoryResourceType({
        resourceName: 'whatever',
        resourceId: 'resource-1',
        getResource: (id) => (id === 'resource-1' ? makeResource({ type: 'vm' }) : undefined),
        allResources: [],
      }),
    ).toBe('VM');
  });

  it('falls through to a name match when getResource returns undefined', () => {
    const result = resolveAlertHistoryResourceType({
      resourceName: 'vm-101',
      resourceId: 'missing',
      getResource: () => undefined,
      allResources: [makeResource({ name: 'vm-101', type: 'vm' })],
    });
    expect(result).toBe('VM');
  });

  it('matches a resource by displayName when the name match misses', () => {
    const result = resolveAlertHistoryResourceType({
      resourceName: 'pretty-vm',
      resourceId: 'missing',
      getResource: () => undefined,
      allResources: [
        makeResource({ name: 'other', displayName: 'pretty-vm', type: 'app-container' }),
      ],
    });
    expect(result).toBe('Container');
  });

  it('returns "Unknown" when no resolution path succeeds', () => {
    expect(
      resolveAlertHistoryResourceType({
        resourceName: 'lonely',
        resourceId: undefined,
        getResource: () => undefined,
        allResources: [],
      }),
    ).toBe('Unknown');
  });

  it('returns "Unknown" even with a resourceId when getResource misses and no name matches', () => {
    expect(
      resolveAlertHistoryResourceType({
        resourceName: 'lonely',
        resourceId: 'also-missing',
        getResource: () => undefined,
        allResources: [makeResource({ name: 'someone-else' })],
      }),
    ).toBe('Unknown');
  });
});

// ---------------------------------------------------------------------------
// buildAlertHistoryItems — active vs history, acknowledged vs resolved, and
// the active-id suppression of duplicate history rows.
// ---------------------------------------------------------------------------

describe('buildAlertHistoryItems', () => {
  const getResource = (id: string): Resource | undefined =>
    id === 'resource-1' ? makeResource({ id: 'resource-1', type: 'vm' }) : undefined;

  it('returns an empty array when both activeAlerts and alertHistory are empty', () => {
    expect(
      buildAlertHistoryItems({
        activeAlerts: {},
        alertHistory: [],
        getResource,
        allResources: [],
      }),
    ).toStrictEqual([]);
  });

  it('marks active alerts with status "active" and acknowledged:false', () => {
    const items = buildAlertHistoryItems({
      activeAlerts: { 'alert-1': makeAlert({ id: 'alert-1', acknowledged: true }) },
      alertHistory: [],
      getResource,
      allResources: [],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      id: 'alert-1',
      status: 'active',
      acknowledged: false,
      resourceType: 'VM',
      title: 'CPU',
    });
    // Active alerts carry no end time.
    expect(items[0].endTime).toBeUndefined();
  });

  it('marks an unacknowledged historical alert as "resolved"', () => {
    const items = buildAlertHistoryItems({
      activeAlerts: {},
      alertHistory: [
        makeAlert({
          id: 'h-1',
          acknowledged: false,
          lastSeen: '2026-03-22T09:30:00.000Z',
        }),
      ],
      getResource,
      allResources: [],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ id: 'h-1', status: 'resolved', acknowledged: false });
    expect(items[0].endTime).toBe('2026-03-22T09:30:00.000Z');
  });

  it('marks an acknowledged historical alert as "acknowledged"', () => {
    const items = buildAlertHistoryItems({
      activeAlerts: {},
      alertHistory: [
        makeAlert({
          id: 'h-2',
          acknowledged: true,
          lastSeen: '2026-03-22T09:30:00.000Z',
        }),
      ],
      getResource,
      allResources: [],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ id: 'h-2', status: 'acknowledged', acknowledged: true });
  });

  it('does not duplicate an alert that is present in both active and history', () => {
    const items = buildAlertHistoryItems({
      activeAlerts: { 'alert-1': makeAlert({ id: 'alert-1' }) },
      alertHistory: [makeAlert({ id: 'alert-1', lastSeen: '2026-03-22T09:30:00.000Z' })],
      getResource,
      allResources: [],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });
    expect(items).toHaveLength(1);
    expect(items[0].status).toBe('active');
  });

  it('interleaves active and historical rows preserving both sets', () => {
    const items = buildAlertHistoryItems({
      activeAlerts: { 'a-1': makeAlert({ id: 'a-1' }) },
      alertHistory: [makeAlert({ id: 'h-1', lastSeen: '2026-03-22T09:30:00.000Z' })],
      getResource,
      allResources: [],
      now: Date.UTC(2026, 2, 22, 10, 0, 0),
    });
    expect(items.map((i) => i.id)).toEqual(['a-1', 'h-1']);
    expect(items.map((i) => i.status)).toEqual(['active', 'resolved']);
  });
});

// ---------------------------------------------------------------------------
// filterAlertHistoryItems — pass-through identity, multi-field de-dupe, and
// the undefined-field safe-access arms.
// ---------------------------------------------------------------------------

describe('filterAlertHistoryItems', () => {
  it('returns the same array reference when no filter is applied', () => {
    const items = [makeItem()];
    expect(filterAlertHistoryItems(items, 'all', '')).toBe(items);
  });

  it('matches a search term against the title only when other fields are blank', () => {
    const items = [
      makeItem({ id: '1', title: 'CPU High', resourceName: 'x', description: '', node: '' }),
      makeItem({ id: '2', title: 'Disk Full', resourceName: 'y', description: '', node: '' }),
    ];
    const result = filterAlertHistoryItems(items, 'all', 'cpu');
    expect(result.map((i) => i.id)).toEqual(['1']);
  });

  it('returns a single item even when the term matches multiple of its fields', () => {
    const items = [
      makeItem({
        id: '1',
        resourceName: 'cpu-thing',
        title: 'CPU',
        description: 'cpu spike',
        node: 'cpunode',
      }),
    ];
    const result = filterAlertHistoryItems(items, 'all', 'cpu');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });

  it('safely filters items whose title/description/node are undefined', () => {
    const item = makeItem({ id: 'u', title: undefined, description: undefined, node: undefined });
    expect(filterAlertHistoryItems([item], 'all', '')).toHaveLength(1);
    expect(filterAlertHistoryItems([item], 'all', 'missing')).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// buildAlertTrends — exercises the `?? rawBucketSize` fallback by forcing a
// range so wide that no nice bucket size is large enough.
// ---------------------------------------------------------------------------

describe('buildAlertTrends (nice-size fallback)', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('falls back to rawBucketSize when the range exceeds the largest nice value', () => {
    // ~1801 days ago → rawRangeHours ≈ 43224 → rawBucketSize = ceil(43224/30) = 1441
    // which is larger than the max nice value (1440), so `.find()` returns undefined
    // and the `?? rawBucketSize` fallback engages.
    const alerts = [makeItem({ startTime: new Date(now - 1801 * 24 * MS_PER_HOUR).toISOString() })];
    const trends = buildAlertTrends(alerts, 'all', now);
    expect(trends.bucketSize).toBe(1441);
    expect(trends.rangeHours).toBe(30 * 1441);
    expect(trends.buckets).toHaveLength(30);
  });

  it('caps the bucket count at maxBuckets (30) for a very wide range', () => {
    const alerts = [makeItem({ startTime: new Date(now - 5000 * 24 * MS_PER_HOUR).toISOString() })];
    const trends = buildAlertTrends(alerts, 'all', now);
    expect(trends.buckets.length).toBeLessThanOrEqual(30);
    expect(trends.rangeHours).toBe(trends.buckets.length * trends.bucketSize);
  });
});

// ---------------------------------------------------------------------------
// applyAlertHistoryWindow — 24h/30d cutoff arms, selected-bar precedence
// over an "all" timeFilter, and the id-equality tiebreaker.
// ---------------------------------------------------------------------------

describe('applyAlertHistoryWindow (cutoff + tiebreak arms)', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('applies the 24h cutoff when no bar is selected', () => {
    const trends = buildAlertTrends([], '24h', now);
    const items = [
      makeItem({ id: 'within', startTime: new Date(now - MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'beyond', startTime: new Date(now - 25 * MS_PER_HOUR).toISOString() }),
    ];
    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: '24h',
      selectedBarIndex: null,
      trends,
      now,
    });
    expect(result.map((i) => i.id)).toEqual(['within']);
  });

  it('applies the 30d cutoff when no bar is selected', () => {
    const trends = buildAlertTrends([], '30d', now);
    const items = [
      makeItem({ id: 'within', startTime: new Date(now - 10 * 24 * MS_PER_HOUR).toISOString() }),
      makeItem({ id: 'beyond', startTime: new Date(now - 60 * 24 * MS_PER_HOUR).toISOString() }),
    ];
    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: '30d',
      selectedBarIndex: null,
      trends,
      now,
    });
    expect(result.map((i) => i.id)).toEqual(['within']);
  });

  it('prefers the selected bar over the timeFilter cutoff (timeFilter "all")', () => {
    const trends = buildAlertTrends([], 'all', now);
    const bucketIndex = 0;
    const bucketStart = trends.bucketTimes[bucketIndex];
    const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;

    const items = [
      makeItem({ id: 'in-bucket', startTime: new Date(bucketStart + 1000).toISOString() }),
      makeItem({ id: 'out-of-bucket', startTime: new Date(bucketEnd + MS_PER_HOUR).toISOString() }),
    ];

    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: 'all',
      selectedBarIndex: bucketIndex,
      trends,
      now,
    });
    expect(result.map((i) => i.id)).toEqual(['in-bucket']);
  });

  it('returns 0 from the comparator when two items share both startTime and id', () => {
    const trends = buildAlertTrends([], 'all', now);
    const sharedStart = new Date(now - MS_PER_HOUR).toISOString();
    // Identical id + startTime — the `a.id > b.id` branch is skipped and 0 is returned.
    const items = [
      makeItem({ id: 'same', startTime: sharedStart }),
      makeItem({ id: 'same', startTime: sharedStart }),
    ];
    const result = applyAlertHistoryWindow({
      filteredItems: items,
      timeFilter: 'all',
      selectedBarIndex: null,
      trends,
      now,
    });
    expect(result).toHaveLength(2);
    expect(result.every((i) => i.id === 'same')).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// getAlertHistoryDaySuffix — every ordinal arm including the 11–13 special case.
// ---------------------------------------------------------------------------

describe('getAlertHistoryDaySuffix', () => {
  it('returns "st" for 1, 21, 31, 101', () => {
    expect(getAlertHistoryDaySuffix(1)).toBe('st');
    expect(getAlertHistoryDaySuffix(21)).toBe('st');
    expect(getAlertHistoryDaySuffix(31)).toBe('st');
    expect(getAlertHistoryDaySuffix(101)).toBe('st');
  });

  it('returns "nd" for 2, 22', () => {
    expect(getAlertHistoryDaySuffix(2)).toBe('nd');
    expect(getAlertHistoryDaySuffix(22)).toBe('nd');
  });

  it('returns "rd" for 3, 23', () => {
    expect(getAlertHistoryDaySuffix(3)).toBe('rd');
    expect(getAlertHistoryDaySuffix(23)).toBe('rd');
  });

  it('returns "th" for the default arm (e.g. 4, 25)', () => {
    expect(getAlertHistoryDaySuffix(4)).toBe('th');
    expect(getAlertHistoryDaySuffix(25)).toBe('th');
  });

  it('returns "th" for the 11–13 special case even though they end in 1/2/3', () => {
    expect(getAlertHistoryDaySuffix(11)).toBe('th');
    expect(getAlertHistoryDaySuffix(12)).toBe('th');
    expect(getAlertHistoryDaySuffix(13)).toBe('th');
  });

  it('documents current behaviour for 111–113 (guard only covers 11–13)', () => {
    // The 11–13 guard is `day >= 11 && day <= 13`, so 111/112/113 fall through
    // to the `% 10` switch and are *not* special-cased. This pins the current
    // output; see GLM_REPORT.md for the suspected source bug.
    expect(getAlertHistoryDaySuffix(111)).toBe('st');
    expect(getAlertHistoryDaySuffix(112)).toBe('nd');
    expect(getAlertHistoryDaySuffix(113)).toBe('rd');
  });
});

// ---------------------------------------------------------------------------
// formatAlertHistoryGroupLabel — direct calls for all three label arms.
// ---------------------------------------------------------------------------

describe('formatAlertHistoryGroupLabel', () => {
  it('labels a date equal to todayStart with the "Today (...)" prefix', () => {
    const todayStart = Date.UTC(2026, 2, 22);
    const date = new Date(todayStart);
    expect(formatAlertHistoryGroupLabel(date, todayStart, 0)).toBe('Today (March 22nd)');
  });

  it('labels a date equal to yesterdayStart with the "Yesterday (...)" prefix', () => {
    const yesterdayStart = Date.UTC(2026, 2, 21);
    const date = new Date(yesterdayStart);
    expect(formatAlertHistoryGroupLabel(date, 0, yesterdayStart)).toBe('Yesterday (March 21st)');
  });

  it('uses the absolute "Month DaySuffix" label for any other date', () => {
    const date = new Date(Date.UTC(2026, 0, 2));
    // Neither todayStart nor yesterdayStart match.
    expect(formatAlertHistoryGroupLabel(date, 0, 0)).toBe('January 2nd');
  });

  it('applies the correct suffix for an 11th day', () => {
    const date = new Date(Date.UTC(2026, 0, 11));
    expect(formatAlertHistoryGroupLabel(date, 0, 0)).toBe('January 11th');
  });
});

// ---------------------------------------------------------------------------
// getIncidentRowKey
// ---------------------------------------------------------------------------

describe('getIncidentRowKey', () => {
  it('joins id and startTime with "::" into a stable composite key', () => {
    expect(
      getIncidentRowKey(makeItem({ id: 'inc-9', startTime: '2026-03-22T09:00:00.000Z' })),
    ).toBe('inc-9::2026-03-22T09:00:00.000Z');
  });
});

// ---------------------------------------------------------------------------
// groupAlertHistoryItems — multi-day grouping, add-to-existing arm, ordering.
// ---------------------------------------------------------------------------

describe('groupAlertHistoryItems', () => {
  it('appends subsequent same-day alerts to an already-created group', () => {
    const items = [
      makeItem({ id: 'a', startTime: '2026-01-15T08:00:00.000Z' }),
      makeItem({ id: 'b', startTime: '2026-01-15T20:00:00.000Z' }),
    ];
    const groups = groupAlertHistoryItems(items);
    expect(groups).toHaveLength(1);
    expect(groups[0].alerts.map((a) => a.id)).toEqual(['a', 'b']);
  });

  it('produces one group per distinct calendar day, newest first', () => {
    const items = [
      makeItem({ id: 'oldest', startTime: '2026-01-10T08:00:00.000Z' }),
      makeItem({ id: 'mid', startTime: '2026-02-10T08:00:00.000Z' }),
      makeItem({ id: 'newest', startTime: '2026-03-10T08:00:00.000Z' }),
    ];
    const groups = groupAlertHistoryItems(items);
    expect(groups.map((g) => g.alerts[0].id)).toEqual(['newest', 'mid', 'oldest']);
  });
});

// ---------------------------------------------------------------------------
// getAlertBucketDurationLabel — every arm of the guard + day/hour formatters.
// ---------------------------------------------------------------------------

describe('getAlertBucketDurationLabel', () => {
  it('returns the em-dash placeholder for non-finite input', () => {
    expect(getAlertBucketDurationLabel(Number.NaN)).toBe('—');
  });

  it('returns the em-dash placeholder for zero and negative input', () => {
    expect(getAlertBucketDurationLabel(0)).toBe('—');
    expect(getAlertBucketDurationLabel(-3)).toBe('—');
  });

  it('uses the singular "1 day" form for exactly 24 hours', () => {
    expect(getAlertBucketDurationLabel(24)).toBe('1 day');
  });

  it('uses the plural "N days" form for whole-day buckets > 24h', () => {
    expect(getAlertBucketDurationLabel(48)).toBe('2 days');
    expect(getAlertBucketDurationLabel(72)).toBe('3 days');
  });

  it('uses the singular "1 hour" form for exactly one hour', () => {
    expect(getAlertBucketDurationLabel(1)).toBe('1 hour');
  });

  it('uses the plural "N hours" form for non-whole-day hour buckets', () => {
    expect(getAlertBucketDurationLabel(6)).toBe('6 hours');
    expect(getAlertBucketDurationLabel(12)).toBe('12 hours');
  });
});

// ---------------------------------------------------------------------------
// formatAlertAxisTickLabel — invalid-timestamp guard, "Now" end tick, and the
// three totalHours formatting tiers (with/without the hour option).
// ---------------------------------------------------------------------------

describe('formatAlertAxisTickLabel', () => {
  it('returns the em-dash placeholder for a non-finite timestamp', () => {
    expect(
      formatAlertAxisTickLabel({
        timestamp: Number.NaN,
        bucketHours: 1,
        totalHours: 24,
        locale: 'en-US',
      }),
    ).toBe('—');
  });

  it('returns "Now" for an end tick within 0.75 * bucketHours of now', () => {
    const now = Date.UTC(2026, 2, 22, 12, 0, 0);
    expect(
      formatAlertAxisTickLabel({
        timestamp: now - 10 * 60 * 1000, // 10 min ago, within 0.75h
        bucketHours: 1,
        totalHours: 24,
        locale: 'en-US',
        isEnd: true,
        now,
      }),
    ).toBe('Now');
  });

  it('does NOT return "Now" for a non-end tick even when close to now', () => {
    const now = Date.UTC(2026, 2, 22, 12, 0, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: now - 10 * 60 * 1000,
      bucketHours: 1,
      totalHours: 24,
      locale: 'en-US',
      isEnd: false,
      now,
    });
    expect(label).not.toBe('Now');
    expect(label).toContain('Mar');
  });

  it('does NOT return "Now" for an end tick that is far from now', () => {
    const now = Date.UTC(2026, 2, 22, 12, 0, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: now - 20 * MS_PER_HOUR, // 20h ago, outside 0.75h window for 1h bucket
      bucketHours: 1,
      totalHours: 24,
      locale: 'en-US',
      isEnd: true,
      now,
    });
    expect(label).not.toBe('Now');
    expect(label).toContain('Mar');
  });

  it('uses month/day/hour/minute options for totalHours <= 48', () => {
    const ts = Date.UTC(2026, 2, 22, 9, 30, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: ts,
      bucketHours: 1,
      totalHours: 24,
      locale: 'en-US',
    });
    // Short range includes the time-of-day.
    expect(label).toContain('Mar 22');
    expect(label).toContain('9:30');
    expect(label).toMatch(/AM|PM/);
  });

  it('includes the hour for a mid-range total with a small bucket', () => {
    const ts = Date.UTC(2026, 2, 22, 9, 30, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: ts,
      bucketHours: 6,
      totalHours: 7 * 24, // 168h, <= 24*90 and bucketHours <= 12 → hour shown
      locale: 'en-US',
    });
    expect(label).toContain('Mar 22');
    // The mid-range branch sets only `hour` (no `minute`), so the time-of-day
    // token is a bare hour like "09 AM" with no colon.
    expect(label).toMatch(/09/);
    expect(label).toMatch(/AM|PM/);
    expect(label).not.toMatch(/\d{1,2}:\d{2}/);
  });

  it('omits the hour for a mid-range total with a large bucket and long span', () => {
    const ts = Date.UTC(2026, 2, 22, 0, 0, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: ts,
      bucketHours: 24, // > 12
      totalHours: 24 * 60, // 1440h: <= 24*90 (2160) but > 24*14 (336) → no hour
      locale: 'en-US',
    });
    expect(label).toContain('Mar 22');
    // No time-of-day token should appear.
    expect(label).not.toMatch(/\d{1,2}:\d{2}/);
  });

  it('uses year/month/day options for very long ranges (totalHours > 24*90)', () => {
    const ts = Date.UTC(2026, 2, 22, 0, 0, 0);
    const label = formatAlertAxisTickLabel({
      timestamp: ts,
      bucketHours: 24,
      totalHours: 24 * 120, // 2880h > 2160 → year branch
      locale: 'en-US',
    });
    expect(label).toContain('2026');
    expect(label).toContain('Mar 22');
    // Year branch never includes time-of-day.
    expect(label).not.toMatch(/\d{1,2}:\d{2}/);
  });
});

// ---------------------------------------------------------------------------
// buildAlertRangeSummary — null guard, normal output shape, and the
// `rangeHours ?? bucketHours` nullish-coalescing fallback.
// ---------------------------------------------------------------------------

describe('buildAlertRangeSummary', () => {
  it('returns null when bucketTimes is empty', () => {
    const trends: AlertTrendSeries = {
      buckets: [],
      max: 0,
      bucketSize: 1,
      bucketTimes: [],
      rangeStart: 0,
      rangeHours: 0,
    };
    expect(buildAlertRangeSummary(trends, 'en-US')).toBeNull();
  });

  it('returns null when bucketSize is zero', () => {
    const trends: AlertTrendSeries = {
      buckets: [1],
      max: 1,
      bucketSize: 0,
      bucketTimes: [Date.UTC(2026, 2, 22)],
      rangeStart: 0,
      rangeHours: 1,
    };
    expect(buildAlertRangeSummary(trends, 'en-US')).toBeNull();
  });

  it('returns concrete startLabel/endLabel for a normal series', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends: AlertTrendSeries = {
      buckets: [0, 0],
      max: 1,
      bucketSize: 6,
      bucketTimes: [start, start + 6 * MS_PER_HOUR],
      rangeStart: start,
      rangeHours: 12,
    };
    const summary = buildAlertRangeSummary(trends, 'en-US');
    expect(summary).not.toBeNull();
    expect(summary!.startLabel).toContain('Mar 22');
    expect(summary!.endLabel).toContain('Mar 22');
  });

  it('falls back to bucketHours when rangeHours is undefined', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends = {
      buckets: [0],
      max: 1,
      bucketSize: 6,
      bucketTimes: [start],
      rangeStart: start,
      rangeHours: undefined,
    } as unknown as AlertTrendSeries;
    // Should not throw; totalHours resolves to bucketHours (6) via the `??` arm.
    const summary = buildAlertRangeSummary(trends, 'en-US');
    expect(summary).not.toBeNull();
    expect(typeof summary!.startLabel).toBe('string');
    expect(summary!.startLabel.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// buildAlertAxisTicks — push-vs-replace last-tick arms and the align mapping.
// (The first-tick `unshift` arm is suspected dead code — see GLM_REPORT.md.)
// ---------------------------------------------------------------------------

describe('buildAlertAxisTicks (last-tick + align arms)', () => {
  const now = Date.UTC(2026, 2, 22, 12, 0, 0);

  it('pushes a new last tick when the loop does not reach position 1', () => {
    const trends = buildAlertTrends([], '24h', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[ticks.length - 1].position).toBe(1);
    // First and last carry the start/end align markers.
    expect(ticks[0].align).toBe('start');
    expect(ticks[ticks.length - 1].align).toBe('end');
  });

  it('replaces the existing last tick when the loop already lands on position 1', () => {
    // 2 bucket times, 1 bucket → loop hits index 0 and 1; index 1 maps to
    // position 1.0, triggering the replace-last-tick branch.
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
    expect(ticks[1].position).toBe(1);
    expect(ticks[1].align).toBe('end');
  });

  it('marks every non-edge tick as center-aligned', () => {
    const trends = buildAlertTrends([], '30d', now);
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    for (let i = 1; i < ticks.length - 1; i++) {
      expect(ticks[i].align).toBe('center');
    }
  });

  it('handles a single-bucket series by still emitting start and end ticks', () => {
    const start = Date.UTC(2026, 2, 22, 0, 0, 0);
    const trends: AlertTrendSeries = {
      buckets: [0],
      max: 1,
      bucketSize: 6,
      bucketTimes: [start],
      rangeStart: start,
      rangeHours: 6,
    };
    const ticks = buildAlertAxisTicks(trends, 'en-US');
    expect(ticks.length).toBeGreaterThanOrEqual(2);
    expect(ticks[0].position).toBe(0);
    expect(ticks[ticks.length - 1].position).toBe(1);
  });
});
