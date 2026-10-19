import { describe, expect, it } from 'vitest';

import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
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
const owners = (result: ReturnType<typeof model>) =>
  result.recoverableArtifacts.map((artifact) => artifact.workload.resourceId);

describe('backup-identity: native ownership rather than first label match', () => {
  it.each([false, true])(
    'keeps case-distinct connections separate, including migrated evidence (reversed: %s)',
    (reversed) => {
      const workloads = [
        identityGuest('upper-100', 'East', 'pve-upper'),
        identityGuest('lower-100', 'east', 'pve-lower'),
      ];
      if (reversed) workloads.reverse();
      const result = model({
        workloads,
        archives: [identityArchive({ instance: 'East', node: 'pve-lower' })],
        snapshots: [identitySnapshot({ instance: 'east', node: 'pve-upper' })],
        pbsBackups: [
          identityPBS({ id: 'pbs-upper', namespace: 'East' }),
          identityPBS({ id: 'pbs-lower', namespace: 'east' }),
        ],
        tasks: [
          identityTask({ id: 'upper-job', instance: 'East', node: 'pve-lower' }),
          identityTask({ id: 'lower-job', instance: 'east', node: 'pve-upper', type: 'vm' }),
        ],
      });
      const owner = (id: string) =>
        result.recoverableArtifacts.find((artifact) => artifact.nativeId === id)?.workload
          .resourceId;
      expect(owner('file-100')).toBe('upper-100');
      expect(owner('snapshot-100')).toBe('lower-100');
      expect(owner('pbs-upper')).toBe('upper-100');
      expect(owner('pbs-lower')).toBe('lower-100');
      expect(result.coverageRows).toHaveLength(2);
      expect(
        result.coverageRows.find((row) => row.workload.resourceId === 'upper-100')?.latestTask?.id,
      ).toBe('upper-job');
      expect(
        result.coverageRows.find((row) => row.workload.resourceId === 'lower-100')?.latestTask?.id,
      ).toBe('lower-job');
    },
  );

  it('does not borrow a singleton guest from a case-distinct removed connection', () => {
    const result = model({
      workloads: [identityGuest('lower-100', 'east', 'pve-1')],
      archives: [identityArchive({ instance: 'East' })],
      snapshots: [identitySnapshot({ instance: 'East' })],
      tasks: [identityTask({ instance: 'East' })],
    });
    expect(owners(result)).toEqual([undefined, undefined]);
    const live = result.coverageRows.find((row) => row.workload.resourceId === 'lower-100')!;
    expect(live.artifacts).toEqual([]);
    expect(live.latestTask).toBeUndefined();
    const orphan = result.coverageRows.find((row) => row.isOrphaned)!;
    expect(orphan.workload.instance).toBe('East');
    expect(orphan.latestTask?.id).toBe('task-100');
    expect(orphan.posture).toBe('not-evaluated');
  });

  it('keeps untyped orphan jobs with their exact source rather than a case-distinct archive', () => {
    const result = model({
      archives: [
        identityArchive({ id: 'upper-file', instance: 'East' }),
        identityArchive({ id: 'lower-file', instance: 'east' }),
      ],
      tasks: [
        identityTask({ id: 'upper-job', instance: 'East' }),
        identityTask({ id: 'lower-job', instance: 'east' }),
      ],
    });
    expect(result.coverageRows).toHaveLength(2);
    expect(
      result.coverageRows.map((row) => [row.workload.instance, row.latestTask?.id]).sort(),
    ).toEqual([
      ['East', 'upper-job'],
      ['east', 'lower-job'],
    ]);
  });

  it('retains hostname case matching without treating connection names as hostnames', () => {
    const result = model({
      workloads: [
        identityGuest('upper-100', 'East', 'pve-upper'),
        identityGuest('lower-100', 'east', 'pve-lower'),
      ],
      archives: [identityArchive({ instance: '', node: 'PVE-UPPER' })],
      pbsBackups: [identityPBS({ namespace: 'PVE-LOWER' })],
      tasks: [identityTask({ instance: '', node: 'PVE-UPPER' })],
    });
    expect(owners(result)).toEqual(['upper-100', 'lower-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'upper-100')?.latestTask?.id,
    ).toBe('task-100');
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'lower-100')?.latestTask,
    ).toBeUndefined();
  });

  it('keeps copied local archives and their location filters independent of guest placement', () => {
    const result = model({
      workloads: [identityGuest('east-100', 'east', 'current-guest-node')],
      archives: [
        identityArchive({ id: 'copy-a', node: 'current-guest-node', storageNode: 'node-a' }),
        identityArchive({ id: 'copy-b', node: 'current-guest-node', storageNode: 'node-b' }),
        identityArchive({ id: 'shared', node: 'current-guest-node' }),
      ],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100', 'east-100']);
    expect(result.coverageRows).toHaveLength(1);
    expect(result.coverageRows[0].archiveCount).toBe(3);
    expect(result.recoverableArtifacts.map((artifact) => artifact.location)).toEqual([
      'local / node-a',
      'local / node-b',
      'local',
    ]);
    expect(result.recoverableArtifacts.map((artifact) => artifact.locationLabel)).toEqual([
      'east / local / node-a',
      'east / local / node-b',
      'east / local',
    ]);
    expect(new Set(result.recoverableArtifacts.map((artifact) => artifact.locationKey)).size).toBe(
      3,
    );
    expect(result.recoverableArtifacts[0].locationGroupKey).toBe(
      result.recoverableArtifacts[2].locationKey,
    );
    expect(result.recoverableArtifacts[1].locationGroupKey).toBe(
      result.recoverableArtifacts[2].locationKey,
    );
  });

  it('does not discard a storage-node identity because its label equals the connection', () => {
    const result = model({
      archives: [
        identityArchive({ id: 'local', storageNode: 'east' }),
        identityArchive({ id: 'shared' }),
      ],
    });
    expect(new Set(result.recoverableArtifacts.map((artifact) => artifact.locationKey)).size).toBe(
      2,
    );
    expect(result.recoverableArtifacts.map((artifact) => artifact.locationLabel)).toEqual([
      'east / local / east',
      'east / local',
    ]);
  });

  it.each([false, true])('is independent of inventory order (reversed: %s)', (reversed) => {
    const workloads = [
      identityGuest('west-100', 'west', 'pve-10'),
      identityGuest('east-100', 'east', 'pve-1'),
    ];
    if (reversed) workloads.reverse();
    const result = model({
      workloads,
      archives: [identityArchive()],
      snapshots: [identitySnapshot()],
      pbsBackups: [identityPBS()],
      tasks: [identityTask()],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100', 'east-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.latestTask,
    ).toBeUndefined();
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id,
    ).toBe('task-100');
  });

  it('uses the PVE instance before a reused node name', () => {
    const result = model({
      workloads: [
        identityGuest('west-100', 'west', 'pve-1'),
        identityGuest('east-100', 'east', 'pve-1'),
      ],
      archives: [identityArchive()],
      snapshots: [identitySnapshot()],
      tasks: [identityTask()],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.latestTask,
    ).toBeUndefined();
  });

  it('keeps same-instance migrated backups without using the old node to select another cluster', () => {
    const result = model({
      workloads: [
        identityGuest('west-100', 'west', 'pve-1'),
        identityGuest('east-100', 'east', 'pve-2'),
      ],
      archives: [identityArchive()],
      tasks: [identityTask()],
    });
    expect(owners(result)).toEqual(['east-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id,
    ).toBe('task-100');
  });

  it('keeps the warning job origin searchable after a migration, without changing ownership or posture', () => {
    const task = identityTask({
      node: 'old-backup-node',
      status: 'warning',
      error: 'Backup completed with warnings; review the original Proxmox task log.',
    });
    const original = structuredClone(task);
    const result = model({
      workloads: [
        identityGuest('west-100', 'west', 'old-backup-node'),
        identityGuest('east-100', 'east', 'new-guest-node'),
      ],
      tasks: [task],
    });
    const row = result.coverageRows.find((entry) => entry.workload.resourceId === 'east-100')!;
    expect(row.workload.node).toBe('new-guest-node');
    expect(row.latestTask).toMatchObject({
      instance: 'east',
      node: 'old-backup-node',
      label: 'Warning',
      status: 'warning',
      error: task.error,
      statusErrorConflict: false,
    });
    expect(coverageRowMatchesSearch(row, 'old-backup-node')).toBe(true);
    expect(coverageRowMatchesSearch(row, 'new-guest-node')).toBe(true);
    expect(
      result.coverageRows.find((entry) => entry.workload.resourceId === 'west-100')?.latestTask,
    ).toBeUndefined();
    expect(row.posture).toBe('unknown');
    expect(result.coverageSummary.recoverableArtifacts).toBe(0);
    expect(task).toEqual(original);
  });

  it('does not borrow a matched guest location when the job does not report one', () => {
    const result = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      tasks: [identityTask({ instance: '', node: 'pve-1' })],
    });
    expect(result.coverageRows[0].latestTask).toMatchObject({ instance: '', node: 'pve-1' });
    const missingNode = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      tasks: [identityTask({ node: '' })],
    });
    expect(missingNode.coverageRows[0].latestTask).toMatchObject({ instance: 'east', node: '' });
  });

  it('does not let a matching node override a conflicting PVE instance, even with one visible guest', () => {
    const result = model({
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      archives: [identityArchive({ instance: 'removed-east' })],
      snapshots: [identitySnapshot({ instance: 'removed-east' })],
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
      archives: [identityArchive({ instance: '' })],
      snapshots: [identitySnapshot({ instance: '' })],
      tasks: [identityTask({ instance: '' })],
    });
    expect(owners(result)).toEqual([undefined, undefined]);
    expect(result.coverageRows.find((row) => row.workload.resourceId)?.latestTask).toBeUndefined();
  });

  it('accepts exact native aliases, not mutable display labels', () => {
    const workloads = [
      identityGuest('west-100', 'west', 'pve-10', { nodeDisplayName: 'pve-1' }),
      identityGuest('east-100', 'east', 'pve-new', { nodeAliases: ['pve-1'] }),
    ];
    const result = model({
      workloads,
      archives: [identityArchive({ instance: '' })],
      pbsBackups: [identityPBS()],
      tasks: [identityTask({ instance: '' })],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'east-100')?.latestTask?.id,
    ).toBe('task-100');
  });

  it('keeps presentation-only parent labels out of native-node matching', () => {
    const misleading = { ...identityGuest('west-100', '', ''), parentName: 'pve-1' };
    const result = model({
      workloads: [misleading, identityGuest('east-100', '', 'pve-1')],
      archives: [identityArchive({ instance: '' })],
      pbsBackups: [identityPBS()],
    });
    expect(owners(result)).toEqual(['east-100', 'east-100']);
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.workload.node,
    ).toBe('pve-1');
    expect(
      result.coverageRows.find((row) => row.workload.resourceId === 'west-100')?.workload
        .nativeNode,
    ).toBeUndefined();
  });

  it('does not collide unresolved repository keys on case or delimiter framing', () => {
    const result = model({
      pbsBackups: [
        identityPBS({ id: 'case-a', instance: 'pbs', datastore: 'main' }),
        identityPBS({ id: 'case-b', instance: 'pbs', datastore: 'Main' }),
        identityPBS({ id: 'parts-a', instance: 'a:b', datastore: 'c' }),
        identityPBS({ id: 'parts-b', instance: 'a', datastore: 'b:c' }),
      ],
    });
    expect(result.coverageRows).toHaveLength(4);
    expect(new Set(result.coverageRows.map((row) => row.key)).size).toBe(4);
  });

  it('keeps source names with spaces or quotes in an IDREF-safe unresolved key', () => {
    const result = model({
      pbsBackups: [
        identityPBS({
          instance: 'backup server',
          datastore: 'main "store"',
          namespace: 'old node',
        }),
      ],
    });
    const key = result.coverageRows[0].key;
    expect(key).not.toMatch(/[\s"']/);
    expect(decodeURIComponent(key)).toContain('backup server');
  });

  it('does not turn a reused namespace/node or alias into a unique PBS owner', () => {
    const result = model({
      workloads: [
        identityGuest('west-100', 'west', 'shared', { nodeAliases: ['pve-1'] }),
        identityGuest('east-100', 'east', 'shared', { nodeAliases: ['pve-1'] }),
      ],
      pbsBackups: [identityPBS()],
    });
    expect(owners(result)).toEqual([undefined]);
    expect(
      result.coverageRows
        .filter((row) => row.workload.resourceId)
        .every((row) => row.pbsCount === 0),
    ).toBe(true);
  });

  it('does not use a datastore label as a PVE workload identity', () => {
    const result = model({
      workloads: [
        identityGuest('west-100', 'west', 'main'),
        identityGuest('east-100', 'east', 'pve-1'),
      ],
      pbsBackups: [identityPBS({ namespace: '' })],
    });
    expect(owners(result)).toEqual([undefined]);
  });

  it('preserves root-namespace PBS for a unique VMID and exact instance namespace with collisions', () => {
    expect(
      owners(
        model({
          workloads: [identityGuest('east-100', 'east', 'pve-1')],
          pbsBackups: [identityPBS({ namespace: '' })],
        }),
      ),
    ).toEqual(['east-100']);
    expect(
      owners(
        model({
          workloads: [
            identityGuest('west-100', 'west', 'pve-1'),
            identityGuest('east-100', 'east', 'pve-1'),
          ],
          pbsBackups: [identityPBS({ namespace: 'east' })],
        }),
      ),
    ).toEqual(['east-100']);
  });

  it('retains exact-node matching without instance metadata and the unscoped singleton control', () => {
    const workload = identityGuest('east-100', '', 'pve-1');
    expect(
      owners(model({ workloads: [workload], archives: [identityArchive({ instance: '' })] })),
    ).toEqual(['east-100']);
    expect(
      owners(
        model({ workloads: [workload], archives: [identityArchive({ instance: '', node: '' })] }),
      ),
    ).toEqual(['east-100']);
  });

  it('leaves an ambiguous unknown-type task unattached across VM and LXC identities', () => {
    const vm = identityGuest('vm-100', 'east', 'pve-1');
    const ct = {
      ...identityGuest('ct-100', 'east', 'pve-1'),
      type: 'system-container',
    } as typeof vm;
    const result = model({ workloads: [vm, ct], tasks: [identityTask()] });
    expect(
      result.coverageRows.filter((row) => row.workload.resourceId).every((row) => !row.latestTask),
    ).toBe(true);
    expect(result.coverageRows.find((row) => row.isOrphaned)?.latestTask?.id).toBe('task-100');
  });

  it('preserves unresolved artifacts but does not pool identically named PBS sources', () => {
    const result = model({
      pbsBackups: [
        identityPBS({ id: 'pbs-a', instance: 'pbs-a' }),
        identityPBS({ id: 'pbs-b', instance: 'pbs-b' }),
      ],
    });
    expect(result.recoverableArtifacts).toHaveLength(2);
    expect(result.coverageRows).toHaveLength(2);
    expect(new Set(result.coverageRows.map((row) => row.key)).size).toBe(2);
    expect(
      result.coverageRows.every((row) => row.pbsCount === 1 && row.posture === 'not-evaluated'),
    ).toBe(true);
  });

  it('does not attach a PVE task to a PBS-only orphan with a similar location label', () => {
    const result = model({ pbsBackups: [identityPBS()], tasks: [identityTask({ instance: '' })] });
    expect(result.coverageRows[0].latestTask).toBeUndefined();
  });

  it('keeps unresolved PVE instances separate while associating their own typed and untyped tasks', () => {
    const result = model({
      archives: [
        identityArchive({ id: 'file-east' }),
        identityArchive({ id: 'file-west', instance: 'west' }),
      ],
      tasks: [identityTask(), identityTask({ id: 'task-west', instance: 'west', type: 'vm' })],
    });
    expect(result.coverageRows).toHaveLength(2);
    expect(
      result.coverageRows.map((row) => [row.workload.instance, row.latestTask?.id]).sort(),
    ).toEqual([
      ['east', 'task-100'],
      ['west', 'task-west'],
    ]);
  });
});
