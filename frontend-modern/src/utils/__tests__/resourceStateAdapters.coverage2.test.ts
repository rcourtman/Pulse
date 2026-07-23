import { describe, expect, it } from 'vitest';

import {
  canonicalizeRealtimeResource,
  mergeCanonicalResource,
  mergeCanonicalResourceSnapshot,
  nodeFromResource,
  pbsInstanceFromResource,
  pmgInstanceFromResource,
} from '../resourceStateAdapters';
import type { Resource } from '@/types/resource';

// ---------------------------------------------------------------------------
// Fixture builders — minimal resources with sensible defaults.  Tests override
// only the fields relevant to the branch under examination.
// ---------------------------------------------------------------------------

const agent = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-1',
    type: 'agent',
    name: 'agent-name',
    displayName: 'Agent',
    platformId: 'agent-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

/** Agent resource pre-loaded with empty platformData so proxmox/memory/disk
 *  facets on the resource itself are picked up by nodeFromResource. */
const agentNode = (overrides: Partial<Resource> = {}): Resource =>
  agent({
    name: 'node-1',
    displayName: 'Node 1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    cpu: { current: 10 },
    memory: { current: 20, total: 1024, used: 256 },
    disk: { current: 30, total: 2048, used: 512 },
    platformData: {},
    ...overrides,
  });

const pbsResource = (
  pbsFacet: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: 'pbs-1',
    type: 'pbs',
    name: 'pbs-name',
    displayName: 'PBS',
    platformId: 'pbs-1',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    platformData: { pbs: pbsFacet },
    ...overrides,
  }) as Resource;

const pmgResource = (
  pmgFacet: Record<string, unknown>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id: 'pmg-1',
    type: 'pmg',
    name: 'pmg-name',
    displayName: 'PMG',
    platformId: 'pmg-1',
    platformType: 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    cpu: { current: 0 },
    memory: { current: 0, total: 0, used: 0 },
    disk: { current: 0, total: 0, used: 0 },
    platformData: { pmg: pmgFacet },
    ...overrides,
  }) as Resource;

/** Shorthand to read canonicalized platformData as a plain record. */
const pd = (r: Resource): Record<string, unknown> =>
  (r.platformData ?? {}) as Record<string, unknown>;

// ===================================================================
// asBoolean — reached through nodeFromResource temperatureMonitoringEnabled
// and isClusterMember, and through buildTemperature available/hasCPU/etc.
// ===================================================================

describe('asBoolean (via nodeFromResource)', () => {
  it('returns true when temperatureMonitoringEnabled is explicitly true on platformData', () => {
    const node = nodeFromResource(
      agentNode({ platformData: { temperatureMonitoringEnabled: true } }),
    );
    expect(node?.temperatureMonitoringEnabled).toBe(true);
  });

  it('returns false (not null) when temperatureMonitoringEnabled is explicitly false', () => {
    const node = nodeFromResource(
      agentNode({ platformData: { temperatureMonitoringEnabled: false } }),
    );
    expect(node?.temperatureMonitoringEnabled).toBe(false);
  });

  it('falls through to proxmox facet then null when platform value is a non-boolean string', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {
          temperatureMonitoringEnabled: 'yes' as unknown as boolean,
          proxmox: { temperatureMonitoringEnabled: true },
        },
      }),
    );
    // asBoolean('yes') → undefined, ?? falls to proxmox → asBoolean(true) → true
    expect(node?.temperatureMonitoringEnabled).toBe(true);
  });

  it('returns null when no temperatureMonitoringEnabled is present anywhere', () => {
    const node = nodeFromResource(agentNode());
    expect(node?.temperatureMonitoringEnabled).toBeNull();
  });

  it('returns the raw boolean for isClusterMember true and false', () => {
    const trueNode = nodeFromResource(
      agentNode({ proxmox: { isClusterMember: true } as unknown as Resource['proxmox'] }),
    );
    expect(trueNode?.isClusterMember).toBe(true);

    const falseNode = nodeFromResource(
      agentNode({ proxmox: { isClusterMember: false } as unknown as Resource['proxmox'] }),
    );
    expect(falseNode?.isClusterMember).toBe(false);
  });

  it('returns undefined for isClusterMember when absent', () => {
    const node = nodeFromResource(agentNode());
    expect(node?.isClusterMember).toBeUndefined();
  });
});

// ===================================================================
// getCanonicalPlatformId — reached through nodeFromResource.instance
// ===================================================================

