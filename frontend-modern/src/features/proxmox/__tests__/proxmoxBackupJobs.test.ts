import { describe, expect, it } from 'vitest';
import {
  backupJobsDayObserved,
  backupJobsHasHistory,
  backupJobsTotal,
  backupJobsUnavailable,
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

  it.each(
    [
      [],
      [null],
      [{ date: '2026-02-30', historyState: 'complete', counts: counts() }],
      [{ date: '', historyState: 'complete', counts: counts() }],
      [
        { date: '2026-11-01', historyState: 'complete', counts: counts() },
        { date: '2026-11-01', historyState: 'complete', counts: counts() },
      ],
    ].map((days) => ({ days })),
  )('rejects missing or ambiguous calendar days rather than dropping them: %j', ({ days }) => {
    expect(parseBackupJobsPayload(body({ days }))).toBeNull();
  });

  it('withdraws counts and names from an unavailable window without losing readable peers', () => {
    const valid = body();
    const payload = parseBackupJobsPayload(
      body({
        last24h: { ...valid.data.last24h, historyState: 'unavailable' },
        days: [
          valid.data.days[0],
          {
            ...valid.data.days[1],
            counts: { success: -1 },
            failures: valid.data.last24h.failures,
          },
        ],
      }),
    )!;
    expect(payload.last24h.historyState).toBe('unavailable');
    expect(payload.last24h.counts).toEqual(counts());
    expect(payload.last24h.failures).toEqual([]);
    expect(payload.days[0].counts).toEqual(counts(4));
    expect(payload.days[1].historyState).toBe('unavailable');
    expect(payload.days[1].failures).toEqual([]);
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

  it('keeps failed subwindows visible even if their known counts are zero', () => {
    const zero = {
      last24h: { historyState: 'complete', counts: counts(), failures: [] },
      days: [{ date: '2026-11-01', historyState: 'complete', counts: counts() }],
    };
    expect(
      backupJobsHasHistory(
        parseBackupJobsPayload(
          body({ ...zero, last24h: { historyState: 'unavailable', counts: counts() } }),
        ),
      ),
    ).toBe(true);
    expect(
      backupJobsHasHistory(
        parseBackupJobsPayload(
          body({ ...zero, days: [{ ...zero.days[0], historyState: 'unavailable' }] }),
        ),
      ),
    ).toBe(true);
  });
});

describe('backupJobsUnavailable', () => {
  it('represents a first failure without inventing dates or zero jobs', () => {
    expect(backupJobsUnavailable(null)).toEqual({
      historyState: 'unavailable',
      timezone: '',
      last24h: { historyState: 'unavailable', counts: counts(), failures: [] },
      days: [],
    });
  });

  it('withdraws every failed-guest name when invalidating current-context observations', () => {
    const prior = parseBackupJobsPayload(body())!;
    prior.days[1].failures = [...prior.last24h.failures];
    const unavailable = backupJobsUnavailable(prior);
    expect(unavailable.last24h.counts).toEqual(counts());
    expect(unavailable.last24h.failures).toEqual([]);
    expect(unavailable.days.map((day) => day.date)).toEqual(prior.days.map((day) => day.date));
    expect(unavailable.days.every((day) => day.failures.length === 0)).toBe(true);
    expect(prior.days[1].failures).toHaveLength(1);
  });
});
