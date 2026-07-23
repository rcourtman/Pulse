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

// Branch-coverage supplement for resourceStateAdapters.ts.  These tests
// exercise the fallback / default arms that the happy-path suites
// (resourceStateAdapters.test.ts, .coverage.test.ts, .coverage2.test.ts)
// leave untouched: absent/partial platform payloads, unknown status strings,
// missing nested metric objects, and the zero-vs-undefined numeric
// distinctions inside buildMemory / buildDisk and the PBS/PMG mappers.
//
// The module-private helpers (asNumber, asString, normalizeSourceToken,
// sourceListHas, buildMemory, mapPBS*, mapPMG*, ...) are only reachable
// through the exported entry points below, so every test drives a public
// export and asserts an observable, specific value.

// ---------------------------------------------------------------------------
// Fixture builders (mirroring the conventions in coverage2.test.ts).
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
    cpu: { current: 0 },
    memory: { current: 0, total: 0, used: 0 },
    disk: { current: 0, total: 0, used: 0 },
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

const pd = (r: Resource): Record<string, unknown> =>
  (r.platformData ?? {}) as Record<string, unknown>;

// ===========================================================================
// normalizeSourceToken / sourceListHas — the `|| value.trim().toLowerCase()`
// fallback arm is only evaluated when normalizeSourcePlatformKey() rejects the
// token.  Reach it by merging a resource whose authoritative sources contain a
// token the platform-key map does not recognise.
// ===========================================================================

describe('normalizeSourceToken unknown-token fallback (via mergeCanonicalResource)', () => {
  it('evaluates the trim/lowercase fallback for an unmapped source token and drops non-matching facets', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['custom-xyz'], agent: { hostname: 'incoming' } },
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['custom-xyz'], agent: { hostname: 'existing' } },
      }),
    );
    // 'agent' is not in the authoritative ['custom-xyz'] sources, so the
    // merged agent facet is dropped — proving sourceListHas ran the unknown
    // token through the fallback normaliser.
    expect(pd(merged).agent).toBeUndefined();
    expect(pd(merged).sources).toEqual(['custom-xyz']);
  });
});

// ===========================================================================
// mergePlatformData — `incomingSources ?? existingSources` fallback arm.
// ===========================================================================

describe('mergePlatformData sources fallback (via mergeCanonicalResource)', () => {
  it('falls back to the existing sources when the incoming platformData has none', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { agent: { hostname: 'incoming' } },
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'], agent: { hostname: 'existing' } },
      }),
    );
    expect(pd(merged).sources).toEqual(['agent']);
    expect((pd(merged).agent as Record<string, unknown>).hostname).toBe('incoming');
  });
});

// ===========================================================================
// deriveLegacySourceList — the per-facet `sources.push(...)` arms for pbs,
// pmg, vmware and kubernetes (proxmox/docker/truenas/availability/agent are
// already exercised by the happy-path suites).
// ===========================================================================

describe('deriveLegacySourceList facet detection (via canonicalizeRealtimeResource)', () => {
  it('derives ["pbs"] from a lone pbs facet', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformData: { pbs: { hostname: 'pbs.local' } } }),
    );
    expect(pd(result).sources).toEqual(['pbs']);
  });

  it('derives ["pmg"] from a lone pmg facet', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformData: { pmg: { hostname: 'pmg.local' } } }),
    );
    expect(pd(result).sources).toEqual(['pmg']);
  });

  it('derives ["vmware"] from a lone vmware facet', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformData: { vmware: { hostname: 'vc.local' } } }),
    );
    expect(pd(result).sources).toEqual(['vmware']);
  });

  it('derives ["kubernetes"] from a lone kubernetes facet', () => {
    const result = canonicalizeRealtimeResource(
      agent({ platformData: { kubernetes: { nodeName: 'worker-1' } } }),
    );
    expect(pd(result).sources).toEqual(['kubernetes']);
  });
});

