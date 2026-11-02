import { describe, expect, it } from 'vitest';
import {
  backupJobsDayObserved,
  backupJobsHasHistory,
  backupJobsTotal,
  parseBackupJobsPayload,
} from '../proxmoxBackupJobs';

const counts = (success = 0, failed = 0) => ({
  success,
  failed,
  warning: 0,
  running: 0,
  unknown: 0,
});

const body = (overrides: Record<string, unknown> = {}) => ({
  data: {
    historyState: 'complete',
    timezone: 'Europe/London',
    last24h: {
      historyState: 'complete',
      counts: counts(3, 1),
      failures: [
        {
          workloadId: 'vm-100',
          workloadName: 'web',
          instance: 'lab',
          at: '2026-11-01T02:00:00Z',
          outcome: 'failed',
        },
      ],
    },
    days: [
      { date: '2026-10-31', historyState: 'complete', counts: counts(4) },
      { date: '2026-11-01', historyState: 'complete', counts: counts(3, 1) },
    ],
    ...overrides,
  },
});

describe('parseBackupJobsPayload', () => {
  it('reads the server summary as sent', () => {
    const payload = parseBackupJobsPayload(body());
    expect(payload?.last24h.counts).toEqual(counts(3, 1));
    expect(payload?.last24h.failures[0].workloadName).toBe('web');
    expect(payload?.days).toHaveLength(2);
  });

  it('treats an unknown history state as unavailable, never clean', () => {
    const payload = parseBackupJobsPayload(
      body({
        historyState: 'fine',
        days: [{ date: '2026-11-01', historyState: 'later', counts: counts() }],
      }),
    );
    expect(payload?.historyState).toBe('unavailable');
    expect(payload?.days[0].historyState).toBe('unavailable');
    expect(backupJobsDayObserved(payload!.days[0])).toBe(false);
  });

  it('rejects a body without days and never turns bad counts into a clean day', () => {
    expect(parseBackupJobsPayload({})).toBeNull();
    expect(parseBackupJobsPayload({ data: { days: 'no' } })).toBeNull();
    const payload = parseBackupJobsPayload(
      body({
        days: [
          { date: '2026-11-01', historyState: 'complete', counts: { success: -2, failed: 'x' } },
        ],
      }),
    );
    expect(backupJobsTotal(payload!.days[0].counts)).toBe(0);
    expect(payload!.days[0].historyState).toBe('unavailable');
    expect(backupJobsDayObserved(payload!.days[0])).toBe(false);
  });
});

describe('backupJobsHasHistory', () => {
  it('hides the card until a backup task is counted, but never hides a read failure', () => {
    expect(backupJobsHasHistory(null)).toBe(false);
    const never = parseBackupJobsPayload(
      body({
        historyState: 'not_observed',
        last24h: { historyState: 'not_observed', counts: counts(), failures: [] },
        days: [{ date: '2026-11-01', historyState: 'not_observed', counts: counts() }],
      }),
    );
    expect(backupJobsHasHistory(never)).toBe(false);
    // Collected without error, but no vzdump job exists.
    const empty = parseBackupJobsPayload(
      body({
        last24h: { historyState: 'complete', counts: counts(), failures: [] },
        days: [{ date: '2026-11-01', historyState: 'complete', counts: counts() }],
      }),
    );
    expect(backupJobsHasHistory(empty)).toBe(false);
    expect(
      backupJobsHasHistory(parseBackupJobsPayload(body({ historyState: 'unavailable' }))),
    ).toBe(true);
    expect(backupJobsHasHistory(parseBackupJobsPayload(body()))).toBe(true);
  });
});
