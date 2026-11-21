import { describe, expect, it } from 'vitest';
import type { BackupTask } from '@/types/api';
import { parsePVEBackupInventory } from '../proxmoxBackupInventory';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityTask,
} from '../__fixtures__/backupIdentity';

const fresh = '2026-10-04T11:59:00Z';
const noEnd = '0001-01-01T00:00:00Z'; // Go time.Time zero in /api/backups/pve.
const guests = [
  identityGuest('west-100', 'west', 'pve-1'),
  identityGuest('east-100', 'east', 'pve-new'),
];

function coverage(
  extra: Partial<Omit<BackupTask, 'observedAt'>> & { observedAt?: unknown } = {},
  nowMs = BACKUP_IDENTITY_NOW,
  workloads = guests,
) {
  const pve = parsePVEBackupInventory({
    data: {
      backupTasks: [
        {
          ...identityTask({ status: '', endTime: noEnd }),
          observedAt: fresh,
          ...extra,
        },
        identityTask({ id: 'west-task', instance: 'west' }),
      ],
      storageBackups: [identityArchive()],
      guestSnapshots: [],
    },
  });
  return buildProxmoxBackupRecoveryModel({
    workloads,
    pbsBackups: [],
    archives: pve.storageBackups,
    snapshots: pve.guestSnapshots,
    tasks: pve.backupTasks,
    nowMs,
  });
}

const eastTask = (result: ReturnType<typeof coverage>) =>
  result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask;

describe('Coverage only calls an open vzdump running on fresh task evidence', () => {
  it.each(['', 'running', 'active'])(
    'keeps provider verdict %j separate from execution',
    (status) => {
      const result = coverage({ status });
      expect(eastTask(result)).toMatchObject({
        id: 'task-100',
        label: 'Running',
        status,
        instance: 'east',
        node: 'pve-1', // Task origin, not the guest's current pve-new placement.
      });
      expect(eastTask(result)?.durationSeconds).toBeUndefined();
      const east = result.coverageRows.find((row) => row.workload.resourceId === 'east-100');
      expect(east?.posture).toBe('unknown');
      expect(east?.latestBackup?.nativeId).toBe('file-100');
      expect(result.coverageSummary.recoverableArtifacts).toBe(1);
      const west = result.coverageRows.find((row) => row.workload.resourceId === 'west-100');
      expect(west?.latestTask).toMatchObject({ id: 'west-task', label: 'OK' });
    },
  );

  it('expires the same task from its observation, not from its start or a reread', () => {
    const expiry = Date.parse(fresh) + 5 * 60_000;
    expect(eastTask(coverage({ endTime: undefined }, expiry))?.label).toBe('Running');
    expect(eastTask(coverage({ endTime: undefined }, expiry + 1))).toMatchObject({
      id: 'task-100',
      status: '',
      label: 'Unknown',
    });
  });

  it.each([
    { name: 'no observation', input: { observedAt: undefined } },
    { name: 'Go zero observation', input: { observedAt: noEnd } },
    { name: 'future observation', input: { observedAt: '2026-10-04T12:00:00.001Z' } },
    { name: 'observation predates start', input: { startTime: '2026-10-04T11:59:01Z' } },
    { name: 'future start', input: { startTime: '2026-10-04T12:00:01Z' } },
    { name: 'missing start', input: { startTime: '' } },
    { name: 'no timezone', input: { observedAt: '2026-10-04T11:59:00' } },
    { name: 'coercible non-string observation', input: { observedAt: [fresh] } },
    { name: 'a task with an end but no verdict', input: { endTime: '2026-10-04T11:05:00Z' } },
    { name: 'an unreadable end', input: { endTime: 'malformed' } },
    { name: 'a non-backup row', input: { type: 'vm' } },
    { name: 'bare node match with no connection', input: { instance: '', node: 'pve-new' } },
    { name: 'failed evidence with no verdict', input: { error: 'worker did not finish' } },
  ])('does not infer running from $name', ({ input }) => {
    const result = coverage(input);
    expect(eastTask(result)?.label).toBe('Unknown');
    expect(result.coverageSummary.recoverableArtifacts).toBe(1);
  });

  it('does not infer running on a guest whose older inventory has no connection', () => {
    const incomplete = [identityGuest('east-100', '', 'pve-1')];
    const row = coverage({}, BACKUP_IDENTITY_NOW, incomplete).coverageRows.find(
      (candidate) => candidate.workload.resourceId === 'east-100',
    );
    // Both connections supplied a task at the same start with only a native
    // node match. Keep candidates rather than making either the latest.
    expect(row?.latestTask).toBeUndefined();
    expect(row?.taskCandidates).toMatchObject([
      { id: 'task-100', label: 'Unknown', instance: 'east' },
      { id: 'west-task', label: 'OK', instance: 'west' },
    ]);
  });

  it('withdraws stale and contradictory active words without hiding the raw verdict', () => {
    const stale = eastTask(coverage({ status: 'running', observedAt: '2026-10-04T11:54:59Z' }));
    expect(stale).toMatchObject({ label: 'Unknown', status: 'running' });
    const ended = eastTask(coverage({ status: 'active', endTime: '2026-10-04T11:05:00Z' }));
    expect(ended).toMatchObject({ label: 'Unknown', status: 'active', durationUnknown: true });
  });

  it.each([
    { status: 'OK', label: 'OK' },
    { status: 'failed', label: 'Failed' },
    { status: 'WARNINGS: 1', label: 'WARNINGS: 1' },
    { status: 'unknown', label: 'unknown' },
  ])('keeps the reported verdict $status', ({ status, label }) => {
    const task = eastTask(coverage({ status, endTime: '2026-10-04T11:05:00Z' }));
    expect(task).toMatchObject({ id: 'task-100', status, label, durationSeconds: 300 });
  });
});
