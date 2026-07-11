import { describe, expect, it } from 'vitest';
import {
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import type { PlatformType, Resource, ResourceType, SourceType } from '@/types/resource';

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

const agentOnly = (agent: Record<string, unknown>): Partial<Resource> => ({
  type: 'agent',
  platformType: 'agent',
  sourceType: 'agent',
  sources: ['agent'],
  platformData: { sources: ['agent'], agent },
});

describe('getTypeBadge — branch coverage', () => {
  it('returns null for undefined', () => {
    expect(getTypeBadge(undefined)).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(getTypeBadge('')).toBeNull();
  });

  it('returns null for null', () => {
    expect(getTypeBadge(null as unknown as ResourceType)).toBeNull();
  });

  it('returns a canonical badge for a known resource type', () => {
    const badge = getTypeBadge('pbs');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('PBS');
  });

  it('returns a badge for an external/legacy type alias', () => {
    const badge = getTypeBadge('proxmox-lxc');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('LXC');
  });

  it('falls back to default presentation for an unrecognized type string', () => {
    const badge = getTypeBadge('totally-fake-type');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('totally-fake-type');
    expect(badge?.title).toBe('totally-fake-type');
    expect(badge?.classes).toContain('bg-surface-alt');
  });
});

describe('getSourceBadge — branch coverage', () => {
  it('returns null for undefined', () => {
    expect(getSourceBadge(undefined)).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(getSourceBadge('' as unknown as SourceType)).toBeNull();
  });

  it('returns presentation badge for a known source type', () => {
    const badge = getSourceBadge('api');
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('API');
    expect(badge?.classes).toContain('bg-blue-100');
    expect(badge?.title).toBe('api');
  });

  it('falls back to raw label and typeClasses for an unrecognized source type', () => {
    const badge = getSourceBadge('custom-collector' as SourceType);
    expect(badge).not.toBeNull();
    expect(badge?.label).toBe('custom-collector');
    expect(badge?.classes).toContain('bg-surface-alt');
    expect(badge?.title).toBe('custom-collector');
  });
});

describe('buildUnifiedSourceBadges (via getUnifiedSourceBadges) — branch coverage', () => {
  it('returns an empty array for null', () => {
    expect(getUnifiedSourceBadges(null)).toEqual([]);
  });

  it('returns an empty array for undefined', () => {
    expect(getUnifiedSourceBadges(undefined)).toEqual([]);
  });

  it('returns an empty array for an empty array', () => {
    expect(getUnifiedSourceBadges([])).toEqual([]);
  });

  it('normalizes alias keys to canonical platform badges', () => {
    const badges = getUnifiedSourceBadges(['pve', 'pbs', 'pmg']);
    expect(badges.map((b) => b.label)).toEqual(['PVE', 'PBS', 'PMG']);
  });

  it('normalizes vmware, k8s, and hyper-v aliases', () => {
    const badges = getUnifiedSourceBadges(['vmware', 'k8s', 'hyper-v']);
    expect(badges.map((b) => b.label)).toEqual(['vSphere', 'K8s', 'Hyper-V']);
  });

  it('handles generic source platform', () => {
    const badges = getUnifiedSourceBadges(['generic']);
    expect(badges).toHaveLength(1);
    expect(badges[0]?.label).toBe('Generic');
  });

  it('handles presentation-only cloud platform sources', () => {
    const badges = getUnifiedSourceBadges(['aws', 'azure', 'gcp']);
    expect(badges.map((b) => b.label)).toEqual(['AWS', 'Azure', 'GCP']);
  });
});

describe('withBadgeVersion (via getInfrastructureSystemIdentityBadges) — branch coverage', () => {
  it('strips the Proxmox version wrapper format', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'debian',
        osName: 'Proxmox VE',
        osVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'PVE 9.1.9',
    ]);
  });

  it.each(['unknown', 'n/a', 'na', 'none', '-'] as const)(
    'treats sentinel version %s as empty so no suffix is appended',
    (sentinelVersion) => {
      const resource = makeResource(
        agentOnly({
          platform: 'linux',
          osName: 'TrueNAS SCALE',
          osVersion: sentinelVersion,
        }),
      );
      expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
        'TrueNAS',
      ]);
    },
  );

  it('accepts a numeric version field from a storage facet', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: 'proxmox-pbs',
      sourceType: 'api',
      sources: ['pbs'],
      platformData: {
        sources: ['pbs'],
        storage: { platform: 'pbs', type: 'pbs-datastore', topology: 'datastore' },
        pbs: { version: 2 },
      },
      storage: {
        platform: 'pbs',
        type: 'pbs-datastore',
        topology: 'datastore',
      } as Resource['storage'],
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'PBS 2',
    ]);
  });

  it('keeps the title unchanged when the version already appears in the title', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'debian',
        osName: 'Debian 12',
        osVersion: '12',
      }),
    );
    const badges = getInfrastructureSystemIdentityBadges(resource);
    expect(badges[0]?.label).toBe('Debian 12');
    expect(badges[0]?.title).toBe('Debian 12 12');
  });

  it('trims whitespace-padded version strings', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'linux',
        osName: 'TrueNAS SCALE',
        osVersion: '  13.0  ',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'TrueNAS 13.0',
    ]);
  });
});

