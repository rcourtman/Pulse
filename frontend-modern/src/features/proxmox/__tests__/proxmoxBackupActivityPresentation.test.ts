import { describe, expect, it, vi } from 'vitest';

import { recoveryDateKeyFromTimestamp } from '@/utils/recoveryDatePresentation';
import { getRecoveryTimelineDayFilterStateLabel } from '@/utils/recoveryTimelinePresentation';

import {
  BACKUP_ACTIVITY_RANGE_DAYS,
  buildBackupActivityTimeline,
  getBackupActivityAxisLabel,
  getBackupActivityColumnAriaLabel,
  getBackupActivityDayFilterStateLabel,
  getBackupActivityPointTotalLabel,
  getBackupActivitySegmentPresentation,
  getBackupActivityTooltipRows,
  type BackupActivityMetricMode,
  type BackupActivityNoun,
  type BackupActivityPoint,
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
} from '../proxmoxBackupActivityPresentation';
import { getProxmoxBackupSourcePresentation } from '../proxmoxBackupSourcePresentation';

const DAY_MS = 24 * 60 * 60 * 1000;

// Anchor "now" at a February date so the window never crosses a DST
// transition (DST never changes in February in either hemisphere). Every
// timestamp below is built with the local Date constructor, matching the
// module's local-day bucketing, so the assertions are timezone-independent.
const NOW = new Date(2026, 1, 13, 10, 30, 0);

const localMs = (year: number, month: number, day: number, hours = 0, minutes = 0): number =>
  new Date(year, month, day, hours, minutes, 0).getTime();

const dateKey = (year: number, month: number, day: number): string =>
  recoveryDateKeyFromTimestamp(localMs(year, month, day));

interface ActivityItem {
  ts?: number;
  kind: BackupActivitySegmentKind | null;
  bytes?: number;
}

const itemMs = (item: ActivityItem): number | undefined => item.ts;
const classify = (item: ActivityItem): BackupActivitySegmentKind | null => item.kind;

const emptyCounts = (): Record<BackupActivitySegmentKind, number> => ({
  archive: 0,
  pbs: 0,
  ok: 0,
  failed: 0,
  running: 0,
  snapshot: 0,
});