describe('getCanonicalPlatformId (via nodeFromResource.instance)', () => {
  it('uses canonical identity platformId for instance when proxmox.instance and platformId are absent', () => {
    const node = nodeFromResource(
      agentNode({
        platformId: '',
        proxmox: {} as unknown as Resource['proxmox'],
        canonicalIdentity: { platformId: 'canonical-pve-1', hostname: 'host' },
      }),
    );
    expect(node?.instance).toBe('canonical-pve-1');
  });

  it('trims whitespace from canonical identity platformId', () => {
    const node = nodeFromResource(
      agentNode({
        platformId: '',
        proxmox: {} as unknown as Resource['proxmox'],
        canonicalIdentity: { platformId: '  trimmed-id  ', hostname: 'host' },
      }),
    );
    expect(node?.instance).toBe('trimmed-id');
  });

  it('falls through to preferredHostLabel when canonical identity platformId is whitespace-only', () => {
    const node = nodeFromResource(
      agentNode({
        id: 'fallback-id',
        platformId: '',
        proxmox: {} as unknown as Resource['proxmox'],
        canonicalIdentity: { platformId: '   ', hostname: 'host' },
      }),
    );
    // platformId empty, canonical platformId whitespace-only → falls to preferredHostLabel
    expect(node?.instance).not.toBe('   ');
  });

  it('falls through to preferredHostLabel when canonicalIdentity is absent', () => {
    const node = nodeFromResource(
      agentNode({
        platformId: '',
        proxmox: {} as unknown as Resource['proxmox'],
      }),
    );
    // canonicalIdentity absent → getCanonicalPlatformId returns undefined
    // → falls through to preferredHostLabel (hostname resolver / displayName / id)
    expect(node?.instance).not.toBe('');
    expect(node?.instance).toBe(node?.name);
  });
});

// ===================================================================
// normalizeResourceIdentityToken + getHostResourceMergeKey — reached
// through coalesceRealtimeResourceSnapshot (via mergeCanonicalResourceSnapshot)
// ===================================================================

describe('normalizeResourceIdentityToken + getHostResourceMergeKey (via mergeCanonicalResourceSnapshot)', () => {
  it('lowercases and trims canonical hostname to form the agent merge key', () => {
    const merged = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'a',
          type: 'agent',
          name: 'Host',
          displayName: 'Host',
          platformId: 'host',
          platformType: 'agent',
          sourceType: 'agent',
          sources: ['agent', 'proxmox'],
          status: 'online',
          lastSeen: 100,
          canonicalIdentity: { hostname: 'HOST.LOCAL', platformId: 'host' },
        } as Resource,
        {
          id: 'b',
          type: 'agent',
          name: 'Host',
          displayName: 'Host',
          platformId: 'host',
          platformType: 'proxmox-pve',
          sourceType: 'api',
          sources: ['proxmox'],
          status: 'online',
          lastSeen: 200,
          canonicalIdentity: { hostname: 'host.local', platformId: 'host' },
        } as Resource,
      ],
      [],
    );
    // Both normalize to 'host.local' → same merge key → coalesced into one
    expect(merged).toHaveLength(1);
  });

  it('returns undefined merge key when all identity candidates are empty, keeping resources separate', () => {
    const resources = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'empty-1',
          type: 'agent',
          name: '',
          displayName: '',
          platformId: '',
          platformType: 'agent',
          sourceType: 'agent',
          status: 'online',
          lastSeen: 100,
        } as Resource,
        {
          id: 'empty-2',
          type: 'agent',
          name: '',
          displayName: '',
          platformId: '',
          platformType: 'agent',
          sourceType: 'agent',
          status: 'online',
          lastSeen: 200,
        } as Resource,
      ],
      [],
    );
    // No host key derivable → both stay separate
    expect(resources).toHaveLength(2);
  });
});

// ===================================================================
// sourceListHas + normalizeSourceToken — reached through many paths.
// normalizeSourceToken tries normalizeSourcePlatformKey first, then falls
// back to trim/lowercase.
// ===================================================================

describe('sourceListHas + normalizeSourceToken (via canonicalizeRealtimeResource)', () => {
  it('matches "proxmox" alias to "proxmox-pve" through normalizeSourcePlatformKey', () => {
    // Resource with sources: ['proxmox'] should resolve platformType to 'proxmox-pve'
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'generic',
        sourceType: 'api',
        sources: ['proxmox'],
      }),
    );
    expect(result.platformType).toBe('proxmox-pve');
  });

  it('matches unknown source tokens through the trim/lowercase fallback', () => {
    // 'Custom-Source' normalizes via fallback (not in platform key map)
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'generic',
        sourceType: 'api',
        sources: ['Custom-Source'],
      }),
    );
    // Unknown source → resolvePlatformTypeFromSources returns undefined → keeps 'generic'
    expect(result.platformType).toBe('generic');
    // But the source is preserved in platformData
    expect(pd(result).sources).toEqual(['Custom-Source']);
  });
});

// ===================================================================
// deriveLegacySourceList — reached through canonicalizeRealtimeResource
// (via getCanonicalSourceList and canonicalizeLegacyPlatformData).
// Exercises the switch/case and facet-detection branches.
// ===================================================================

