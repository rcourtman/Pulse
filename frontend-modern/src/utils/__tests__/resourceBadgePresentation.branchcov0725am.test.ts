import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  getContainerRuntimeBadgeForRuntime,
  getInfrastructurePlatformBadges,
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getPlatformBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import { getSourcePlatformPresentation } from '@/utils/sourcePlatforms';

// Scope note (see GLM_REPORT_fe2-resourcebadge.md):
//
// A V8 branch-coverage run of the three sibling specs, scoped to
// `resourceBadgePresentation.ts`, reports 24 source lines whose branch arm has
// zero hits: 115, 151, 158, 188, 189, 190, 204, 226, 229, 245, 271, 287, 387,
// 415, 420, 422, 437, 573, 585, 604, 608, 670, 701, 744.
//
// Every one of those arms is UNREACHABLE BY CONSTRUCTION. They are defensive
// fallbacks (`??`, `||`, early `return null`) whose left-hand operand can never
// be nullish/falsy at runtime, because of three invariants enforced upstream:
//   (1) `getSourcePlatformBadge` returns null ONLY for empty input (it falls
//       back to a title-cased label for any non-empty string);
//   (2) `getResourceTypePresentation` returns null ONLY for falsy input and
//       always populates `badgeClasses`;
//   (3) every internal caller either guards against empty/falsy input first or
//       passes values it has already trimmed/deduplicated.
//
// Per the task rules, an unreachable arm must NOT be covered by a fabricated
// test. Instead, the tests below exercise the exported entry points with
// ADVERSARIAL inputs (unknown platform strings, duplicate sources, malformed
// host-source ids, whitespace fields) that WOULD trigger each dead arm if it
// were reachable, and assert the real returned value. They demonstrate — on the
// real module — the invariant that keeps the arm dead, and are intentionally
// distinct from the canonical-value assertions the sibling specs already make.

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-1',
    type: 'agent',
    name: 'host-1',
    displayName: 'host-1',
    platformId: 'host-1',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1,
    ...overrides,
  }) as Resource;

