import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import type {
  StorageAdapter,
  StorageAdapterContext,
  StorageRecord,
} from '@/features/storageBackups/models';
import {
  buildStorageRecords,
  DEFAULT_STORAGE_ADAPTERS,
} from '@/features/storageBackups/storageAdapters';

// Minimal but complete State factory (mirrors the sibling storageAdapters.test.ts
// baseState shape so the StorageAdapterContext.state contract always type-checks).
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

// Full-required-fields Resource factory. `as Resource` is safe because every
// required field (id, type, name, displayName, platformId, platformType,
// sourceType, status, lastSeen) has a default; overrides may only widen values.
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

// A StorageRecord factory whose canonical identity (platform|location|name|category)
// is held constant so two records collide inside buildStorageRecords and trigger
// the private mergeStorageRecords(current, incoming).
const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'record-a',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'test-adapter',
  },
  observedAt: 1000,
  ...overrides,
});

// Custom adapter that emits pre-built records so we can drive mergeStorageRecords
// with precisely-controlled current/incoming payloads (the function is private,
// reachable only when buildStorageRecords detects a duplicate canonical key).
const recordAdapter = (records: StorageRecord[], id = 'test-adapter'): StorageAdapter => ({
  id,
  supports: () => true,
  build: () => records,
});

// buildStorageRecords processes records in array order: the first colliding
// record becomes `existing` (== `current` to merge), the next is `incoming`.
// Both records share name/location/platform/category so they collide.
const mergeTwo = (current: StorageRecord, incoming: StorageRecord): StorageRecord => {
  const records = buildStorageRecords(ctx([]), [recordAdapter([current, incoming])]);
  expect(records).toHaveLength(1);
  return records[0];
};

const resourceAdapter = DEFAULT_STORAGE_ADAPTERS[0];

describe('storageAdapters.branchcov - build (adapter) + supports + buildStorageRecords', () => {
  it('supports() is true for a non-empty resources array and false for empty/non-array', () => {
    expect(resourceAdapter.supports(ctx([makeResource()]))).toBe(true);
    expect(resourceAdapter.supports(ctx([]))).toBe(false);
    expect(
      resourceAdapter.supports({
        state: baseState(),
        resources: undefined,
      } as unknown as StorageAdapterContext),
    ).toBe(false);
  });

  it('build() falls back to an empty array when ctx.resources is undefined (the `|| []` arm)', () => {
    const result = resourceAdapter.build({
      state: baseState(),
      resources: undefined,
    } as unknown as StorageAdapterContext);
    expect(result).toEqual([]);
  });

  it('build() keeps storage and datastore resources but filters every other type', () => {
    const resources: Resource[] = [
      makeResource({ id: 's1', type: 'storage', name: 's1' }),
      makeResource({ id: 'd1', type: 'datastore', name: 'd1' }),
      makeResource({ id: 'v1', type: 'vm', name: 'v1' }),
      makeResource({ id: 'a1', type: 'agent', name: 'a1' }),
    ];
    const result = resourceAdapter.build(ctx(resources));
    expect(result.map((record) => record.id)).toEqual(['s1', 'd1']);
  });

  it('buildStorageRecords returns an empty array when the adapter list is empty', () => {
    expect(buildStorageRecords(ctx([makeResource()]), [])).toEqual([]);
  });

  it('buildStorageRecords skips an adapter whose supports() is false', () => {
    const neverSupports: StorageAdapter = {
      id: 'never',
      supports: () => false,
      build: () => [makeRecord({ id: 'should-not-appear' })],
    };
    expect(buildStorageRecords(ctx([]), [neverSupports])).toEqual([]);
  });

  it('buildStorageRecords uses DEFAULT_STORAGE_ADAPTERS when no adapter list is supplied', () => {
    const records = buildStorageRecords(ctx([makeResource({ id: 'only', name: 'only' })]));
    expect(records).toHaveLength(1);
    expect(records[0].id).toBe('only');
    expect(records[0].source.adapterId).toBe('resource-storage');
  });

  it('buildStorageRecords threads records from multiple adapters, merging on collision', () => {
    const a: StorageAdapter = recordAdapter(
      [makeRecord({ id: 'from-a', incidentPriority: 1, incidentCategory: 'a' })],
      'adapter-a',
    );
    const b: StorageAdapter = recordAdapter(
      [makeRecord({ id: 'from-b', incidentPriority: 9, incidentCategory: 'b' })],
      'adapter-b',
    );
    // Same canonical key -> merged; preferred is the higher-priority record (from-b).
    const records = buildStorageRecords(ctx([]), [a, b]);
    expect(records).toHaveLength(1);
    expect(records[0].incidentPriority).toBe(9);
    expect(records[0].incidentCategory).toBe('b');
  });

  it('buildStorageRecords keeps records apart when canonical keys differ', () => {
    const rec1 = makeRecord({ id: 'r1', name: 'alpha' });
    const rec2 = makeRecord({ id: 'r2', name: 'beta' });
    const records = buildStorageRecords(ctx([]), [recordAdapter([rec1, rec2])]);
    expect(records).toHaveLength(2);
    expect(new Set(records.map((record) => record.name))).toEqual(new Set(['alpha', 'beta']));
  });
});