// ===========================================================================
// canonicalizeLegacyPlatformData — the `platformData.disks` carry-over arms
// for the agent / docker / proxmox legacy-payload builders.
// ===========================================================================

describe('legacy payload disks carry-over (via canonicalizeRealtimeResource)', () => {
  it('attaches disks to a synthesised agent payload', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { agentId: 'legacy-agent', disks: [{ device: 'sda', mountpoint: '/' }] },
      }),
    );
    const agentFacet = pd(result).agent as Record<string, unknown> | undefined;
    expect(agentFacet?.agentId).toBe('legacy-agent');
    expect(agentFacet?.disks).toEqual([{ device: 'sda', mountpoint: '/' }]);
  });

  it('attaches disks to a synthesised docker payload', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'docker',
        sourceType: 'api',
        platformData: { runtime: 'docker', disks: [{ device: 'sdb' }] },
      }),
    );
    const dockerFacet = pd(result).docker as Record<string, unknown> | undefined;
    expect(dockerFacet?.runtime).toBe('docker');
    expect(dockerFacet?.disks).toEqual([{ device: 'sdb' }]);
  });

  it('attaches disks to a synthesised proxmox payload', () => {
    const result = canonicalizeRealtimeResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'api',
        platformData: { node: 'pve-1', disks: [{ device: 'sdc' }] },
      }),
    );
    const proxmoxFacet = pd(result).proxmox as Record<string, unknown> | undefined;
    expect(proxmoxFacet?.nodeName).toBe('pve-1');
    expect(proxmoxFacet?.disks).toEqual([{ device: 'sdc' }]);
  });
});

// ===========================================================================
// canonicalizeRealtimeResource — the availability-consequent arm of the
// platformType ternary (reached only when resolvePlatformTypeFromSources
// cannot resolve the sources but an availability facet is still present).
// ===========================================================================

describe('platformType availability consequent (via canonicalizeRealtimeResource)', () => {
  it('classifies as availability when sources are unmapped but an availability facet exists', () => {
    const result = canonicalizeRealtimeResource({
      id: 'a1',
      type: 'agent',
      name: 'a',
      displayName: 'A',
      platformId: 'a1',
      platformType: 'generic',
      sourceType: 'api',
      status: 'online',
      lastSeen: 1_700_000_000_000,
      sources: ['custom-unknown'],
      availability: { targetId: 'x' } as unknown as Resource['availability'],
    } as Resource);
    expect(result.platformType).toBe('availability');
  });
});

// ===========================================================================
// mergeCanonicalIdentity / mergeCanonicalResource — the no-existing and
// platformType-?? fallback arms, plus the tags/labels truthy arms.
// ===========================================================================

describe('mergeCanonicalIdentity + merge fallbacks (via mergeCanonicalResource)', () => {
  it('returns the incoming canonical identity unchanged when the existing resource has none', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        canonicalIdentity: { primaryId: 'inc-primary', hostname: 'h' },
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        canonicalIdentity: undefined,
      }),
    );
    expect(merged.canonicalIdentity?.primaryId).toBe('inc-primary');
  });

  it('falls back to the existing platformType when the incoming omits it', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: undefined as unknown as Resource['platformType'],
        sourceType: 'hybrid',
        platformData: { sources: ['proxmox', 'agent'] },
      }),
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'hybrid',
        platformData: { sources: ['proxmox', 'agent'] },
      }),
    );
    // The incoming.platformType ?? existingCanonical.platformType arm feeds the
    // scope normaliser, so platformScopes is derived from the existing
    // proxmox-pve type even though the incoming platformType is absent.
    expect(merged.platformScopes).toEqual(['proxmox-pve']);
  });

  it('keeps the incoming non-empty tags array', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        tags: ['new'],
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        tags: ['old'],
      }),
    );
    expect(merged.tags).toEqual(['new']);
  });

  it('keeps the incoming non-empty labels record', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        labels: { env: 'prod' },
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'] },
        labels: { env: 'dev' },
      }),
    );
    expect(merged.labels).toEqual({ env: 'prod' });
  });
});

