import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { buildStorageRecordsV2 } from '@/features/storageBackupsV2/storageAdapters';

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

const makeLegacyStorage = (overrides: Record<string, unknown> = {}) => ({
  id: 'legacy-storage-1',
  name: 'local-zfs',
  node: 'pve1',
  instance: 'cluster-a',
  type: 'zfspool',
  status: 'available',
  total: 1000,
  used: 900,
  free: 100,
  usage: 90,
  content: 'images',
  shared: true,
  enabled: true,
  active: true,
  ...overrides,
});

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
  it('prefers enriched storage metadata over legacy platformData inference', () => {
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

    const records = buildStorageRecordsV2({ state: baseState(), resources: [enriched] });

    expect(records).toHaveLength(1);
    expect(records[0].details?.type).toBe('rbd');
    expect(records[0].details?.content).toBe('images,rootdir');
    expect(records[0].details?.shared).toBe(false);
    expect(records[0].details?.isCeph).toBe(true);
    expect(records[0].category).toBe('pool');
    expect(records[0].capabilities).toContain('replication');
  });

  it('collapses mixed-origin records by canonical identity and keeps resource-origin winner', () => {
    const state = baseState({
      storage: [makeLegacyStorage({ id: 'legacy-storage-id', status: 'degraded', used: 950, usage: 95, shared: true })],
    });
    const resources: Resource[] = [
      makeResourceStorage({
        id: 'resource-storage-id',
        status: 'online',
        disk: { current: 40, total: 1000, used: 400, free: 600 },
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
          shared: false,
        },
      }),
    ];

    const records = buildStorageRecordsV2({ state, resources });

    expect(records).toHaveLength(1);
    expect(records[0].id).toBe('resource-storage-id');
    expect(records[0].source.origin).toBe('resource');
    expect(records[0].health).toBe('healthy');
    expect(records[0].capacity.usedBytes).toBe(400);
    expect(records[0].details?.shared).toBe(false);
  });

  it('applies deterministic precedence where resource-origin data wins over legacy', () => {
    const state = baseState({
      storage: [makeLegacyStorage({ status: 'degraded', used: 950, usage: 95, shared: true })],
    });
    const resources: Resource[] = [
      makeResourceStorage({
        status: 'online',
        disk: { current: 40, total: 1000, used: 400, free: 600 },
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
          shared: false,
        },
      }),
    ];
    const records = buildStorageRecordsV2({ state, resources });

    expect(records).toHaveLength(1);
    expect(records[0].source.origin).toBe('resource');
    expect(records[0].health).toBe('healthy');
    expect(records[0].capacity.usedBytes).toBe(400);
    expect(records[0].details?.shared).toBe(false);
  });

  it('keeps records separate when canonical identities differ', () => {
    const state = baseState({
      storage: [makeLegacyStorage({ id: 'legacy-storage-a', node: 'pve1' })],
    });
    const resources: Resource[] = [
      makeResourceStorage({
        id: 'resource-storage-b',
        platformData: {
          type: 'zfspool',
          node: 'pve2',
          instance: 'cluster-a',
          shared: false,
        },
      }),
    ];

    const records = buildStorageRecordsV2({ state, resources });

    expect(records).toHaveLength(2);
    expect(new Set(records.map((record) => record.location.label))).toEqual(new Set(['pve1', 'pve2']));
  });

  it('deduplicates capabilities when mixed-origin records are merged', () => {
    const state = baseState({
      pbs: [
        {
          id: 'pbs-1',
          name: 'pbs-main',
          host: 'https://pbs.local',
          status: 'online',
          version: '3.0',
          cpu: 10,
          memory: 20,
          memoryUsed: 2,
          memoryTotal: 10,
          uptime: 100,
          datastores: [
            {
              name: 'primary',
              total: 1000,
              used: 250,
              free: 750,
              usage: 25,
              status: 'available',
              error: '',
              namespaces: [],
              deduplicationFactor: 2.1,
            },
          ],
          backupJobs: [],
          syncJobs: [],
          verifyJobs: [],
          pruneJobs: [],
          garbageJobs: [],
          connectionHealth: 'healthy',
          lastSeen: new Date().toISOString(),
        },
      ],
    });

    const resources: Resource[] = [
      {
        id: 'resource-datastore-1',
        type: 'datastore',
        name: 'primary',
        displayName: 'primary',
        platformId: 'pbs-1',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        status: 'online',
        disk: { current: 25, total: 1000, used: 250, free: 750 },
        lastSeen: 1731000005000,
        parentId: 'pbs-1',
        platformData: {
          pbsInstanceName: 'pbs-main',
          type: 'pbs',
        },
      },
    ];

    const records = buildStorageRecordsV2({ state, resources });
    expect(records).toHaveLength(1);
    expect(records[0].capabilities.filter((capability) => capability === 'deduplication')).toHaveLength(1);
    expect(records[0].capabilities.filter((capability) => capability === 'backup-repository')).toHaveLength(1);
  });
});