describe('storageAdapters.branchcov - mapResourceStorageRecord (via resource adapter)', () => {
  it('tolerates missing platformData and falls back to the resourceType-derived storage type', () => {
    // No platformData, no storage -> `platformData || {}` arm; storageType falls
    // through to the resourceType ('storage') because no pbs hint is present.
    const resource = makeResource({
      id: 'bare',
      type: 'storage',
      platformData: undefined,
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records).toHaveLength(1);
    expect(records[0].details?.type).toBe('storage');
    expect(records[0].details?.platform).toBe('proxmox-pve');
  });

  it('derives storageType from platformData.type when no enriched storage meta exists', () => {
    const resource = makeResource({
      id: 'lvm',
      platformData: { type: 'lvm', node: 'pve1', instance: 'cluster-a' },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].details?.type).toBe('lvm');
  });

  it('uses resource.platformType when neither storage meta nor platformData declares a platform', () => {
    const resource = makeResource({
      id: 'plat-only',
      platformData: { node: 'pve2', instance: 'cluster-a' },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].details?.platform).toBe('proxmox-pve');
  });

  it('reports shared as undefined when neither storage meta nor platformData is boolean', () => {
    const resource = makeResource({
      id: 'shared-undef',
      platformData: { type: 'dir', node: 'pve1', instance: 'cluster-a' },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].details?.shared).toBeUndefined();
  });

  it('reads shared from platformData.shared when storage meta is absent', () => {
    const resource = makeResource({
      id: 'shared-pd',
      platformData: { type: 'dir', node: 'pve1', instance: 'cluster-a', shared: true },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].details?.shared).toBe(true);
  });

  it('reads shared from enriched storage meta, overriding platformData', () => {
    const resource = {
      ...makeResource({
        id: 'shared-meta',
        platformData: { type: 'dir', node: 'pve1', instance: 'cluster-a', shared: true },
      }),
      storage: { type: 'dir', shared: false },
    } as Resource;
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].details?.shared).toBe(false);
  });

  it('uses Date.now() for observedAt when lastSeen is NaN (number but not finite)', () => {
    const resource = makeResource({ id: 'nan-seen', lastSeen: Number.NaN });
    const before = Date.now();
    const records = resourceAdapter.build(ctx([resource]));
    const after = Date.now();
    expect(records[0].observedAt).toBeGreaterThanOrEqual(before);
    expect(records[0].observedAt).toBeLessThanOrEqual(after);
  });

  it('uses Date.now() for observedAt when lastSeen is not a number at all', () => {
    const resource = makeResource({
      id: 'str-seen',
      lastSeen: 'not-a-timestamp' as unknown as number,
    });
    const before = Date.now();
    const records = resourceAdapter.build(ctx([resource]));
    const after = Date.now();
    expect(records[0].observedAt).toBeGreaterThanOrEqual(before);
    expect(records[0].observedAt).toBeLessThanOrEqual(after);
  });

  it('uses the numeric lastSeen verbatim when it is a finite number', () => {
    const resource = makeResource({ id: 'finite-seen', lastSeen: 9999999 });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].observedAt).toBe(9999999);
  });

  it('defaults incidentPriority to 0 when the resource does not declare one', () => {
    const resource = makeResource({ id: 'no-prio' });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].incidentPriority).toBe(0);
  });

  it('classifies a non-shared VMware datastore with host scope and datastore category', () => {
    const datastore = {
      ...makeResource({
        id: 'local-vmfs',
        type: 'storage',
        name: 'local-vmfs',
        platformType: 'vmware-vsphere',
        platformData: { sources: ['vmware-vsphere'] },
      }),
      storage: {
        type: 'vmfs',
        platform: 'vmware-vsphere',
        topology: 'datastore',
        shared: false,
      },
      vmware: { entityType: 'datastore' },
    } as Resource;
    const records = resourceAdapter.build(ctx([datastore]));
    expect(records[0].category).toBe('datastore');
    expect(records[0].location.scope).toBe('host');
  });

  it('classifies a plain proxmox storage resource with node scope', () => {
    const resource = makeResource({
      id: 'node-scope',
      platformData: { type: 'dir', node: 'pve1' },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].location.scope).toBe('node');
  });

  it('classifies a PBS backup repository with cluster scope and backup-repository category', () => {
    const resource = makeResource({
      id: 'pbs-repo',
      type: 'datastore',
      name: 'pbs-store',
      platformData: { type: 'pbs', node: 'pbs01', instance: 'pbs-cluster' },
    });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].category).toBe('backup-repository');
    expect(records[0].location.scope).toBe('cluster');
  });

  it('keeps warning health for an Unraid array that has an attention issue (disabled disks)', () => {
    // normalizedHealth is 'warning' (incidentSeverity warning + status degraded),
    // isUnraid is true (arrayState present), but hasUnraidStorageAttentionIssue
    // is true (unraid_disabled_disks risk) -> the `!attention` guard is false,
    // so health stays at the normalized 'warning' instead of being flipped.
    const unraid = {
      ...makeResource({
        id: 'unraid-attention',
        name: 'tower',
        platformType: 'unraid',
        status: 'degraded',
        incidentSeverity: 'warning',
      }),
      storage: {
        type: 'unraid-array',
        platform: 'unraid',
        topology: 'array',
        arrayState: 'STARTED',
        risk: {
          level: 'critical',
          reasons: [
            {
              code: 'unraid_disabled_disks',
              severity: 'critical',
              summary: 'Unraid array reports disabled disks',
            },
          ],
        },
      },
    } as Resource;
    const records = resourceAdapter.build(ctx([unraid]));
    expect(records[0].health).toBe('warning');
  });

  it('uses resource.status as statusLabel for a non-Unraid resource', () => {
    const resource = makeResource({ id: 'status-arm', status: 'running' });
    const records = resourceAdapter.build(ctx([resource]));
    expect(records[0].statusLabel).toBe('running');
    expect(records[0].details?.status).toBe('running');
  });
});

