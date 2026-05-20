import { describe, expect, it } from 'vitest';

import { recoveryDateKeyFromTimestamp } from '@/utils/recoveryDatePresentation';
import {
  buildBackupActivityTimeline,
  getBackupActivityColumnAriaLabel,
  getBackupActivityTooltipRows,
  type BackupActivitySegmentKind,
} from '../proxmoxBackupActivityPresentation';

describe('proxmoxBackupActivityPresentation', () => {
  it('buckets backup activity by local day and segment kind', () => {
    const now = new Date(2026, 4, 20, 12);
    const today = new Date(2026, 4, 20, 9).getTime();
    const yesterday = new Date(2026, 4, 19, 9).getTime();
    const outsideWindow = new Date(2026, 4, 10, 9).getTime();
    const future = new Date(2026, 4, 21, 9).getTime();

    const timeline = buildBackupActivityTimeline(
      7,
      [
        { ts: today, kind: 'ok' },
        { ts: today, kind: 'failed' },
        { ts: yesterday, kind: 'ok' },
        { ts: outsideWindow, kind: 'ok' },
        { ts: future, kind: 'running' },
      ],
      (item) => item.ts,
      (item) => item.kind as BackupActivitySegmentKind,
      now,
    );

    const todayPoint = timeline.points.find(
      (point) => point.key === recoveryDateKeyFromTimestamp(today),
    );
    const yesterdayPoint = timeline.points.find(
      (point) => point.key === recoveryDateKeyFromTimestamp(yesterday),
    );

    expect(timeline.points).toHaveLength(7);
    expect(todayPoint?.total).toBe(2);
    expect(todayPoint?.counts.ok).toBe(1);
    expect(todayPoint?.counts.failed).toBe(1);
    expect(yesterdayPoint?.total).toBe(1);
    expect(timeline.points.reduce((sum, point) => sum + point.total, 0)).toBe(3);
    expect(timeline.axisMax).toBeGreaterThanOrEqual(2);
  });

  it('formats activity labels and tooltip rows from segment totals', () => {
    const point = {
      key: '2026-05-20',
      total: 4,
      counts: { archive: 0, ok: 3, failed: 1, running: 0 },
    };

    expect(getBackupActivityColumnAriaLabel('Wed 20 May', point.total, true, 'task')).toBe(
      'Wed 20 May: 4 tasks, selected',
    );
    expect(getBackupActivityTooltipRows(point, ['ok', 'failed', 'running'])).toMatchObject([
      { kind: 'ok', label: 'OK', value: '3 (75%)', muted: false },
      { kind: 'failed', label: 'Failed', value: '1 (25%)', muted: false },
      { kind: 'running', label: 'Running', value: '0', muted: true },
    ]);
  });
});
