import { describe, expect, it } from 'vitest';

import type { BackupTask, GuestSnapshot, PBSBackup, StorageBackup } from '@/types/api';
import type { ProtectionPosture, ProtectionState } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  getRecoveryAgeBand,
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

const protectionPosture = (resourceId: string, state: ProtectionState): ProtectionPosture => ({
  subjectResourceId: resourceId,
  state,
  freshness: state === 'protected' ? 'current' : 'unknown',
  verification: state === 'protected' ? 'verified' : 'unknown',
  coverage: state === 'unprotected' ? 'none' : state === 'unknown' ? 'unknown' : 'complete',
  providerStates: [],
  repositoryResourceIds: [],
  evidenceIds: [],
  explanation: `Canonical ${state} fixture`,
  evaluatedAt: '2026-05-26T08:00:00Z',
});

describe('proxmoxBackupRecoveryModel', () => {
  it('classifies backup ages for restore-point presentation', () => {
    const nowMs = Date.parse('2026-05-26T08:00:00Z');

    expect(getRecoveryAgeBand(nowMs - 2 * 24 * 60 * 60 * 1000, nowMs)).toBe('current');
    expect(getRecoveryAgeBand(nowMs - 14 * 24 * 60 * 60 * 1000, nowMs)).toBe('aging');
    expect(getRecoveryAgeBand(nowMs - 45 * 24 * 60 * 60 * 1000, nowMs)).toBe('stale');
    expect(getRecoveryAgeBand(undefined, nowMs)).toBe('unknown');
  });

  it('groups PBS, archive, snapshot, and task evidence under the resolved workload', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup()],
      archives: [archive()],
      snapshots: [snapshot()],
      tasks: [task()],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
      protectionPostures: new Map([['vm-112', protectionPosture('vm-112', 'protected')]]),
    });

    expect(model.coverageRows).toHaveLength(1);
    expect(model.recoverableArtifacts).toHaveLength(3);

    const row = model.coverageRows[0];
    expect(row.workload.label).toBe('pbs-docker (LXC 112)');
    expect(row.pbsCount).toBe(1);
    expect(row.archiveCount).toBe(1);
    expect(row.snapshotCount).toBe(1);
    expect(row.latestTask?.label).toBe('OK');
    expect(getWorkloadRecoveryPostureLabel(row.posture)).toBe('Protected');
    expect(model.recoverableArtifacts.map((artifact) => artifact.sourceLabel)).toEqual([
      'PBS',
      'PVE file',
      'Snapshot',
    ]);
    expect(
      model.recoverableArtifacts.map((artifact) => [artifact.locationKey, artifact.locationLabel]),
    ).toEqual([
      ['pbs:pbs-main:main', 'pbs-main / main'],
      ['archive:homelab:local', 'homelab / local'],
      ['snapshot:homelab:minipc', 'homelab / minipc'],
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

  it('shows the archive format as its detail and keeps the volid on hover and in search', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [],
      archives: [archive({ format: 'tar.zst' })],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const [artifact] = model.recoverableArtifacts;
    expect(artifact.detail).toBe('tar.zst');
    expect(artifact.detailTitle).toBe('local:backup/vzdump-lxc-112-2026_05_24-02_00_00.tar.zst');
    expect(recoverableArtifactMatchesSearch(artifact, 'vzdump-lxc-112')).toBe(true);
  });

  it('falls back to the archive file name when the provider reports no format', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [],
      archives: [archive({ format: '' })],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.recoverableArtifacts[0].detail).toBe('vzdump-lxc-112-2026_05_24-02_00_00.tar.zst');
  });

  it('keeps a terminally incomplete PBS artifact out of latest recovery', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [
        pbsBackup({
          id: 'failed-offsite-sync',
          backupTime: '2026-05-26T06:00:00Z',
          size: 0,
          verified: false,
          files: ['pct.conf.blob'],
          inProgress: true,
          writeActivityObserved: true,
          writeActive: false,
        }),
        pbsBackup({ backupTime: '2026-05-25T01:34:25Z' }),
      ],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const failed = model.recoverableArtifacts.find((artifact) => artifact.id.includes('failed'));
    expect(failed).toMatchObject({ failed: true, running: false, verified: undefined });
    expect(model.coverageRows[0].latestBackup?.createdAt).toBe('2026-05-25T01:34:25Z');
    expect(recoverableArtifactMatchesSearch(failed!, 'failed incomplete')).toBe(true);
  });

  it('keeps a fresh guest snapshot from standing in for the last backup', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup({ backupTime: '2026-05-05T01:34:25Z' })],
      archives: [],
      snapshots: [snapshot({ time: '2026-05-26T03:00:00Z' })],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const row = model.coverageRows[0];
    expect(row.latestSnapshot?.createdAt).toBe('2026-05-26T03:00:00Z');
    expect(row.latestBackup?.sourceKind).toBe('pbs');
    expect(row.latestBackup?.createdAt).toBe('2026-05-05T01:34:25Z');
    expect(
      getRecoveryAgeBand(
        row.latestBackup?.createdMs,
        model.coverageRows.length && Date.parse('2026-05-26T08:00:00Z'),
      ),
    ).toBe('aging');
  });

  it('reports no last backup for a snapshot-only workload', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [],
      archives: [],
      snapshots: [snapshot()],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    const row = model.coverageRows[0];
    expect(row.latestBackup).toBeUndefined();
    expect(row.latestSnapshot?.sourceKind).toBe('snapshot');
    expect(row.artifacts).toHaveLength(1);
    expect(model.coverageSummary.recoverableArtifacts).toBe(1);
  });

  it('uses canonical workload attention while retaining the failed task as evidence', () => {
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
      protectionPostures: new Map([['vm-112', protectionPosture('vm-112', 'attention')]]),
    });

    const row = model.coverageRows[0];
    expect(row.posture).toBe('attention');
    expect(getWorkloadRecoveryPostureLabel(row.posture)).toBe('Needs attention');
    expect(coverageRowMatchesSearch(row, 'storage unavailable')).toBe(true);
  });

  it('keeps an inventory workload with canonical unprotected evidence visible', () => {
    const resource = workload({ id: 'vm-200', proxmox: { vmid: 200, node: 'delly' } });
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [resource],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
      protectionPostures: new Map([[resource.id, protectionPosture(resource.id, 'unprotected')]]),
    });

    expect(model.coverageRows).toHaveLength(1);
    expect(model.coverageRows[0].posture).toBe('unprotected');
    expect(model.coverageSummary.unprotected).toBe(1);
  });

  it('does not let one successful guest hide missing coverage or a failed sibling', () => {
    const resources = [112, 200, 201, 202].map((vmid) =>
      workload({ id: `vm-${vmid}`, proxmox: { vmid, node: 'minipc', instance: 'homelab' } }),
    );
    const model = buildProxmoxBackupRecoveryModel({
      workloads: resources,
      pbsBackups: [pbsBackup()],
      archives: [],
      snapshots: [],
      tasks: [
        task(),
        task({ id: 'task-201', vmid: 201, status: 'failed', error: 'storage unavailable' }),
        // Task success alone is not canonical protection evidence.
        task({ id: 'task-202', vmid: 202 }),
      ],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
      protectionPosturesResolved: true,
      protectionPostures: new Map([
        ['vm-112', protectionPosture('vm-112', 'protected')],
        ['vm-200', protectionPosture('vm-200', 'unprotected')],
        ['vm-201', protectionPosture('vm-201', 'attention')],
      ]),
    });

    expect(model.coverageSummary).toMatchObject({
      totalWorkloads: 4,
      protected: 1,
      unprotected: 1,
      attention: 1,
      unknown: 1,
    });
    const rows = new Map(model.coverageRows.map((row) => [row.workload.resourceId, row]));
    expect(rows.get('vm-200')).toMatchObject({ artifacts: [], posture: 'unprotected' });
    expect(rows.get('vm-201')?.latestTask?.error).toBe('storage unavailable');
    expect(rows.get('vm-202')).toMatchObject({ posture: 'unknown', latestTask: { status: 'OK' } });
  });

  it('distinguishes an in-flight posture read from a completed unknown evaluation', () => {
    const resource = workload({ id: 'vm-202', proxmox: { vmid: 202, node: 'delly' } });
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [resource],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
      protectionPosturesResolved: false,
    });

    expect(model.coverageRows[0].posture).toBe('checking');
    expect(getWorkloadRecoveryPostureLabel(model.coverageRows[0].posture)).toBe('Checking');
    expect(model.coverageSummary.unknown).toBe(0);
  });

  it('does not treat a linked Pulse agent facet as recovery evidence or authority', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [
        workload({
          id: 'vm-201',
          sourceType: 'hybrid',
          proxmox: { vmid: 201, node: 'delly', instance: 'homelab' },
          agent: {
            agentId: 'agent-delly',
            agentVersion: '6.0.0-rc.5',
            commandsEnabled: true,
          },
        }),
      ],
      pbsBackups: [],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: Date.parse('2026-05-26T08:00:00Z'),
    });

    expect(model.coverageRows).toHaveLength(1);
    expect(model.coverageRows[0].posture).toBe('unknown');
    expect(model.coverageSummary.unknown).toBe(1);
    expect(model.coverageRows[0].pbsCount).toBe(0);
    expect(model.coverageRows[0].archiveCount).toBe(0);
    expect(model.coverageRows[0].snapshotCount).toBe(0);
    expect(model.recoverableArtifacts).toHaveLength(0);
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
    expect(model.coverageRows[0].posture).toBe('not-evaluated');
    expect(getWorkloadRecoveryPostureLabel(model.coverageRows[0].posture)).toBe('Not evaluated');
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

