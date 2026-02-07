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

describe('storageAdapters', () => {
  it('prefers resource-origin records over legacy duplicates by ID', () => {
    const state = baseState({
      storage: [
        {
          id: 'storage-1',
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
          shared: false,
          enabled: true,
          active: true,
        },
      ],
    });

    const resources: Resource[] = [
      {
        id: 'storage-1',
        type: 'storage',
        name: 'local-zfs',
        displayName: 'local-zfs',
        platformId: 'cluster-a',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'online',
        disk: { current: 40, total: 1000, used: 400, free: 600 },
        lastSeen: Date.now(),
        platformData: {
          type: 'zfspool',
          node: 'pve1',
          instance: 'cluster-a',
        },
      },
    ];

    const records = buildStorageRecordsV2({ state, resources });
    expect(records).toHaveLength(1);
    expect(records[0].source.origin).toBe('resource');
    expect(records[0].capacity.usedBytes).toBe(400);
  });

  it('adds legacy PBS datastore records when resource datastores are unavailable', () => {
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

    const records = buildStorageRecordsV2({ state, resources: [] });
    expect(records).toHaveLength(1);
    expect(records[0].category).toBe('backup-repository');
    expect(records[0].source.platform).toBe('proxmox-pbs');
    expect(records[0].capabilities).toContain('deduplication');
  });
});

