import { describe, expect, it } from 'vitest';
import {
  getContainerRuntimeBadgeForRuntime,
  getInfrastructurePlatformBadges,
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getPlatformBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import type { Resource } from '@/types/resource';

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

describe('getPlatformBadge (branch coverage)', () => {
  it('returns null when no platform type is supplied', () => {
    expect(getPlatformBadge()).toBeNull();
    expect(getPlatformBadge(undefined)).toBeNull();
    expect(getPlatformBadge('' as never)).toBeNull();
  });

  it('returns the shared availability badge for the availability platform type', () => {
    expect(getPlatformBadge('availability')).toStrictEqual({
      label: 'Availability',
      classes:
        'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
      title: 'Availability',
    });
  });

  it('returns shared presentation badges for non-availability platform types', () => {
    expect(getPlatformBadge('kubernetes')?.label).toBe('K8s');
    expect(getPlatformBadge('truenas')?.label).toBe('TrueNAS');
    expect(getPlatformBadge('vmware-vsphere')?.label).toBe('vSphere');
  });
});

describe('getTypeBadge (branch coverage)', () => {
  it('returns null when no resource type is supplied', () => {
    expect(getTypeBadge()).toBeNull();
    expect(getTypeBadge('')).toBeNull();
  });

  it('emits a canonical label and title for a known resource type', () => {
    const badge = getTypeBadge('host');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('Agent');
    expect(badge?.title).toBe('agent');
    expect(badge?.classes).toContain('inline-flex');
    expect(badge?.classes).toContain('bg-orange-100');
  });
});

describe('buildUnifiedSourceBadges via getUnifiedSourceBadges (branch coverage)', () => {
  it('maps canonical aliases through the shared platform badge presentation', () => {
    expect(
      getUnifiedSourceBadges(['kubernetes', 'vmware', 'synology-dsm']).map((b) => b.label),
    ).toEqual(['K8s', 'vSphere', 'Synology']);
  });

  it('renders the availability and generic source presentations', () => {
    expect(getUnifiedSourceBadges(['availability']).map((b) => b.label)).toEqual(['Availability']);
    expect(getUnifiedSourceBadges(['generic']).map((b) => b.label)).toEqual(['Generic']);
  });
});

describe('getInfrastructurePlatformBadges (branch coverage)', () => {
  it('returns an empty array when no sources normalize to a known platform', () => {
    expect(getInfrastructurePlatformBadges([])).toEqual([]);
    expect(getInfrastructurePlatformBadges(undefined)).toEqual([]);
    expect(getInfrastructurePlatformBadges(['totally-unknown-source'])).toEqual([]);
  });

  it('keeps every non-agent platform source when multiple infrastructure platforms remain', () => {
    expect(getInfrastructurePlatformBadges(['docker', 'kubernetes']).map((b) => b.label)).toEqual([
      'Docker / Podman',
      'K8s',
    ]);
  });
});

describe('getContainerRuntimeTone via getContainerRuntimeBadgeForRuntime (branch coverage)', () => {
  it('returns null for an empty or whitespace-only runtime', () => {
    expect(getContainerRuntimeBadgeForRuntime('')).toBeNull();
    expect(getContainerRuntimeBadgeForRuntime('   ')).toBeNull();
  });

  it('normalizes Docker casing before selecting the docker tone', () => {
    const badge = getContainerRuntimeBadgeForRuntime('DOCKER');
    expect(badge?.label).toBe('Docker');
    expect(badge?.title).toBe('Runtime: Docker');
    expect(badge?.classes).toContain('bg-sky-100');
  });

  it('falls back to the neutral type tone for an unrecognized runtime label', () => {
    const badge = getContainerRuntimeBadgeForRuntime('containerd');
    expect(badge?.label).toBe('containerd');
    expect(badge?.title).toBe('Runtime: containerd');
    expect(badge?.classes).toContain('bg-surface-alt');
    expect(badge?.classes).toContain('text-base-content');
  });
});

describe('proxmoxLxcDockerVmid via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('surfaces the trailing VMID when the host source id has multiple colon segments', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'agent',
        docker: {
          hostSourceId: 'proxmox-lxc-docker:pve-a:node-a:250',
          hostname: 'svc',
          runtime: 'docker',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['LXC']);
    expect(badges[0]?.title).toBe('Docker running inside Proxmox LXC 250');
  });

  it('omits the VMID from the tooltip when the trailing segment is zero', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'agent',
        docker: {
          hostSourceId: 'proxmox-lxc-docker:0',
          hostname: 'svc',
          runtime: 'docker',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['LXC']);
    expect(badges[0]?.title).toBe('Docker running inside a Proxmox LXC');
  });

  it('omits the VMID from the tooltip when the trailing segment is a negative integer', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'agent',
        docker: {
          hostSourceId: 'proxmox-lxc-docker:-5',
          hostname: 'svc',
          runtime: 'docker',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['LXC']);
    expect(badges[0]?.title).toBe('Docker running inside a Proxmox LXC');
  });
});

