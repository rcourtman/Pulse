import { describe, expect, it } from 'vitest';

import {
  BACKUP_ACTIVITY_RANGE_DAYS,
  buildBackupActivityTimeline,
} from '../proxmoxBackupActivityPresentation';

interface Item {
  ts: number;
  bytes: number;
}

// Expected keys use UTC only as a calendar, independently of the local-time
// arithmetic under test. Run this file with TZ set before starting Vitest.
const calendarKeys = (year: number, month: number, day: number, days: number): string[] =>
  Array.from({ length: days }, (_, index) =>
    new Date(Date.UTC(year, month, day - days + 1 + index)).toISOString().slice(0, 10),
  );
const localTime = (key: string, hour = 0, minute = 0): number => {
  const [year, month, day] = key.split('-').map(Number);
  return new Date(year, month - 1, day, hour, minute).getTime();
};

// Both sides of northern and southern hemisphere transitions, including the
// half-hour changes on Lord Howe. UTC provides the no-transition control.
const dates = [
  [2026, 2, 9],
  [2026, 2, 30],
  [2026, 3, 5],
  [2026, 3, 6],
  [2026, 9, 5],
  [2026, 9, 25],
  [2026, 9, 26],
  [2026, 10, 1],
  [2026, 10, 2],
] as const;

for (const [year, month, day] of dates) {
  describe(`local calendar window ending ${year}-${month + 1}-${day}`, () => {
    it.each(BACKUP_ACTIVITY_RANGE_DAYS)('counts each local date once over %s days', (days) => {
      const keys = calendarKeys(year, month, day, days);
      const now = new Date(year, month, day, 23, 45);
      // The first half-hour of the first day and the last hour of today must
      // not be lost when the offset changes inside the window or today.
      const items: Item[] = keys.flatMap((key) => [
        { ts: localTime(key, 0, 15), bytes: 2 },
        { ts: localTime(key, 23, 30), bytes: 3 },
      ]);
      items.push(
        { ts: localTime(keys[0]) - 1, bytes: 1000 },
        { ts: localTime(keys[0]), bytes: 5 },
        { ts: now.getTime(), bytes: 7 },
        { ts: now.getTime() + 1, bytes: 1000 },
      );

      for (const volume of [false, true]) {
        const timeline = buildBackupActivityTimeline(
          days,
          items,
          (item) => item.ts,
          () => 'pbs',
          { now, getValue: volume ? (item) => item.bytes : undefined },
        );
        expect(now.getTime()).toBe(new Date(year, month, day, 23, 45).getTime());
        expect(timeline.points.map((point) => point.key)).toEqual(keys);
        expect(new Set(timeline.points.map((point) => point.key)).size).toBe(days);
        for (const [index, point] of timeline.points.entries()) {
          const start = index === 0 ? (volume ? 5 : 1) : 0;
          const end = index === days - 1 ? (volume ? 7 : 1) : 0;
          const expected = (volume ? 5 : 2) + start + end;
          expect(point.total, `${point.key} ${volume ? 'volume' : 'count'}`).toBe(expected);
          expect(point.counts.pbs).toBe(expected);
          expect(point.counts.archive).toBe(0);
        }
        expect(timeline.points.reduce((sum, point) => sum + point.total, 0)).toBe(
          volume ? days * 5 + 12 : days * 2 + 2,
        );
      }
    });
  });
}
