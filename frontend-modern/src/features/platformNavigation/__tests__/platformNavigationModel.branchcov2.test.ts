import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildNavigableResourcePlatformScopeSet,
  buildPrimaryPlatformNavigationVisibility,
  collectResourcePlatformEvidence,
  createEmptyPlatformNavigationVisibility,
  filterPlatformNavigationShortcuts,
  platformNavigationVisibilityFromResources,
  type PlatformNavigationShortcut,
  type PlatformNavigationVisibility,
} from '../platformNavigationModel';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors the sibling platformNavigationModel.test.ts
// factory so import style and default platform posture stay aligned.
// ---------------------------------------------------------------------------

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'resource-1',
    name: overrides.name ?? overrides.id ?? 'resource-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'platform-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'api',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const ALL_FALSE_VISIBILITY: PlatformNavigationVisibility = {
  proxmox: false,
  docker: false,
  kubernetes: false,
  truenas: false,
  vmware: false,
  standalone: false,
};

// ===========================================================================
// addManifestPlatformId — module-private, exercised transitively through
// collectResourcePlatformEvidence (its only call site among exported funcs).
//
// Branches:
//   a) normalized is null (value doesn't normalize to a known platform)
//   b) normalized === 'generic'
//   c) getSourcePlatformManifestEntry(normalized) is null (e.g. 'availability')
//   d) valid — adds the normalized id
// ===========================================================================