describe('deriveLegacySourceList (via canonicalizeRealtimeResource)', () => {
  it('returns explicit resource sources when present, overriding everything else', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'hybrid',
        sources: ['docker'],
      }),
    );
    // Explicit sources override platformType-based derivation
    expect(pd(result).sources).toEqual(['docker']);
    expect(result.platformType).toBe('docker');
  });

  it('derives ["proxmox","agent"] for proxmox-pve hybrid', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pve', sourceType: 'hybrid' }),
    );
    expect(pd(result).sources).toEqual(['proxmox', 'agent']);
    expect(result.platformType).toBe('proxmox-pve');
    expect(result.sourceType).toBe('hybrid');
  });

  it('derives ["proxmox"] for proxmox-pve non-hybrid', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pve', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['proxmox']);
    expect(result.sourceType).toBe('api');
  });

  it('derives ["docker"] for docker platformType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'docker', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['docker']);
    expect(result.platformType).toBe('docker');
  });

  it('derives ["agent","kubernetes"] for kubernetes hybrid', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'kubernetes', sourceType: 'hybrid' }),
    );
    expect(pd(result).sources).toEqual(['agent', 'kubernetes']);
  });

  it('derives ["kubernetes"] for kubernetes non-hybrid', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'kubernetes', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['kubernetes']);
  });

  it('derives ["pbs"] for proxmox-pbs platformType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pbs', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['pbs']);
  });

  it('derives ["pmg"] for proxmox-pmg platformType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pmg', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['pmg']);
  });

  it('derives ["truenas"] for truenas platformType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'truenas', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['truenas']);
  });

  it('derives ["vmware"] for vmware-vsphere platformType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'vmware-vsphere', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['vmware']);
  });

  it('derives ["agent"] for default platformType with sourceType agent', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'generic', sourceType: 'agent' }),
    );
    expect(pd(result).sources).toEqual(['agent']);
    expect(result.platformType).toBe('agent');
  });

  it('returns undefined sources for default platformType with non-agent sourceType', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'generic', sourceType: 'api' }),
    );
    // No derivable sources → platformData stays undefined
    expect(result.platformData).toBeUndefined();
    expect(result.platformType).toBe('generic');
  });

  it('returns ["availability"] for network-endpoint type', () => {
    const result = canonicalizeRealtimeResource({
      id: 'ne-1',
      type: 'network-endpoint',
      name: 'endpoint',
      displayName: 'Endpoint',
      platformId: 'ne-1',
      platformType: 'generic',
      sourceType: 'api',
      status: 'online',
      lastSeen: 1_700_000_000_000,
    } as Resource);
    expect(pd(result).sources).toEqual(['availability']);
    expect(result.platformType).toBe('availability');
  });

  it('returns ["availability"] when platformType is "availability"', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'availability', sourceType: 'api' }),
    );
    expect(pd(result).sources).toEqual(['availability']);
  });

  it('detects facet records on the resource to derive multi-source lists', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        proxmox: { nodeName: 'n1' } as unknown as Resource['proxmox'],
        docker: { runtime: 'docker' } as unknown as Resource['docker'],
        agent: { hostname: 'h1' } as unknown as Resource['agent'],
      }),
    );
    // proxmox, docker (with evidence), and agent facets all detected
    expect(pd(result).sources).toEqual(['proxmox', 'docker', 'agent']);
  });
});

// ===================================================================
// canonicalizeLegacyPlatformData — reached through canonicalizeRealtimeResource
// ===================================================================

