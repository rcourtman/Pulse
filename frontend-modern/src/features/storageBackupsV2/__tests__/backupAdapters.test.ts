import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { buildBackupRecordsV2 } from '@/features/storageBackupsV2/backupAdapters';

const baseState = (overrides: Partial<State> = {}): State =>
  ({
    nodes: [],
    vms: [],
    containers: [],
    dockerHosts: [],
    hosts: [],
    replicationJobs: [],
    storage: [],
    cephClusters: [],
    physicalDisks: [],
    pbs: [],
    pmg: [],
    pbsBackups: [],
    pmgBackups: [],
    backups: {
      pve: { backupTasks: [], storageBackups: [], guestSnapshots: [] },
      pbs: [],
      pmg: [],
    },
    metrics: [],
    pveBackups: { backupTasks: [], storageBackups: [], guestSnapshots: [] },
    performance: {
      apiCallDuration: {},
      lastPollDuration: 0,
      pollingStartTime: '',
      totalApiCalls: 0,
      failedApiCalls: 0,
      cacheHits: 0,
      cacheMisses: 0,
    },
    connectionHealth: {},
    stats: { startTime: '', uptime: 0, pollingCycles: 0, webSocketClients: 0, version: 'dev' },
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: '',
    resources: [],
    ...overrides,
  }) as State;

describe('backupAdapters', () => {
  it('normalizes PVE storage backups and filters non-backup artifacts', () => {
    const state = baseState({
      backups: {
        pve: {
          backupTasks: [],
          guestSnapshots: [],
          storageBackups: [
            {
              id: 'sb-1',
              storage: 'local',
              node: 'pve1',
              instance: 'pve-a',
              type: 'qemu',
              vmid: 101,
              time: '2024-01-01T00:00:00Z',
              ctime: 1_700_000_000,
              size: 1024,
              format: 'vma.zst',
              protected: true,
              volid: 'backup/vzdump-qemu-101.vma.zst',
              isPBS: false,
              verified: false,
              verification: 'failed',
              encryption: 'on',
            },
            {
              id: 'sb-2',
              storage: 'local',
              node: 'pve1',
              instance: 'pve-a',
              type: 'iso',
              vmid: 0,
              time: '2024-01-01T00:00:00Z',
              ctime: 1_700_000_000,
              size: 512,
              format: 'iso',
              protected: false,
              volid: 'iso/template.iso',
              isPBS: false,
              verified: false,
            },
          ],
        },
        pbs: [],
        pmg: [],
      },
    });

    const records = buildBackupRecordsV2({ state, resources: [] });
    expect(records).toHaveLength(1);

    const record = records[0];
    expect(record.category).toBe('vm-backup');
    expect(record.outcome).toBe('failed');
    expect(record.completedAt).toBe(1_700_000_000_000);
    expect(record.encrypted).toBe(true);
    expect(record.capabilities).toEqual(
      expect.arrayContaining(['retention', 'incremental', 'verification', 'encryption']),
    );
  });

  it('normalizes PBS and PMG records into source-agnostic backup records', () => {
    const state = baseState({
      backups: {
        pve: { backupTasks: [], storageBackups: [], guestSnapshots: [] },
        pbs: [
          {
            id: 'pbs-1',
            instance: 'pbs-a',
            datastore: 'primary',
            namespace: '',
            backupType: 'host',
            vmid: '0',
            backupTime: '2024-01-01T01:00:00Z',
            size: 2048,
            protected: true,
            verified: true,
            comment: 'host backup',
            files: ['index.fidx', 'blob.enc'],
            owner: 'root@pam',
          },
        ],
        pmg: [
          {
            id: 'pmg-1',
            instance: 'pmg-a',
            node: 'pmg-node-1',
            filename: 'backup.tar.zst',
            backupTime: '2024-01-01T02:00:00Z',
            size: 512,
          },
        ],
      },
    });

    const records = buildBackupRecordsV2({ state, resources: [] });
    expect(records).toHaveLength(2);

    const pbsRecord = records.find((record) => record.source.platform === 'proxmox-pbs');
    expect(pbsRecord).toBeTruthy();
    expect(pbsRecord?.category).toBe('host-backup');
    expect(pbsRecord?.outcome).toBe('success');
    expect(pbsRecord?.encrypted).toBe(true);
    expect(pbsRecord?.capabilities).toEqual(
      expect.arrayContaining(['retention', 'verification', 'immutability', 'encryption']),
    );

    const pmgRecord = records.find((record) => record.source.platform === 'proxmox-pmg');
    expect(pmgRecord).toBeTruthy();
    expect(pmgRecord?.category).toBe('config-backup');
    expect(pmgRecord?.outcome).toBe('success');
  });

  it('builds resource-origin backup records from unified resources with lastBackup metadata', () => {
    const state = baseState();

    const resources: Resource[] = [
      {
        id: 'k8s-pod-1',
        type: 'pod',
        name: 'api-pod',
        displayName: 'api-pod',
        platformId: 'cluster-a',
        platformType: 'kubernetes',
        sourceType: 'api',
        status: 'running',
        lastSeen: Date.now(),
        platformData: {
          namespace: 'default',
          nodeName: 'worker-1',
          lastBackup: '2024-02-01T01:00:00Z',
          backupMode: 'remote',
          verified: true,
        },
      },
    ];

    const records = buildBackupRecordsV2({ state, resources });
    expect(records).toHaveLength(1);
    expect(records[0].source.origin).toBe('resource');
    expect(records[0].source.platform).toBe('kubernetes');
    expect(records[0].scope.scope).toBe('workload');
    expect(records[0].category).toBe('container-backup');
    expect(records[0].completedAt).toBe(Date.parse('2024-02-01T01:00:00Z'));
    expect(records[0].details?.mode).toBe('remote');
  });

  it('does not duplicate Proxmox fallback resource records when legacy backup artifacts are available', () => {
    const state = baseState({
      backups: {
        pve: {
          backupTasks: [],
          guestSnapshots: [],
          storageBackups: [
            {
              id: 'sb-legacy',
              storage: 'local',
              node: 'pve1',
              instance: 'pve-a',
              type: 'qemu',
              vmid: 101,
              time: '2024-01-01T00:00:00Z',
              ctime: 1_700_000_000,
              size: 1024,
              format: 'vma.zst',
              protected: true,
              volid: 'backup/vzdump-qemu-101.vma.zst',
              isPBS: false,
              verified: true,
              verification: 'ok',
              encryption: 'on',
            },
          ],
        },
        pbs: [],
        pmg: [],
      },
    });

    const resources: Resource[] = [
      {
        id: 'vm-101',
        type: 'vm',
        name: 'vm-101',
        displayName: 'vm-101',
        platformId: 'pve-a',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'running',
        lastSeen: Date.now(),
        platformData: {
          vmid: 101,
          node: 'pve1',
          lastBackup: '2024-01-01T00:00:00Z',
          backupMode: 'local',
        },
      },
    ];

    const records = buildBackupRecordsV2({ state, resources });
    expect(records).toHaveLength(1);
    expect(records.every((record) => !record.id.startsWith('resource:'))).toBe(true);
  });
});
