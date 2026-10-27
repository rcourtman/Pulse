import { describe, expect, it } from 'vitest';
import type { BackupTask } from '@/types/api';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
} from '../proxmoxBackupRecoveryModel';
import { parsePBSBackupInventory, parsePVEBackupInventory } from '../proxmoxBackupInventory';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
  identityTask,
} from '../__fixtures__/backupIdentity';

const malformed = [
  '2026-09-31T11:00:00Z',
  '2026-02-29T11:00:00Z',
  '1900-02-29T11:00:00Z',
  '2026-04-31T11:00:00+02:00',
  '2026-00-01T11:00:00Z',
  '2026-10-00T11:00:00Z',
  '2026-10-04T24:00:00Z',
  '2026-10-04T11:60:00Z',
  '2026-10-04T11:00:60Z',
  '2026-10-04T11:00:00+24:00',
  '2026-10-04T11:00:00+00:60',
  '2026-10-04T11:00:00',
  '2026-10-04',
  '10/04/2026',
  '04 Oct 2026 11:00:00 GMT',
  '2026-10-04T11:00:00.1234567890Z',
];
const valid = [
  ['2026-10-04T11:00:00Z', '2026-10-04T11:00:00.000Z'],
  ['2026-10-04T11:00:00+02:30', '2026-10-04T08:30:00.000Z'],
  ['2026-10-03T23:00:00-03:30', '2026-10-04T02:30:00.000Z'],
  ['2026-10-04T11:00:00+00:00', '2026-10-04T11:00:00.000Z'],
  ['2026-10-04T11:00:00-00:00', '2026-10-04T11:00:00.000Z'],
  ['2026-10-04T11:00:00.1Z', '2026-10-04T11:00:00.100Z'],
  ['2026-10-04T11:00:00.123456789Z', '2026-10-04T11:00:00.123Z'],
  ['2024-02-29T23:59:59Z', '2024-02-29T23:59:59.000Z'],
  ['2000-02-29T00:00:00Z', '2000-02-29T00:00:00.000Z'],
] as const;
const kinds = ['pbs', 'archive', 'snapshot'] as const;
type Kind = (typeof kinds)[number];
const guest = identityGuest('east-100', 'east', 'pve-1');

function build(kind: Kind, date: string, tasks: BackupTask[] = []) {
  // Real JSON boundary: malformed dates remain observations, not a rejected
  // whole inventory, a dropped row or guessed values from ctime / other dates.
  const pve = parsePVEBackupInventory({
    data: {
      backupTasks: tasks,
      storageBackups: kind === 'archive' ? [identityArchive({ time: date })] : [],
      guestSnapshots: kind === 'snapshot' ? [identitySnapshot({ time: date })] : [],
    },
  });
  const pbs = parsePBSBackupInventory({
    data: {
      backups: kind === 'pbs' ? [identityPBS({ backupTime: date })] : [],
    },
  });
  return buildProxmoxBackupRecoveryModel({
    workloads: [guest],
    pbsBackups: pbs.backups,
    archives: pve.storageBackups,
    snapshots: pve.guestSnapshots,
    tasks: pve.backupTasks,
    nowMs: BACKUP_IDENTITY_NOW,
  });
}

describe.each(kinds)('%s backup timestamp evidence', (kind) => {
  it.each(malformed)('keeps non-RFC3339 or impossible dates unknown: %s', (date) => {
    const model = build(kind, date);
    const [artifact] = model.recoverableArtifacts;
    expect(artifact.createdAt).toBe(date);
    expect(artifact.createdMs).toBeUndefined();
    expect(model.coverageSummary.recoverableArtifacts).toBe(1);
    expect(model.coverageSummary.totalBytes).toBe(artifact.size ?? 0);
    const row = model.coverageRows[0];
    expect(row.artifacts).toHaveLength(1);
    expect(row.latestBackup).toBeUndefined();
    expect(row.ageUnknown?.[kind]).toBe(true);
    expect(row.workload.resourceId).toBe(guest.id);
    expect(row.posture).toBe('unknown');
  });

  it.each(valid)('valid-date control: preserves %s as %s', (date, utc) => {
    const model = build(kind, date);
    expect(model.recoverableArtifacts[0].createdAt).toBe(date);
    expect(model.recoverableArtifacts[0].createdMs).toBe(Date.parse(utc));
    expect(model.coverageRows[0].ageUnknown?.[kind]).toBe(false);
    expect(model.coverageSummary.recoverableArtifacts).toBe(1);
  });
});

describe('task timestamp evidence', () => {
  it.each(malformed.flatMap((date) => [false, true].map((reverse) => ({ date, reverse }))))(
    'never lets a guessed start hide a failure behind dated OK: $date reversed=$reverse',
    ({ date, reverse }) => {
      const tasks = [
        identityTask({
          id: 'malformed-failure',
          startTime: date,
          status: 'failed',
          error: 'Retain this failure',
        }),
        identityTask({ id: 'dated-ok' }),
      ];
      const row = build('archive', '2026-10-04T11:00:00Z', reverse ? tasks.reverse() : tasks)
        .coverageRows[0];
      expect(row.latestTask).toBeUndefined();
      expect(row.taskCandidates?.map((task) => task.id)).toEqual(['malformed-failure', 'dated-ok']);
      expect(row.taskCandidates?.[0]).toMatchObject({
        startedAt: date,
        label: 'Failed',
        error: 'Retain this failure',
        instance: 'east',
        node: 'pve-1',
      });
      expect(row.taskCandidates?.[0].startedMs).toBeUndefined();
      expect(row.taskCandidates?.[0].durationSeconds).toBeUndefined();
      expect(coverageRowMatchesSearch(row, 'retain this failure')).toBe(true);
    },
  );

  it.each(malformed)('does not invent a duration from a malformed end: %s', (endTime) => {
    const row = build('archive', '2026-10-04T11:00:00Z', [
      identityTask({ startTime: '2026-01-01T00:00:00Z', endTime }),
    ]).coverageRows[0];
    expect(row.latestTask?.durationSeconds).toBeUndefined();
    expect(row.latestTask?.label).toBe('OK');
  });

  it('valid-date control: orders instants rather than local clock text and measures across offsets', () => {
    const row = build('archive', '2026-10-04T11:00:00Z', [
      identityTask({ id: 'earlier', startTime: '2026-10-04T11:00:00+02:00' }),
      identityTask({
        id: 'later',
        startTime: '2026-10-04T10:00:00Z',
        endTime: '2026-10-04T12:05:00+02:00',
      }),
    ]).coverageRows[0];
    expect(row.latestTask?.id).toBe('later');
    expect(row.latestTask?.durationSeconds).toBe(300);
    expect(row.taskCandidates).toBeUndefined();
  });
});