describe('canonicalizeLegacyPlatformData (via canonicalizeRealtimeResource)', () => {
  it('returns undefined platformData when no sources can be derived and platformData is absent', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'generic', sourceType: 'api', platformData: undefined }),
    );
    expect(result.platformData).toBeUndefined();
  });

  it('deletes docker facet without evidence from existing platformData', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { docker: {} },
      }),
    );
    // Empty docker → no evidence → deleted
    expect(pd(result).docker).toBeUndefined();
  });

  it('builds agent payload from legacy platformData fields', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: {
          agentId: 'legacy-agent',
          hostname: 'legacy-host',
          memory: { total: 4096 },
          interfaces: [{ name: 'eth0' }],
        },
      }),
    );
    const agentFacet = pd(result).agent as Record<string, unknown> | undefined;
    expect(agentFacet?.agentId).toBe('legacy-agent');
    expect(agentFacet?.hostname).toBe('legacy-host');
    expect(agentFacet?.memory).toEqual({ total: 4096 });
    expect(agentFacet?.networkInterfaces).toEqual([{ name: 'eth0' }]);
  });

  it('builds docker payload from legacy platformData fields', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'docker',
        sourceType: 'api',
        platformData: {
          runtime: 'docker',
          dockerVersion: '24.0',
          containerCount: 5,
          swarm: { nodeId: 'node-1' },
        },
      }),
    );
    const dockerFacet = pd(result).docker as Record<string, unknown> | undefined;
    expect(dockerFacet?.runtime).toBe('docker');
    expect(dockerFacet?.dockerVersion).toBe('24.0');
    expect(dockerFacet?.containerCount).toBe(5);
    expect(dockerFacet?.swarm).toEqual({ nodeId: 'node-1' });
  });

  it('builds proxmox payload from legacy shape (vmid present)', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'api',
        platformData: {
          node: 'pve-1',
          instance: 'cluster-a',
          vmid: 100,
          cpus: 4,
        },
      }),
    );
    const proxmoxFacet = pd(result).proxmox as Record<string, unknown> | undefined;
    expect(proxmoxFacet?.nodeName).toBe('pve-1');
    expect(proxmoxFacet?.instance).toBe('cluster-a');
    expect(proxmoxFacet?.vmid).toBe(100);
    expect(proxmoxFacet?.cpus).toBe(4);
  });

  it('builds pbs payload from legacy fields (host, version, numDatastores)', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        platformData: {
          host: 'pbs.local',
          version: '3.2',
          connectionHealth: 'healthy',
          numDatastores: 3,
        },
      }),
    );
    const pbsFacet = pd(result).pbs as Record<string, unknown> | undefined;
    expect(pbsFacet?.hostname).toBe('pbs.local');
    expect(pbsFacet?.version).toBe('3.2');
    expect(pbsFacet?.connectionHealth).toBe('healthy');
    expect(pbsFacet?.datastoreCount).toBe(3);
  });

  it('builds pmg payload from legacy fields', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pmg',
        sourceType: 'api',
        platformData: {
          host: 'pmg.local',
          version: '7.3',
          nodeCount: 2,
          queueActive: 5,
          queueTotal: 10,
        },
      }),
    );
    const pmgFacet = pd(result).pmg as Record<string, unknown> | undefined;
    expect(pmgFacet?.hostname).toBe('pmg.local');
    expect(pmgFacet?.version).toBe('7.3');
    expect(pmgFacet?.nodeCount).toBe(2);
    expect(pmgFacet?.queueActive).toBe(5);
    expect(pmgFacet?.queueTotal).toBe(10);
  });

  it('builds kubernetes payload from legacy fields', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'kubernetes',
        sourceType: 'api',
        platformData: {
          clusterId: 'k8s-1',
          clusterName: 'my-cluster',
          nodeName: 'worker-1',
          context: 'default',
        },
      }),
    );
    const k8sFacet = pd(result).kubernetes as Record<string, unknown> | undefined;
    expect(k8sFacet?.clusterId).toBe('k8s-1');
    expect(k8sFacet?.clusterName).toBe('my-cluster');
    expect(k8sFacet?.nodeName).toBe('worker-1');
    expect(k8sFacet?.context).toBe('default');
  });
});

// ===================================================================
// hasAvailabilityFacet — reached through canonicalizeRealtimeResource
// ===================================================================

describe('hasAvailabilityFacet (via canonicalizeRealtimeResource)', () => {
  it('is true when resource.availability meta is set (truthy but not a record)', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'generic',
        sourceType: 'api',
        availability: 'enabled' as unknown as Resource['availability'],
      }),
    );
    // hasAvailabilityFacet true → platformType becomes 'availability'
    expect(result.platformType).toBe('availability');
  });

  it('is true when platformData.availability record exists', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'generic',
        sourceType: 'api',
        platformData: { availability: { targetId: 'test' } },
      }),
    );
    expect(result.platformType).toBe('availability');
  });

  it('is true when sources contain "availability"', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'generic',
        sourceType: 'api',
        sources: ['availability'],
      }),
    );
    expect(result.platformType).toBe('availability');
  });

  it('is false → preserves the original platformType when no availability signal exists', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'generic', sourceType: 'api' }),
    );
    expect(result.platformType).toBe('generic');
  });
});

// ===================================================================
// buildMemory — reached through nodeFromResource
// ===================================================================