// ===========================================================================
// mergeCanonicalResourceSnapshot — empty-incoming early return, and the
// getHostResourceMergeKey `undefined` arm (reached only when every identity
// candidate — including the resource id — is blank).
// ===========================================================================

describe('mergeCanonicalResourceSnapshot edge arms', () => {
  it('returns an empty array when the incoming snapshot is empty', () => {
    const existing: Resource = agent({ platformData: { sources: ['agent'] } });
    expect(mergeCanonicalResourceSnapshot([], [existing])).toEqual([]);
  });

  it('does not coalesce two agents whose every identity candidate is blank', () => {
    const blank = (): Resource =>
      ({
        id: '',
        type: 'agent',
        name: '',
        displayName: '',
        platformId: '',
        platformType: 'agent',
        sourceType: 'agent',
        status: 'online',
        lastSeen: 100,
      }) as unknown as Resource;
    const out = mergeCanonicalResourceSnapshot([blank(), blank()], []);
    // No host key derivable (getHostResourceMergeKey returns undefined) → both
    // stay as separate entries instead of being coalesced.
    expect(out).toHaveLength(2);
  });
});

// ===========================================================================
// nodeFromResource — non-agent guard, fully-blank identity host chain, missing
// metrics, and the cpuInfo value-present arms.
// ===========================================================================

describe('nodeFromResource fallback arms', () => {
  it('returns null for a non-agent resource', () => {
    expect(nodeFromResource({ id: 'x', type: 'pbs' } as Resource)).toBeNull();
  });

  it('resolves host/status/cpu/connectionHealth/memory to defaults for a fully-blank agent', () => {
    const node = nodeFromResource({
      id: '',
      type: 'agent',
      name: '',
      displayName: '',
      platformId: '',
      platformType: 'agent',
      sourceType: 'agent',
      status: '' as unknown as Resource['status'],
      lastSeen: 1_700_000_000_000,
      cpu: undefined,
      memory: undefined,
    } as unknown as Resource);
    // preferredHostLabel chain falls through every resolver down to resource.id
    expect(node?.host).toBe('');
    expect(node?.status).toBe('unknown');
    expect(node?.cpu).toBe(0);
    expect(node?.connectionHealth).toBe('unknown');
    // buildMemory with no metric and no fallback → every numeric ?? 0 arm.
    expect(node?.memory.total).toBe(0);
    expect(node?.memory.used).toBe(0);
    expect(node?.memory.usage).toBe(0);
  });

  it('maps cpuInfo fields when proxmox.cpuInfo is fully populated', () => {
    const node = nodeFromResource(
      agent({
        proxmox: {
          nodeName: 'n1',
          cpuInfo: { model: 'X86_64', cores: 8, sockets: 2, mhz: '3200' },
        } as unknown as Resource['proxmox'],
      }),
    );
    expect(node?.cpuInfo).toEqual({ model: 'X86_64', cores: 8, sockets: 2, mhz: '3200' });
  });
});

// ===========================================================================
// pbsInstanceFromResource — non-pbs guard, id-||'' arms for sync/verify/prune/
// garbage jobs, hostName/status/connectionHealth fallbacks.
// ===========================================================================

