import { describe, expect, it } from 'vitest';
import type {
  PlatformType,
  Resource,
  ResourceAlert,
  ResourceMetric,
  ResourceType,
} from '@/types/resource';

import {
  buildProxmoxPageModel,
  getMetricPercent,
  getResourceClusterLabel,
  getResourceLastBackup,
  getResourceNodeName,
  getResourceVersion,
  getResourceVmid,
  isProxmoxStorageResource,
  resolveProxmoxPlatformScope,
} from '../proxmoxPageModel';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors the sibling proxmoxPageModel.test.ts factory so
// import style and default platform posture stay aligned.
// ---------------------------------------------------------------------------

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

const alert = (overrides: Partial<ResourceAlert> = {}): ResourceAlert => ({
  id: 'alert-1',
  type: 'cpu',
  level: 'warning',
  message: 'high load',
  value: 95,
  threshold: 90,
  startTime: 1_700_000_000_000,
  ...overrides,
});

// ===========================================================================
// getPlatformSources — module-private, exercised transitively through
// resolveProxmoxPlatformScope (its only call site).
// ===========================================================================

describe('getPlatformSources branches (via resolveProxmoxPlatformScope)', () => {
  it('falls back to resource.sources when platformData.sources is not an array', () => {
    // platformData.sources is a string -> not Array.isArray -> returns resource.sources.
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      sources: ['pbs'],
      platformData: { sources: 'not-an-array' },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pbs');
  });

  it('returns [] when platformData.sources is not an array and resource.sources is undefined', () => {
    // `resource.sources ?? []` -> [] -> no source match -> null.
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      platformData: { sources: 'not-an-array' },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBeNull();
  });

  it('filters platformData.sources to strings only (non-strings would crash toLowerCase)', () => {
    // If the filter did not strip the number/boolean/null, the downstream
    // `.toLowerCase()` in resolveProxmoxPlatformScope would throw.
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      platformData: { sources: ['proxmox-pmg', 123, true, null, 'noise'] },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pmg');
  });

  it('returns null when the filtered source list contains no known scope hint', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      platformData: { sources: ['docker', 'kubernetes'] },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBeNull();
  });
});

// ===========================================================================
// resolveProxmoxPlatformScope — direct arms beyond getPlatformSources.
// ===========================================================================

describe('resolveProxmoxPlatformScope direct arms', () => {
  it.each<[string, ResourceType, PlatformType]>([
    ['resolves platformType proxmox-pve', 'agent', 'proxmox-pve'],
    ['resolves platformType proxmox-pbs for a datastore', 'datastore', 'proxmox-pbs'],
    ['resolves platformType proxmox-pmg', 'agent', 'proxmox-pmg'],
  ])('%s', (_label, type, platformType) => {
    const resource = makeResource({ id: 'r', type, platformType });
    expect(resolveProxmoxPlatformScope(resource)).toBe(platformType);
  });

  it('resolves proxmox-pve when resource.proxmox is a truthy object', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      proxmox: { nodeName: 'n1' },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pve');
  });

  it('resolves proxmox-pve when platformData.proxmox is a record (resource.proxmox absent)', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      platformData: { proxmox: { hint: 'pve' } },
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pve');
  });

  it.each<[string, string]>([
    ['pbs short hint', 'pbs'],
    ['PBS uppercase hint (case-insensitive)', 'PBS'],
    ['proxmox-pbs long hint', 'proxmox-pbs'],
  ])('resolves proxmox-pbs from source %s', (_label, source) => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      sources: [source],
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pbs');
  });

  it.each<[string, string]>([
    ['pmg short hint', 'pmg'],
    ['proxmox-pmg long hint', 'proxmox-pmg'],
  ])('resolves proxmox-pmg from source %s', (_label, source) => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      sources: [source],
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pmg');
  });

  it.each<[string, string]>([
    ['pve short hint', 'pve'],
    ['proxmox-pve long hint', 'proxmox-pve'],
  ])('resolves proxmox-pve from source %s', (_label, source) => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformType: 'generic',
      sources: [source],
    });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pve');
  });

  it('returns null when no scope signal is present', () => {
    const resource = makeResource({ id: 'r', type: 'agent', platformType: 'generic' });
    expect(resolveProxmoxPlatformScope(resource)).toBeNull();
  });

  it('prefers type pbs over a contradicting platformType', () => {
    const resource = makeResource({ id: 'r', type: 'pbs', platformType: 'proxmox-pve' });
    expect(resolveProxmoxPlatformScope(resource)).toBe('proxmox-pbs');
  });
});