describe('buildMemory (via nodeFromResource)', () => {
  it('uses metric values directly when all fields are present', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { current: 50, total: 1000, used: 500, free: 400 },
      }),
    );
    expect(node?.memory.total).toBe(1000);
    expect(node?.memory.used).toBe(500);
    expect(node?.memory.free).toBe(400);
    expect(node?.memory.usage).toBe(50);
    expect(node?.memory.cache).toBeUndefined();
  });

  it('uses fallback record from proxmox.memory when metric is absent', () => {
    const node = nodeFromResource(
      agentNode({
        memory: undefined,
        proxmox: {
          nodeName: 'n1',
          memory: { total: 2000, used: 1000, free: 800, cache: 200, usage: 50, balloon: 30 },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.total).toBe(2000);
    expect(node?.memory.used).toBe(1000);
    expect(node?.memory.free).toBe(800);
    expect(node?.memory.usage).toBe(50);
    expect(node?.memory.cache).toBe(200);
    expect(node?.memory.balloon).toBe(30);
  });

  it('preserves explicit unavailable usage from the fallback memory contract', () => {
    const node = nodeFromResource(
      agentNode({
        memory: undefined,
        proxmox: {
          nodeName: 'n1',
          memory: {
            total: 8192,
            used: 0,
            free: 0,
            usage: 0,
            usageUnavailable: true,
          },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory).toMatchObject({
      total: 8192,
      used: 0,
      usage: 0,
      usageUnavailable: true,
    });
  });

  it('lets a trusted merged metric override an unavailable Proxmox fallback', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { current: 50, total: 8192, used: 4096, free: 4096 },
        proxmox: {
          nodeName: 'n1',
          memory: {
            total: 8192,
            used: 0,
            free: 0,
            usage: 0,
            usageUnavailable: true,
          },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory).toMatchObject({
      total: 8192,
      used: 4096,
      usage: 50,
      usageUnavailable: false,
    });
  });

  it('prefers proxmoxMeta.memoryCache over fallback.cache', () => {
    const node = nodeFromResource(
      agentNode({
        memory: undefined,
        proxmox: {
          nodeName: 'n1',
          memoryCache: 150,
          memory: { cache: 200 },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.cache).toBe(150);
  });

  it('computes free as max(total - used - cache, 0) when neither metric.free nor fallback.free exist', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { total: 1000, used: 800 } as unknown as Resource['memory'],
        proxmox: {
          nodeName: 'n1',
          memoryCache: 100,
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.free).toBe(100); // 1000 - 800 - 100
  });

  it('clamps computed free to zero when used + cache exceeds total', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { total: 1000, used: 1200 } as unknown as Resource['memory'],
        proxmox: {
          nodeName: 'n1',
          memoryCache: 100,
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.free).toBe(0); // max(-300, 0)
  });

  it('computes usage from used/total ratio when metric.current is absent', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { total: 1024, used: 256 } as unknown as Resource['memory'],
      }),
    );
    expect(node?.memory.usage).toBe(25); // (256/1024)*100
  });

  it('uses fallback.usage when total is 0 and metric.current is absent', () => {
    const node = nodeFromResource(
      agentNode({
        memory: undefined,
        proxmox: {
          nodeName: 'n1',
          memory: { total: 0, used: 0, usage: 42 },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.usage).toBe(42);
  });

  it('returns cache as undefined when cache is 0', () => {
    const node = nodeFromResource(
      agentNode({
        memory: { total: 100, used: 50 } as unknown as Resource['memory'],
      }),
    );
    expect(node?.memory.cache).toBeUndefined();
  });

  it('prefers proxmoxMeta.swapUsed over fallback.swapUsed', () => {
    const node = nodeFromResource(
      agentNode({
        memory: undefined,
        proxmox: {
          nodeName: 'n1',
          swapUsed: 10,
          swapTotal: 20,
          memory: { swapUsed: 99, swapTotal: 99 },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.memory.swapUsed).toBe(10);
    expect(node?.memory.swapTotal).toBe(20);
  });
});

// ===================================================================
// buildDisk — reached through nodeFromResource
// ===================================================================

describe('buildDisk (via nodeFromResource)', () => {
  it('uses metric values directly when all fields are present', () => {
    const node = nodeFromResource(
      agentNode({
        disk: { current: 40, total: 2000, used: 800, free: 1200 },
      }),
    );
    expect(node?.disk.total).toBe(2000);
    expect(node?.disk.used).toBe(800);
    expect(node?.disk.free).toBe(1200);
    expect(node?.disk.usage).toBe(40);
  });

  it('uses fallback record from proxmox.disk when metric is absent', () => {
    const node = nodeFromResource(
      agentNode({
        disk: undefined,
        proxmox: {
          nodeName: 'n1',
          disk: {
            total: 4000,
            used: 1000,
            free: 3000,
            usage: 25,
            mountpoint: '/',
            type: 'ext4',
            device: '/dev/sda1',
          },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.disk.total).toBe(4000);
    expect(node?.disk.usage).toBe(25);
    expect(node?.disk.mountpoint).toBe('/');
    expect(node?.disk.type).toBe('ext4');
    expect(node?.disk.device).toBe('/dev/sda1');
  });

  it('computes free as max(total - used, 0) and clamps when over', () => {
    const node = nodeFromResource(
      agentNode({
        disk: { total: 500, used: 800 } as unknown as Resource['disk'],
      }),
    );
    expect(node?.disk.free).toBe(0);
    expect(node?.disk.usage).toBe(160); // (800/500)*100
  });

  it('computes usage from ratio when metric.current is absent', () => {
    const node = nodeFromResource(
      agentNode({
        disk: { total: 2048, used: 512 } as unknown as Resource['disk'],
      }),
    );
    expect(node?.disk.usage).toBe(25);
  });

  it('returns undefined mountpoint/type/device when fallback has none', () => {
    const node = nodeFromResource(
      agentNode({
        disk: { total: 100, used: 50 } as unknown as Resource['disk'],
      }),
    );
    expect(node?.disk.mountpoint).toBeUndefined();
    expect(node?.disk.type).toBeUndefined();
    expect(node?.disk.device).toBeUndefined();
  });
});

// ===================================================================
// buildTemperature — reached through nodeFromResource
// ===================================================================

describe('buildTemperature (via nodeFromResource)', () => {
  it('maps a full temperature record from platformData.temperature', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {
          temperature: {
            available: true,
            cpuPackage: 55,
            cpuMax: 60,
            cpuMin: 40,
            cpuMaxRecord: 65,
            minRecorded: '2026-01-01T00:00:00Z',
            maxRecorded: '2026-06-01T00:00:00Z',
            lastUpdate: '2026-07-01T00:00:00Z',
            hasGPU: true,
            hasNVMe: false,
          },
        },
      }),
    );
    expect(node?.temperature?.cpuPackage).toBe(55);
    expect(node?.temperature?.cpuMax).toBe(60);
    expect(node?.temperature?.cpuMin).toBe(40);
    expect(node?.temperature?.cpuMaxRecord).toBe(65);
    expect(node?.temperature?.available).toBe(true);
    expect(node?.temperature?.hasGPU).toBe(true);
    expect(node?.temperature?.hasNVMe).toBe(false);
    expect(node?.temperature?.lastUpdate).toBe('2026-07-01T00:00:00Z');
  });

  it('reads cpuPackage from the "temperature" sub-field when cpuPackage is absent', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: { temperature: { available: true, temperature: 45 } },
      }),
    );
    expect(node?.temperature?.cpuPackage).toBe(45);
  });

  it('reads cpuPackage from the "cpu" sub-field as last resort', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: { temperature: { available: true, cpu: 48 } },
      }),
    );
    expect(node?.temperature?.cpuPackage).toBe(48);
  });

  it('defaults available to true and derives hasCPU when only cpuPackage is present', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: { temperature: { cpuPackage: 50 } },
      }),
    );
    expect(node?.temperature?.available).toBe(true);
    expect(node?.temperature?.hasCPU).toBe(true);
  });

  it('falls back to nodeMeta (proxmox) temperature when platformData.temperature is absent', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {},
        proxmox: {
          nodeName: 'n1',
          temperature: { available: true, cpuPackage: 42 },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.temperature?.cpuPackage).toBe(42);
  });

  it('returns undefined when available is false and no cpuPackage exists, with no numeric resource.temperature', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: { temperature: { available: false } },
      }),
    );
    expect(node?.temperature).toBeUndefined();
  });

  it('falls back to top-level numeric resource.temperature when raw record has no usable data', () => {
    const node = nodeFromResource(
      agentNode({
        temperature: 42,
        platformData: { temperature: { available: false } },
      }),
    );
    expect(node?.temperature?.cpuPackage).toBe(42);
    expect(node?.temperature?.cpuMax).toBe(42);
    expect(node?.temperature?.cpuMin).toBe(42);
    expect(node?.temperature?.available).toBe(true);
    expect(node?.temperature?.hasCPU).toBe(true);
  });

  it('filters invalid core entries (missing core or temp) and keeps valid ones', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {
          temperature: {
            available: true,
            cores: [
              { core: 0, temp: 45 },
              { core: 1 },
              { temp: 50 },
              null,
              'bad',
              { core: 2, temp: 55 },
            ],
          },
        },
      }),
    );
    expect(node?.temperature?.cores).toEqual([
      { core: 0, temp: 45 },
      { core: 2, temp: 55 },
    ]);
  });

  it('filters invalid gpu entries (missing device) and keeps valid ones', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {
          temperature: {
            available: true,
            gpu: [
              { device: 'gpu0', edge: 50, junction: 60 },
              { edge: 40 },
              null,
              { device: 'gpu1', mem: 55 },
            ],
          },
        },
      }),
    );
    expect(node?.temperature?.gpu).toEqual([
      { device: 'gpu0', edge: 50, junction: 60 },
      { device: 'gpu1', mem: 55 },
    ]);
  });

  it('filters invalid nvme entries (missing device or temp) and keeps valid ones', () => {
    const node = nodeFromResource(
      agentNode({
        platformData: {
          temperature: {
            available: true,
            nvme: [{ device: 'nvme0', temp: 40 }, { device: 'nvme1' }, { temp: 35 }, null],
          },
        },
      }),
    );
    expect(node?.temperature?.nvme).toEqual([{ device: 'nvme0', temp: 40 }]);
  });
});

