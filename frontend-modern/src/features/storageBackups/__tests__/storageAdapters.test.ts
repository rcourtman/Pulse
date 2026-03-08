import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { buildStorageRecords } from '@/features/storageBackups/storageAdapters';

const baseState = (overrides: Partial<State> = {}): State =>
  ({
    removedDockerHosts: [],
    removedKubernetesClusters: [],
    metrics: [],
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

const makeResourceStorage = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-storage-1',
    type: 'storage',
    name: 'local-zfs',
    displayName: 'local-zfs',
    platformId: 'cluster-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    disk: { current: 40, total: 1000, used: 400, free: 600 },
    lastSeen: 1731000000000,
    platformData: {
      type: 'zfspool',
      node: 'pve1',
      instance: 'cluster-a',
      shared: false,
    },
    ...overrides,
  }) as Resource;

describe('storageAdapters', () => {
  it('returns no records when unified resources are absent', () => {
    const state = baseState();
    const records = buildStorageRecords({ state, resources: [] });
    expect(records).toHaveLength(0);
  });

  it('prefers enriched storage metadata over platformData inference', () => {
    const enriched = {
      ...makeResourceStorage({
        platformData: {
          type: 'dir',
          node: 'pve1',
          instance: 'cluster-a',
          content: 'images',
          shared: true,
        },
      }),
      storage: {
        type: 'rbd',
        content: '',
        contentTypes: ['images', 'rootdir'],
        shared: false,
        isCeph: true,
        isZfs: false,
      },
    } as Resource;

    const records = buildStorageRecords({ state: baseState(), resources: [enriched] });

    expect(records).toHaveLength(1);
    expect(records[0].details?.type).toBe('rbd');
    expect(records[0].details?.content).toBe('images,rootdir');
    expect(records[0].details?.shared).toBe(false);
    expect(records[0].details?.isCeph).toBe(true);
    expect(records[0].category).toBe('pool');
    expect(records[0].capabilities).toContain('replication');
  });

  it('preserves parent node labels and node hints for storage resources', () => {
    const records = buildStorageRecords({
      state: baseState(),
      resources: [
        makeResourceStorage({
          id: 'resource-storage-parent-name',
          parentId: 'agent-123',
          parentName: 'pve1',
          platformData: {
            type: 'dir',
            proxmox: { nodeName: 'pve1' },
          },
        }),
      ],
    });

    expect(records).toHaveLength(1);
    expect(records[0].location.label).toBe('pve1');
    expect(records[0].details?.node).toBe('pve1');
    expect(records[0].details?.parentName).toBe('pve1');
    expect(records[0].details?.nodeHints).toEqual(
      expect.arrayContaining(['pve1', 'agent-123', 'cluster-a']),
    );
  });

  it('collapses duplicate resource records by canonical identity and merges capabilities/details', () => {
    const resources: Resource[] = [
      makeResourceStorage({
        id: 'resource-storage-a',
        status: 'online',
        disk: { current: 40, total: 1000, used: 400, free: 600 },
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
          shared: false,
        },
      }),
      makeResourceStorage({
        id: 'resource-storage-b',
        status: 'online',
        disk: { current: 55, total: 1000, used: 550, free: 450 },
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
          shared: false,
        },
        storage: {
          type: 'zfspool',
          // Add a capability-bearing hint to ensure merge doesn't duplicate entries
          isZfs: true,
        } as any,
      }),
    ];

    const records = buildStorageRecords({ state: baseState(), resources });

    expect(records).toHaveLength(1);
    expect(records[0].id).toBe('resource-storage-a');
    expect(records[0].source.origin).toBe('resource');
    expect(records[0].health).toBe('healthy');
    expect(records[0].capacity.usedBytes).toBe(400);
    expect(records[0].details?.shared).toBe(false);
    expect(
      records[0].capabilities.filter((capability) => capability === 'compression'),
    ).toHaveLength(1);
  });

  it('keeps records separate when canonical identities differ', () => {
    const resources: Resource[] = [
      makeResourceStorage({
        id: 'resource-storage-a',
        name: 'local-zfs',
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
          shared: false,
        },
      }),
      makeResourceStorage({
        id: 'resource-storage-b',
        name: 'local-zfs-2',
        platformData: {
          type: 'zfspool',
          node: 'pve2',
          instance: 'cluster-a',
          shared: false,
        },
      }),
    ];

    const records = buildStorageRecords({ state: baseState(), resources });

    expect(records).toHaveLength(2);
    expect(new Set(records.map((record) => record.name))).toEqual(
      new Set(['local-zfs', 'local-zfs-2']),
    );
  });

  it('maps a TrueNAS pool to StorageRecord with zfs metadata and healthy health', () => {
    const truenasPool = {
      ...makeResourceStorage({
        id: 'truenas-pool-1',
        name: 'tank',
        displayName: 'tank',
        platformId: 'truenas-1',
        platformType: 'truenas',
        status: 'online',
        tags: ['truenas', 'pool', 'zfs', 'health:online'],
        disk: { current: 42, total: 2000, used: 840, free: 1160 },
      }),
      storage: {
        type: 'zfs-pool',
        isZfs: true,
      },
    } as Resource;

    const records = buildStorageRecords({ state: baseState(), resources: [truenasPool] });

    expect(records).toHaveLength(1);
    expect(records[0].health).toBe('healthy');
    expect(records[0].details?.type).toBe('zfs-pool');
    expect(records[0].details?.isZfs).toBe(true);
  });

  it('maps a TrueNAS degraded pool to warning health', () => {
    const truenasPoolDegraded = {
      ...makeResourceStorage({
        id: 'truenas-pool-2',
        name: 'tank-degraded',
        displayName: 'tank-degraded',
        platformId: 'truenas-1',
        platformType: 'truenas',
        status: 'warning' as Resource['status'],
        tags: ['truenas', 'pool', 'zfs', 'health:degraded'],
      }),
      storage: {
        type: 'zfs-pool',
        isZfs: true,
      },
    } as Resource;

    const records = buildStorageRecords({ state: baseState(), resources: [truenasPoolDegraded] });

    expect(records).toHaveLength(1);
    expect(records[0].health).toBe('warning');
    expect(records[0].details?.type).toBe('zfs-pool');
    expect(records[0].details?.isZfs).toBe(true);
  });

  it('maps a TrueNAS faulted pool to critical or offline health', () => {
    const truenasPoolFaulted = {
      ...makeResourceStorage({
        id: 'truenas-pool-3',
        name: 'tank-faulted',
        displayName: 'tank-faulted',
        platformId: 'truenas-1',
        platformType: 'truenas',
        status: 'offline',
        tags: ['truenas', 'pool', 'zfs', 'health:faulted'],
      }),
      storage: {
        type: 'zfs-pool',
        isZfs: true,
      },
    } as Resource;

    const records = buildStorageRecords({ state: baseState(), resources: [truenasPoolFaulted] });

    expect(records).toHaveLength(1);
    expect(['critical', 'offline']).toContain(records[0].health);
    expect(records[0].details?.type).toBe('zfs-pool');
    expect(records[0].details?.isZfs).toBe(true);
  });

  it('maps a TrueNAS dataset with mounted state and zfs metadata', () => {
    const truenasDataset = {
      ...makeResourceStorage({
        id: 'truenas-dataset-1',
        name: 'tank/ds1',
        displayName: 'tank/ds1',
        platformId: 'truenas-1',
        platformType: 'truenas',
        tags: ['truenas', 'dataset', 'zfs', 'state:mounted'],
      }),
      storage: {
        type: 'zfs-dataset',
        isZfs: true,
      },
    } as Resource;

    const records = buildStorageRecords({ state: baseState(), resources: [truenasDataset] });

    expect(records).toHaveLength(1);
    expect(records[0].details?.type).toBe('zfs-dataset');
    expect(records[0].details?.isZfs).toBe(true);
  });

  it('maps canonical posture, impact, and action fields from unified resources', () => {
    const records = buildStorageRecords({
      state: baseState(),
      resources: [
        makeResourceStorage({
          id: 'pbs-datastore-1',
          type: 'datastore',
          name: 'pbs-fast',
          platformId: 'pbs-1',
          platformType: 'proxmox-pbs',
          parentId: 'pbs-parent-1',
          parentName: 'pbs01',
          status: 'degraded',
          incidentCategory: 'recoverability',
          incidentSeverity: 'critical',
          incidentPriority: 95,
          incidentLabel: 'Backup Coverage At Risk',
          incidentSummary: 'Datastore is unavailable to backup jobs.',
          incidentImpactSummary: 'Puts backups for 3 protected workloads at risk.',
          incidentAction: 'Restore backup target health immediately',
          storage: {
            type: 'pbs',
            platform: 'pbs',
            topology: 'datastore',
            protection: 'backup',
            protectionReduced: true,
            protectionSummary: 'Backup target unavailable',
            postureSummary: 'Recoverability degraded',
            riskSummary: 'Datastore unavailable',
          },
          pbs: {
            protectedWorkloadCount: 3,
            affectedDatastoreCount: 1,
            protectedWorkloadSummary: '3 protected workloads',
          },
        }),
      ],
    });

    expect(records).toHaveLength(1);
    expect(records[0].platformLabel).toBe('PBS');
    expect(records[0].hostLabel).toBe('pbs01');
    expect(records[0].topologyLabel).toBe('Datastore');
    expect(records[0].protectionLabel).toBe('Backup target unavailable');
    expect(records[0].issueLabel).toBe('Backup Coverage At Risk');
    expect(records[0].issueSummary).toBe('Datastore is unavailable to backup jobs.');
    expect(records[0].impactSummary).toBe('Puts backups for 3 protected workloads at risk.');
    expect(records[0].actionSummary).toBe('Restore backup target health immediately');
    expect(records[0].incidentPriority).toBe(95);
    expect(records[0].protectedWorkloadCount).toBe(3);
    expect(records[0].details?.platformLabel).toBe('PBS');
    expect(records[0].details?.protectionReduced).toBe(true);
  });
});