// ===========================================================================
// isProxmoxStorageResource
// ===========================================================================

describe('isProxmoxStorageResource branches', () => {
  it('returns false for a ceph-typed resource', () => {
    const resource = makeResource({ id: 'ceph-1', type: 'ceph' });
    expect(isProxmoxStorageResource(resource)).toBe(false);
  });

  it('returns false when storage.isCeph is true even on a storage type', () => {
    const resource = makeResource({
      id: 'stor-ceph',
      type: 'storage',
      storage: { isCeph: true },
    });
    expect(isProxmoxStorageResource(resource)).toBe(false);
  });

  it('returns true for a storage resource under proxmox-pve', () => {
    const resource = makeResource({
      id: 'stor-1',
      type: 'storage',
      storage: { isCeph: false },
    });
    expect(isProxmoxStorageResource(resource)).toBe(true);
  });

  it('returns true for a datastore resource under proxmox-pbs', () => {
    const resource = makeResource({
      id: 'ds-1',
      type: 'datastore',
      platformType: 'proxmox-pbs',
    });
    expect(isProxmoxStorageResource(resource)).toBe(true);
  });

  it('returns false for a storage resource under proxmox-pmg', () => {
    const resource = makeResource({
      id: 'stor-pmg',
      type: 'storage',
      platformType: 'proxmox-pmg',
    });
    expect(isProxmoxStorageResource(resource)).toBe(false);
  });

  it('returns false for a storage resource with no resolved scope', () => {
    const resource = makeResource({
      id: 'stor-orphan',
      type: 'storage',
      platformType: 'generic',
    });
    expect(isProxmoxStorageResource(resource)).toBe(false);
  });

  it('returns false for a non-storage proxmox-pve resource', () => {
    const resource = makeResource({ id: 'agent-1', type: 'agent' });
    expect(isProxmoxStorageResource(resource)).toBe(false);
  });
});

// ===========================================================================
// getMetricPercent
// ===========================================================================

describe('getMetricPercent branches', () => {
  it('returns 0 when no metric is provided', () => {
    expect(getMetricPercent(undefined)).toBe(0);
  });

  it('returns the finite current value unchanged within range', () => {
    expect(getMetricPercent({ current: 42.7 })).toBe(42.7);
  });

  it('clamps an over-range current to 100', () => {
    expect(getMetricPercent({ current: 150 })).toBe(100);
  });

  it('clamps a negative current to 0', () => {
    expect(getMetricPercent({ current: -20 })).toBe(0);
  });

  it('does not clamp the exact boundary 100', () => {
    expect(getMetricPercent({ current: 100 })).toBe(100);
  });

  it('falls back to the used/total ratio when current is NaN', () => {
    const metric: ResourceMetric = { current: NaN, total: 200, used: 50 };
    expect(getMetricPercent(metric)).toBe(25);
  });

  it('falls back to the used/total ratio clamped to 100', () => {
    const metric: ResourceMetric = { current: NaN, total: 10, used: 30 };
    expect(getMetricPercent(metric)).toBe(100);
  });

  it('uses used/total when current is a non-number (defensive cast)', () => {
    const metric = {
      current: 'bad' as unknown as number,
      total: 100,
      used: 100,
    } satisfies ResourceMetric;
    expect(getMetricPercent(metric)).toBe(100);
  });

  it('returns 0 when current is not a finite number and total/used are absent', () => {
    const metric: ResourceMetric = { current: NaN };
    expect(getMetricPercent(metric)).toBe(0);
  });
});

// ===========================================================================
// getResourceVmid
// ===========================================================================