describe('getAgentSystemIdentityBadge (via getInfrastructureSystemIdentityBadges) — branch coverage', () => {
  it('returns null when no agent record exists (falls through to platform badge)', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: 'agent',
      sourceType: 'agent',
      platformData: { sources: ['agent'] },
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });

  it('produces a badge from a hostProfile that is a valid platform but not an agent host profile', () => {
    const resource = makeResource({
      type: 'docker-host',
      platformType: 'docker',
      sourceType: 'hybrid',
      platformData: {
        sources: ['agent', 'docker'],
        agent: {
          platform: 'linux',
          hostProfile: 'proxmox-pve',
          osName: 'Proxmox VE',
          osVersion: '8.2.4',
        },
      },
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'PVE 8.2.4',
    ]);
  });

  it('derives a known source from a raw platform key via normalizeSourcePlatformKey', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'linux',
        osName: 'truenas',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('derives a known source from manifest display tokens in the osName', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'linux',
        osName: 'Synology DSM',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Synology']);
  });

  it.each([
    ['Ubuntu 24.04.2 LTS', 'Ubuntu'],
    ['Debian GNU/Linux 12', 'Debian'],
    ['Rocky Linux 9.3', 'Rocky'],
    ['Alpine Linux 3.19', 'Alpine'],
    ['AlmaLinux 9.3', 'AlmaLinux'],
    ['CentOS Stream 9', 'CentOS'],
    ['RHEL 9.4', 'RHEL'],
    ['Arch Linux', 'Arch'],
    ['openSUSE Tumbleweed', 'openSUSE'],
    ['SUSE Linux Enterprise', 'SUSE'],
    ['macOS Sonoma 14.4', 'macOS'],
    ['Windows Server 2022', 'Windows'],
    ['FreeBSD 14.1', 'FreeBSD'],
    ['Linux Mint 22', 'Linux'],
  ] as const)('derives the OS label %s from osName via hostOsLabelPatterns', (osName, expected) => {
    const resource = makeResource(
      agentOnly({
        platform: 'linux',
        osName,
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([expected]);
  });

  it('derives the OS label from platform when osName is empty', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'freebsd',
        osName: '',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['FreeBSD']);
  });

  it('returns null for an agent with no recognizable identity (falls through)', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'barbaz',
        osName: 'Foobar OS',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });
});

