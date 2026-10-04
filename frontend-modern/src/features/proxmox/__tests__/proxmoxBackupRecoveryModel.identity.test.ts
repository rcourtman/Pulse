import { describe, expect, it } from 'vitest';

import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
  identityTask,
} from '../__fixtures__/backupIdentity';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';

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
const owners = (result: ReturnType<typeof model>) =>
  result.recoverableArtifacts.map((artifact) => artifact.workload.resourceId);

describe('backup-identity: native ownership rather than first label match', () => {
  it.each([false, true])('is independent of inventory order (reversed: %s)', (reversed) => {
    const workloads = [identityGuest('west-100', 'west', 'pve-10'), identityGuest('east-100', 'east', 'pve-1')];
    if (reversed) workloads.reverse();
    const result = model({ workloads, archives: [identityArchive()], snapshots: [identitySnapshot()], pbsBackups: [identityPBS()], tasks: [identityTask()] });
    expect(owners(result)).toEqual(['east-100', 'east-100', 'east-100']);
    expect(result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.latestTask).toBeUndefined();
    expect(result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id).toBe('task-100');
  });

  it('uses the PVE instance before a reused node name', () => {
    const result = model({
      workloads: [identityGuest('west-100', 'west', 'pve-1'), identityGuest('east-100', 'east', 'pve-1')],
      archives: [identityArchive()], snapshots: [identitySnapshot()], tasks: [identityTask()],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100']);
    expect(result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.latestTask).toBeUndefined();
  });

  it('keeps same-instance migrated backups without using the old node to select another cluster', () => {
    const result = model({
      workloads: [identityGuest('west-100', 'west', 'pve-1'), identityGuest('east-100', 'east', 'pve-2')],
      archives: [identityArchive()], tasks: [identityTask()],
    });
    expect(owners(result)).toEqual(['east-100']);
    expect(result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id).toBe('task-100');
  });

  it('does not let a matching node override a conflicting PVE instance, even with one visible guest', () => {
    const result = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      archives: [identityArchive({ instance: 'removed-east' })], snapshots: [identitySnapshot({ instance: 'removed-east' })],
      tasks: [identityTask({ instance: 'removed-east' })],
    });
    expect(owners(result)).toEqual([undefined, undefined]);
    const live = result.coverageRows.find((row) => row.workload.resourceId === 'east-100')!;
    expect(live.artifacts).toEqual([]);
    expect(live.latestTask).toBeUndefined();
    const orphan = result.coverageRows.find((row) => row.isOrphaned)!;
    expect(orphan.latestTask?.id).toBe('task-100');
    expect(orphan.posture).toBe('not-evaluated');
  });

  it('does not accept a partial node match or contradictory singleton when PVE instance is absent', () => {
    const result = model({
      workloads: [identityGuest('west-100', '', 'pve-10')],
      archives: [identityArchive({ instance: '' })], snapshots: [identitySnapshot({ instance: '' })], tasks: [identityTask({ instance: '' })],
    });
    expect(owners(result)).toEqual([undefined, undefined]);
    expect(result.coverageRows.find((row) => row.workload.resourceId)?.latestTask).toBeUndefined();
  });

  it('accepts exact native aliases, not mutable display labels', () => {
    const workloads = [identityGuest('west-100', 'west', 'pve-10', { nodeDisplayName: 'pve-1' }), identityGuest('east-100', 'east', 'pve-new', { nodeAliases: ['pve-1'] })];
    const result = model({ workloads, archives: [identityArchive({ instance: '' })], pbsBackups: [identityPBS()], tasks: [identityTask({ instance: '' })] });
    expect(owners(result)).toEqual(['east-100', 'east-100']);
    expect(result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id).toBe('task-100');
  });

  it('does not turn a reused namespace/node or alias into a unique PBS owner', () => {
    const result = model({ workloads: [identityGuest('west-100', 'west', 'shared', { nodeAliases: ['pve-1'] }), identityGuest('east-100', 'east', 'shared', { nodeAliases: ['pve-1'] })], pbsBackups: [identityPBS()] });
    expect(owners(result)).toEqual([undefined]);
    expect(result.coverageRows.filter((row) => row.workload.resourceId).every((row) => row.pbsCount === 0)).toBe(true);
  });

  it('does not use a datastore label as a PVE workload identity', () => {
    const result = model({ workloads: [identityGuest('west-100', 'west', 'main'), identityGuest('east-100', 'east', 'pve-1')], pbsBackups: [identityPBS({ namespace: '' })] });
    expect(owners(result)).toEqual([undefined]);
  });

  it('preserves root-namespace PBS for a unique VMID and exact instance namespace with collisions', () => {
    expect(owners(model({ workloads: [identityGuest('east-100', 'east', 'pve-1')], pbsBackups: [identityPBS({ namespace: '' })] }))).toEqual(['east-100']);
    expect(owners(model({ workloads: [identityGuest('west-100', 'west', 'pve-1'), identityGuest('east-100', 'east', 'pve-1')], pbsBackups: [identityPBS({ namespace: ' EAST ' })] }))).toEqual(['east-100']);
  });

  it('retains exact-node matching without instance metadata and the unscoped singleton control', () => {
    const workload = identityGuest('east-100', '', 'pve-1');
    expect(owners(model({ workloads: [workload], archives: [identityArchive({ instance: '' })] }))).toEqual(['east-100']);
    expect(owners(model({ workloads: [workload], archives: [identityArchive({ instance: '', node: '' })] }))).toEqual(['east-100']);
  });

  it('leaves an ambiguous unknown-type task unattached across VM and LXC identities', () => {
    const vm = identityGuest('vm-100', 'east', 'pve-1');
    const ct = { ...identityGuest('ct-100', 'east', 'pve-1'), type: 'system-container' } as typeof vm;
    const result = model({ workloads: [vm, ct], tasks: [identityTask()] });
    expect(result.coverageRows.every((row) => row.latestTask === undefined)).toBe(true);
  });

  it('preserves unresolved artifacts but does not pool identically named PBS sources', () => {
    const result = model({ pbsBackups: [identityPBS({ id: 'pbs-a', instance: 'pbs-a' }), identityPBS({ id: 'pbs-b', instance: 'pbs-b' })] });
    expect(result.recoverableArtifacts).toHaveLength(2);
    expect(result.coverageRows).toHaveLength(2);
    expect(new Set(result.coverageRows.map((row) => row.key)).size).toBe(2);
    expect(result.coverageRows.every((row) => row.pbsCount === 1 && row.posture === 'not-evaluated')).toBe(true);
  });

  it('does not attach a PVE task to a PBS-only orphan with a similar location label', () => {
    const result = model({ pbsBackups: [identityPBS()], tasks: [identityTask({ instance: '' })] });
    expect(result.coverageRows[0].latestTask).toBeUndefined();
  });

  it('keeps unresolved PVE instances separate while associating their own typed and untyped tasks', () => {
    const result = model({ archives: [identityArchive({ id: 'file-east' }), identityArchive({ id: 'file-west', instance: 'west' })], tasks: [identityTask(), identityTask({ id: 'task-west', instance: 'west', type: 'vm' })] });
    expect(result.coverageRows).toHaveLength(2);
    expect(result.coverageRows.map((row) => [row.workload.instance, row.latestTask?.id]).sort()).toEqual([['east', 'task-100'], ['west', 'task-west']]);
  });
});