describe('storageAdapters.branchcov - mergeStorageRecords (via duplicate canonical keys)', () => {
  it('selects current as preferred when current.incidentPriority >= incoming.incidentPriority', () => {
    const current = makeRecord({ id: 'c1', incidentPriority: 5, incidentCategory: 'cur' });
    const incoming = makeRecord({ id: 'i1', incidentPriority: 2, incidentCategory: 'inc' });
    const merged = mergeTwo(current, incoming);
    expect(merged.incidentPriority).toBe(5);
    // incidentCategory is a spread-only top-level field -> preferred (current) wins.
    expect(merged.incidentCategory).toBe('cur');
  });

  it('selects incoming as preferred when incoming.incidentPriority > current.incidentPriority', () => {
    const current = makeRecord({ id: 'c2', incidentPriority: 2, incidentCategory: 'cur' });
    const incoming = makeRecord({ id: 'i2', incidentPriority: 5, incidentCategory: 'inc' });
    const merged = mergeTwo(current, incoming);
    expect(merged.incidentPriority).toBe(5);
    expect(merged.incidentCategory).toBe('inc');
  });

  it('breaks equal priorities in favour of current (the `>=` arm)', () => {
    const current = makeRecord({ id: 'c3', incidentPriority: 3, incidentCategory: 'cur' });
    const incoming = makeRecord({ id: 'i3', incidentPriority: 3, incidentCategory: 'inc' });
    const merged = mergeTwo(current, incoming);
    expect(merged.incidentCategory).toBe('cur');
  });

  it('treats undefined priorities as 0 and still prefers current', () => {
    const current = makeRecord({ id: 'c4', incidentCategory: 'cur' });
    const incoming = makeRecord({ id: 'i4', incidentCategory: 'inc' });
    const merged = mergeTwo(current, incoming);
    expect(merged.incidentCategory).toBe('cur');
  });

  it('deduplicates capabilities across both records preserving current-first order', () => {
    const current = makeRecord({ id: 'c5', capabilities: ['capacity', 'health'] });
    const incoming = makeRecord({ id: 'i5', capabilities: ['health', 'snapshots'] });
    const merged = mergeTwo(current, incoming);
    expect(merged.capabilities).toEqual(['capacity', 'health', 'snapshots']);
  });

  it('falls back to [] for capabilities when either side is missing', () => {
    const current = makeRecord({ id: 'c6', capabilities: undefined });
    const incoming = makeRecord({ id: 'i6', capabilities: undefined });
    const merged = mergeTwo(current, incoming);
    expect(merged.capabilities).toEqual([]);
  });

  it('promotes secondary.health when preferred.health is unknown but secondary is not', () => {
    const current = makeRecord({ id: 'c7', incidentPriority: 1, health: 'unknown' });
    const incoming = makeRecord({ id: 'i7', incidentPriority: 0, health: 'warning' });
    // current preferred (1 >= 0); preferred.health 'unknown' && secondary 'warning' -> 'warning'.
    const merged = mergeTwo(current, incoming);
    expect(merged.health).toBe('warning');
  });

  it('keeps preferred.health when preferred.health is not unknown', () => {
    const current = makeRecord({ id: 'c8', incidentPriority: 1, health: 'critical' });
    const incoming = makeRecord({ id: 'i8', incidentPriority: 0, health: 'warning' });
    const merged = mergeTwo(current, incoming);
    expect(merged.health).toBe('critical');
  });

  it('keeps unknown when both preferred and secondary health are unknown', () => {
    const current = makeRecord({ id: 'c9', incidentPriority: 1, health: 'unknown' });
    const incoming = makeRecord({ id: 'i9', incidentPriority: 0, health: 'unknown' });
    const merged = mergeTwo(current, incoming);
    expect(merged.health).toBe('unknown');
  });

  it('falls back to secondary.statusLabel when preferred.statusLabel is empty', () => {
    const current = makeRecord({
      id: 'c10',
      incidentPriority: 1,
      statusLabel: '',
      hostLabel: 'cur-host',
      platformLabel: 'PVE',
      platformKey: 'proxmox-pve',
      topologyLabel: 'Pool',
    });
    const incoming = makeRecord({ id: 'i10', incidentPriority: 0, statusLabel: 'Online' });
    const merged = mergeTwo(current, incoming);
    expect(merged.statusLabel).toBe('Online');
  });

  it('keeps preferred.statusLabel when it is set', () => {
    const current = makeRecord({ id: 'c11', incidentPriority: 1, statusLabel: 'Started' });
    const incoming = makeRecord({ id: 'i11', incidentPriority: 0, statusLabel: 'Online' });
    const merged = mergeTwo(current, incoming);
    expect(merged.statusLabel).toBe('Started');
  });

  it('uses preferred protectionLabel when protectionReduced is set on preferred', () => {
    const current = makeRecord({
      id: 'c12',
      incidentPriority: 0,
      protectionReduced: true,
      protectionLabel: 'Reduced',
    });
    const incoming = makeRecord({ id: 'i12', incidentPriority: 0, protectionLabel: 'OK' });
    const merged = mergeTwo(current, incoming);
    expect(merged.protectionLabel).toBe('Reduced');
  });

  it('falls back to secondary protectionLabel under the protection guard when preferred is empty', () => {
    const current = makeRecord({
      id: 'c13',
      incidentPriority: 0,
      protectionReduced: true,
      protectionLabel: '',
    });
    const incoming = makeRecord({ id: 'i13', incidentPriority: 0, protectionLabel: 'OK' });
    const merged = mergeTwo(current, incoming);
    expect(merged.protectionLabel).toBe('OK');
  });

  it('uses secondary protectionLabel when preferred has no protection signals', () => {
    const current = makeRecord({ id: 'c14', incidentPriority: 0, protectionLabel: '' });
    const incoming = makeRecord({ id: 'i14', incidentPriority: 0, protectionLabel: 'Sec' });
    const merged = mergeTwo(current, incoming);
    expect(merged.protectionLabel).toBe('Sec');
  });

  it('prefers preferred issueLabel/summary/action only when preferred.incidentPriority > 0', () => {
    const current = makeRecord({
      id: 'c15',
      incidentPriority: 7,
      issueLabel: 'CurIssue',
      issueSummary: 'cur summary',
      actionSummary: 'cur action',
    });
    const incoming = makeRecord({
      id: 'i15',
      incidentPriority: 0,
      issueLabel: 'IncIssue',
      issueSummary: 'inc summary',
      actionSummary: 'inc action',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.issueLabel).toBe('CurIssue');
    expect(merged.issueSummary).toBe('cur summary');
    expect(merged.actionSummary).toBe('cur action');
  });

  it('falls back to secondary issue fields when preferred.incidentPriority is 0', () => {
    const current = makeRecord({
      id: 'c16',
      incidentPriority: 0,
      issueLabel: '',
      issueSummary: '',
      actionSummary: '',
    });
    const incoming = makeRecord({
      id: 'i16',
      incidentPriority: 0,
      issueLabel: 'IncIssue',
      issueSummary: 'inc summary',
      actionSummary: 'inc action',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.issueLabel).toBe('IncIssue');
    expect(merged.issueSummary).toBe('inc summary');
    expect(merged.actionSummary).toBe('inc action');
  });

  it('promotes a meaningful secondary impactSummary over preferred "No dependent resources"', () => {
    const current = makeRecord({
      id: 'c17',
      incidentPriority: 0,
      impactSummary: 'No dependent resources',
    });
    const incoming = makeRecord({
      id: 'i17',
      incidentPriority: 0,
      impactSummary: 'Affects 3 dependent resources',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.impactSummary).toBe('Affects 3 dependent resources');
  });

  it('promotes a meaningful secondary impactSummary when preferred impactSummary is empty', () => {
    const current = makeRecord({ id: 'c18', incidentPriority: 0, impactSummary: '' });
    const incoming = makeRecord({
      id: 'i18',
      incidentPriority: 0,
      impactSummary: 'Affects 2 vms',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.impactSummary).toBe('Affects 2 vms');
  });

  it('keeps a meaningful preferred impactSummary over a meaningful secondary one', () => {
    const current = makeRecord({
      id: 'c19',
      incidentPriority: 0,
      impactSummary: 'Preferred impact',
    });
    const incoming = makeRecord({
      id: 'i19',
      incidentPriority: 0,
      impactSummary: 'Secondary impact',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.impactSummary).toBe('Preferred impact');
  });

  it('falls back to preferred impactSummary when secondary is "No dependent resources"', () => {
    const current = makeRecord({
      id: 'c20',
      incidentPriority: 0,
      impactSummary: 'Preferred impact',
    });
    const incoming = makeRecord({
      id: 'i20',
      incidentPriority: 0,
      impactSummary: 'No dependent resources',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.impactSummary).toBe('Preferred impact');
  });

  it('takes the max of consumer/protected/affected counts treating missing as 0', () => {
    const current = makeRecord({
      id: 'c21',
      incidentPriority: 1,
      consumerCount: 5,
      protectedWorkloadCount: undefined,
      affectedDatastoreCount: 2,
    });
    const incoming = makeRecord({
      id: 'i21',
      incidentPriority: 0,
      consumerCount: undefined,
      protectedWorkloadCount: 3,
      affectedDatastoreCount: 4,
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.consumerCount).toBe(5);
    expect(merged.protectedWorkloadCount).toBe(3);
    expect(merged.affectedDatastoreCount).toBe(4);
  });

  it('defaults all counts to 0 when both sides omit them', () => {
    const current = makeRecord({ id: 'c22', incidentPriority: 1 });
    const incoming = makeRecord({ id: 'i22', incidentPriority: 0 });
    const merged = mergeTwo(current, incoming);
    expect(merged.consumerCount).toBe(0);
    expect(merged.protectedWorkloadCount).toBe(0);
    expect(merged.affectedDatastoreCount).toBe(0);
  });

  it('ORs protectionReduced and rebuildInProgress across both records', () => {
    const current = makeRecord({
      id: 'c23',
      incidentPriority: 1,
      protectionReduced: false,
      rebuildInProgress: true,
    });
    const incoming = makeRecord({
      id: 'i23',
      incidentPriority: 0,
      protectionReduced: true,
      rebuildInProgress: false,
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.protectionReduced).toBe(true);
    expect(merged.rebuildInProgress).toBe(true);
  });

  it('prefers preferred.metricsTarget and falls back to secondary when preferred has none', () => {
    const current = makeRecord({
      id: 'c24',
      incidentPriority: 1,
      metricsTarget: { resourceType: 'storage', resourceId: 'preferred' },
    });
    const incoming = makeRecord({
      id: 'i24',
      incidentPriority: 0,
      metricsTarget: { resourceType: 'storage', resourceId: 'secondary' },
    });
    expect(mergeTwo(current, incoming).metricsTarget).toEqual({
      resourceType: 'storage',
      resourceId: 'preferred',
    });

    const currentNoMt = makeRecord({ id: 'c24b', incidentPriority: 1 });
    const incomingMt = makeRecord({
      id: 'i24b',
      incidentPriority: 0,
      metricsTarget: { resourceType: 'storage', resourceId: 'secondary' },
    });
    expect(mergeTwo(currentNoMt, incomingMt).metricsTarget).toEqual({
      resourceType: 'storage',
      resourceId: 'secondary',
    });
  });

  it('resolves refs.resourceId through metricsTarget then refs on both sides', () => {
    // No metricsTarget on either side; preferred.refs.resourceId wins.
    const current = makeRecord({
      id: 'c25',
      incidentPriority: 1,
      refs: { resourceId: 'cur-id', platformEntityId: 'cur-pe' },
    });
    const incoming = makeRecord({
      id: 'i25',
      incidentPriority: 0,
      refs: { resourceId: 'inc-id', platformEntityId: 'inc-pe' },
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.refs?.resourceId).toBe('cur-id');
    expect(merged.refs?.platformEntityId).toBe('cur-pe');

    // preferred.refs absent -> fall back to secondary.refs.
    const mergedFallback = mergeTwo(
      makeRecord({ id: 'c25b', incidentPriority: 1, metricsTarget: undefined }),
      makeRecord({
        id: 'i25b',
        incidentPriority: 0,
        refs: { resourceId: 'inc-id', platformEntityId: 'inc-pe' },
      }),
    );
    expect(mergedFallback.refs?.resourceId).toBe('inc-id');
    expect(mergedFallback.refs?.platformEntityId).toBe('inc-pe');
  });

  it('resolves refs.resourceId from metricsTarget before refs', () => {
    const current = makeRecord({
      id: 'c26',
      incidentPriority: 1,
      metricsTarget: { resourceType: 'storage', resourceId: 'mt-cur' },
      refs: { resourceId: 'ref-cur' },
    });
    const incoming = makeRecord({ id: 'i26', incidentPriority: 0 });
    const merged = mergeTwo(current, incoming);
    expect(merged.refs?.resourceId).toBe('mt-cur');
  });

  it('merges details with preferred overriding secondary, and tolerates missing details', () => {
    // Both present -> preferred (current) keys win on conflict.
    const current = makeRecord({
      id: 'c27',
      incidentPriority: 1,
      details: { a: 'cur', b: 'cur', onlyCur: true },
    });
    const incoming = makeRecord({
      id: 'i27',
      incidentPriority: 0,
      details: { b: 'inc', c: 'inc', onlyInc: true },
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.details).toEqual({ a: 'cur', b: 'cur', onlyCur: true, c: 'inc', onlyInc: true });

    // Both absent -> {} (the `|| {}` fallbacks on both sides).
    const mergedEmpty = mergeTwo(
      makeRecord({ id: 'c27b', incidentPriority: 1, details: undefined }),
      makeRecord({ id: 'i27b', incidentPriority: 0, details: undefined }),
    );
    expect(mergedEmpty.details).toEqual({});

    // Preferred details absent -> secondary.details surface through.
    const mergedSecondary = mergeTwo(
      makeRecord({ id: 'c27c', incidentPriority: 1, details: undefined }),
      makeRecord({ id: 'i27c', incidentPriority: 0, details: { from: 'secondary' } }),
    );
    expect(mergedSecondary.details).toEqual({ from: 'secondary' });
  });

  it('fills empty hostLabel/platformLabel/platformKey/topologyLabel from secondary', () => {
    const current = makeRecord({
      id: 'c28',
      incidentPriority: 1,
      hostLabel: '',
      platformLabel: '',
      platformKey: '',
      topologyLabel: '',
    });
    const incoming = makeRecord({
      id: 'i28',
      incidentPriority: 0,
      hostLabel: 'inc-host',
      platformLabel: 'INC',
      platformKey: 'inc-platform',
      topologyLabel: 'IncTopo',
    });
    const merged = mergeTwo(current, incoming);
    expect(merged.hostLabel).toBe('inc-host');
    expect(merged.platformLabel).toBe('INC');
    expect(merged.platformKey).toBe('inc-platform');
    expect(merged.topologyLabel).toBe('IncTopo');
  });
});