// ===================================================================
// mergePlatformData — reached through mergeCanonicalResource
// ===================================================================

describe('mergePlatformData (via mergeCanonicalResource)', () => {
  it('returns existing platformData when incoming has none', () => {
    const merged = mergeCanonicalResource(
      agent({ id: 'r1', platformData: undefined }),
      agent({
        id: 'r1',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { agent: { hostname: 'h' } },
      }),
    );
    const agentFacet = pd(merged).agent as Record<string, unknown> | undefined;
    expect(agentFacet?.hostname).toBe('h');
  });

  it('returns incoming platformData when existing has none', () => {
    const merged = mergeCanonicalResource(
      agent({
        id: 'r1',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { agent: { hostname: 'incoming-h' } },
      }),
      agent({ id: 'r1', platformType: 'generic', sourceType: 'api', platformData: undefined }),
    );
    const agentFacet = pd(merged).agent as Record<string, unknown> | undefined;
    expect(agentFacet?.hostname).toBe('incoming-h');
  });

  it('deletes agent facet from merged when incoming sources do not include agent', () => {
    const merged = mergeCanonicalResource(
      agent({
        id: 'r1',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'], docker: { runtime: 'docker' } },
      }),
      agent({
        id: 'r1',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'], agent: { hostname: 'existing-h' } },
      }),
    );
    // Incoming sources = ['docker'] → agent facet should be deleted
    expect(pd(merged).agent).toBeUndefined();
    expect(pd(merged).docker).toEqual({ runtime: 'docker' });
  });

  it('deletes docker facet from merged when merged nested record has no evidence', () => {
    const merged = mergeCanonicalResource(
      agent({
        id: 'r1',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'], docker: {} },
      }),
      agent({
        id: 'r1',
        platformType: 'docker',
        sourceType: 'api',
        // Existing canonicalized will have no docker (empty docker deleted by canonicalize)
        platformData: { sources: ['docker'] },
      }),
    );
    // Both docker facets lack evidence → deleted from merged
    expect(pd(merged).docker).toBeUndefined();
  });

  it('merges storage and sourceStatus nested records', () => {
    const merged = mergeCanonicalResource(
      agent({
        id: 'r1',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: {
          sources: ['agent'],
          storage: { platform: 'zfs' },
          sourceStatus: { agent: { healthy: true } },
        },
      }),
      agent({
        id: 'r1',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: {
          sources: ['agent'],
          storage: { topology: 'pool' },
          sourceStatus: { proxmox: { healthy: false } },
        },
      }),
    );
    expect(pd(merged).storage).toEqual({ platform: 'zfs', topology: 'pool' });
    expect(pd(merged).sourceStatus).toEqual({
      agent: { healthy: true },
      proxmox: { healthy: false },
    });
  });
});

