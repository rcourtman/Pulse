import { describe, expect, it } from 'vitest';

import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import type { Resource, ResourceDiscoveryTarget } from '@/types/resource';

/**
 * Branch-coverage suite for the currently-uncovered functions in
 * resourceDetailDiscoveryModel.ts.
 *
 * `hasSource` and `getPreferredHostLabel` are module-private (never exported),
 * so they are driven exclusively through the single exported entry point
 * `toDiscoveryConfig`:
 *   - `getPreferredHostLabel` is observed via the `hostname` field of the
 *     returned DiscoveryConfig (computed by `getPreferredHostLabel` in the
 *     main switch path, and as a fallback for an absent explicit hostname).
 *   - `hasSource` is observed via `hasVMwareScope`, which gates the `vm`
 *     switch case's early-null arm. A vm that carries a vmware source yields
 *     `null`; a vm without one yields a vm-shaped config.
 *
 * No private symbol is imported. Real exported functions are called with
 * fixture resources and the actual returned values are asserted.
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

describe('getPreferredHostLabel (observed via toDiscoveryConfig hostname field)', () => {
  it('uses the identity hostname when getPreferredResourceHostname resolves it (arm 1)', () => {
    // identity.hostname is the first source getPreferredResourceHostname checks
    // after canonicalIdentity.hostname; with no canonical identity set, the
    // identity hostname wins and is surfaced as the discovery hostname.
    const resource = baseResource({
      type: 'vm',
      identity: { hostname: 'host-via-identity' },
    });
    expect(toDiscoveryConfig(resource)?.hostname).toBe('host-via-identity');
  });

  it('falls back to the infrastructure display name when no hostname source resolves (arm 2)', () => {
    // name/platformId are blanked so getPreferredResourceHostname returns
    // undefined; displayName then becomes the winning label via
    // getPreferredInfrastructureDisplayName.
    const resource = baseResource({
      type: 'vm',
      name: '',
      platformId: '',
      displayName: 'Display-Label',
    });
    expect(toDiscoveryConfig(resource)?.hostname).toBe('Display-Label');
  });

  it('falls back to resource.id when neither hostname nor display name resolve (arm 3)', () => {
    // Every hostname and display-name source is blanked, so the label chain
    // bottoms out at resource.id (via getPrimaryResourceIdentity).
    const resource = baseResource({
      type: 'vm',
      id: 'fallback-id',
      name: '',
      platformId: '',
      displayName: '',
    });
    expect(toDiscoveryConfig(resource)?.hostname).toBe('fallback-id');
  });
});

describe('hasSource (observed via hasVMwareScope in the vm switch case)', () => {
  it('matches a vmware entry in resource.sources and returns null for the vm', () => {
    const resource = baseResource({ type: 'vm', sources: ['vmware'] });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('matches a vmware-vsphere entry in platformData.sources and returns null for the vm', () => {
    const resource = baseResource({
      type: 'vm',
      platformData: { sources: ['vmware-vsphere'] },
    });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('matches a VMWARE entry (case-insensitive) in resource.platformScopes and returns null', () => {
    const resource = baseResource({ type: 'vm', platformScopes: ['VMWARE'] });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('trims whitespace before comparing a vmware candidate', () => {
    const resource = baseResource({ type: 'vm', sources: ['  vmware  '] });
    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('ignores non-string candidates and falls through to a vm config', () => {
    // A numeric source entry is not a string, so asString() drops it; with no
    // vmware match anywhere the vm case proceeds and returns a config.
    const resource = baseResource({
      type: 'vm',
      id: 'vm-nostring',
      name: 'vm-nostring',
      sources: [42, { platform: 'vmware-vsphere' }] as unknown as string[],
    });
    const result = toDiscoveryConfig(resource);
    expect(result).not.toBeNull();
    expect(result?.resourceType).toBe('vm');
  });

  it('returns a vm config when every source array is absent', () => {
    // No sources, no platformScopes, no platformData: hasSource sees zero
    // candidates from every array (Array.isArray guards all short-circuit).
    const resource = baseResource({ type: 'vm', id: 'vm-bare', name: 'vm-bare' });
    const result = toDiscoveryConfig(resource);
    expect(result).not.toBeNull();
    expect(result?.resourceType).toBe('vm');
  });
});

describe('toDiscoveryConfig — explicit discoveryTarget branch', () => {
  describe('explicit happy paths (each supported resourceType)', () => {
    it('maps an explicit agent target to an agent discovery config', () => {
      const resource = baseResource({
        type: 'agent',
        discoveryTarget: discoveryTarget({
          resourceType: 'agent',
          agentId: 'agent-explicit',
          resourceId: 'rid-explicit',
          hostname: 'explicit-host',
        }),
      });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'agent',
        agentId: 'agent-explicit',
        resourceId: 'rid-explicit',
        hostname: 'explicit-host',
        metadataKind: 'agent',
        metadataId: 'agent-explicit',
        targetLabel: 'agent',
      });
    });

    it('maps an explicit vm target to a guest config and derives hostname from getPreferredHostLabel', () => {
      const resource = baseResource({
        type: 'vm',
        name: 'vm-name',
        discoveryTarget: discoveryTarget({
          resourceType: 'vm',
          agentId: 'vm-agent',
          resourceId: 'vm-rid',
        }),
      });
      // No explicit hostname => getPreferredHostLabel(resource) wins.
      // Guest metadata keys off the canonical workload id (resource id here,
      // no PVE identity on the fixture), not the discovery resource id.
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'vm',
        agentId: 'vm-agent',
        resourceId: 'vm-rid',
        hostname: 'vm-name',
        metadataKind: 'guest',
        metadataId: 'res-1',
        targetLabel: 'guest',
      });
    });

    it('keys explicit vm guest metadata by the canonical instance:node:vmid id', () => {
      const resource = baseResource({
        type: 'vm',
        name: 'vm-name',
        proxmox: { instance: 'pve-main', nodeName: 'node1', vmid: 105 },
        discoveryTarget: discoveryTarget({
          resourceType: 'vm',
          agentId: 'vm-agent',
          resourceId: 'vm-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.metadataKind).toBe('guest');
      expect(result?.metadataId).toBe('pve-main:node1:105');
    });

    it('maps an explicit system-container target to a guest config', () => {
      const resource = baseResource({
        type: 'system-container',
        discoveryTarget: discoveryTarget({
          resourceType: 'system-container',
          agentId: 'sc-agent',
          resourceId: 'sc-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('system-container');
      expect(result?.metadataKind).toBe('guest');
      expect(result?.metadataId).toBe('res-1');
      expect(result?.targetLabel).toBe('guest');
    });

    it('maps an explicit pod target to a workload config', () => {
      const resource = baseResource({
        type: 'pod',
        discoveryTarget: discoveryTarget({
          resourceType: 'pod',
          agentId: 'pod-agent',
          resourceId: 'pod-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('pod');
      expect(result?.metadataKind).toBe('guest');
      expect(result?.targetLabel).toBe('workload');
    });

    it('maps an explicit app-container target without docker metadata to a guest container config', () => {
      const resource = baseResource({
        type: 'app-container',
        discoveryTarget: discoveryTarget({
          resourceType: 'app-container',
          agentId: 'app-agent',
          resourceId: 'app-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('app-container');
      expect(result?.metadataKind).toBe('guest');
      expect(result?.metadataId).toBe('res-1');
      expect(result?.targetLabel).toBe('container');
    });

    it('maps an explicit app-container target with docker metadata to the stable guest key', () => {
      const resource = baseResource({
        type: 'app-container',
        docker: { hostSourceId: 'dhost', containerId: 'dcont' },
        discoveryTarget: discoveryTarget({
          resourceType: 'app-container',
          agentId: 'app-agent',
          resourceId: 'app-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.metadataKind).toBe('guest');
      expect(result?.metadataId).toBe('app-container:dhost:name:res-name');
      expect(result?.targetLabel).toBe('container');
    });

    it('falls back to the runtime docker key when the container name is unavailable', () => {
      const resource = baseResource({
        type: 'app-container',
        name: '',
        displayName: '',
        docker: { hostSourceId: 'dhost', containerId: 'dcont' },
        discoveryTarget: discoveryTarget({
          resourceType: 'app-container',
          agentId: 'app-agent',
          resourceId: 'app-rid',
        }),
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.metadataKind).toBe('docker');
      expect(result?.metadataId).toBe('dhost:container:dcont');
      expect(result?.targetLabel).toBe('container');
    });
  });

  describe('explicit hostname fallback arm', () => {
    it('uses the explicit hostname when present (short-circuits getPreferredHostLabel)', () => {
      const resource = baseResource({
        type: 'vm',
        name: 'should-be-ignored',
        discoveryTarget: discoveryTarget({
          resourceType: 'vm',
          agentId: 'a',
          resourceId: 'r',
          hostname: 'from-explicit-target',
        }),
      });
      expect(toDiscoveryConfig(resource)?.hostname).toBe('from-explicit-target');
    });
  });

  describe('explicit resourceType switch default (falls through to the main switch)', () => {
    it("drops an explicit 'disk' target and falls back to the main switch for an agent", () => {
      // 'disk' canonicalizes to 'disk' which hits the switch default in the
      // explicit branch => resourceType becomes null => the explicit block is
      // skipped and the main switch handles the resource by its real type.
      // Discriminator: the explicit resourceType was 'disk', yet the returned
      // config carries resourceType 'agent' (derived from resource.type), never
      // 'disk' — proving the explicit target was discarded.
      const resource = baseResource({
        type: 'agent',
        name: 'real-agent',
        discoveryTarget: discoveryTarget({
          resourceType: 'disk',
          agentId: 'explicit-agent-id',
          resourceId: 'explicit-rid',
        } as ResourceDiscoveryTarget),
      });
      const result = toDiscoveryConfig(resource);
      expect(result).not.toBeNull();
      expect(result?.resourceType).toBe('agent');
      // NOTE: getActionableAgentIdFromResource falls back to
      // discoveryTarget.agentId unconditionally, so the explicit agentId leaks
      // into the main-switch agentLookupId even though the explicit resourceType
      // ('disk') was dropped. Observed here as the Section B agentId value.
      expect(result?.agentId).toBe('explicit-agent-id');
      expect(result?.targetLabel).toBe('agent');
    });
  });

  describe('explicit guard rejections (falls through to the main switch)', () => {
    it('returns null when no discoveryTarget is set and the type is unmapped', () => {
      expect(toDiscoveryConfig(baseResource({ type: 'storage' }))).toBeNull();
    });

    it('rejects an explicit target with a blank resourceType', () => {
      const resource = baseResource({
        type: 'storage',
        discoveryTarget: discoveryTarget({
          resourceType: '' as ResourceDiscoveryTarget['resourceType'],
          agentId: 'a',
          resourceId: 'r',
        }),
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });

    it("rejects an explicit agentId that is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'storage',
        discoveryTarget: discoveryTarget({
          resourceType: 'agent',
          agentId: 'redacted by policy',
          resourceId: 'r',
        }),
      });
      // isDiscoveryLookupValue treats the redaction sentinel as absent, so the
      // guard fails and the storage resource maps to null.
      expect(toDiscoveryConfig(resource)).toBeNull();
    });

    it('rejects an explicit resourceId that is empty', () => {
      const resource = baseResource({
        type: 'storage',
        discoveryTarget: discoveryTarget({
          resourceType: 'agent',
          agentId: 'a',
          resourceId: '',
        }),
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });
});

describe('toDiscoveryConfig — main switch (no usable explicit target)', () => {
  describe('agent-family cases (agent / docker-host / pbs / pmg / k8s-cluster / k8s-node)', () => {
    it('maps an agent resource to an agent config derived from its name', () => {
      const resource = baseResource({ type: 'agent', name: 'node-a' });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'agent',
        agentId: 'node-a',
        resourceId: 'node-a',
        hostname: 'node-a',
        metadataKind: 'agent',
        metadataId: 'node-a',
        targetLabel: 'agent',
      });
    });

    it('maps a docker-host resource through the shared agent case', () => {
      const resource = baseResource({ type: 'docker-host', name: 'docker-1' });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('agent');
      expect(result?.agentId).toBe('docker-1');
      expect(result?.targetLabel).toBe('agent');
    });

    it('maps a pbs resource through the shared agent case', () => {
      const resource = baseResource({ type: 'pbs', name: 'pbs-1' });
      expect(toDiscoveryConfig(resource)?.resourceType).toBe('agent');
    });

    it('maps a k8s-cluster resource through the shared agent case', () => {
      const resource = baseResource({ type: 'k8s-cluster', name: 'k8s-1' });
      expect(toDiscoveryConfig(resource)?.resourceType).toBe('agent');
    });

    it("returns null when the agent lookup id is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'agent',
        name: 'redacted by policy',
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });

  describe('vm case', () => {
    it('returns a vm config with resource.id as resourceId when no proxmox vmid is present', () => {
      const resource = baseResource({ type: 'vm', id: 'vm-plain', name: 'vm-plain' });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'vm',
        agentId: 'vm-plain',
        resourceId: 'vm-plain',
        hostname: 'vm-plain',
        metadataKind: 'guest',
        metadataId: 'vm-plain',
        targetLabel: 'guest',
      });
    });

    it('uses the proxmox vmid as resourceId when a vmware scope is overridden by proxmox coords', () => {
      // hasVMwareScope is true (platformData.vmware), but proxmoxNodeName +
      // vmidResourceId are both present, so the early-null guard is bypassed
      // and the vm resolves against the proxmox coordinates.
      const resource = baseResource({
        type: 'vm',
        id: 'vm-with-proxmox',
        name: 'vm-with-proxmox',
        proxmox: { nodeName: 'pve-1', vmid: 100 },
        platformData: { vmware: { clusterName: 'vc-1' } },
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceId).toBe('100');
      expect(result?.agentId).toBe('pve-1');
      expect(result?.resourceType).toBe('vm');
    });

    it('returns null when hasVMwareScope is true and there is no proxmox override', () => {
      const resource = baseResource({
        type: 'vm',
        vmware: { clusterName: 'vc-1' } as Resource['vmware'],
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });

    it("returns null when the workload agent id is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'vm',
        name: 'redacted by policy',
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });

  describe('system-container / oci-container cases', () => {
    it('maps a system-container resource to a guest config', () => {
      const resource = baseResource({ type: 'system-container', id: 'lxc-1', name: 'lxc-1' });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'system-container',
        agentId: 'lxc-1',
        resourceId: 'lxc-1',
        hostname: 'lxc-1',
        metadataKind: 'guest',
        metadataId: 'lxc-1',
        targetLabel: 'guest',
      });
    });

    it('routes an oci-container resource through the shared system-container case', () => {
      const resource = baseResource({ type: 'oci-container', id: 'oci-1', name: 'oci-1' });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('system-container');
      expect(result?.targetLabel).toBe('guest');
    });

    it("returns null when the workload agent id is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'system-container',
        name: 'redacted by policy',
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });

  describe('app-container case', () => {
    it('maps an app-container without docker metadata to a guest container config', () => {
      const resource = baseResource({ type: 'app-container', id: 'c-1', name: 'c-1' });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'app-container',
        agentId: 'c-1',
        resourceId: 'c-1',
        hostname: 'c-1',
        metadataKind: 'guest',
        metadataId: 'c-1',
        targetLabel: 'container',
      });
    });

    it('uses the docker container id and docker metadata when platformData.docker is populated', () => {
      const resource = baseResource({
        type: 'app-container',
        id: 'c-docker',
        name: 'c-docker',
        platformData: {
          docker: { hostSourceId: 'dhost', containerId: 'dcont', hostname: 'dhost-h' },
        },
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceId).toBe('dcont');
      expect(result?.metadataKind).toBe('guest');
      expect(result?.metadataId).toBe('app-container:dhost:name:c-docker');
      expect(result?.targetLabel).toBe('container');
    });

    it("returns null when the workload agent id is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'app-container',
        name: 'redacted by policy',
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });

  describe('pod / k8s-deployment / k8s-service cases', () => {
    it('falls back to resource.id for a pod without complete stable metadata scope', () => {
      const resource = baseResource({ type: 'pod', id: 'pod-1', name: 'pod-1' });
      expect(toDiscoveryConfig(resource)).toEqual({
        resourceType: 'pod',
        agentId: 'pod-1',
        resourceId: 'pod-1',
        hostname: 'pod-1',
        metadataKind: 'guest',
        metadataId: 'pod-1',
        targetLabel: 'workload',
      });
    });

    it('uses the kubernetes podUid as resourceId when present', () => {
      const resource = baseResource({
        type: 'pod',
        id: 'pod-uid',
        name: 'pod-uid',
        kubernetes: { podUid: 'uid-1' } as Resource['kubernetes'],
      });
      expect(toDiscoveryConfig(resource)?.resourceId).toBe('uid-1');
    });

    it('uses stable logical metadata identity for Kubernetes workload kinds', () => {
      for (const type of ['pod', 'k8s-deployment', 'k8s-service'] as const) {
        const resource = baseResource({
          type,
          id: `runtime-${type}`,
          name: 'checkout',
          kubernetes: {
            clusterId: 'cluster-a',
            namespace: 'payments',
            podUid: type === 'pod' ? 'pod-uid-old' : undefined,
          } as Resource['kubernetes'],
        });

        expect(toDiscoveryConfig(resource)?.metadataId).toBe(
          `k8s-workload:cluster-a:${type === 'pod' ? 'pod' : type.slice(4)}:payments:checkout`,
        );
      }
    });

    it('derives resourceId from namespace/podName when podUid is absent', () => {
      const resource = baseResource({
        type: 'pod',
        id: 'pod-ns',
        name: 'pod-ns',
        kubernetes: { namespace: 'ns', podName: 'web' } as Resource['kubernetes'],
      });
      expect(toDiscoveryConfig(resource)?.resourceId).toBe('ns/web');
    });

    it('routes a k8s-deployment resource through the shared pod case', () => {
      const resource = baseResource({
        type: 'k8s-deployment',
        id: 'dep-1',
        name: 'dep-1',
      });
      const result = toDiscoveryConfig(resource);
      expect(result?.resourceType).toBe('pod');
      expect(result?.targetLabel).toBe('workload');
    });

    it("returns null when the workload agent id is 'redacted by policy'", () => {
      const resource = baseResource({
        type: 'pod',
        name: 'redacted by policy',
      });
      expect(toDiscoveryConfig(resource)).toBeNull();
    });
  });

  describe('default case', () => {
    it('returns null for an unmapped resource type', () => {
      expect(toDiscoveryConfig(baseResource({ type: 'storage' }))).toBeNull();
    });

    it('returns null for a jail resource type', () => {
      expect(toDiscoveryConfig(baseResource({ type: 'jail', name: 'jail-1' }))).toBeNull();
    });
  });
});