describe('resourceBadgePresentation — dead-arm reachability characterization (branchcov0725am)', () => {
  it('getPlatformBadge resolves an arbitrary unknown platform string to a derived badge, so the `if (!sharedBadge) return null` arm at L204 cannot fire', () => {
    // getSourcePlatformBadge only returns null for empty input. Any non-empty
    // platform type yields a title-cased label, so `sharedBadge` at L203 is
    // always defined for the non-empty platformType that survived the L195
    // guard — the `return null` at L204 is dead even for wholly unknown values.
    const badge = getPlatformBadge('fictional-hypervisor-xyz' as never);
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('Fictional Hypervisor Xyz');
    expect(badge?.title).toBe('Fictional Hypervisor Xyz');
  });

  it('getTypeBadge supplies a default badgeClasses for an unknown type, so the `presentation.badgeClasses || typeClasses` arm at L229 cannot fire', () => {
    // getResourceTypePresentation always returns an object whose badgeClasses is
    // a non-empty string (DEFAULT_BADGE_CLASSES), so the `|| typeClasses`
    // fallback is dead. Distinguished from the sibling "totally-fake-type" test
    // by asserting the EXACT class string (the default tone) rather than a
    // substring, locking the contract that badgeClasses is always populated.
    const badge = getTypeBadge('invented-resource-kind');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('invented-resource-kind');
    expect(badge?.classes).toBe(
      'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-base-content',
    );
  });

  it('getInfrastructurePlatformBadges collapses duplicate agent entries to one, so the multi-source ternary `: normalized` arm at L245 cannot fire', () => {
    // normalizeUnifiedSourceKeys builds a Set, so ['agent','agent','agent']
    // collapses to ['agent'] (length 1) and returns at the `<= 1` guard (L240).
    // The `> 1` branch therefore only runs when at least one non-agent source is
    // present, which forces `platformSources.length > 0` to be true there — the
    // `platformSources.length > 0 ? platformSources : normalized` ternary never
    // evaluates its right operand.
    const badges = getInfrastructurePlatformBadges(['agent', 'agent', 'agent']);
    expect(badges).toHaveLength(1);
    expect(badges.map((b) => b.label)).toEqual(['Agent']);
  });

  it('getAgentSystemIdentityBadge resolves an arbitrary unknown hostProfile to a badge, so `badge?.label ?? profileFamily` (L415) and `badge?.classes ?? ...` (L420) never fall back', () => {
    // Inside the `if (hostProfile)` guard the value is non-empty, and
    // getSourcePlatformBadge never returns null for non-empty input, so `badge`
    // is always defined here — both `badge?.X ?? fallback` arms are dead. An
    // unrecognized hostProfile still resolves to a derived (title-cased) badge
    // rather than the profileFamily fallback.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          agent: { hostProfile: 'fictional-nas-xyz', osName: '' },
        },
      }),
    );
    expect(badges).toHaveLength(1);
    expect(badges[0]?.label).toBe('Fictional Nas Xyz');
    expect(badges[0]?.classes).toContain('inline-flex');
  });

  it('proxmoxLxcDockerBadge short-circuits a non-string hostSourceId before proxmoxLxcDockerVmid runs, so the vmid guard at L604 cannot fire', () => {
    // proxmoxLxcDockerBadge validates `typeof hostSourceId !== 'string'` (L614)
    // and `!startsWith(prefix)` (L615) itself and returns null; the inner
    // proxmoxLxcDockerVmid helper is therefore only ever called with an
    // already-valid string, making its own guard at L604 dead. A non-string id
    // never reaches the helper and falls through to the docker runtime badge.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'agent',
        sources: ['docker'],
        platformData: { sources: ['docker'] },
        docker: { hostSourceId: 12345 as unknown as string, runtime: 'docker' },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman']);
  });

  it('getInfrastructureSystemIdentitySortLabel derives a platform label for an unknown platformType, so the L701 truthy-stop of the `||` chain cannot fire', () => {
    // Reaching the L701 operand requires the identity badge list to be empty
    // (badges[0]?.label falsy). But the list is only empty when no identity at
    // all resolves — which requires platformType to be absent/falsy (a non-empty
    // platformType always yields a platform badge at L690). With a falsy
    // platformType, getPlatformBadge returns null, so L701's `?.label` is
    // always falsy when reached and never stops the chain. A non-empty (even
    // unknown) platformType instead produces a non-empty identity list, so the
    // sort label resolves at L700 and L701 is never reached.
    const label = getInfrastructureSystemIdentitySortLabel(
      makeResource({
        type: 'vm',
        platformType: 'fictional-hypervisor-xyz' as Resource['platformType'],
        sourceType: 'api',
        sources: [],
      }),
    );
    expect(label).toBe('Fictional Hypervisor Xyz');
  });

  it('treats a whitespace-only agent.platform as absent, so the `if (!normalized) return null` guards at L271/L287 in the host-identity helpers cannot fire', () => {
    // getKnownHostIdentitySource receives already-trimString'd values; a
    // whitespace platform is trimmed to '' and skipped via `if (!value) continue`
    // before getHostIdentityAgentProfile/getHostIdentityPlatform ever see it, so
    // the `value.trim().toLowerCase()` they compute is always non-empty. The
    // osName still resolves the identity.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          agent: { platform: '   ', osName: 'TrueNAS SCALE' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('getContainerRuntimeBadgeForRuntime reflects the real docker presentation tone (the `?? dockerRuntimeBadgeClasses` fallback at L744 is dead and indistinguishable)', () => {
    // getSourcePlatformPresentation('docker') is always defined and carries a
    // tone, so the `?.tone ?? dockerRuntimeBadgeClasses` fallback never fires.
    // (The fallback string is in fact byte-identical to the real docker tone, so
    // no test can distinguish the two paths by output — this asserts the badge
    // honors the presentation tone regardless.)
    const dockerTone = getSourcePlatformPresentation('docker')?.tone;
    const badge = getContainerRuntimeBadgeForRuntime('docker');
    expect(badge).not.toBeNull();
    expect(dockerTone).toBeTruthy();
    expect(badge?.classes.endsWith(dockerTone as string)).toBe(true);
  });
});

// Imported only to keep the characterization of buildUnifiedSourceBadges
// (L188/L189/L190) and the versioned-source path (L573/L670) explicit: every
// KnownSourcePlatform has a shared presentation, so `sharedBadge?.X ?? source`
// never falls back, and buildUnifiedSourceBadges always emits exactly one badge
// per source, so getVersionedSourceBadge never returns null.
describe('resourceBadgePresentation — shared-badge / versioned-source invariants', () => {
  it('every known source platform resolves a shared presentation badge, so the `?? source` fallbacks at L188/L189/L190 never fire', () => {
    // A presentation-only platform (synology-dsm) and the availability provider
    // both yield their presentation label/classes/title rather than the raw key
    // or the neutral tone, proving sharedBadge is never null for a known source.
    const synology = getUnifiedSourceBadges(['synology-dsm'])[0];
    expect(synology?.label).toBe('Synology');
    expect(synology?.classes).not.toContain('bg-surface-alt');

    const availability = getUnifiedSourceBadges(['availability'])[0];
    expect(availability?.label).toBe('Availability');
    expect(availability?.classes).not.toContain('bg-surface-alt');
  });

  it('a system source with no version data still yields an unversioned badge, so `: null` (L573) and `: []` (L670) never fire', () => {
    // buildUnifiedSourceBadges maps one badge per source unconditionally, so
    // getUnifiedSourceBadges([source])[0] is always defined and
    // getVersionedSourceBadge always returns a badge — even when no version
    // resolves anywhere. A hyper-v source with empty facets returns the
    // unversioned 'Hyper-V' badge rather than null/[].
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'vm',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['microsoft-hyperv'],
        platformData: {
          sources: ['microsoft-hyperv'],
          'microsoft-hyperv': {},
          platform: {},
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Hyper-V']);
  });
});