describe('pbsInstanceFromResource fallback arms', () => {
  it('returns null for a non-pbs resource', () => {
    expect(pbsInstanceFromResource({ id: 'x', type: 'agent' } as Resource)).toBeNull();
  });

  it('defaults sync job id to empty string when absent', () => {
    const instance = pbsInstanceFromResource(pbsResource({ syncJobs: [{ store: 's1' }] }));
    expect(instance?.syncJobs[0]).toMatchObject({ id: '', store: 's1' });
  });

  it('defaults verify job id to empty string when absent', () => {
    const instance = pbsInstanceFromResource(pbsResource({ verifyJobs: [{ store: 's1' }] }));
    expect(instance?.verifyJobs[0]).toMatchObject({ id: '', store: 's1' });
  });

  it('defaults prune job id to empty string when absent', () => {
    const instance = pbsInstanceFromResource(pbsResource({ pruneJobs: [{ store: 's1' }] }));
    expect(instance?.pruneJobs[0]).toMatchObject({ id: '', store: 's1' });
  });

  it('defaults garbage job id to empty string when absent', () => {
    const instance = pbsInstanceFromResource(pbsResource({ garbageJobs: [{ store: 's1' }] }));
    expect(instance?.garbageJobs[0]).toMatchObject({ id: '', store: 's1', removedBytes: 0 });
  });

  it('falls back to resource.id for hostName when no hostname is resolvable', () => {
    const instance = pbsInstanceFromResource(
      pbsResource({}, { platformId: '', name: '', displayName: '' }),
    );
    // hostName === resource.id ('pbs-1'); platformId empty → synthesised URL.
    expect(instance?.host).toBe('https://pbs-1:8007');
  });

  it('reports unknown status and connectionHealth when resource status is blank', () => {
    const instance = pbsInstanceFromResource(
      pbsResource({}, { status: '' as unknown as Resource['status'] }),
    );
    expect(instance?.status).toBe('unknown');
    expect(instance?.connectionHealth).toBe('unknown');
  });
});

// ===========================================================================
// pmgInstanceFromResource — non-pmg guard, domain-||'' arm, hostName/status/
// connectionHealth fallbacks.
// ===========================================================================

describe('pmgInstanceFromResource fallback arms', () => {
  it('returns null for a non-pmg resource', () => {
    expect(pmgInstanceFromResource({ id: 'x', type: 'agent' } as Resource)).toBeNull();
  });

  it('defaults domain stat domain to empty string when absent', () => {
    const instance = pmgInstanceFromResource(pmgResource({ domainStats: [{ mailCount: 5 }] }));
    expect(instance?.domainStats?.[0]).toMatchObject({ domain: '', mailCount: 5 });
  });

  it('falls back to resource.id for hostName when no hostname is resolvable', () => {
    const instance = pmgInstanceFromResource(
      pmgResource({}, { platformId: '', name: '', displayName: '' }),
    );
    expect(instance?.host).toBe('https://pmg-1:8006');
  });

  it('reports unknown status and connectionHealth when resource status is blank', () => {
    const instance = pmgInstanceFromResource(
      pmgResource({}, { status: '' as unknown as Resource['status'] }),
    );
    expect(instance?.status).toBe('unknown');
    expect(instance?.connectionHealth).toBe('unknown');
  });
});

// ===========================================================================
// normalizeResourceIdentityToken — the `normalized.length > 0 ? normalized :
// undefined` false arm.  Only reachable via getHostResourceMergeKey, which runs
// every identity candidate through the normaliser.  A whitespace-only candidate
// is truthy (so it skips the `!value` guard) but trims to ''.
// ===========================================================================

describe('normalizeResourceIdentityToken whitespace candidate (via mergeCanonicalResourceSnapshot)', () => {
  it('drops a whitespace-only platformId candidate while still coalescing on a valid hostname', () => {
    const mkAgent = (id: string, lastSeen: number): Resource =>
      ({
        id,
        type: 'agent',
        name: 'merge-host',
        displayName: 'merge-host',
        platformId: 'merge-host',
        platformType: 'proxmox-pve',
        sourceType: 'hybrid',
        status: 'online',
        lastSeen,
        canonicalIdentity: { platformId: '   ', hostname: 'merge-host' },
      }) as unknown as Resource;

    const out = mergeCanonicalResourceSnapshot([mkAgent('a', 100), mkAgent('b', 200)], []);
    // The whitespace platformId candidate exercises the length===0 false arm
    // (→ undefined) but 'merge-host' still forms hostKey 'agent:merge-host',
    // so the two hybrid agents coalesce into one.
    expect(out).toHaveLength(1);
  });
});

