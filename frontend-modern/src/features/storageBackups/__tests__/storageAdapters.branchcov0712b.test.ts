import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { StorageAdapterContext } from '@/features/storageBackups/models';
import { DEFAULT_STORAGE_ADAPTERS } from '@/features/storageBackups/storageAdapters';

const baseState = (overrides: Partial<State> = {}): State => ({
  connectedInfrastructure: [],
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
  lastUpdate: 0,
  resources: [],
  ...overrides,
  pveTagColors: overrides.pveTagColors ?? {},
  pveTagStyles: overrides.pveTagStyles ?? {},
});

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'storage-1',
    type: 'storage',
    name: 'local',
    displayName: 'local',
    platformId: 'cluster-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1731000000000,
    platformData: {
      type: 'dir',
      node: 'pve1',
      instance: 'cluster-a',
      shared: false,
    },
    ...overrides,
  }) as Resource;

const ctx = (resources: Resource[]): StorageAdapterContext => ({
  state: baseState(),
  resources,
});

const resourceAdapter = DEFAULT_STORAGE_ADAPTERS[0];

describe('storageAdapters.branchcov2 - mapResourceStorageRecord', () => {
  it('falls back to platformData.platform when no storage meta or storage.platform is set', () => {
    const resource = makeResource({
      id: 'pd-platform',
      platformData: { platform: 'custom-plat', topology: 'custom-topo' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.platform).toBe('custom-plat');
    expect(record.details?.topology).toBe('custom-topo');
  });

  it('reads proxmoxNode from resource.proxmox.node (the first || arm)', () => {
    const resource = makeResource({
      id: 'prox-node',
      parentName: 'parent-host',
      proxmox: { node: 'pve-prox' },
      platformData: { type: 'dir', node: 'pd-node' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.node).toBe('pve-prox');
    expect(record.details?.nodeHints).toEqual(expect.arrayContaining(['pve-prox']));
    expect(record.location.label).toBe('parent-host');
  });

  it('resolves the non-backup location label to "Unknown" when every source is falsy', () => {
    const resource = makeResource({
      id: 'unknown-node',
      parentName: undefined,
      parentId: '',
      platformId: '',
      platformData: { type: 'dir' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.location.label).toBe('Unknown');
    expect(record.location.scope).toBe('node');
  });

  it('resolves the backup-repository location label to "Unknown" through the pbs arm', () => {
    const resource = makeResource({
      id: 'unknown-pbs',
      type: 'datastore',
      parentName: undefined,
      parentId: '',
      platformId: '',
      platformData: { type: 'pbs' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.location.label).toBe('Unknown');
    expect(record.location.scope).toBe('cluster');
    expect(record.category).toBe('backup-repository');
  });

  it('uses platformData.pbsInstanceName for the backup location label when parentName is absent', () => {
    const resource = makeResource({
      id: 'pbs-inst',
      type: 'datastore',
      parentName: undefined,
      platformData: { type: 'pbs', pbsInstanceName: 'pbs-inst-7' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.location.label).toBe('pbs-inst-7');
    expect(record.details?.content).toBe('backup');
  });

  it('dedupes nodeHints and drops whitespace-only storage node entries', () => {
    const resource = makeResource({
      id: 'hints',
      name: 'hints-store',
      parentName: 'pve1',
      parentId: 'agent-x',
      platformId: 'cluster-a',
      platformData: { type: 'dir', node: 'pve2' },
      proxmox: { node: 'pve1' },
      storage: { nodes: ['pve2', '   ', 'pve3'] },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.nodeHints).toEqual(['pve1', 'pve2', 'pve3', 'agent-x', 'cluster-a']);
  });

  it('falls back to storageNodes[0] for details.node and the location label', () => {
    const resource = makeResource({
      id: 'sn-node',
      parentName: undefined,
      platformData: { type: 'dir' },
      storage: { nodes: ['sn1', 'sn2'] },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.node).toBe('sn1');
    expect(record.location.label).toBe('sn1');
  });

  it('uses platformData.zfsPool when storageMeta has no zfsPool (the ?? right arm)', () => {
    const resource = makeResource({
      id: 'zfs-pd',
      platformData: { zfsPool: { name: 'rpool', state: 'ONLINE' } },
      storage: { type: 'dir' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.zfsPool).toEqual({ name: 'rpool', state: 'ONLINE' });
  });

  it('threads metricsTarget into refs.resourceId and the metricsTarget field', () => {
    const resource = makeResource({
      id: 'mt-res',
      platformId: 'plat-9',
      metricsTarget: { resourceType: 'storage', resourceId: 'metric-ref-9' },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.metricsTarget).toEqual({ resourceType: 'storage', resourceId: 'metric-ref-9' });
    expect(record.refs?.resourceId).toBe('metric-ref-9');
    expect(record.refs?.platformEntityId).toBe('plat-9');
  });

  it('falls back to resource.id for refs.resourceId when metricsTarget is absent', () => {
    const resource = makeResource({ id: 'no-mt', platformId: 'plat-fb' });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.metricsTarget).toBeUndefined();
    expect(record.refs?.resourceId).toBe('no-mt');
    expect(record.refs?.platformEntityId).toBe('plat-fb');
  });

  it('produces all-null capacity when the resource has no disk metric', () => {
    const resource = makeResource({ id: 'no-disk' });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.capacity).toStrictEqual({
      totalBytes: null,
      usedBytes: null,
      freeBytes: null,
      usagePercent: null,
    });
  });

  it('coerces non-finite and non-number disk values to null capacity slots', () => {
    const resource = makeResource({
      id: 'bad-disk',
      disk: {
        current: Number.NaN,
        total: Number.POSITIVE_INFINITY,
        used: 'big' as unknown as number,
      },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.capacity).toStrictEqual({
      totalBytes: null,
      usedBytes: null,
      freeBytes: null,
      usagePercent: null,
    });
  });

  it('passes consumer/protected/affected counts, protection flags and incident fields straight through', () => {
    const resource = makeResource({
      id: 'rich',
      platformData: { type: 'dir' },
      storage: { consumerCount: 7, protectionReduced: true, rebuildInProgress: true },
      pbs: { protectedWorkloadCount: 4, affectedDatastoreCount: 2 },
      incidentCategory: 'recoverability',
      incidentSeverity: 'critical',
      incidentPriority: 42,
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.consumerCount).toBe(7);
    expect(record.protectedWorkloadCount).toBe(4);
    expect(record.affectedDatastoreCount).toBe(2);
    expect(record.protectionReduced).toBe(true);
    expect(record.rebuildInProgress).toBe(true);
    expect(record.incidentCategory).toBe('recoverability');
    expect(record.incidentSeverity).toBe('critical');
    expect(record.incidentPriority).toBe(42);
    expect(record.details?.protectionReduced).toBe(true);
    expect(record.details?.rebuildInProgress).toBe(true);
    expect(record.details?.incidentCategory).toBe('recoverability');
    expect(record.details?.incidentSeverity).toBe('critical');
    expect(record.details?.incidentPriority).toBe(42);
  });

  it('surfaces enriched storage meta fields (content/contentTypes/path/protection/counts) into details', () => {
    const resource = makeResource({
      id: 'meta-detail',
      platformData: {},
      storage: {
        type: 'dir',
        contentTypes: ['images', 'rootdir'],
        path: '/mnt/dir',
        protection: 'replicated',
        syncProgress: 55,
        numProtected: 4,
        numDisabled: 1,
        numInvalid: 2,
        numMissing: 3,
      },
    });
    const [record] = resourceAdapter.build(ctx([resource]));
    expect(record.details?.content).toBe('images,rootdir');
    expect(record.details?.contentTypes).toEqual(['images', 'rootdir']);
    expect(record.details?.path).toBe('/mnt/dir');
    expect(record.details?.protection).toBe('replicated');
    expect(record.details?.syncProgress).toBe(55);
    expect(record.details?.numProtected).toBe(4);
    expect(record.details?.numDisabled).toBe(1);
    expect(record.details?.numInvalid).toBe(2);
    expect(record.details?.numMissing).toBe(3);
  });
});
