import { describe, expect, it } from 'vitest';

import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identityTask,
} from '../__fixtures__/backupIdentity';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
} from '../proxmoxBackupRecoveryModel';

type Input = Parameters<typeof buildProxmoxBackupRecoveryModel>[0];
const model = (input: Partial<Input>) =>
  buildProxmoxBackupRecoveryModel({
    workloads: [],
    archives: [],
    snapshots: [],
    pbsBackups: [],
    tasks: [],
    nowMs: BACKUP_IDENTITY_NOW,
    ...input,
  });

describe('collected backup jobs without a resolved guest', () => {
  it.each(['vzdump', '', 'future-provider-type'])(
    'retains a positive-VMID %s job without inventing a guest type or restore point',
    (type) => {
      const task = identityTask({ type, status: 'error', error: 'Guest backup could not start.' });
      const before = structuredClone(task);
      const result = model({ tasks: [task] });
      expect(result.coverageRows).toHaveLength(1);
      const row = result.coverageRows[0];
      expect(row.workload).toMatchObject({ type: 'unknown', vmid: '100', label: 'Guest 100' });
      expect(row.workload.resourceId).toBeUndefined();
      expect(row.isOrphaned).toBe(true);
      expect(row.posture).toBe('not-evaluated');
      expect(row.latestTask).toMatchObject({ id: task.id, label: 'Failed', error: task.error });
      expect(row.artifacts).toEqual([]);
      expect(result.recoverableArtifacts).toEqual([]);
      expect(result.coverageSummary.recoverableArtifacts).toBe(0);
      expect(result.coverageSummary.protected).toBe(0);
      for (const term of [task.id, task.instance, task.node, task.error!])
        expect(coverageRowMatchesSearch(row, term)).toBe(true);
      expect(task).toEqual(before);
    },
  );

  it('retains ambiguity as its own unrated evidence, not a job on either live guest', () => {
    const vm = identityGuest('vm-100', 'east', 'pve-1');
    const ct = {
      ...identityGuest('ct-100', 'east', 'pve-1'),
      type: 'system-container',
    } as typeof vm;
    const result = model({ workloads: [vm, ct], tasks: [identityTask({ status: 'error' })] });
    expect(result.coverageRows).toHaveLength(3);
    expect(result.coverageRows.filter((row) => !row.isOrphaned)).toHaveLength(2);
    expect(
      result.coverageRows.filter((row) => row.workload.resourceId).every((row) => !row.latestTask),
    ).toBe(true);
    expect(result.coverageRows.find((row) => row.isOrphaned)?.latestTask?.label).toBe('Failed');
  });

  it('never borrows PBS repository identity for an untyped PVE job', () => {
    const result = model({ pbsBackups: [identityPBS()], tasks: [identityTask()] });
    expect(result.coverageRows).toHaveLength(2);
    expect(result.coverageRows.find((row) => row.pbsCount > 0)?.latestTask).toBeUndefined();
    expect(result.coverageRows.find((row) => row.workload.type === 'unknown')?.latestTask?.id).toBe(
      'task-100',
    );
    expect(result.recoverableArtifacts).toHaveLength(1);
  });

  it('keeps conflicting PVE scopes separate even when just one live VMID remains', () => {
    const result = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      tasks: [identityTask({ instance: 'retired-east', status: 'error' })],
    });
    expect(result.coverageRows.find((row) => row.workload.resourceId)?.latestTask).toBeUndefined();
    expect(result.coverageRows.find((row) => row.isOrphaned)?.latestTask?.instance).toBe(
      'retired-east',
    );
  });

  it('preserves complete raw source framing, repeated VMIDs and unknown chronology', () => {
    const tasks = [
      identityTask({ id: 'case-a', instance: 'east', node: 'pve 1', startTime: '' }),
      identityTask({ id: 'case-b', instance: 'East', node: 'pve 1', startTime: '' }),
      identityTask({ id: 'parts-a', instance: 'a:b', node: 'c', startTime: '' }),
      identityTask({ id: 'parts-b', instance: 'a', node: 'b:c', startTime: '' }),
    ];
    const result = model({ tasks });
    expect(result.coverageRows).toHaveLength(4);
    expect(new Set(result.coverageRows.map((row) => row.key)).size).toBe(4);
    for (const row of result.coverageRows) {
      expect(row.key).not.toMatch(/[\s"']/);
      expect(row.latestTask).toBeUndefined();
      expect(row.taskChronologyUnknown).toBe(true);
      expect(row.unresolvedTasks).toHaveLength(1);
    }
  });

  it.each([false, true])(
    'does not let task-created rows teach another job its guest type (reversed: %s)',
    (reversed) => {
      const tasks = [identityTask({ id: 'typed', type: 'vm' }), identityTask({ id: 'untyped' })];
      const result = model({ tasks: reversed ? tasks.reverse() : tasks });
      expect(result.coverageRows).toHaveLength(2);
      expect(result.coverageRows.find((row) => row.workload.type === 'vm')?.latestTask?.id).toBe(
        'typed',
      );
      expect(
        result.coverageRows.find((row) => row.workload.type === 'unknown')?.latestTask?.id,
      ).toBe('untyped');
    },
  );

  it('still resolves an untyped job from a unique live guest or PVE artifact', () => {
    const live = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      tasks: [identityTask()],
    });
    expect(live.coverageRows).toHaveLength(1);
    expect(live.coverageRows[0].workload.resourceId).toBe('east-100');
    expect(live.coverageRows[0].latestTask?.id).toBe('task-100');
    const archive = model({ archives: [identityArchive()], tasks: [identityTask()] });
    expect(archive.coverageRows).toHaveLength(1);
    expect(archive.coverageRows[0].workload.type).toBe('vm');
    expect(archive.coverageRows[0].latestTask?.id).toBe('task-100');
  });

  it('does not invent a guest for a VMID-zero parent job', () => {
    const result = model({ tasks: [identityTask({ vmid: 0 })] });
    expect(result.coverageRows).toEqual([]);
    expect(result.recoverableArtifacts).toEqual([]);
  });
});