// ===========================================================================
// sourceListHas — the `!sources || sources.length === 0` early-return arm.
// sourceListHas is only ever called with an empty/undefined list through
// sourceListContainsRuntimePlatform when unionSources is undefined, which
// happens when two coalescing agents each derive no canonical sources.
// ===========================================================================

describe('sourceListHas empty-sources arm (via mergeCanonicalResourceSnapshot)', () => {
  it('does not merge two agents that share a host key but derive no canonical sources', () => {
    const mkAgent = (id: string, lastSeen: number): Resource =>
      ({
        id,
        type: 'agent',
        name: 'gen-host',
        displayName: 'gen-host',
        platformId: 'gen-host',
        platformType: 'generic',
        sourceType: 'api',
        status: 'online',
        lastSeen,
        canonicalIdentity: { hostname: 'gen-host' },
      }) as unknown as Resource;

    const out = mergeCanonicalResourceSnapshot([mkAgent('a', 100), mkAgent('b', 200)], []);
    // Both derive hostKey 'agent:gen-host', but getCanonicalSourceList returns
    // undefined for each (generic + api, no facets) → unionSources undefined →
    // sourceListHas(undefined, 'agent') takes its empty-sources arm → false.
    expect(out).toHaveLength(2);
    expect(out.map((r) => r.id).sort()).toEqual(['a', 'b']);
  });
});

// ===========================================================================
// readStringArray — the `normalized.length > 0 ? ... : undefined` false arm.
// Reached when an array is supplied but every entry is blank after trim.  The
// only public path that feeds an already-arrayed platformData.sources through
// readStringArray is mergePlatformData (mergeCanonicalResource).
// ===========================================================================

describe('readStringArray all-invalid arm (via mergeCanonicalResource)', () => {
  it('treats a sources array of only blank entries as absent and falls back to existing sources', () => {
    const merged = mergeCanonicalResource(
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: {
          sources: ['', '   ', ' \t '],
          agent: { hostname: 'incoming' },
        },
      }),
      agent({
        platformType: 'agent',
        sourceType: 'agent',
        platformData: { sources: ['agent'], agent: { hostname: 'existing' } },
      }),
    );
    // readStringArray(['', '   ', ' \t ']) → every entry blank after trim →
    // length 0 → returns undefined (false arm); mergePlatformData then falls
    // back to existingSources ['agent'].
    expect(pd(merged).sources).toEqual(['agent']);
    expect((pd(merged).agent as Record<string, unknown>).hostname).toBe('incoming');
  });
});

// ===========================================================================
// buildDisk — the `total > 0 ? (used/total)*100 : (asNumber(fallback?.usage)
// ?? 0)` false arm.  This is the zero-vs-undefined distinction: an explicitly
// zero metric.total is preserved by `??` (zero is not nullish), so the ratio
// branch is skipped in favour of the fallback-usage default.
// ===========================================================================

describe('buildDisk zero-total usage fallback (via nodeFromResource)', () => {
  it('uses fallback.usage when metric.current is absent and total is explicitly zero', () => {
    const node = nodeFromResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'api',
        proxmox: { nodeName: 'n1', disk: { usage: 7 } } as unknown as Resource['proxmox'],
        disk: { total: 0, used: 0 } as unknown as Resource['disk'],
      }),
    );
    // total === 0 (kept by ??) and metric.current absent → total > 0 is false →
    // asNumber(fallback.usage) → 7.
    expect(node?.disk.total).toBe(0);
    expect(node?.disk.usage).toBe(7);
  });

  it('defaults usage to 0 when total is zero and no fallback usage exists', () => {
    const node = nodeFromResource(
      agent({
        platformType: 'proxmox-pve',
        sourceType: 'api',
        disk: { total: 0, used: 0 } as unknown as Resource['disk'],
      }),
    );
    // No fallback at all → asNumber(undefined) ?? 0 → 0.
    expect(node?.disk.usage).toBe(0);
  });
});