describe('getKnownHostIdentitySource (via identity badges) — branch coverage', () => {
  it('matches an agent host profile identity token in the second value', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'unknown-os',
        osName: 'my unraid server',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Unraid']);
  });

  it('uses the second value when the first does not match', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'unknown-os',
        osName: 'truenas',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('excludes docker from host identity even when docker tokens are present', () => {
    const resource = makeResource(
      agentOnly({
        platform: '',
        osName: 'docker desktop',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });

  it('excludes generic from host identity sources', () => {
    const resource = makeResource(
      agentOnly({
        platform: 'generic',
        osName: 'generic host',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });
});

describe('getHostIdentityAgentProfile (via identity badges) — branch coverage', () => {
  it('matches the exact profile id', () => {
    const resource = makeResource(
      agentOnly({
        platform: '',
        osName: 'unraid',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Unraid']);
  });

  it('returns null for a whitespace-only value (falls through to other sources)', () => {
    const resource = makeResource(
      agentOnly({
        platform: '   ',
        osName: 'truenas',
      }),
    );
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['TrueNAS']);
  });
});

describe('getStorageSystemIdentityBadge (via getInfrastructureSystemIdentityBadges) — branch coverage', () => {
  it('returns a versioned badge for storage with a known platform and topology', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: 'truenas',
      sourceType: 'api',
      sources: ['truenas'],
      platformData: {
        sources: ['truenas'],
        storage: { platform: 'truenas', type: 'zpool', topology: 'pool-0' },
        truenas: { version: '13.0' },
      },
      storage: {
        platform: 'truenas',
        type: 'zpool',
        topology: 'pool-0',
      } as Resource['storage'],
    });
    const badges = getInfrastructureSystemIdentityBadges(resource);
    expect(badges[0]?.label).toBe('TrueNAS 13.0');
    expect(badges[0]?.title).toContain('pool-0');
  });

  it('hits the docker version switch case when storage platform is docker', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: 'docker',
      sourceType: 'agent',
      sources: ['docker'],
      platformData: {
        sources: ['docker'],
        storage: { platform: 'docker', type: 'docker-volume' },
        docker: { runtimeVersion: '24.0.7' },
      },
      storage: {
        platform: 'docker',
        type: 'docker-volume',
      } as Resource['storage'],
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'Docker / Podman 24.0.7',
    ]);
  });

  it('returns null for storage with an unrecognized platform (falls through)', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      platformData: {
        sources: ['agent'],
        storage: { platform: 'mystery-nas', type: 'mystery-array' },
      },
      storage: {
        platform: 'mystery-nas',
        type: 'mystery-array',
      } as Resource['storage'],
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });
});

describe('getDockerHostOsIdentityBadge (via getInfrastructureSystemIdentityBadges) — branch coverage', () => {
  const dockerHostNoAgent = (docker: Resource['docker']): Resource =>
    makeResource({
      type: 'docker-host',
      platformType: 'docker',
      sourceType: 'agent',
      platformData: { sources: ['docker'] },
      docker,
    });

  it('returns a known-source badge when docker.os matches an agent host profile token', () => {
    const resource = dockerHostNoAgent({ os: 'Unraid OS 7.2.2', runtime: 'docker' });
    const badges = getInfrastructureSystemIdentityBadges(resource);
    expect(badges[0]?.label).toBe('Unraid');
    expect(badges[0]?.title).toContain('Unraid OS 7.2.2');
  });

  it('returns an OS-label badge when docker.os matches a hostOsLabelPattern but no known source', () => {
    const resource = dockerHostNoAgent({ os: 'Ubuntu Server 22.04 LTS', runtime: 'docker' });
    const badges = getInfrastructureSystemIdentityBadges(resource);
    expect(badges[0]?.label).toBe('Ubuntu');
    expect(badges[0]?.title).toBe('Ubuntu Server 22.04 LTS');
  });

  it('returns null for an unrecognized docker os (falls through to docker platform badge)', () => {
    const resource = dockerHostNoAgent({ os: 'MysteryOS 9000', runtime: 'docker' });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'Docker / Podman',
    ]);
  });

  it('returns null when there is no docker facet (falls through to docker platform badge)', () => {
    const resource = makeResource({
      type: 'docker-host',
      platformType: 'docker',
      sourceType: 'agent',
      platformData: { sources: ['docker'] },
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'Docker / Podman',
    ]);
  });

  it('returns null when docker.os is missing (falls through to docker platform badge)', () => {
    const resource = dockerHostNoAgent({ runtime: 'docker' });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual([
      'Docker / Podman',
    ]);
  });
});

describe('getInfrastructureSystemIdentitySortLabel — branch coverage', () => {
  it('returns the first badge label when identity badges exist', () => {
    const resource = makeResource({
      type: 'k8s-node',
      platformType: 'kubernetes',
      sourceType: 'api',
      sources: ['kubernetes'],
      platformData: {
        sources: ['kubernetes'],
        kubernetes: { version: '1.28.4' },
      },
    });
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('K8s 1.28.4');
  });

  it('returns empty string when no badges and platformType is falsy', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: '' as unknown as PlatformType,
      sourceType: 'agent',
    });
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('');
  });

  it('returns the platform badge label when no system identity is found', () => {
    const resource = makeResource({
      type: 'agent',
      platformType: 'truenas',
      sourceType: 'agent',
      platformData: { sources: ['agent'] },
    });
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('TrueNAS');
  });
});