// ===================================================================
// mergeCanonicalResource — no-existing branch
// ===================================================================

describe('mergeCanonicalResource (no existing)', () => {
  it('canonicalizes the incoming resource when existing is undefined', () => {
    const result = mergeCanonicalResource(
      agent({ platformType: 'proxmox-pve', sourceType: 'hybrid' }),
    );
    expect(result.platformType).toBe('proxmox-pve');
    expect(pd(result).sources).toEqual(['proxmox', 'agent']);
  });
});

// ===================================================================
// shouldMergeRealtimeHostResources + preferHostResourcePrimary +
// withMergedSnapshotSources — reached through mergeCanonicalResourceSnapshot
// ===================================================================

describe('shouldMergeRealtimeHostResources + preferHostResourcePrimary (via mergeCanonicalResourceSnapshot)', () => {
  it('does not merge two docker-sourced agents because union lacks "agent" source', () => {
    const resources = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'docker-a',
          type: 'agent',
          name: 'same-host',
          displayName: 'same-host',
          platformId: 'same-host',
          platformType: 'docker',
          sourceType: 'api',
          sources: ['docker'],
          status: 'online',
          lastSeen: 100,
          canonicalIdentity: { hostname: 'same-host', platformId: 'same-host' },
        } as Resource,
        {
          id: 'docker-b',
          type: 'agent',
          name: 'same-host',
          displayName: 'same-host',
          platformId: 'same-host',
          platformType: 'docker',
          sourceType: 'api',
          sources: ['docker'],
          status: 'online',
          lastSeen: 200,
          canonicalIdentity: { hostname: 'same-host', platformId: 'same-host' },
        } as Resource,
      ],
      [],
    );
    // shouldMergeRealtimeHostResources: union has 'docker' (runtime) but not 'agent' → false
    expect(resources).toHaveLength(2);
  });

  it('uses lastSeen as tiebreaker when both merged agents have agent sources', () => {
    const merged = mergeCanonicalResourceSnapshot(
      [
        {
          id: 'older',
          type: 'agent',
          name: 'tie-host',
          displayName: 'tie-host',
          platformId: 'tie-host',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          sources: ['agent', 'proxmox'],
          status: 'online',
          lastSeen: 1000,
          canonicalIdentity: { hostname: 'tie-host', platformId: 'tie-host' },
          proxmox: { nodeName: 'tie-host' } as unknown as Resource['proxmox'],
        } as Resource,
        {
          id: 'newer',
          type: 'agent',
          name: 'tie-host',
          displayName: 'tie-host',
          platformId: 'tie-host',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          sources: ['agent', 'proxmox'],
          status: 'online',
          lastSeen: 2000,
          canonicalIdentity: { hostname: 'tie-host', platformId: 'tie-host' },
          proxmox: { nodeName: 'tie-host' } as unknown as Resource['proxmox'],
        } as Resource,
      ],
      [],
    );
    // Both have agent + runtime → merge. Both have agent sources → lastSeen tiebreaker.
    // 'newer' (lastSeen 2000) >= 'older' (1000) → primary = newer
    expect(merged).toHaveLength(1);
    expect(merged[0].id).toBe('newer');
  });
});

// ===================================================================
// mapPMGQuarantine — reached through pmgInstanceFromResource
// ===================================================================

