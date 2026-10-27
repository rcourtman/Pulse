import { describe, expect, it } from 'vitest';
import type { BackupTask } from '@/types/api';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
} from '../proxmoxBackupRecoveryModel';
import { BACKUP_IDENTITY_NOW, identityGuest, identityTask } from '../__fixtures__/backupIdentity';

const invalidStarts = ['', 'invalid', '2027-01-01T00:00:00Z', '1970-01-01T00:00:00Z'];
const guests = ['east', 'west'].map((instance) =>
  identityGuest(`guest-${instance}`, instance, 'pve-1'),
);
function build(tasks: BackupTask[]) {
  return buildProxmoxBackupRecoveryModel({
    workloads: guests,
    pbsBackups: [],
    archives: [],
    snapshots: [],
    tasks,
    nowMs: BACKUP_IDENTITY_NOW,
  }).coverageRows;
}
const failed = (startTime: string) =>
  identityTask({ id: 'undated-failure', status: 'failed', error: 'retained error', startTime });
const ok = identityTask({ id: 'dated-ok' });

describe('backup task chronology', () => {
  it.each(invalidStarts.flatMap((start) => [false, true].map((reverse) => ({ start, reverse }))))(
    'never ranks an unknown start against a dated OK: $start, reversed=$reverse',
    ({ start, reverse }) => {
      const tasks = [failed(start), ok];
      const [east, west] = build(reverse ? tasks.reverse() : tasks);
      expect(east.latestTask).toBeUndefined();
      expect(east.taskCandidates?.map((task) => task.id)).toEqual(['undated-failure', 'dated-ok']);
      expect(east.taskCandidates?.[0].startedMs).toBeUndefined();
      expect(east.taskCandidates?.[0].error).toBe('retained error');
      expect(coverageRowMatchesSearch(east, 'RETAINED ERROR')).toBe(true);
      expect(coverageRowMatchesSearch(east, 'undated-failure')).toBe(true);
      expect(west.latestTask).toBeUndefined();
      expect(west.taskCandidates).toBeUndefined();
      expect(coverageRowMatchesSearch(west, 'retained error')).toBe(false);
      expect(east.posture).toBe('unknown');
    },
  );

  it.each([false, true])(
    'does not break equal starts by status or input order: reversed=%s',
    (reverse) => {
      const tasks = [failed(ok.startTime), ok];
      const [east] = build(reverse ? tasks.reverse() : tasks);
      expect(east.latestTask).toBeUndefined();
      expect(east.taskCandidates?.map((task) => task.id)).toEqual(['dated-ok', 'undated-failure']);
      expect(
        east.taskCandidates?.every((task) => task.startedMs === Date.parse(ok.startTime)),
      ).toBe(true);
    },
  );

  it('retains all unknown starts and only the newest dated candidates in a stable non-time order', () => {
    const tasks = [
      failed('invalid'),
      identityTask({ id: 'other-unknown', startTime: '' }),
      ok,
      identityTask({ id: 'same-time-failure', status: 'error' }),
      identityTask({ id: 'older', startTime: '2026-10-03T11:00:00Z' }),
    ];
    const [east] = build(tasks);
    expect(east.latestTask).toBeUndefined();
    expect(east.taskCandidates?.map((task) => task.id)).toEqual([
      'other-unknown',
      'undated-failure',
      'dated-ok',
      'same-time-failure',
    ]);
    expect(build(tasks.reverse())[0].taskCandidates).toEqual(east.taskCandidates);
  });

  it.each(invalidStarts)(
    'keeps a sole observed task status without inventing a start: %s',
    (start) => {
      const [east] = build([failed(start)]);
      expect(east.taskCandidates).toBeUndefined();
      expect(east.latestTask).toMatchObject({ id: 'undated-failure', label: 'Failed' });
      expect(east.latestTask?.startedMs).toBeUndefined();
    },
  );

  it('selects a strictly newer valid task and leaves no task when none was read', () => {
    const [east] = build([failed('2026-10-03T11:00:00Z'), ok]);
    expect(east.latestTask?.id).toBe('dated-ok');
    expect(east.taskCandidates).toBeUndefined();
    expect(build([])[0].latestTask).toBeUndefined();
    expect(build([])[0].taskCandidates).toBeUndefined();
  });
});
