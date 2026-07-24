import { describe, expect, it } from 'vitest';

import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import type { Resource, ResourceDiscoveryTarget } from '@/types/resource';

/**
 * Branch-coverage suite for the REMAINING uncovered arms of
 * resourceDetailDiscoveryModel.ts (the 18 branches the existing
 * resourceDetailDiscoveryModel.branchcov.test.ts and
 * ResourceDetailDrawer.discovery.test.ts leave cold).
 *
 * Every private helper below (`getMetadataTarget`, `getHostMetadataTarget`,
 * `getDockerContainerMetadataId`, `getPreferredHostLabel`) is driven through
 * the single export `toDiscoveryConfig`, with assertions on the concrete
 * returned values — never a delegate-equals-delegate tautology.
 *
 * The dominant cold pattern in the baseline is that the existing specs always
 * set kubernetes/docker/proxmox identity directly on the resource and leave
 * `platformData` absent, so every `platformData?.kubernetes?.X` and
 * `platformData?.docker?.X` optional-chaining "defined" arm was never taken.
 * These tests populate those nested platformData records to exercise them.
 */

const baseResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'res-1',
    type: 'storage',
    name: 'res-name',
    displayName: 'Res Name',
    platformId: 'platform-1',
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as unknown as Resource;

const discoveryTarget = (
  overrides: Partial<ResourceDiscoveryTarget> = {},
): ResourceDiscoveryTarget =>
  ({
    resourceType: 'agent',
    agentId: 'agent-default',
    resourceId: 'rid-default',
    ...overrides,
  }) as ResourceDiscoveryTarget;

describe('getMetadataTarget — Kubernetes platformData-defined arms', () => {
  it('builds the stable k8s guest id from platformData.kubernetes when resource.kubernetes is absent', () => {
    // Drives the optional-chaining "defined" arms that the baseline leaves cold:
    //   - line 150 `kubernetesPlatformData?.clusterId`
    //   - line 153 `kubernetesPlatformData?.namespace`
    // (baseline always passed resource.kubernetes with platformData undefined,
    //  so kubernetesPlatformData was always nullish here).
    // Discriminator: the metadata id is keyed by the platformData cluster +
    // namespace ('cl-a' / 'payments'), NOT by resource.id, proving the
    // platformData arms supplied the values.
    const resource = baseResource({
      id: 'dep-runtime',
      type: 'k8s-deployment',
      name: 'checkout',
      platformData: {
        kubernetes: { clusterId: 'cl-a', namespace: 'payments' },
      },
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'pod',
      agentId: 'cl-a',
      resourceId: 'payments/checkout',
      metadataKind: 'guest',
      metadataId: 'k8s-workload:cl-a:deployment:payments:checkout',
      targetLabel: 'workload',
    });
  });
});

describe('getHostMetadataTarget — Kubernetes platformData-defined cluster arm', () => {
  it('resolves the k8s-cluster host metadata id from platformData.kubernetes.clusterId', () => {
    // Drives the "defined" arm of line 183
    // `asString(kubernetesPlatformData?.clusterId)`. The baseline k8s-cluster
    // spec set resource.kubernetes.clusterId with platformData undefined.
    // Discriminator: metadataId is the platformData cluster id 'pd-cluster'
    // (not resource.id), proving the platformData arm supplied it.
    const resource = baseResource({
      id: 'k8s-cluster-runtime',
      type: 'k8s-cluster',
      name: 'mycluster',
      platformData: {
        kubernetes: { clusterId: 'pd-cluster' },
      },
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'agent',
      agentId: 'pd-cluster',
      resourceId: 'pd-cluster',
      metadataKind: 'agent',
      metadataId: 'pd-cluster',
      targetLabel: 'host',
    });
  });
});