describe('addManifestPlatformId (via collectResourcePlatformEvidence)', () => {
  it('skips a platformType that normalizes to null (unknown value)', () => {
    // normalizeSourcePlatformKey('zzz-unknown') → null → early return, no add.
    // resolveResourcePlatformType also yields null, so ids stays empty.
    const r = resource({
      id: 'unknown-pt',
      platformType: 'zzz-unknown' as unknown as Resource['platformType'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual([]);
  });

  it('skips a platformType that normalizes to "generic"', () => {
    // normalizeSourcePlatformKey('generic') → 'generic' → early return, no add.
    const r = resource({
      id: 'generic-pt',
      platformType: 'generic',
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual([]);
  });

  it('skips a source that normalizes to a known key without a manifest entry (availability)', () => {
    // normalizeSourcePlatformKey('availability') → 'availability', but
    // getSourcePlatformManifestEntry('availability') → null → early return.
    // 'agent' is still admitted from platformType.
    const r = resource({
      id: 'avail-src',
      platformType: 'agent',
      sources: ['availability'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['agent']);
  });

  it('skips "generic" in platformScopes but admits a valid scope alongside it', () => {
    // platformScopes: ['generic', 'docker'] → 'generic' skipped, 'docker' added.
    // Early-return path fires; addDirectDockerRuntimeEvidence does not add
    // (ids has 'docker', not 'agent').
    const r = resource({
      id: 'mixed-scopes',
      platformType: 'agent',
      platformScopes: ['generic', 'docker'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['docker']);
  });
});

// ===========================================================================
// addDirectDockerRuntimeEvidence — module-private, exercised transitively
// through collectResourcePlatformEvidence's early-return path (platformScopes
// non-empty → ids.size > 0).
//
// Branches:
//   1) resource.platformType normalizes to 'docker'
//   2) resolveResourcePlatformType(resource) normalizes to 'docker'
//   3) DOCKER_RESOURCE_TYPES.has(resource.type)
//   4) ids.has('agent') && hasDockerRuntimeHostEvidence(resource)
//   5) none of the above — no docker added
// ===========================================================================

describe('addDirectDockerRuntimeEvidence (via collectResourcePlatformEvidence early-return path)', () => {
  it('adds docker when resource.platformType is "docker" (branch 1)', () => {
    const r = resource({
      id: 'docker-pt',
      platformType: 'docker',
      platformScopes: ['agent'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['agent', 'docker']);
  });

  it('adds docker when resolvedPlatformType is "docker" via sources fallback (branch 2)', () => {
    // platformType is empty → resolveResourcePlatformType falls back to
    // sources ['docker'] → returns 'docker'.
    const r = resource({
      id: 'resolved-docker',
      platformType: '' as unknown as Resource['platformType'],
      sources: ['docker'],
      platformScopes: ['agent'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['agent', 'docker']);
  });

  it('adds docker when resource.type is a DOCKER_RESOURCE_TYPE (branch 3)', () => {
    // type 'docker-service' is in DOCKER_RESOURCE_TYPES but
    // hasDockerRuntimeHostEvidence returns false for it (not 'docker-host',
    // not 'agent'), isolating branch 3 from branch 4.
    const r = resource({
      id: 'docker-svc',
      platformType: 'truenas',
      platformScopes: ['truenas'],
      type: 'docker-service',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['truenas', 'docker']);
  });

  it('does not add docker when agent scope lacks runtime evidence (branch 4 negative)', () => {
    // ids has 'agent' but hasDockerRuntimeHostEvidence is false (no docker
    // facet), so branch 4's condition is not met.
    const r = resource({
      id: 'agent-no-docker',
      platformType: 'agent',
      platformScopes: ['agent'],
      type: 'agent',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['agent']);
  });
});

// ===========================================================================
// collectResourcePlatformEvidence — full-path facet branches
//
// Each if-block has three OR arms (where applicable): resource.type,
// resource.<facet>, asRecord(platformData?.<facet>). Using platformType
// 'generic' ensures no 'agent' is added, so the only id in the result
// comes from the facet under test.
// ===========================================================================

describe('collectResourcePlatformEvidence — full-path PBS evidence', () => {
  it('admits proxmox-pbs from the resource.pbs facet', () => {
    const r = resource({
      id: 'pbs-facet',
      platformType: 'generic',
      type: 'agent',
      pbs: { hostname: 'pbs-1' },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pbs']);
  });

  it('admits proxmox-pbs from platformData.pbs', () => {
    const r = resource({
      id: 'pbs-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { pbs: { hostname: 'pbs-1' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pbs']);
  });
});

describe('collectResourcePlatformEvidence — full-path PMG evidence', () => {
  it('admits proxmox-pmg from the resource.pmg facet', () => {
    const r = resource({
      id: 'pmg-facet',
      platformType: 'generic',
      type: 'agent',
      pmg: { hostname: 'pmg-1' },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pmg']);
  });

  it('admits proxmox-pmg from platformData.pmg', () => {
    const r = resource({
      id: 'pmg-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { pmg: { hostname: 'pmg-1' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pmg']);
  });
});

describe('collectResourcePlatformEvidence — full-path Proxmox PVE evidence', () => {
  it('admits proxmox-pve from the resource.proxmox facet', () => {
    const r = resource({
      id: 'proxmox-facet',
      platformType: 'generic',
      type: 'agent',
      proxmox: { vmid: 100 },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pve']);
  });

  it('admits proxmox-pve from platformData.proxmox', () => {
    const r = resource({
      id: 'proxmox-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { proxmox: { node: 'pve-1' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pve']);
  });
});

describe('collectResourcePlatformEvidence — full-path Ceph evidence', () => {
  it('admits proxmox-pve from resource.type === "ceph"', () => {
    const r = resource({
      id: 'ceph-type',
      platformType: 'generic',
      type: 'ceph',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pve']);
  });

  it('admits proxmox-pve from the resource.ceph facet', () => {
    const r = resource({
      id: 'ceph-facet',
      platformType: 'generic',
      type: 'agent',
      ceph: {
        healthStatus: 'ok',
        numMons: 1,
        numMgrs: 1,
        numOsds: 1,
        numOsdsUp: 1,
        numOsdsIn: 1,
        numPGs: 1,
      },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pve']);
  });

  it('admits proxmox-pve from platformData.ceph', () => {
    const r = resource({
      id: 'ceph-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { ceph: { healthStatus: 'ok' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['proxmox-pve']);
  });
});

describe('collectResourcePlatformEvidence — full-path VMware evidence', () => {
  it('admits vmware-vsphere from the resource.vmware facet', () => {
    const r = resource({
      id: 'vmware-facet',
      platformType: 'generic',
      type: 'agent',
      vmware: { connectionName: 'vc-1' },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['vmware-vsphere']);
  });

  it('admits vmware-vsphere from platformData.vmware', () => {
    const r = resource({
      id: 'vmware-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { vmware: { connectionName: 'vc-1' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['vmware-vsphere']);
  });
});

describe('collectResourcePlatformEvidence — full-path Kubernetes evidence', () => {
  it('admits kubernetes from the resource.kubernetes facet', () => {
    const r = resource({
      id: 'k8s-facet',
      platformType: 'generic',
      type: 'agent',
      kubernetes: { clusterName: 'k8s-1' },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['kubernetes']);
  });

  it('admits kubernetes from platformData.kubernetes', () => {
    const r = resource({
      id: 'k8s-platformdata',
      platformType: 'generic',
      type: 'agent',
      platformData: { kubernetes: { clusterName: 'k8s-1' } },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['kubernetes']);
  });
});

// ===========================================================================
// collectResourcePlatformEvidence — full-path runtime/type branches
// ===========================================================================

describe('collectResourcePlatformEvidence — full-path Docker and Kubernetes type branches', () => {
  it('admits docker from hasDockerRuntimeHostEvidence in the full path', () => {
    // type 'agent' + docker.runtime set → hasDockerRuntimeHostEvidence true.
    // This is the FULL path (no platformScopes), distinct from the
    // early-return path tested by the sibling test.
    const r = resource({
      id: 'docker-runtime-fullpath',
      platformType: 'generic',
      type: 'agent',
      docker: { runtime: 'docker' },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['docker']);
  });

  it('admits docker from DOCKER_RESOURCE_TYPES in the full path', () => {
    // type 'docker-service' → hasDockerRuntimeHostEvidence false (not
    // 'docker-host', not 'agent'), but DOCKER_RESOURCE_TYPES.has is true.
    const r = resource({
      id: 'docker-service-fullpath',
      platformType: 'generic',
      type: 'docker-service',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['docker']);
  });

  it('admits kubernetes from KUBERNETES_RESOURCE_TYPES', () => {
    const r = resource({
      id: 'k8s-cluster-type',
      platformType: 'generic',
      type: 'k8s-cluster',
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['kubernetes']);
  });
});

describe('collectResourcePlatformEvidence — full-path source-derivation branches', () => {
  it('admits a platform id from resource.sources via addManifestPlatformId', () => {
    const r = resource({
      id: 'sources-derived',
      platformType: 'generic',
      type: 'agent',
      sources: ['docker'],
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['docker']);
  });

  it('admits a platform id from platformData.sources array via addPlatformDataSources', () => {
    const r = resource({
      id: 'platformdata-sources',
      platformType: 'generic',
      type: 'agent',
      platformData: { sources: ['docker'] },
    });
    expect(collectResourcePlatformEvidence(r)).toEqual(['docker']);
  });
});

// ===========================================================================
// buildNavigableResourcePlatformScopeSet
//
// Branches:
//   - manifestEntry is null → continue (unreachable through normal inputs;
//     collectResourcePlatformEvidence only returns ids with manifest entries)
//   - !NAVIGABLE_PLATFORM_ID_SET.has(manifestEntry.id) → continue
//   - both checks pass → add to set
// ===========================================================================

describe('buildNavigableResourcePlatformScopeSet', () => {
  it('returns an empty set for no resources', () => {
    expect(buildNavigableResourcePlatformScopeSet([])).toEqual(new Set<string>());
  });

  it('includes navigable platform ids and excludes non-navigable ones', () => {
    // 'proxmox-pve' and 'docker' are navigable (in SUPPORTED_PLATFORM_IDS).
    // 'unraid' has a manifest entry but is presentation-only (not in
    // NAVIGABLE_PLATFORM_ID_SET), so it is skipped.
    const resources = [
      resource({ id: 'pve-1', platformType: 'proxmox-pve' }),
      resource({ id: 'docker-1', platformType: 'docker' }),
      resource({ id: 'unraid-1', platformType: 'unraid' }),
    ];
    expect(buildNavigableResourcePlatformScopeSet(resources)).toEqual(
      new Set(['proxmox-pve', 'docker']),
    );
  });
});

// ===========================================================================
// buildPrimaryPlatformNavigationVisibility
//
// Branches:
//   - hasAvailabilityEndpoints: three OR arms
//     a) resource.type === 'network-endpoint'
//     b) resource.platformType === 'availability'
//     c) resource.sources?.includes('availability')
//   - standalone: isPulseAgentPlatformResource || hasAvailabilityEndpoints
//   - per-platform scope .some() checks
// ===========================================================================

describe('buildPrimaryPlatformNavigationVisibility — standalone via availability endpoints', () => {
  it('shows standalone when resource.type is "network-endpoint"', () => {
    const visibility = buildPrimaryPlatformNavigationVisibility([
      resource({ id: 'endpoint-1', type: 'network-endpoint', platformType: 'availability' }),
    ]);
    expect(visibility.standalone).toBe(true);
  });

  it('shows standalone when resource.platformType is "availability"', () => {
    const visibility = buildPrimaryPlatformNavigationVisibility([
      resource({ id: 'avail-1', type: 'agent', platformType: 'availability' }),
    ]);
    expect(visibility.standalone).toBe(true);
  });

  it('shows standalone when resource.sources includes "availability"', () => {
    const visibility = buildPrimaryPlatformNavigationVisibility([
      resource({ id: 'avail-src-1', type: 'agent', platformType: 'agent', sources: ['availability'] }),
    ]);
    expect(visibility.standalone).toBe(true);
  });
});

describe('buildPrimaryPlatformNavigationVisibility — platform scope visibility', () => {
  it('shows proxmox from a ceph resource type', () => {
    const visibility = buildPrimaryPlatformNavigationVisibility([
      resource({ id: 'ceph-1', type: 'ceph', platformType: 'generic' }),
    ]);
    expect(visibility.proxmox).toBe(true);
    expect(visibility.docker).toBe(false);
    expect(visibility.kubernetes).toBe(false);
    expect(visibility.truenas).toBe(false);
    expect(visibility.vmware).toBe(false);
    expect(visibility.standalone).toBe(false);
  });

  it('returns all false for a non-navigable platform (unraid)', () => {
    const visibility = buildPrimaryPlatformNavigationVisibility([
      resource({ id: 'unraid-1', type: 'agent', platformType: 'unraid' }),
    ]);
    expect(visibility).toEqual(ALL_FALSE_VISIBILITY);
  });
});

// ===========================================================================
// createEmptyPlatformNavigationVisibility
// ===========================================================================

describe('createEmptyPlatformNavigationVisibility', () => {
  it('returns every primary platform hidden', () => {
    expect(createEmptyPlatformNavigationVisibility()).toEqual(ALL_FALSE_VISIBILITY);
  });
});

// ===========================================================================
// filterPlatformNavigationShortcuts
//
// Branches:
//   - !primaryPlatformNavigationIsVisible → continue (skip shortcut)
//   - visible → routes[shortcut.key] = shortcut.route
// ===========================================================================

describe('filterPlatformNavigationShortcuts', () => {
  const allShortcuts: Record<string, PlatformNavigationShortcut> = {
    proxmox: { key: 'p', route: '/proxmox' },
    docker: { key: 'd', route: '/docker' },
    kubernetes: { key: 'k', route: '/kubernetes' },
    truenas: { key: 't', route: '/truenas' },
    vmware: { key: 'v', route: '/vmware' },
    standalone: { key: 's', route: '/standalone' },
  };

  it('returns all routes when every platform is visible', () => {
    const allVisible: PlatformNavigationVisibility = {
      proxmox: true,
      docker: true,
      kubernetes: true,
      truenas: true,
      vmware: true,
      standalone: true,
    };
    expect(filterPlatformNavigationShortcuts(allShortcuts, allVisible)).toEqual({
      p: '/proxmox',
      d: '/docker',
      k: '/kubernetes',
      t: '/truenas',
      v: '/vmware',
      s: '/standalone',
    });
  });

  it('returns an empty record when no platforms are visible', () => {
    expect(filterPlatformNavigationShortcuts(allShortcuts, ALL_FALSE_VISIBILITY)).toEqual({});
  });

  it('returns only routes for visible platforms (mixed visibility)', () => {
    const mixed: PlatformNavigationVisibility = {
      proxmox: true,
      docker: false,
      kubernetes: true,
      truenas: false,
      vmware: false,
      standalone: false,
    };
    expect(filterPlatformNavigationShortcuts(allShortcuts, mixed)).toEqual({
      p: '/proxmox',
      k: '/kubernetes',
    });
  });
});

// ===========================================================================
// platformNavigationVisibilityFromResources
//
// Thin wrapper: delegates to buildPrimaryPlatformNavigationVisibility with
// the Accessor's current value.
// ===========================================================================

describe('platformNavigationVisibilityFromResources', () => {
  it('returns all-false for an empty accessor', () => {
    expect(platformNavigationVisibilityFromResources(() => [])).toEqual(ALL_FALSE_VISIBILITY);
  });

  it('delegates to buildPrimaryPlatformNavigationVisibility for non-empty resources', () => {
    const resources: readonly Resource[] = [
      resource({ id: 'pve-1', platformType: 'proxmox-pve', type: 'agent' }),
      resource({ id: 'docker-1', platformType: 'docker', type: 'docker-host' }),
    ];
    expect(platformNavigationVisibilityFromResources(() => resources)).toEqual({
      proxmox: true,
      docker: true,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    });
  });
});