describe('getResourceVmid branches', () => {
  it('returns the proxmox.vmid as a string when it is a finite number', () => {
    const resource = makeResource({ id: 'vm-1', type: 'vm', proxmox: { vmid: 101 } });
    expect(getResourceVmid(resource)).toBe('101');
  });

  it('ignores a non-finite proxmox.vmid (NaN) and reads platformData.proxmox.vmid', () => {
    const resource = makeResource({
      id: 'vm-2',
      type: 'vm',
      proxmox: { vmid: NaN },
      platformData: { proxmox: { vmid: 202 } },
    });
    expect(getResourceVmid(resource)).toBe('202');
  });

  it('ignores a wrong-typed proxmox.vmid (string) when no platformData vmid exists', () => {
    const resource = makeResource({
      id: 'vm-3',
      type: 'vm',
      proxmox: { vmid: '103' as unknown as number },
    });
    expect(getResourceVmid(resource)).toBe('');
  });

  it('reads vmid from platformData.proxmox when no meta proxmox block exists', () => {
    const resource = makeResource({
      id: 'vm-4',
      type: 'vm',
      platformData: { proxmox: { vmid: 303 } },
    });
    expect(getResourceVmid(resource)).toBe('303');
  });

  it('returns empty string when neither source carries a numeric vmid', () => {
    const resource = makeResource({ id: 'vm-5', type: 'vm' });
    expect(getResourceVmid(resource)).toBe('');
  });

  it('returns empty string when platformData.proxmox is not a record', () => {
    const resource = makeResource({
      id: 'vm-6',
      type: 'vm',
      platformData: { proxmox: 'not-a-record' },
    });
    expect(getResourceVmid(resource)).toBe('');
  });
});

// ===========================================================================
// getResourceVersion
// ===========================================================================

describe('getResourceVersion branches', () => {
  it('skips a non-formatable platformData.proxmox.pveVersion and falls through to pbs.version', () => {
    // formatProxmoxVersion('unknown') -> '' -> inner `if (version)` is false.
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformData: { proxmox: { pveVersion: 'unknown' } },
      pbs: { version: '3.0' },
    });
    expect(getResourceVersion(resource)).toBe('3.0');
  });

  it('returns resource.pbs.version when no pve signal is present', () => {
    const resource = makeResource({
      id: 'pbs-1',
      type: 'pbs',
      platformType: 'proxmox-pbs',
      pbs: { version: '3.2' },
    });
    expect(getResourceVersion(resource)).toBe('3.2');
  });

  it('returns platformData.pbs.version when meta pbs.version is absent', () => {
    const resource = makeResource({
      id: 'pbs-2',
      type: 'pbs',
      platformType: 'proxmox-pbs',
      platformData: { pbs: { version: '3.3' } },
    });
    expect(getResourceVersion(resource)).toBe('3.3');
  });

  it('returns platformData.pmg.version for a pmg resource', () => {
    const resource = makeResource({
      id: 'pmg-1',
      type: 'pmg',
      platformType: 'proxmox-pmg',
      platformData: { pmg: { version: '8.1' } },
    });
    expect(getResourceVersion(resource)).toBe('8.1');
  });

  it('formats agent.osVersion when agent.osName mentions proxmox', () => {
    const resource = makeResource({
      id: 'agent-1',
      type: 'agent',
      agent: { osName: 'Proxmox VE', osVersion: 'pve-manager/9.0/abc' },
    });
    expect(getResourceVersion(resource)).toBe('9.0');
  });

  it('falls back to the raw osVersion when it is not formatable', () => {
    // formatProxmoxVersion('unknown') === '' -> returns raw osVersion.
    const resource = makeResource({
      id: 'agent-2',
      type: 'agent',
      agent: { osName: 'proxmox', osVersion: 'unknown' },
    });
    expect(getResourceVersion(resource)).toBe('unknown');
  });

  it('does not take the agent branch when osName omits proxmox', () => {
    const resource = makeResource({
      id: 'agent-3',
      type: 'agent',
      agent: { osName: 'Debian', osVersion: '12' },
    });
    expect(getResourceVersion(resource)).toBe('');
  });

  it('returns empty string when no version signal is present at all', () => {
    const resource = makeResource({ id: 'agent-4', type: 'agent' });
    expect(getResourceVersion(resource)).toBe('');
  });
});