describe('getDockerContainerMetadataId — platformData.docker-defined arms', () => {
  it('builds the runtime docker metadata id from platformData.docker when resource.docker is absent', () => {
    // Drives the "defined" arms of:
    //   - line 99  `platformData?.docker`
    //   - line 101 `dockerPlatformData?.hostSourceId`
    //   - line 103 `dockerPlatformData?.containerId`
    // (baseline only reached this helper with platformData undefined).
    // name is blanked so the stable guest id (buildAppContainerMetadataId)
    // returns null and execution falls through to getDockerContainerMetadataId
    // with platformData.docker populated.
    // Discriminator: metadataId is 'pd-host:container:pd-cont' (the
    // platformData values), metadataKind 'docker' — not the resource id.
    const resource = baseResource({
      id: 'app-runtime',
      type: 'app-container',
      name: '',
      platformId: 'app-platform',
      platformData: {
        docker: { hostSourceId: 'pd-host', containerId: 'pd-cont' },
      },
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'app-container',
      agentId: 'pd-host',
      resourceId: 'pd-cont',
      metadataKind: 'docker',
      metadataId: 'pd-host:container:pd-cont',
      targetLabel: 'container',
    });
  });
});

describe('getHostMetadataTarget — pmg branch (no existing pmg spec)', () => {
  it('keys pmg host metadata by resource.pmg.instanceId', () => {
    // Drives the previously-untaken `canonicalResourceType === 'pmg' && ...`
    // branch (lines 211-213): with a pmg resource the left operand is finally
    // truthy and the right operand (resource.pmg.instanceId) is evaluated.
    // ALSO drives the "defined" arm of line 322
    // `asString(resource.pmg?.instanceId)` in the agentLookupId chain — the
    // baseline never had resource.pmg populated when this chain was computed.
    // Discriminator: agentId/resourceId/metadataId all equal 'pmg-inst-1',
    // targetLabel is 'host' (pmg != 'agent').
    const resource = baseResource({
      type: 'pmg',
      name: 'pmg-node',
      pmg: { instanceId: 'pmg-inst-1' } as Resource['pmg'],
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'agent',
      agentId: 'pmg-inst-1',
      resourceId: 'pmg-inst-1',
      metadataKind: 'agent',
      metadataId: 'pmg-inst-1',
      targetLabel: 'host',
    });
  });

  it('falls past the pmg branch when instanceId is missing and still produces a config', () => {
    // Drives the `&&` short-circuit where canonicalResourceType === 'pmg' is
    // true but `asString(resource.pmg?.instanceId)` is falsy (line 211 right
    // operand false) — execution falls through to the generic agent fallback.
    // Discriminator: with no other host id, agentLookupId bottoms out at the
    // resource name 'pmg-node', so metadataId === 'pmg-node' (NOT null), and
    // metadataKind stays 'agent'.
    const resource = baseResource({
      type: 'pmg',
      name: 'pmg-node',
      pmg: {} as Resource['pmg'],
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'agent',
      agentId: 'pmg-node',
      resourceId: 'pmg-node',
      metadataKind: 'agent',
      metadataId: 'pmg-node',
      targetLabel: 'host',
    });
  });
});