describe('backup-date-evidence model', () => {
  const nowMs = Date.parse('2026-10-04T12:00:00Z');
  const invalidDates = ['', 'not-a-date', '0001-01-01T00:00:00Z', '2026-10-04T12:00:01Z'];
  const modelFor = (kind: 'pbs' | 'archive' | 'snapshot', date: string) =>
    buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: kind === 'pbs' ? [pbsBackup({ backupTime: date })] : [],
      archives: kind === 'archive' ? [archive({ time: date })] : [],
      snapshots: kind === 'snapshot' ? [snapshot({ time: date })] : [],
      tasks: [],
      nowMs,
      protectionPostures: new Map([['vm-112', protectionPosture('vm-112', 'protected')]]),
    });

  for (const kind of ['pbs', 'archive', 'snapshot'] as const) {
    it.each(invalidDates)('[regression] keeps %s date uncertainty in ' + kind, (date) => {
      const model = modelFor(kind, date);
      const row = model.coverageRows[0];
      expect(model.recoverableArtifacts).toHaveLength(1);
      expect(model.recoverableArtifacts[0].createdAt).toBe(date);
      expect(model.recoverableArtifacts[0].createdMs).toBeUndefined();
      expect(row.ageUnknown?.[kind]).toBe(true);
      expect(row.ageUnknown?.backup).toBe(kind !== 'snapshot');
      expect(row.latestBackup).toBeUndefined();
      expect(row.posture).toBe('protected'); // server posture is a separate fact
    });
  }

  it('[regression] cannot call a dated older backup the latest beside an undated completion', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup({ backupTime: 'not-a-date' })],
      archives: [archive()],
      snapshots: [],
      tasks: [],
      nowMs,
    });
    expect(model.coverageRows[0].latestBackup).toBeUndefined();
    expect(model.coverageRows[0].latestArchive?.nativeId).toBe('archive-112');
    expect(model.coverageRows[0].ageUnknown?.backup).toBe(true);
  });

  it.each([nowMs + 1, 0, -1, 9e15])(
    '[regression] rejects unorderable age %s rather than current/stale',
    (createdMs) => expect(getRecoveryAgeBand(createdMs, nowMs)).toBe('unknown'),
  );

  it('[control] keeps valid newest completed backup and guest-local snapshot separate', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup()],
      archives: [archive()],
      snapshots: [snapshot({ time: '2026-10-04T11:00:00Z' })],
      tasks: [],
      nowMs,
    });
    expect(model.coverageRows[0].latestBackup?.sourceKind).toBe('pbs');
    expect(model.coverageRows[0].latestSnapshot?.sourceKind).toBe('snapshot');
  });

  it('[control] incomplete unknown-date artifacts do not contaminate completed backup chronology', () => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [workload({})],
      pbsBackups: [
        pbsBackup({ id: 'running', inProgress: true, backupTime: 'not-a-date' }),
        pbsBackup({
          id: 'failed',
          inProgress: true,
          writeActivityObserved: true,
          writeActive: false,
          backupTime: '',
        }),
      ],
      archives: [archive()],
      snapshots: [],
      tasks: [],
      nowMs,
    });
    expect(model.coverageRows[0].latestBackup?.nativeId).toBe('archive-112');
    expect(model.recoverableArtifacts).toHaveLength(3);
    expect(
      model.recoverableArtifacts.find((artifact) => artifact.nativeId === 'running')?.running,
    ).toBe(true);
    expect(
      model.recoverableArtifacts.find((artifact) => artifact.nativeId === 'failed')?.failed,
    ).toBe(true);
  });

  it('[control] keeps explicit absence and snapshot-only inventory distinct', () => {
    const row = modelFor('snapshot', '2026-10-04T11:00:00Z').coverageRows[0];
    expect(row.latestBackup).toBeUndefined();
    expect(row.pbsCount + row.archiveCount).toBe(0);
    expect(row.latestSnapshot).toBeDefined();
  });

  it('[control] preserves age-band boundaries and an exactly current completion', () => {
    expect(getRecoveryAgeBand(Infinity, nowMs)).toBe('unknown');
    expect(getRecoveryAgeBand(NaN, nowMs)).toBe('unknown');
    expect(getRecoveryAgeBand(nowMs, NaN)).toBe('unknown');
    expect(getRecoveryAgeBand(nowMs, nowMs)).toBe('current');
    expect(getRecoveryAgeBand(nowMs - 7 * 86400000, nowMs)).toBe('current');
    expect(getRecoveryAgeBand(nowMs - 7 * 86400000 - 1, nowMs)).toBe('aging');
    expect(getRecoveryAgeBand(nowMs - 30 * 86400000, nowMs)).toBe('aging');
    expect(getRecoveryAgeBand(nowMs - 30 * 86400000 - 1, nowMs)).toBe('stale');
  });
});