// ===========================================================================
// getResourceClusterLabel
// ===========================================================================

describe('getResourceClusterLabel branches', () => {
  it('prefers proxmox.clusterName', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      proxmox: { clusterName: 'alpha' },
      identity: { clusterName: 'shadow' },
      clusterId: 'cid',
    });
    expect(getResourceClusterLabel(resource)).toBe('alpha');
  });

  it('falls back to identity.clusterName when proxmox.clusterName is absent', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      identity: { clusterName: 'beta' },
      clusterId: 'cid',
    });
    expect(getResourceClusterLabel(resource)).toBe('beta');
  });

  it('falls back to clusterId when no clusterName is present', () => {
    const resource = makeResource({ id: 'r', type: 'agent', clusterId: 'clus-1' });
    expect(getResourceClusterLabel(resource)).toBe('clus-1');
  });

  it('returns "Standalone" when nothing is set', () => {
    const resource = makeResource({ id: 'r', type: 'agent' });
    expect(getResourceClusterLabel(resource)).toBe('Standalone');
  });
});

// ===========================================================================
// getResourceNodeName
// ===========================================================================

describe('getResourceNodeName branches', () => {
  it('prefers proxmox.nodeName', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      proxmox: { nodeName: 'n1', node: 'shadow' },
      parentName: 'p',
      identity: { hostname: 'h' },
    });
    expect(getResourceNodeName(resource)).toBe('n1');
  });

  it('falls back to proxmox.node when nodeName is absent', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      proxmox: { node: 'n2' },
      parentName: 'p',
    });
    expect(getResourceNodeName(resource)).toBe('n2');
  });

  it('falls back to parentName when no proxmox node field is set', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      parentName: 'p1',
      identity: { hostname: 'h' },
    });
    expect(getResourceNodeName(resource)).toBe('p1');
  });

  it('falls back to identity.hostname when no node/parent is set', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      identity: { hostname: 'h1' },
    });
    expect(getResourceNodeName(resource)).toBe('h1');
  });

  it('falls back to resource.name when nothing else is set', () => {
    const resource = makeResource({ id: 'fallback-id', type: 'vm' });
    // makeResource defaults name === id.
    expect(getResourceNodeName(resource)).toBe('fallback-id');
  });
});

// ===========================================================================
// getResourceLastBackup
// ===========================================================================

describe('getResourceLastBackup branches', () => {
  it('returns null when platformData is absent', () => {
    const resource = makeResource({ id: 'r', type: 'vm' });
    expect(getResourceLastBackup(resource)).toBeNull();
  });

  it('returns null when platformData.proxmox is not a record', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      platformData: { proxmox: 'not-a-record' },
    });
    expect(getResourceLastBackup(resource)).toBeNull();
  });

  it('returns the lastBackup string verbatim', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      platformData: { proxmox: { lastBackup: '2024-01-01T00:00:00Z' } },
    });
    expect(getResourceLastBackup(resource)).toBe('2024-01-01T00:00:00Z');
  });

  it('returns the lastBackup number verbatim', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      platformData: { proxmox: { lastBackup: 1_700_000_000_000 } },
    });
    expect(getResourceLastBackup(resource)).toBe(1_700_000_000_000);
  });

  it('returns null when lastBackup is a boolean', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      platformData: { proxmox: { lastBackup: true } },
    });
    expect(getResourceLastBackup(resource)).toBeNull();
  });

  it('returns null when lastBackup is an object', () => {
    const resource = makeResource({
      id: 'r',
      type: 'vm',
      platformData: { proxmox: { lastBackup: { ts: 1 } } },
    });
    expect(getResourceLastBackup(resource)).toBeNull();
  });
});

// ===========================================================================
// buildProxmoxPageModel
// ===========================================================================