describe('proxmoxBackupActivityPresentation', () => {
  describe('BACKUP_ACTIVITY_RANGE_DAYS', () => {
    it('exposes the canonical backup activity ranges in ascending order', () => {
      expect(BACKUP_ACTIVITY_RANGE_DAYS).toEqual([7, 30, 90, 365]);
    });

    it.each([...BACKUP_ACTIVITY_RANGE_DAYS])('supports the %s-day range', (days) => {
      expect(BACKUP_ACTIVITY_RANGE_DAYS).toContain(days);
    });
  });

  describe('getBackupActivitySegmentPresentation', () => {
    it.each([
      ['archive', 'PVE backup files', 'bg-blue-500', 'bg-blue-500'],
      ['pbs', 'PBS snapshots', 'bg-cyan-500', 'bg-cyan-500'],
      ['snapshot', 'Guest snapshots', 'bg-violet-500', 'bg-violet-500'],
      ['ok', 'OK', 'bg-emerald-500', 'bg-emerald-500'],
      ['failed', 'Failed', 'bg-red-500', 'bg-red-500'],
      ['running', 'Running', 'bg-amber-500', 'bg-amber-500'],
    ])(
      'returns the canonical presentation for %s',
      (kind, label, segmentClassName, swatchClassName) => {
        expect(getBackupActivitySegmentPresentation(kind as BackupActivitySegmentKind)).toEqual({
          label,
          segmentClassName,
          swatchClassName,
        });
      },
    );

    it('returns a stable reference for repeated lookups', () => {
      expect(getBackupActivitySegmentPresentation('ok')).toBe(
        getBackupActivitySegmentPresentation('ok'),
      );
    });

    it('delegates source kinds to the proxmox backup source vocabulary', () => {
      for (const kind of ['pbs', 'archive', 'snapshot'] as const) {
        const source = getProxmoxBackupSourcePresentation(kind);
        const segment = getBackupActivitySegmentPresentation(kind);
        expect(segment.label).toBe(source.timelineLabel);
        expect(segment.segmentClassName).toBe(source.timelineSegmentClassName);
        expect(segment.swatchClassName).toBe(source.timelineSwatchClassName);
      }
    });

    it('returns undefined for an unknown segment kind at runtime', () => {
      expect(
        getBackupActivitySegmentPresentation('unknown' as BackupActivitySegmentKind),
      ).toBeUndefined();
    });
  });

  describe('buildBackupActivityTimeline', () => {
    it('creates one empty bucket per day in chronological order', () => {
      const timeline = buildBackupActivityTimeline(7, [], itemMs, classify, { now: NOW });

      expect(timeline.points).toHaveLength(7);
      expect(timeline.points.map((point) => point.key)).toEqual([
        dateKey(2026, 1, 7),
        dateKey(2026, 1, 8),
        dateKey(2026, 1, 9),
        dateKey(2026, 1, 10),
        dateKey(2026, 1, 11),
        dateKey(2026, 1, 12),
        dateKey(2026, 1, 13),
      ]);

      for (const point of timeline.points) {
        expect(point.total).toBe(0);
        expect(point.counts).toEqual(emptyCounts());
      }
      expect(timeline.axisMax).toBe(2);
      expect(timeline.labelEvery).toBe(1);
    });

    it.each<[BackupActivityRangeDays, string, string, number]>([
      [7, '2026-02-07', '2026-02-13', 1],
      [30, '2026-01-15', '2026-02-13', 5],
      [90, '2025-11-16', '2026-02-13', 10],
      [365, '2025-02-14', '2026-02-13', 30],
    ])(
      'sizes a %s-day window from %s to %s with labelEvery %s when empty',
      (days, firstKey, lastKey, labelEvery) => {
        const timeline = buildBackupActivityTimeline(days, [], itemMs, classify, { now: NOW });

        expect(timeline.points).toHaveLength(days);
        expect(timeline.points.at(0)!.key).toBe(firstKey);
        expect(timeline.points.at(-1)!.key).toBe(lastKey);
        expect(timeline.axisMax).toBe(2);
        expect(timeline.labelEvery).toBe(labelEvery);
      },
    );

    it('keeps bucket keys strictly increasing and unique across long ranges', () => {
      const timeline = buildBackupActivityTimeline(365, [], itemMs, classify, { now: NOW });
      const keys = timeline.points.map((point) => point.key);
      const asTimestamps = keys.map((key) => new Date(`${key}T00:00:00`).getTime());
      expect(new Set(keys).size).toBe(keys.length);
      for (let i = 1; i < asTimestamps.length; i += 1) {
        expect(asTimestamps[i]).toBeGreaterThan(asTimestamps[i - 1]);
      }
    });

    it('buckets items by their local-day timestamp and accumulates per-kind counts', () => {
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 13, 9, 0), kind: 'ok' },
        { ts: localMs(2026, 1, 12, 23, 30), kind: 'failed' },
        { ts: localMs(2026, 1, 7, 0, 0), kind: 'pbs' },
        { ts: localMs(2026, 1, 10, 12, 0), kind: 'archive' },
        { ts: localMs(2026, 1, 10, 18, 0), kind: 'snapshot' },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, { now: NOW });
      const byKey = new Map(timeline.points.map((point) => [point.key, point]));

      expect(byKey.get(dateKey(2026, 1, 13))!.counts.ok).toBe(1);
      expect(byKey.get(dateKey(2026, 1, 13))!.total).toBe(1);
      expect(byKey.get(dateKey(2026, 1, 12))!.counts.failed).toBe(1);
      expect(byKey.get(dateKey(2026, 1, 7))!.counts.pbs).toBe(1);

      const feb10 = byKey.get(dateKey(2026, 1, 10))!;
      expect(feb10.counts.archive).toBe(1);
      expect(feb10.counts.snapshot).toBe(1);
      expect(feb10.total).toBe(2);
    });

    it('includes the window-start edge and excludes anything at or past start-of-tomorrow', () => {
      const todayStart = localMs(2026, 1, 13);
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 6, 23, 59), kind: 'ok' },
        { ts: localMs(2026, 1, 7, 0, 0), kind: 'ok' },
        { ts: todayStart, kind: 'failed' },
        { ts: todayStart + DAY_MS - 1, kind: 'running' },
        { ts: todayStart + DAY_MS, kind: 'ok' },
        { ts: localMs(2026, 1, 14, 12, 0), kind: 'ok' },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, { now: NOW });
      const totals = new Map(timeline.points.map((point) => [point.key, point.total]));

      expect(totals.get(dateKey(2026, 1, 7))).toBe(1);
      expect(totals.get(dateKey(2026, 1, 13))).toBe(2);
      expect(totals.get(dateKey(2026, 1, 6))).toBeUndefined();
      expect(totals.get(dateKey(2026, 1, 14))).toBeUndefined();
    });

    it.each([
      ['undefined', undefined],
      ['NaN', Number.NaN],
      ['Infinity', Number.POSITIVE_INFINITY],
      ['-Infinity', Number.NEGATIVE_INFINITY],
    ])('drops items whose timestamp is %s', (_label, ts) => {
      const items: ActivityItem[] = [{ ts, kind: 'ok' }];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, { now: NOW });
      expect(timeline.points.every((point) => point.total === 0)).toBe(true);
    });

    it('skips in-window items classified as null without classifying out-of-window items', () => {
      const classifySpy = vi.fn((item: ActivityItem) => item.kind);
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 6, 12, 0), kind: 'ok' },
        { ts: localMs(2026, 1, 13, 9, 0), kind: null },
        { ts: localMs(2026, 1, 13, 10, 0), kind: 'ok' },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classifySpy, { now: NOW });

      expect(classifySpy).toHaveBeenCalledTimes(2);
      expect(timeline.points.at(-1)!.total).toBe(1);
    });

    it('counts one per item when no getValue is supplied', () => {
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 13, 9, 0), kind: 'ok' },
        { ts: localMs(2026, 1, 13, 10, 0), kind: 'ok' },
        { ts: localMs(2026, 1, 13, 11, 0), kind: 'failed' },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, { now: NOW });

      expect(timeline.points.at(-1)!.counts.ok).toBe(2);
      expect(timeline.points.at(-1)!.counts.failed).toBe(1);
      expect(timeline.points.at(-1)!.total).toBe(3);
    });

    it('accumulates per-item byte values in volume mode and rejects non-positive/non-finite values', () => {
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 1073741824 },
        { ts: localMs(2026, 1, 13, 10, 0), kind: 'failed', bytes: 2147483648 },
        { ts: localMs(2026, 1, 13, 11, 0), kind: 'ok', bytes: 0 },
        { ts: localMs(2026, 1, 13, 12, 0), kind: 'ok', bytes: -100 },
        { ts: localMs(2026, 1, 13, 13, 0), kind: 'ok', bytes: Number.NaN },
        { ts: localMs(2026, 1, 13, 14, 0), kind: 'ok', bytes: Number.POSITIVE_INFINITY },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, {
        now: NOW,
        getValue: (item) => item.bytes ?? 0,
      });
      const today = timeline.points.at(-1)!;

      expect(today.counts.ok).toBe(1073741824);
      expect(today.counts.failed).toBe(2147483648);
      expect(today.total).toBe(1073741824 + 2147483648);
    });

    it('keeps fractional positive values in volume mode', () => {
      const items: ActivityItem[] = [
        { ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 0.5 },
        { ts: localMs(2026, 1, 13, 10, 0), kind: 'ok', bytes: 1.25 },
      ];
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, {
        now: NOW,
        getValue: (item) => item.bytes ?? 0,
      });

      expect(timeline.points.at(-1)!.counts.ok).toBeCloseTo(1.75);
      expect(timeline.points.at(-1)!.total).toBeCloseTo(1.75);
    });

    it.each<[string, ActivityItem[], number]>([
      ['empty stays at the floor of 2', [], 2],
      [
        'a max of 1 stays at the floor of 2',
        [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 1 }],
        2,
      ],
      ['a max of 2 maps to 2', [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 2 }], 2],
      ['a max of 5 maps to 5', [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 5 }], 5],
      [
        'a max of 6 rounds up to 10',
        [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 6 }],
        10,
      ],
      ['a max of 50 maps to 50', [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 50 }], 50],
      [
        'a max of 100 maps to 100',
        [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 100 }],
        100,
      ],
      [
        'a max of 1000 maps to 1000',
        [{ ts: localMs(2026, 1, 13, 9, 0), kind: 'ok', bytes: 1000 }],
        1000,
      ],
    ])('axisMax %s', (_label, items, axisMax) => {
      const timeline = buildBackupActivityTimeline(7, items, itemMs, classify, {
        now: NOW,
        getValue: (item) => item.bytes ?? 0,
      });
      expect(timeline.axisMax).toBe(axisMax);
    });

    it('defaults `now` to the current wall-clock time', () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date(2026, 1, 13, 10, 30, 0));
      try {
        const timeline = buildBackupActivityTimeline(7, [], itemMs, classify);
        expect(timeline.points).toHaveLength(7);
        expect(timeline.points.at(0)!.key).toBe('2026-02-07');
        expect(timeline.points.at(-1)!.key).toBe('2026-02-13');
      } finally {
        vi.useRealTimers();
      }
    });
  });

  describe('getBackupActivityTooltipRows', () => {
    const point = (
      overrides: Partial<Record<BackupActivitySegmentKind, number>> & { total?: number },
    ): BackupActivityPoint => ({
      key: '2026-02-13',
      total: overrides.total ?? 0,
      counts: { ...emptyCounts(), ...overrides },
    });

    it('omits the percentage when a single kind is requested', () => {
      const rows = getBackupActivityTooltipRows(point({ total: 5, ok: 5 }), ['ok']);
      expect(rows).toEqual([
        {
          kind: 'ok',
          label: 'OK',
          count: 5,
          value: '5',
          segmentClassName: 'bg-emerald-500',
          muted: false,
        },
      ]);
    });

    it('appends rounded percentages when multiple kinds are requested', () => {
      const rows = getBackupActivityTooltipRows(point({ total: 10, ok: 6, failed: 4 }), [
        'ok',
        'failed',
      ]);
      expect(rows.map((row) => [row.value, row.muted])).toEqual([
        ['6 (60%)', false],
        ['4 (40%)', false],
      ]);
    });

    it('mutes zero-count kinds and shows no percentage for them', () => {
      const rows = getBackupActivityTooltipRows(point({ total: 5, ok: 5 }), ['ok', 'failed']);
      expect(rows[0]).toMatchObject({ value: '5 (100%)', muted: false });
      expect(rows[1]).toMatchObject({ value: '0', muted: true });
    });

    it('rounds the displayed count while keeping the raw count field', () => {
      const rows = getBackupActivityTooltipRows(point({ total: 3, ok: 2.6 }), ['ok']);
      expect(rows[0].count).toBe(2.6);
      expect(rows[0].value).toBe('3');
    });

    it('clamps negative totals and counts to zero', () => {
      const rows = getBackupActivityTooltipRows(point({ total: -5, ok: -3, failed: -2 }), [
        'ok',
        'failed',
      ]);
      expect(rows.map((row) => row.value)).toEqual(['0', '0']);
      expect(rows.every((row) => row.count === 0 && row.muted)).toBe(true);
    });

    it('reads missing count keys as zero', () => {
      const partial = {
        key: '2026-02-13',
        total: 2,
        counts: { ok: 2 },
      } as unknown as BackupActivityPoint;
      const rows = getBackupActivityTooltipRows(partial, ['ok', 'failed']);
      expect(rows[1].count).toBe(0);
      expect(rows[1].muted).toBe(true);
    });

    it('returns no rows for an empty kind list', () => {
      expect(getBackupActivityTooltipRows(point({ total: 5, ok: 5 }), [])).toEqual([]);
    });

    it('formats volume-mode values as bytes with percentages', () => {
      const rows = getBackupActivityTooltipRows(
        point({ total: 3221225472, ok: 1073741824, failed: 2147483648 }),
        ['ok', 'failed'],
        'volume',
      );
      expect(rows[0].value).toBe('1.00 GB (33%)');
      expect(rows[1].value).toBe('2.00 GB (67%)');
    });

    it('formats a single volume-mode row without a percentage', () => {
      const rows = getBackupActivityTooltipRows(
        point({ total: 1073741824, ok: 1073741824 }),
        ['ok'],
        'volume',
      );
      expect(rows[0].value).toBe('1.00 GB');
    });

    it('shows 0 B for zero-count volume rows', () => {
      const rows = getBackupActivityTooltipRows(point({}), ['ok'], 'volume');
      expect(rows[0].value).toBe('0 B');
      expect(rows[0].muted).toBe(true);
    });

    it('defaults to count mode when mode is omitted', () => {
      expect(getBackupActivityTooltipRows(point({ total: 1, ok: 1 }), ['ok'])[0].value).toBe('1');
    });
  });

  describe('getBackupActivityPointTotalLabel', () => {
    it.each([
      [0, 'backup', '0 backups'],
      [1, 'backup', '1 backup'],
      [2, 'backup', '2 backups'],
      [1, 'snapshot', '1 snapshot'],
      [5, 'task', '5 tasks'],
      [2, 'archive', '2 archives'],
      [-3, 'backup', '0 backups'],
      [2.6, 'backup', '3 backups'],
      [2.4, 'backup', '2 backups'],
    ])('count mode: total %s %s -> %s', (total, noun, expected) => {
      expect(getBackupActivityPointTotalLabel(total, noun as BackupActivityNoun)).toBe(expected);
    });

    it.each([
      [0, '0 B'],
      [1073741824, '1.00 GB'],
      [2147483648, '2.00 GB'],
      [-5, '0 B'],
    ])('volume mode: total %s -> %s', (total, expected) => {
      expect(getBackupActivityPointTotalLabel(total, 'backup', 'volume')).toBe(expected);
    });

    it('defaults to count mode when mode is omitted', () => {
      expect(getBackupActivityPointTotalLabel(1, 'backup')).toBe('1 backup');
    });
  });

  describe('getBackupActivityColumnAriaLabel', () => {
    it.each([
      ['Feb 13, 2026', 5, false, 'backup', 'Feb 13, 2026: 5 backups'],
      ['Feb 13, 2026', 5, true, 'backup', 'Feb 13, 2026: 5 backups, selected'],
      ['Feb 13, 2026', 1, false, 'snapshot', 'Feb 13, 2026: 1 snapshot'],
      ['Feb 13, 2026', 0, true, 'backup', 'Feb 13, 2026: 0 backups, selected'],
    ])('builds aria label for %s/%s/%s/%s', (dateLabel, total, selected, noun, expected) => {
      expect(
        getBackupActivityColumnAriaLabel(dateLabel, total, selected, noun as BackupActivityNoun),
      ).toBe(expected);
    });

    it('formats volume totals in the aria label', () => {
      expect(
        getBackupActivityColumnAriaLabel('Feb 13, 2026', 1073741824, false, 'backup', 'volume'),
      ).toBe('Feb 13, 2026: 1.00 GB');
    });
  });

  describe('getBackupActivityAxisLabel', () => {
    it.each([
      [0, 'count', '0'],
      [1, 'count', '1'],
      [2.6, 'count', '3'],
      [100, 'count', '100'],
      [-5, 'count', '0'],
    ])('count axis label for %s is %s', (value, mode, expected) => {
      expect(getBackupActivityAxisLabel(value, mode as BackupActivityMetricMode)).toBe(expected);
    });

    it.each([
      [0, '0 B'],
      [1073741824, '1.00 GB'],
      [-5, '0 B'],
    ])('volume axis label for %s is %s', (value, expected) => {
      expect(getBackupActivityAxisLabel(value, 'volume')).toBe(expected);
    });
  });

  describe('getBackupActivityDayFilterStateLabel', () => {
    it.each([
      [true, true, 'Day filter'],
      [false, true, 'Outside day filter'],
      [false, false, 'Timeline day'],
      [true, false, 'Day filter'],
    ])('selected=%s hasDayFilter=%s -> %s', (selected, hasDayFilter, expected) => {
      expect(getBackupActivityDayFilterStateLabel(selected, hasDayFilter)).toBe(expected);
    });

    it('is a re-export of the recovery timeline day filter state label', () => {
      expect(getBackupActivityDayFilterStateLabel).toBe(getRecoveryTimelineDayFilterStateLabel);
    });
  });
});
