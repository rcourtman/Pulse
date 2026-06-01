import { describe, expect, it } from 'vitest';

import type { BackupTask, GuestSnapshot, PBSBackup, StorageBackup } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  getWorkloadRecoveryPostureLabel,
  recoverableArtifactMatchesSearch,
} from '../proxmoxBackupRecoveryModel';

const workload = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'vm-112',
    type: 'system-container',
    name: 'pbs-docker',
    displayName: 'pbs-docker',
    platformId: 'pve-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: Date.parse('2026-05-26T00:00:00Z'),
    proxmox: { vmid: 112, node: 'minipc', instance: 'homelab' },
    ...overrides,
  }) as Resource;

const pbsBackup = (overrides: Partial<PBSBackup> = {}): PBSBackup => ({
  id: 'pbs-main/main/minipc/ct/112/2026-05-25T01:34:25Z',
  instance: 'pbs-main',
  datastore: 'main',
  namespace: 'minipc',
  backupType: 'ct',
  vmid: '112',
  backupTime: '2026-05-25T01:34:25Z',
  size: 8_589_934_592,
  protected: false,
  verified: true,
  files: ['index.json.blob', 'root.pxar.didx'],
  owner: 'backup@pbs',
  ...overrides,
});

const archive = (overrides: Partial<StorageBackup> = {}): StorageBackup => ({
  id: 'archive-112',
  storage: 'local',
  node: 'minipc',
  instance: 'homelab',
  type: 'ct',
  vmid: 112,
  time: '2026-05-24T02:00:00Z',
  ctime: 1_769_390_400,
  size: 1_048_576,
  format: 'zst',
  protected: false,
  volid: 'local:backup/vzdump-lxc-112-2026_05_24-02_00_00.tar.zst',
  isPBS: false,
  verified: false,
  ...overrides,
});

const snapshot = (overrides: Partial<GuestSnapshot> = {}): GuestSnapshot => ({
  id: 'snap-112',
  name: 'pre-upgrade',
  node: 'minipc',
  instance: 'homelab',
  type: 'ct',
  vmid: 112,
  time: '2026-05-23T03:00:00Z',
  vmstate: false,
  ...overrides,
});

const task = (overrides: Partial<BackupTask> = {}): BackupTask => ({
  id: 'task-112',
  node: 'minipc',
  instance: 'homelab',
  type: 'vzdump',
  vmid: 112,
  status: 'OK',
  startTime: '2026-05-25T02:00:00Z',
  endTime: '2026-05-25T02:05:00Z',
  ...overrides,
});