describe('getAvailabilitySystemIdentityBadge via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('maps http and https protocols to uppercased labels with port suffixes', () => {
    const http = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'network-endpoint',
        platformType: 'generic',
        sourceType: 'api',
        platformData: {
          sources: ['availability'],
          availability: { protocol: 'http', address: 'health.local', port: 80 },
        },
      }),
    );
    expect(http.map((b) => b.label)).toEqual(['HTTP']);
    expect(http[0]?.title).toBe('HTTP availability probe health.local:80');

    const https = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'network-endpoint',
        platformType: 'generic',
        sourceType: 'api',
        platformData: {
          sources: ['availability'],
          availability: { protocol: 'https', address: 'secure.local' },
        },
      }),
    );
    expect(https.map((b) => b.label)).toEqual(['HTTPS']);
    expect(https[0]?.title).toBe('HTTPS availability probe secure.local');
  });

  it('uppercases an unrecognized non-empty protocol', () => {
    const snmp = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'network-endpoint',
        platformType: 'generic',
        sourceType: 'api',
        platformData: {
          sources: ['availability'],
          availability: { protocol: 'snmp', address: 'switch-1' },
        },
      }),
    );
    expect(snmp.map((b) => b.label)).toEqual(['SNMP']);
    expect(snmp[0]?.title).toBe('SNMP availability probe switch-1');
  });

  it('drops the port suffix when the port is zero and falls back to the Probe label when protocol is empty', () => {
    const tcp = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'network-endpoint',
        platformType: 'generic',
        sourceType: 'api',
        platformData: {
          sources: ['availability'],
          availability: { protocol: 'tcp', address: 'host-a', port: 0 },
        },
      }),
    );
    expect(tcp.map((b) => b.label)).toEqual(['TCP']);
    expect(tcp[0]?.title).toBe('TCP availability probe host-a');
  });

  it('treats a raw availability source as an availability endpoint even without a facet', () => {
    const probe = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['availability'],
        platformData: { sources: ['availability'] },
      }),
    );
    expect(probe.map((b) => b.label)).toEqual(['Probe']);
    expect(probe[0]?.title).toBe('Probe availability probe');
  });
});

describe('getStorageSystemIdentityBadge via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('appends a TrueNAS storage version and uses storageType in the title when topology is absent', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'truenas',
        sourceType: 'api',
        sources: ['truenas'],
        platformData: {
          sources: ['truenas'],
          storage: { platform: 'truenas', type: 'zfs-pool' },
          truenas: { version: '24.10.0' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS 24.10.0']);
    expect(badges[0]?.title).toBe('TrueNAS zfs-pool 24.10.0');
  });

  it('resolves the unraid storage version from the agent osVersion', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          storage: { platform: 'unraid' },
          agent: { hostProfile: 'unraid', osVersion: '7.0.0' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid 7.0.0']);
  });

  it('resolves the docker version arm when storage is owned by a docker platform', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          storage: { platform: 'docker' },
          docker: { dockerVersion: '27.0.0' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman 27.0.0']);
  });
});

describe('getAgentSystemIdentityBadge via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('uses the known platform source branch when the reported OS text matches a governed platform token', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          agent: { platform: 'linux', osName: 'VMware ESXi' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['vSphere']);
    expect(badges[0]?.title).toBe('VMware ESXi');
  });

  it('keeps the agent host-profile identity ahead of a co-reported docker runtime', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'hybrid',
        sources: ['agent'],
        platformData: {
          sources: ['agent', 'docker'],
          docker: { runtime: 'docker' },
          agent: { platform: 'linux', hostProfile: 'unraid', osName: 'Unraid', osVersion: '7.3.0' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid 7.3.0']);
  });
});