describe('buildProxmoxPageModel empty input', () => {
  it('returns a fully empty model', () => {
    expect(buildProxmoxPageModel([])).toEqual({
      resources: [],
      pveNodes: [],
      guests: [],
      storage: [],
      pbs: [],
      pmg: [],
      ceph: [],
      physicalDisks: [],
      clusterGroups: [],
      summary: {
        clusterCount: 0,
        nodeCount: 0,
        guestCount: 0,
        runningGuestCount: 0,
        degradedGuestCount: 0,
        stoppedGuestCount: 0,
        storageCount: 0,
        pbsCount: 0,
        pmgCount: 0,
        cephCount: 0,
        alertCount: 0,
      },
    });
  });
});

describe('buildProxmoxPageModel estate classification, status counts, and alert sum', () => {
  const nodeA = makeResource({
    id: 'node-a',
    type: 'agent',
    proxmox: { nodeName: 'node-a', clusterName: 'cluster-x' },
    alerts: [alert()],
    incidentCount: 2,
  });

  const guests = [
    makeResource({
      id: 'vm-r',
      type: 'vm',
      status: 'running',
      proxmox: { vmid: 1, nodeName: 'node-a' },
    }),
    makeResource({
      id: 'vm-o',
      type: 'vm',
      status: 'online',
      proxmox: { vmid: 2, nodeName: 'node-a' },
    }),
    makeResource({
      id: 'vm-d',
      type: 'vm',
      status: 'degraded',
      proxmox: { vmid: 3, nodeName: 'node-a' },
    }),
    makeResource({
      id: 'ct-w',
      type: 'system-container',
      status: 'warning',
      proxmox: { vmid: 4, nodeName: 'node-a' },
    }),
    makeResource({
      id: 'vm-off',
      type: 'vm',
      status: 'offline',
      proxmox: { vmid: 5, nodeName: 'node-a' },
    }),
    makeResource({
      id: 'vm-st',
      type: 'vm',
      status: 'stopped',
      proxmox: { vmid: 6, nodeName: 'node-a' },
    }),
  ];

  it('classifies resources, excludes non-proxmox, and aggregates summary + alerts', () => {
    const model = buildProxmoxPageModel([
      makeResource({ id: 'dh', type: 'docker-host', platformType: 'docker' }),
      nodeA,
      ...guests,
    ]);

    expect(model.resources.map((r) => r.id)).not.toContain('dh');
    expect(model.pveNodes.map((r) => r.id)).toEqual(['node-a']);
    expect(model.guests.map((r) => r.id)).toEqual([
      'vm-r',
      'vm-o',
      'vm-d',
      'ct-w',
      'vm-off',
      'vm-st',
    ]);
    expect(model.summary).toMatchObject({
      guestCount: 6,
      runningGuestCount: 2,
      degradedGuestCount: 2,
      stoppedGuestCount: 2,
      nodeCount: 1,
      clusterCount: 1,
      alertCount: 3,
    });
  });
});

describe('buildProxmoxPageModel pbs / pmg / ceph / physicalDisks sections', () => {
  it('routes resources into the pbs, pmg, ceph, and physicalDisks arrays', () => {
    const model = buildProxmoxPageModel([
      makeResource({ id: 'pbs-1', type: 'pbs', platformType: 'proxmox-pbs' }),
      makeResource({ id: 'pmg-1', type: 'pmg', platformType: 'proxmox-pmg' }),
      makeResource({
        id: 'ceph-typed',
        type: 'ceph',
        platformData: { ceph: { healthStatus: 'healthy' } },
      }),
      makeResource({
        id: 'stor-ceph',
        type: 'storage',
        storage: { isCeph: true },
      }),
      makeResource({
        id: 'disk-1',
        type: 'physical_disk',
        parentName: 'node-a',
      }),
    ]);

    expect(model.pbs.map((r) => r.id)).toEqual(['pbs-1']);
    expect(model.pmg.map((r) => r.id)).toEqual(['pmg-1']);
    // ceph-typed (type ceph) and stor-ceph (storage.isCeph) both count as ceph.
    expect(model.ceph.map((r) => r.id)).toEqual(['ceph-typed', 'stor-ceph']);
    expect(model.physicalDisks.map((r) => r.id)).toEqual(['disk-1']);
    // A Ceph-backed storage is intentionally NOT a Proxmox storage resource.
    expect(model.storage.map((r) => r.id)).not.toContain('stor-ceph');
    // physical_disk is a Proxmox storage type, so it appears in storage too.
    expect(model.storage.map((r) => r.id)).toEqual(['disk-1']);
    expect(model.summary).toMatchObject({
      pbsCount: 1,
      pmgCount: 1,
      cephCount: 2,
      storageCount: 1,
    });
  });
});