describe('proxmoxBackupRecoveryModel', () => {
  it('groups PBS, archive, snapshot, and task evidence under the resolved workload', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup()],
      archives: [archive()],
      snapshots: [snapshot()],
      tasks: [task()],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.coverageRows).toHaveLength(1);
    expect(model.recoverableArtifacts).toHaveLength(3);

    const row = model.coverageRows[0];
    expect(row.workload.label).toBe('pbs-docker (LXC 112)');
    expect(row.pbsCount).toBe(1);
    expect(row.archiveCount).toBe(1);
    expect(row.snapshotCount).toBe(1);
    expect(row.latestTask?.label).toBe('OK');
    expect(getWorkloadRecoveryPostureLabel(row.posture)).toBe('Current');
    expect(model.recoverableArtifacts.map((artifact) => artifact.sourceLabel)).toEqual([
      'PBS',
      'PVE file',
      'Snapshot',
    ]);
    expect(model.recoverableArtifacts[0].detail).toBe('2 PBS files');
    expect(coverageRowMatchesSearch(row, 'pbs-docker')).toBe(true);
    expect(coverageRowMatchesSearch(row, 'PVE backup file')).toBe(true);
    expect(recoverableArtifactMatchesSearch(model.recoverableArtifacts[0], 'main')).toBe(true);
  });

  it('does not describe PBS backups with omitted file manifests as zero-file backups', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup({ files: [] })],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.recoverableArtifacts[0].detail).toBe('PBS files not listed');
    expect(recoverableArtifactMatchesSearch(model.recoverableArtifacts[0], 'not listed')).toBe(
      true,
    );
  });

  it('surfaces a failed latest backup task as workload attention', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup({ backupTime: '2026-05-25T01:34:25Z' })],
      archives: [],
      snapshots: [],
      tasks: [
        task({
          status: 'failed',
          startTime: '2026-05-26T01:00:00Z',
          endTime: '2026-05-26T01:01:00Z',
          error: 'storage unavailable',
        }),
      ],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const row = model.coverageRows[0];
    expect(row.posture).toBe('failed');
    expect(getWorkloadRecoveryPostureLabel(row.posture)).toBe('Failed latest task');
    expect(coverageRowMatchesSearch(row, 'storage unavailable')).toBe(true);
  });

  it('keeps inventory workloads with no restore point visible as uncovered', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({ id: 'vm-200', proxmox: { vmid: 200, node: 'delly' } })],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.coverageRows).toHaveLength(1);
    expect(model.coverageRows[0].posture).toBe('uncovered');
    expect(model.coverageSummary.uncovered).toBe(1);
  });

  it('flags backups whose guest is absent from inventory as orphaned, live guests as not', () => {
    const model = buildProxmoxBackupRecoveryModel({
      // Only VMID 112 exists in inventory; the PBS backup for 999 does not.
      workloads: [workload({})],
      pbsBackups: [
        pbsBackup(),
        pbsBackup({
          id: 'pbs-main/main/minipc/ct/999/2026-05-25T01:34:25Z',
          vmid: '999',
        }),
      ],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const live = model.coverageRows.find((row) => row.workload.vmid === '112');
    const orphan = model.coverageRows.find((row) => row.workload.vmid === '999');

    expect(live?.isOrphaned).toBe(false);
    expect(live?.workload.name).toBe('pbs-docker');
    expect(orphan?.isOrphaned).toBe(true);
    // Orphans have no live guest to name them; label falls back to "LXC <vmid>".
    expect(orphan?.workload.name).toBeUndefined();
    expect(orphan?.workload.label).toBe('LXC 999');
  });

  it('keeps host backups in coverage without counting zero-id aggregate artifacts as guests', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [],
      pbsBackups: [
        pbsBackup({
          id: 'pbs-main/main/root/ct/0/2026-05-25T01:34:25Z',
          namespace: 'root',
          vmid: '0',
        }),
        pbsBackup({
          id: 'pbs-main/main/root/host/mail-gateway/2026-05-25T01:34:25Z',
          namespace: 'root',
          backupType: 'host',
          vmid: 'mail-gateway',
        }),
      ],
      archives: [
        archive({
          id: 'host-archive-mail-gateway',
          type: 'host',
          vmid: 0,
          instance: 'mail-gateway',
          node: 'pmg-01',
        }),
      ],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.recoverableArtifacts).toHaveLength(3);
    expect(model.coverageRows).toHaveLength(1);
    expect(model.coverageSummary.totalWorkloads).toBe(1);
    expect(model.coverageRows[0].isOrphaned).toBe(false);
    expect(model.coverageRows[0].workload.type).toBe('host');
    expect(model.coverageRows[0].workload.vmid).toBe('mail-gateway');
    expect(model.coverageRows[0].workload.label).toBe('Host mail-gateway');
    expect(model.coverageRows[0].pbsCount).toBe(1);
    expect(model.coverageRows[0].archiveCount).toBe(1);
    expect(model.recoverableArtifacts.map((artifact) => artifact.workload.label)).toEqual(
      expect.arrayContaining(['LXC backup', 'Host mail-gateway']),
    );
  });

  it('does not create phantom workload rows from aggregate vzdump jobs', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      tasks: [
        task({
          id: 'aggregate-vzdump',
          type: 'vzdump',
          vmid: 0,
          startTime: '2026-05-26T04:00:00Z',
          endTime: '2026-05-26T04:12:00Z',
        }),
        task({
          id: 'orphan-vzdump',
          type: 'vzdump',
          vmid: 105,
          startTime: '2026-05-20T04:00:00Z',
          endTime: '2026-05-20T04:12:00Z',
        }),
      ],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.coverageRows).toHaveLength(0);
    expect(model.coverageSummary.totalWorkloads).toBe(0);
  });
});