describe('agentLookupId chain — lower-priority fallback arms', () => {
  it('uses platformData.docker.hostname when no higher-priority host id resolves', () => {
    // Drives the "defined" arm of line 325
    // `asString(dockerPlatformData?.hostname)`. Requires platformData.docker
    // to be present (with a hostname but no hostSourceId, so the actionable
    // docker runtime id at line 318 stays falsy) and every earlier operand
    // (docker/k8s/agent actionable ids, pbs, pmg.instanceId, proxmox node,
    // platformData.agent.hostname) to be empty.
    // Discriminator: agentId === 'docker-host-hint' (the platformData docker
    // hostname), proving line 325 won the chain.
    const resource = baseResource({
      type: 'agent',
      name: '',
      platformId: '',
      platformData: {
        docker: { hostname: 'docker-host-hint' },
      },
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'agent',
      agentId: 'docker-host-hint',
      resourceId: 'docker-host-hint',
      metadataKind: 'agent',
      metadataId: 'docker-host-hint',
      targetLabel: 'agent',
    });
  });

  it('falls through to getPreferredInfrastructureDisplayName (primary identity) when every hostname source is blank', () => {
    // Drives line 327 `getPreferredInfrastructureDisplayName(resource)` as the
    // winning operand: name/platformId/displayName and all platform hostnames
    // are blank, so getPreferredResourceHostname (line 326) is falsy and the
    // infra display name — which bottoms out at getPrimaryResourceIdentity →
    // resource.id — wins.
    // Discriminator: agentId === resource.id 'bare-agent-id', proving the
    // infra-display-name operand (not name/platformId) supplied the value.
    const resource = baseResource({
      id: 'bare-agent-id',
      type: 'agent',
      name: '',
      platformId: '',
      displayName: '',
    });
    expect(toDiscoveryConfig(resource)).toMatchObject({
      resourceType: 'agent',
      agentId: 'bare-agent-id',
      resourceId: 'bare-agent-id',
      metadataKind: 'agent',
      metadataId: 'bare-agent-id',
      targetLabel: 'agent',
    });
  });

  it('returns null for a malformed agent whose id and platformId are both empty', () => {
    // Drives the genuinely-deep defensive arms only reachable when resource.id
    // is empty (so getPrimaryResourceIdentity, and therefore
    // getPreferredInfrastructureDisplayName, returns ''):
    //   - line 92  `|| resource.id` tail of getPreferredHostLabel
    //   - line 327 falsy arm of getPreferredInfrastructureDisplayName
    //   - line 328 `resource.platformId || resource.id` tail of agentLookupId
    // With no id, hostMetadataTarget.metadataId resolves to '' and the agent
    // case's isDiscoveryLookupValue guard rejects it.
    // These arms are defensive fallbacks unreachable for any valid (non-empty
    // id) resource; exercised here via a deliberately malformed input.
    const resource = baseResource({
      id: '',
      type: 'agent',
      name: '',
      platformId: '',
      displayName: '',
    }) as unknown as Resource;
    expect(toDiscoveryConfig(resource)).toBeNull();
  });
});

describe('canonicalDiscoveryResourceType || resource.type defensive fallback', () => {
  it('treats an empty type as unmapped and returns null even with rich platform data', () => {
    // Drives the `|| resource.type` fallback at line 341 (main switch) and the
    // same fallback at line 180 (inside getHostMetadataTarget).
    // canonicalDiscoveryResourceType('') returns undefined, so the right
    // operand (resource.type '') wins, canonicalResourceType becomes '' and
    // the switch hits its default -> null. The fallback is unreachable for any
    // non-empty type (canonicalDiscoveryResourceType always returns at least
    // the lowercased type), so it is exercised here via an empty type cast.
    // Discriminator: rich docker + kubernetes platform data is present yet the
    // result is null, proving the empty-type canonicalization dominates.
    const resource = baseResource({
      id: 'empty-type-res',
      type: '' as unknown as Resource['type'],
      name: 'whatever',
      platformData: {
        docker: { hostSourceId: 'ignored-host' },
        kubernetes: { clusterId: 'ignored-cluster' },
      },
    });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('drops an explicit discoveryTarget whose resourceType is unmapped and falls back to the main switch default', () => {
    // Companion to the empty-type case above: an explicit discoveryTarget with
    // a resourceType that canonicalizes to a value the explicit switch's
    // `default` rejects (so resourceType becomes null) AND whose resource's own
    // type is also unmapped -> main switch default -> null.
    // Confirms the explicit resourceType guard's default arm and the main
    // switch default compose to null without relying on platformData.
    const resource = baseResource({
      type: '' as unknown as Resource['type'],
      discoveryTarget: discoveryTarget({
        resourceType: 'storage' as ResourceDiscoveryTarget['resourceType'],
        agentId: 'explicit-agent',
        resourceId: 'explicit-rid',
      }),
    });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });
});