describe('buildProxmoxPageModel cluster grouping', () => {
  it('assigns guests and storage to a named cluster via the owning node', () => {
    const nodeA = makeResource({
      id: 'node-a',
      type: 'agent',
      proxmox: { nodeName: 'node-a', clusterName: 'cluster-x' },
    });
    const guestOnNode = makeResource({
      id: 'vm-1',
      type: 'vm',
      status: 'running',
      proxmox: { vmid: 1, nodeName: 'node-a' },
    });
    const storageOnNode = makeResource({
      id: 'stor-1',
      type: 'storage',
      parentName: 'node-a',
      storage: { isCeph: false },
    });

    const model = buildProxmoxPageModel([nodeA, guestOnNode, storageOnNode]);

    expect(model.clusterGroups.map((g) => g.id)).toEqual(['cluster-x']);
    const group = model.clusterGroups[0];
    expect(group.nodes.map((r) => r.id)).toEqual(['node-a']);
    expect(group.guests.map((r) => r.id)).toEqual(['vm-1']);
    expect(group.storage.map((r) => r.id)).toEqual(['stor-1']);
  });

  it('drops orphan guests/storage into the standalone bucket using their own cluster label', () => {
    const nodeA = makeResource({
      id: 'node-a',
      type: 'agent',
      proxmox: { nodeName: 'node-a', clusterName: 'cluster-x' },
    });
    const orphanGuest = makeResource({
      id: 'vm-orphan',
      type: 'vm',
      status: 'running',
      proxmox: { vmid: 9 },
      parentName: 'ghost-node',
    });
    const orphanStorage = makeResource({
      id: 'stor-orphan',
      type: 'storage',
      parentName: 'ghost-node',
      storage: {},
    });

    const model = buildProxmoxPageModel([nodeA, orphanGuest, orphanStorage]);

    expect(model.clusterGroups.map((g) => g.id)).toEqual(['cluster-x', '__standalone__']);
    const standalone = model.clusterGroups[1];
    expect(standalone.id).toBe('__standalone__');
    expect(standalone.label).toBe('Standalone');
    expect(standalone.nodes).toEqual([]);
    expect(standalone.guests.map((r) => r.id)).toEqual(['vm-orphan']);
    expect(standalone.storage.map((r) => r.id)).toEqual(['stor-orphan']);
    // Standalone never counts as a real cluster.
    expect(model.summary.clusterCount).toBe(1);
  });

  it('sorts named clusters by label and keeps standalone last', () => {
    const nodeBravo = makeResource({
      id: 'node-bravo',
      type: 'agent',
      proxmox: { nodeName: 'node-bravo', clusterName: 'bravo' },
    });
    const nodeAlpha = makeResource({
      id: 'node-alpha',
      type: 'agent',
      proxmox: { nodeName: 'node-alpha', clusterName: 'alpha' },
    });
    const orphanGuest = makeResource({
      id: 'vm-solo',
      type: 'vm',
      status: 'running',
      proxmox: { vmid: 1 },
      parentName: 'solo-node',
    });

    const model = buildProxmoxPageModel([nodeBravo, nodeAlpha, orphanGuest]);

    expect(model.clusterGroups.map((g) => g.label)).toEqual(['alpha', 'bravo', 'Standalone']);
    expect(model.clusterGroups.map((g) => g.id)).toEqual(['alpha', 'bravo', '__standalone__']);
  });
});
