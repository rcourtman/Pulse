import { describe, expect, it } from 'vitest';
import type { BackupTask } from '@/types/api';
import { parsePVEBackupInventory } from '../proxmoxBackupInventory';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import { BACKUP_IDENTITY_NOW, identityGuest, identityTask } from '../__fixtures__/backupIdentity';

function observe(extra: Partial<BackupTask>, nowMs = BACKUP_IDENTITY_NOW) {
  const pve = parsePVEBackupInventory({
    data: { backupTasks: [identityTask(extra)], storageBackups: [], guestSnapshots: [] },
  });
  return buildProxmoxBackupRecoveryModel({
    workloads: [identityGuest('east-100', 'east', 'pve-1')],
    pbsBackups: [],
    archives: pve.storageBackups,
    snapshots: pve.guestSnapshots,
    tasks: pve.backupTasks,
    nowMs,
  });
}

const unknownCases: { name: string; task: Partial<BackupTask> }[] = [
  { name: 'future end', task: { endTime: '2026-10-04T12:00:01Z' } },
  { name: 'future end across offset', task: { endTime: '2026-10-04T14:00:01+02:00' } },
  {
    name: 'both endpoints in future',
    task: { startTime: '2027-01-01T00:00:00Z', endTime: '2027-01-01T00:05:00Z' },
  },
  { name: 'running with reported end', task: { status: 'running' } },
  { name: 'normalised running status', task: { status: ' RUNNING ' } },
  { name: 'epoch start', task: { startTime: '1970-01-01T00:00:00Z' } },
  {
    name: 'negative instants',
    task: { startTime: '1969-01-01T00:00:00Z', endTime: '1969-01-01T00:05:00Z' },
  },
  { name: 'missing start with reported end', task: { startTime: '' } },
  { name: 'impossible end', task: { endTime: '2026-09-31T11:05:00Z' } },
  { name: 'timezone-less end', task: { endTime: '2026-10-04T11:05:00' } },
  { name: 'reversed interval', task: { endTime: '2026-10-04T10:59:59Z' } },
  { name: 'equal endpoints', task: { endTime: '2026-10-04T11:00:00Z' } },
];

describe('Coverage task duration needs observed, consistent endpoints', () => {
  it.each(unknownCases)('keeps $name unknown without changing provider evidence', ({ task }) => {
    const model = observe({ ...task, error: 'retained provider error' });
    const row = model.coverageRows[0];
    expect(row.latestTask?.durationSeconds).toBeUndefined();
    expect(row.latestTask?.durationUnknown).toBe(true);
    expect(row.latestTask?.status).toBe(task.status ?? 'OK');
    expect(row.latestTask?.error).toBe('retained provider error');
    expect(row.latestTask?.id).toBe('task-100');
    expect(row.workload.resourceId).toBe('east-100');
    expect(row.posture).toBe('unknown');
    expect(model.coverageSummary.recoverableArtifacts).toBe(0);
  });

  it.each([undefined, ''])('keeps an unreported end distinct from bad evidence: %s', (endTime) => {
    const task = observe({ endTime }).coverageRows[0].latestTask!;
    expect(task.durationSeconds).toBeUndefined();
    expect(task.durationUnknown).not.toBe(true);
  });

  it.each([
    { name: 'ordinary OK', task: {}, seconds: 300 },
    { name: 'reported failure', task: { status: 'failed' }, seconds: 300 },
    { name: 'success alias', task: { status: 'success' }, seconds: 300 },
    { name: 'completed alias', task: { status: 'completed' }, seconds: 300 },
    { name: 'end at observation', task: { endTime: '2026-10-04T12:00:00Z' }, seconds: 3600 },
    {
      name: 'offsets describe instants',
      task: { startTime: '2026-10-04T13:00:00+02:00', endTime: '2026-10-04T08:05:00-03:00' },
      seconds: 300,
    },
    { name: 'observed subsecond rounds to zero', task: { endTime: '2026-10-04T11:00:00.1Z' }, seconds: 0 },
  ])('retains $name', ({ task: input, seconds }) => {
    const task = observe(input).coverageRows[0].latestTask!;
    expect(task.durationSeconds).toBe(seconds);
    expect(task.durationUnknown).not.toBe(true);
  });

  it('does not fabricate elapsed time without a finite observation clock', () => {
    expect(observe({}, Number.NaN).coverageRows[0].latestTask).toMatchObject({ durationUnknown: true });
  });
});