describe('mapPMGQuarantine (via pmgInstanceFromResource)', () => {
  it('maps a well-formed quarantine totals record', () => {
    const instance = pmgInstanceFromResource(
      pmgResource({ quarantine: { spam: 10, virus: 3, attachment: 2, blacklisted: 1 } }),
    );
    expect(instance?.quarantine).toEqual({
      spam: 10,
      virus: 3,
      attachment: 2,
      blacklisted: 1,
    });
  });

  it('defaults every count to 0 for an empty quarantine record', () => {
    const instance = pmgInstanceFromResource(pmgResource({ quarantine: {} }));
    expect(instance?.quarantine).toEqual({
      spam: 0,
      virus: 0,
      attachment: 0,
      blacklisted: 0,
    });
  });

  it('returns undefined quarantine when the value is not a record', () => {
    const instance = pmgInstanceFromResource(pmgResource({ quarantine: 'not-a-record' }));
    expect(instance?.quarantine).toBeUndefined();
  });

  it('returns undefined quarantine when absent from the pmg facet', () => {
    const instance = pmgInstanceFromResource(pmgResource({}));
    expect(instance?.quarantine).toBeUndefined();
  });
});

// ===================================================================
// pmgInstanceFromResource — mailStats fallback and identity branches
// ===================================================================

describe('pmgInstanceFromResource additional branches', () => {
  it('falls back to flat legacy fields for mailStats when mailStats object is absent', () => {
    const instance = pmgInstanceFromResource(
      pmgResource({
        mailCountTotal: 500,
        spamIn: 20,
        virusIn: 5,
        lastUpdated: '2026-01-01T00:00:00Z',
      }),
    );
    expect(instance?.mailStats?.countTotal).toBe(500);
    expect(instance?.mailStats?.spamIn).toBe(20);
    expect(instance?.mailStats?.virusIn).toBe(5);
  });

  it('falls back to resource.id for name when displayName is empty', () => {
    const instance = pmgInstanceFromResource(
      pmgResource(
        { pmg: { hostname: 'pmg.local' } },
        { displayName: '' as unknown as Resource['displayName'], platformId: '' },
      ),
    );
    // displayName empty → name comes from getPreferredInfrastructureDisplayName fallback chain
    // With no displayName and no canonical identity, hostname or id is used
    expect(instance?.name).toBeTruthy();
  });

  it('synthesizes host URL from hostname when hostUrl is absent', () => {
    const instance = pmgInstanceFromResource(
      pmgResource({ hostname: 'pmg-service.local' }, { platformId: '' }),
    );
    expect(instance?.host).toBe('https://pmg-service.local:8006');
  });
});

// ===================================================================
// pbsInstanceFromResource — metric fallback branches
// ===================================================================

describe('pbsInstanceFromResource metric fallbacks', () => {
  it('reads memory and cpu from pbs facet fields when resource metrics are absent', () => {
    const instance = pbsInstanceFromResource(
      pbsResource({
        memoryTotal: 4096,
        memoryUsed: 1024,
        cpuPercent: 33,
        uptimeSeconds: 3600,
      }),
    );
    expect(instance?.memoryTotal).toBe(4096);
    expect(instance?.memoryUsed).toBe(1024);
    expect(instance?.cpu).toBe(33);
    expect(instance?.memory).toBe(25); // (1024/4096)*100
    expect(instance?.uptime).toBe(3600);
  });

  it('falls back to resource.status for connectionHealth when pbs facet lacks it', () => {
    const instance = pbsInstanceFromResource(pbsResource({}, { status: 'degraded' }));
    expect(instance?.connectionHealth).toBe('degraded');
  });

  it('defaults cpu, memory, and uptime to 0 when no source provides them', () => {
    const instance = pbsInstanceFromResource(
      pbsResource({}, { cpu: undefined, memory: undefined, uptime: undefined }),
    );
    expect(instance?.cpu).toBe(0);
    expect(instance?.memory).toBe(0);
    expect(instance?.memoryUsed).toBe(0);
    expect(instance?.memoryTotal).toBe(0);
    expect(instance?.uptime).toBe(0);
  });

  it('synthesizes host URL from hostname when platformId is absent', () => {
    const instance = pbsInstanceFromResource(
      pbsResource(
        { hostname: 'pbs-service.local' },
        { platformId: '', displayName: 'PBS Display' },
      ),
    );
    expect(instance?.host).toBe('https://pbs-service.local:8007');
    expect(instance?.id).toBe('pbs-1');
  });
});

// ===================================================================
// canonicalizeRealtimeResource — platformScopes synthesis option
// ===================================================================

describe('canonicalizeRealtimeResource platformScopes', () => {
  it('leaves platformScopes undefined when synthesizePlatformScopes is false and no explicit scopes', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pve', sourceType: 'hybrid' }),
      { synthesizePlatformScopes: false },
    );
    expect(result.platformScopes).toBeUndefined();
  });

  it('synthesizes platformScopes from platformType when synthesizePlatformScopes is not false', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformType: 'proxmox-pve', sourceType: 'hybrid' }),
    );
    expect(result.platformScopes).toEqual(['proxmox-pve']);
  });

  it('uses explicit resource platformScopes when present', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'hybrid',
        platformScopes: ['docker', 'proxmox-pve'],
      }),
      { synthesizePlatformScopes: false },
    );
    expect(result.platformScopes).toEqual(['docker', 'proxmox-pve']);
  });
});