describe('getDockerHostOsIdentityBadge via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('derives a host OS label from docker metadata when no agent identity is present', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'] },
        docker: { os: 'Ubuntu 24.04.2 LTS' },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Ubuntu']);
    expect(badges[0]?.title).toBe('Ubuntu 24.04.2 LTS');
  });

  it('maps a docker OS string that matches a governed platform to the shared platform badge', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'] },
        docker: { os: 'TrueNAS SCALE' },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('falls back to the docker runtime badge when the docker OS string is unrecognized', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'] },
        docker: { os: 'acme-custom-os' },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman']);
  });

  it('falls back to the docker runtime badge when docker metadata has no os field', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'api',
        platformData: { sources: ['docker'] },
        docker: {},
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman']);
  });
});

describe('getVersionedSourceBadge via getInfrastructureSystemIdentityBadges (branch coverage)', () => {
  it('unwraps a Proxmox pve-manager version wrapper into a dotted version segment', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        sources: ['proxmox'],
        platformData: {
          sources: ['proxmox'],
          proxmox: { pveVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE 9.1.9']);
  });

  it('drops an unknown sentinel version instead of appending it', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        sources: ['proxmox'],
        platformData: {
          sources: ['proxmox'],
          proxmox: { pveVersion: 'unknown' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE']);
  });

  it('versions PBS and PMG identities from their respective facet versions', () => {
    const pbs = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'pbs',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        sources: ['pbs'],
        platformData: { sources: ['pbs'], pbs: { version: '3.3.2' } },
      }),
    );
    expect(pbs.map((b) => b.label)).toEqual(['PBS 3.3.2']);

    const pmg = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'pmg',
        platformType: 'proxmox-pmg',
        sourceType: 'api',
        sources: ['pmg'],
        platformData: { sources: ['pmg'], pmg: { version: '9.1.2' } },
      }),
    );
    expect(pmg.map((b) => b.label)).toEqual(['PMG 9.1.2']);
  });

  it('versions TrueNAS, vSphere and K8s identities from their facet versions', () => {
    const truenas = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'truenas',
        sourceType: 'api',
        sources: ['truenas'],
        platformData: { sources: ['truenas'], truenas: { version: '24.10.0' } },
      }),
    );
    expect(truenas.map((b) => b.label)).toEqual(['TrueNAS 24.10.0']);

    const vsphere = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'vm',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        platformData: { sources: ['vmware'], vmware: { version: '8.0.3' } },
      }),
    );
    expect(vsphere.map((b) => b.label)).toEqual(['vSphere 8.0.3']);

    const k8s = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'k8s-cluster',
        platformType: 'kubernetes',
        sourceType: 'api',
        sources: ['kubernetes'],
        platformData: { sources: ['kubernetes'], kubernetes: { version: '1.31.0' } },
      }),
    );
    expect(k8s.map((b) => b.label)).toEqual(['K8s 1.31.0']);
  });

  it('versions a presentation-only platform through the default version-resolution arm', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['synology-dsm'],
        platformData: {
          sources: ['synology-dsm'],
          'synology-dsm': { version: '7.2.1' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Synology 7.2.1']);
  });
});

describe('getInfrastructureSystemIdentityBadges and getInfrastructureSystemIdentitySortLabel (branch coverage)', () => {
  it('falls back to the platform badge for a bare agent resource with no other identity', () => {
    const resource = makeResource({
      type: 'agent',
      platformType: 'agent',
      sourceType: 'agent',
      sources: [],
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('Agent');
  });

  it('returns an empty badge set and empty sort label when nothing resolves', () => {
    const resource = makeResource({
      id: 'lonely',
      name: 'lonely',
      type: 'vm',
      platformType: undefined,
      sourceType: 'api',
      sources: [],
    });
    expect(getInfrastructureSystemIdentityBadges(resource)).toEqual([]);
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('');
  });
});
