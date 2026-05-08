import { describe, expect, it } from 'vitest';
import {
  dedupeResourceBadges,
  getInfrastructurePlatformBadges,
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import type { Resource } from '@/types/resource';

const makeResource = (overrides: Partial<Resource>): Resource => ({
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
});

describe('resourceBadgePresentation', () => {
  it('returns canonical platform badges via shared platform presentation', () => {
    expect(getPlatformBadge('proxmox-pve')?.label).toBe('PVE');
    expect(getPlatformBadge('proxmox-pbs')?.label).toBe('PBS');
    expect(getPlatformBadge('docker')?.label).toBe('Docker / Podman');
    expect(getPlatformBadge('availability')?.label).toBe('Availability');
  });

  it('returns source badges for infrastructure source types', () => {
    expect(getSourceBadge('agent')).toMatchObject({ label: 'Agent', title: 'agent' });
    expect(getSourceBadge('hybrid')).toMatchObject({ label: 'Hybrid', title: 'hybrid' });
  });

  it('returns canonical type badges from shared resource type presentation', () => {
    expect(getTypeBadge('host')?.label).toBe('Agent');
    expect(getTypeBadge('docker_host')?.label).toBe('Container Runtime');
  });

  it('deduplicates and normalizes unified source badges', () => {
    const badges = getUnifiedSourceBadges(['TrueNAS', 'PROXMOX', 'truenas']);
    expect(badges.map((badge) => badge.label)).toEqual(['TrueNAS', 'PVE']);
  });

  it('keeps agent as telemetry detail when another infrastructure platform is present', () => {
    expect(getUnifiedSourceBadges(['agent', 'proxmox']).map((badge) => badge.label)).toEqual([
      'Agent',
      'PVE',
    ]);
    expect(
      getInfrastructurePlatformBadges(['agent', 'proxmox']).map((badge) => badge.label),
    ).toEqual(['PVE']);
    expect(getInfrastructurePlatformBadges(['agent']).map((badge) => badge.label)).toEqual([
      'Agent',
    ]);
  });

  it('shows explicit host identity before container runtime capability', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'docker-host',
          platformType: 'docker',
          sourceType: 'hybrid',
          platformData: {
            sources: ['agent', 'docker'],
            agent: {
              platform: 'linux',
              hostProfile: 'unraid',
              osName: 'Unraid',
              osVersion: '7.1.0',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid 7.1.0']);

    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'docker-host',
          platformType: 'docker',
          sourceType: 'api',
          platformData: { sources: ['docker'] },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Docker / Podman']);
  });

  it('shows Unraid as the system identity for agent resources that also report Docker', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'agent',
          platformType: 'docker',
          sourceType: 'hybrid',
          platformData: {
            sources: ['docker', 'agent'],
            docker: {
              runtime: 'docker',
            },
            agent: {
              platform: 'linux',
              hostProfile: 'unraid',
              osName: 'Unraid',
              osVersion: '7.2.2',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid 7.2.2']);
  });

  it('shows storage-owned platform identity without using presentation profiles as PlatformType', () => {
    const unraidStorage = makeResource({
      type: 'storage',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      platformData: {
        sources: ['agent'],
        storage: {
          platform: 'unraid',
          type: 'unraid-array',
          topology: 'array',
        },
      },
      storage: {
        platform: 'unraid',
        type: 'unraid-array',
        topology: 'array',
      } as Resource['storage'],
    });

    expect(getInfrastructureSystemIdentityBadges(unraidStorage).map((badge) => badge.label)).toEqual(
      ['Unraid'],
    );
    expect(getInfrastructureSystemIdentitySortLabel(unraidStorage)).toBe('Unraid');

    const pbsDatastore = makeResource({
      type: 'storage',
      platformType: 'proxmox-pbs',
      sourceType: 'api',
      sources: ['pbs'],
      storage: {
        platform: 'pbs',
        type: 'pbs-datastore',
        topology: 'datastore',
      } as Resource['storage'],
    });

    expect(getInfrastructureSystemIdentityBadges(pbsDatastore).map((badge) => badge.label)).toEqual([
      'PBS',
    ]);
  });

  it('does not let stale platform sources override explicit agent host profiles', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          platformData: {
            sources: ['proxmox', 'docker', 'agent'],
            docker: {
              runtime: 'docker',
            },
            agent: {
              platform: 'linux',
              hostProfile: 'unraid',
              osName: 'Unraid',
              osVersion: '7.2.2',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid 7.2.2']);
  });

  it('uses governed host identity tokens for legacy agent profile reports', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'docker-host',
          platformType: 'docker',
          sourceType: 'hybrid',
          platformData: {
            sources: ['agent', 'docker'],
            agent: {
              platform: 'linux',
              osName: 'Unraid OS 7.1.0',
              osVersion: '7.1.0',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid 7.1.0']);
  });

  it('uses authoritative top-level sources before stale platformData source hints', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'agent',
          platformType: 'agent',
          sourceType: 'hybrid',
          sources: ['agent', 'docker'],
          platformData: {
            sources: ['kubernetes'],
            agent: {
              platform: 'linux',
              osName: 'Unraid OS 7.2.2',
              osVersion: '7.2.2',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Unraid 7.2.2']);
  });

  it('matches governed platform display tokens inside reported host identity text', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          type: 'agent',
          platformType: 'agent',
          sourceType: 'agent',
          platformData: {
            sources: ['agent'],
            agent: {
              platform: 'linux',
              osName: 'TrueNAS SCALE',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['TrueNAS']);
  });

  it('keeps API-backed platform identity ahead of reported host OS', () => {
    const resource = makeResource({
      platformType: 'proxmox-pve',
      sourceType: 'hybrid',
      platformData: {
        sources: ['agent', 'proxmox-pve'],
        agent: {
          platform: 'debian',
          osName: 'Debian GNU/Linux',
          osVersion: '12',
        },
      },
    });

    expect(getInfrastructureSystemIdentityBadges(resource).map((badge) => badge.label)).toEqual([
      'PVE',
    ]);
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('PVE');
  });

  it('shows platform versions when the reported version belongs to the platform identity', () => {
    const agentDiscoveredPve = makeResource({
      type: 'agent',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      platformData: {
        sources: ['agent'],
        agent: {
          platform: 'debian',
          osName: 'Proxmox VE',
          osVersion: '9.1.9',
        },
      },
    });

    expect(
      getInfrastructureSystemIdentityBadges(agentDiscoveredPve).map((badge) => badge.label),
    ).toEqual(['PVE 9.1.9']);

    const apiBackedPve = makeResource({
      type: 'agent',
      platformType: 'proxmox-pve',
      sourceType: 'api',
      sources: ['proxmox'],
      platformData: {
        sources: ['proxmox'],
        proxmox: {
          pveVersion: '8.3.2',
        },
      },
    });

    expect(getInfrastructureSystemIdentityBadges(apiBackedPve).map((badge) => badge.label)).toEqual(
      ['PVE 8.3.2'],
    );
  });

  it('keeps top-level platform facets ahead of collection method identity', () => {
    const resource = makeResource({
      platformType: 'agent',
      sourceType: 'agent',
      proxmox: {
        nodeName: 'pi',
        instance: 'pi',
      },
      agent: {
        hostname: 'pi',
        platform: 'debian',
        osName: 'Debian GNU/Linux',
        osVersion: '12',
      },
    } as Partial<Resource>);

    expect(getInfrastructureSystemIdentityBadges(resource).map((badge) => badge.label)).toEqual([
      'PVE',
    ]);
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('PVE');
  });

  it('uses probe protocol identity for agentless network endpoints', () => {
    const resource = makeResource({
      type: 'network-endpoint',
      platformType: 'generic',
      sourceType: 'api',
      platformData: {
        sources: ['availability'],
        availability: {
          protocol: 'tcp',
          address: 'power-meter-01.lab.local',
          port: 1883,
        },
      },
    });

    expect(getInfrastructureSystemIdentityBadges(resource).map((badge) => badge.label)).toEqual([
      'TCP',
    ]);
    expect(getInfrastructureSystemIdentityBadges(resource)[0]?.title).toBe(
      'TCP availability probe power-meter-01.lab.local:1883',
    );
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('TCP');
  });

  it('uses ICMP identity for ping-only availability endpoints', () => {
    const resource = makeResource({
      type: 'network-endpoint',
      platformType: 'generic',
      sourceType: 'api',
      platformData: {
        sources: ['availability'],
        availability: {
          protocol: 'icmp',
          address: 'ups-rack-a.lab.local',
        },
      },
    });

    expect(getInfrastructureSystemIdentityBadges(resource).map((badge) => badge.label)).toEqual([
      'ICMP',
    ]);
    expect(getInfrastructureSystemIdentityBadges(resource)[0]?.title).toBe(
      'ICMP availability probe ups-rack-a.lab.local',
    );
  });

  it('falls back to reported OS identity for agent-only systems', () => {
    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          platformData: {
            sources: ['agent'],
            agent: {
              platform: 'linux',
              osName: 'Ubuntu 24.04.2 LTS',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['Ubuntu']);

    expect(
      getInfrastructureSystemIdentityBadges(
        makeResource({
          platformData: {
            sources: ['agent'],
            agent: {
              platform: 'qnap',
              osName: 'QNAP QTS',
            },
          },
        }),
      ).map((badge) => badge.label),
    ).toEqual(['QNAP']);
  });

  it('deduplicates repeated header badge labels', () => {
    const badges = dedupeResourceBadges([
      getTypeBadge('agent'),
      getPlatformBadge('proxmox-pve'),
      getSourceBadge('agent'),
    ]);
    expect(badges.map((badge) => badge.label)).toEqual(['Agent', 'PVE']);
  });
});
