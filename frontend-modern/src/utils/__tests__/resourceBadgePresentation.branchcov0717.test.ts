import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  getContainerRuntimeBadgeForRuntime,
  getInfrastructurePlatformBadges,
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemIdentitySortLabel,
  getInfrastructureSystemTitleBadges,
  getPlatformBadge,
  getSourceBadge,
  getTypeBadge,
  getUnifiedSourceBadges,
  dedupeResourceBadges,
} from '@/utils/resourceBadgePresentation';

// Module-private helpers (`titleFromParts`, `withBadgeVersion`, `normalizeVersion`,
// `buildUnifiedSourceBadges`, `deriveSourceKeysFromFacets`, `getKnownHostIdentitySource`,
// `getHostIdentityAgentProfile`, `getHostIdentityPlatform`, `getAgentSystemIdentityBadge`,
// `getDockerHostOsIdentityBadge`, `getAvailabilitySystemIdentityBadge`,
// `getStorageSystemIdentityBadge`, `getStoragePlatformSource`, `getSystemSourceVersion`,
// `getVersionedSourceBadge`, `proxmoxLxcDockerBadge`, `proxmoxLxcDockerVmid`,
// `badgeIdentityLabels`) are exercised transitively through the exported entry points
// below, asserting on their observable outputs.

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

const agentOnly = (agent: Record<string, unknown>): Partial<Resource> => ({
  type: 'agent',
  platformType: 'agent',
  sourceType: 'agent',
  sources: ['agent'],
  platformData: { sources: ['agent'], agent },
});

describe('getPlatformBadge — branch coverage', () => {
  it('returns null when platformType is absent', () => {
    expect(getPlatformBadge()).toBeNull();
    expect(getPlatformBadge(undefined)).toBeNull();
    expect(getPlatformBadge('' as never)).toBeNull();
  });

  it('renders the dedicated availability badge for the availability platform type', () => {
    const badge = getPlatformBadge('availability');
    expect(badge).toStrictEqual({
      label: 'Availability',
      classes:
        'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
      title: 'Availability',
    });
  });

  it('renders shared presentation badges for recognized platform types', () => {
    expect(getPlatformBadge('kubernetes')?.label).toBe('K8s');
    expect(getPlatformBadge('truenas')?.label).toBe('TrueNAS');
    expect(getPlatformBadge('vmware-vsphere')?.label).toBe('vSphere');
  });
});

describe('getSourceBadge — branch coverage', () => {
  it('returns null when sourceType is absent', () => {
    expect(getSourceBadge(undefined)).toBeNull();
    expect(getSourceBadge('' as never)).toBeNull();
  });

  it('renders the canonical badge for a known source type', () => {
    const badge = getSourceBadge('api');
    expect(badge?.label).toBe('API');
    expect(badge?.classes).toContain('bg-blue-100');
    expect(badge?.title).toBe('api');
  });

  it('falls back to the raw label and neutral tone for an unrecognized source type', () => {
    const badge = getSourceBadge('custom-collector' as never);
    expect(badge?.label).toBe('custom-collector');
    expect(badge?.classes).toContain('bg-surface-alt');
    expect(badge?.title).toBe('custom-collector');
  });
});

describe('getTypeBadge — branch coverage', () => {
  it('returns null when resourceType is absent', () => {
    expect(getTypeBadge(undefined)).toBeNull();
    expect(getTypeBadge('')).toBeNull();
    expect(getTypeBadge(null as unknown as string)).toBeNull();
  });

  it('renders a canonical label for a known resource type', () => {
    expect(getTypeBadge('pbs')?.label).toBe('PBS');
  });

  it('maps a legacy/external alias to its presentation', () => {
    expect(getTypeBadge('proxmox-lxc')?.label).toBe('LXC');
  });

  it('falls back to a default presentation for an unrecognized type string', () => {
    const badge = getTypeBadge('totally-fake-type');
    expect(badge?.label).toBe('totally-fake-type');
    expect(badge?.title).toBe('totally-fake-type');
    expect(badge?.classes).toContain('bg-surface-alt');
  });
});

describe('getUnifiedSourceBadges / normalizeUnifiedSourceKeys — branch coverage', () => {
  it('returns an empty array for nullish or empty input', () => {
    expect(getUnifiedSourceBadges(null)).toEqual([]);
    expect(getUnifiedSourceBadges(undefined)).toEqual([]);
    expect(getUnifiedSourceBadges([])).toEqual([]);
  });

  it('drops values that do not normalize to a known platform', () => {
    expect(getUnifiedSourceBadges(['totally-unknown-source'])).toEqual([]);
  });

  it('maps canonical aliases and dedupes to a single badge per platform', () => {
    expect(getUnifiedSourceBadges(['pve', 'proxmox', 'pbs', 'pmg']).map((b) => b.label)).toEqual([
      'PVE',
      'PBS',
      'PMG',
    ]);
  });

  it('normalizes vmware, k8s, and hyper-v aliases', () => {
    expect(getUnifiedSourceBadges(['vmware', 'k8s', 'hyper-v']).map((b) => b.label)).toEqual([
      'vSphere',
      'K8s',
      'Hyper-V',
    ]);
  });

  it('renders the availability and generic source presentations', () => {
    expect(getUnifiedSourceBadges(['availability']).map((b) => b.label)).toEqual(['Availability']);
    expect(getUnifiedSourceBadges(['generic']).map((b) => b.label)).toEqual(['Generic']);
  });
});

describe('getInfrastructurePlatformBadges — branch coverage', () => {
  it('returns an empty array when no source normalizes to a known platform', () => {
    expect(getInfrastructurePlatformBadges([])).toEqual([]);
    expect(getInfrastructurePlatformBadges(undefined)).toEqual([]);
    expect(getInfrastructurePlatformBadges(['totally-unknown-source'])).toEqual([]);
  });

  it('passes a single source straight through', () => {
    expect(getInfrastructurePlatformBadges(['docker']).map((b) => b.label)).toEqual([
      'Docker / Podman',
    ]);
  });

  it('keeps every non-agent platform source when multiple platforms remain', () => {
    expect(getInfrastructurePlatformBadges(['docker', 'kubernetes']).map((b) => b.label)).toEqual([
      'Docker / Podman',
      'K8s',
    ]);
  });
});

describe('getContainerRuntimeBadgeForRuntime / getContainerRuntimeTone — branch coverage', () => {
  it('returns null for an empty or whitespace-only runtime', () => {
    expect(getContainerRuntimeBadgeForRuntime('')).toBeNull();
    expect(getContainerRuntimeBadgeForRuntime('   ')).toBeNull();
    expect(getContainerRuntimeBadgeForRuntime(null)).toBeNull();
    expect(getContainerRuntimeBadgeForRuntime(undefined)).toBeNull();
  });

  it('normalizes Docker casing before selecting the docker tone', () => {
    const badge = getContainerRuntimeBadgeForRuntime('DOCKER');
    expect(badge?.label).toBe('Docker');
    expect(badge?.title).toBe('Runtime: Docker');
    expect(badge?.classes).toContain('bg-sky-100');
  });

  it('selects the podman tone for a Podman runtime', () => {
    const badge = getContainerRuntimeBadgeForRuntime('podman');
    expect(badge?.label).toBe('Podman');
    expect(badge?.title).toBe('Runtime: Podman');
    expect(badge?.classes).toContain('bg-violet-100');
  });

  it('falls back to the neutral tone for an unrecognized runtime label', () => {
    const badge = getContainerRuntimeBadgeForRuntime('containerd');
    expect(badge?.label).toBe('containerd');
    expect(badge?.classes).toContain('bg-surface-alt');
    expect(badge?.classes).toContain('text-base-content');
  });
});

describe('dedupeResourceBadges — branch coverage', () => {
  it('drops nullish entries, empty labels, and duplicate identities', () => {
    expect(
      dedupeResourceBadges([
        null,
        { label: 'PVE', classes: 'c' },
        { label: 'pve', classes: 'c' },
        { label: '   ', classes: 'c' },
        undefined,
      ]),
    ).toEqual([{ label: 'PVE', classes: 'c' }]);
  });
});

describe('badgeIdentityLabels via getInfrastructureSystemTitleBadges — branch coverage', () => {
  it('treats a system badge with no title as its label-only identity (value ? ... : "" arm)', () => {
    // badgeIdentityLabels maps [badge.label, badge.title]; when title is undefined
    // the `(value ? normalizeBadgeIdentityLabel(value) : '')` ternary takes the ""
    // arm and the entry is dropped by filter(Boolean). The label identity survives,
    // so a source badge sharing it is deduped away.
    const result = getInfrastructureSystemTitleBadges(
      [{ label: 'PVE', classes: 'type-cls' }],
      [{ label: 'pve', classes: 'type-cls', title: 'pve' }],
    );
    expect(result).toEqual([{ label: 'PVE', classes: 'type-cls' }]);
  });

  it('retains a source badge whose identity is absent from the system identity set', () => {
    const result = getInfrastructureSystemTitleBadges(
      [{ label: 'PVE', classes: 'type-cls' }],
      [{ label: 'K8s', classes: 'type-cls', title: 'K8s' }],
    );
    expect(result.map((b) => b.label)).toEqual(['PVE', 'K8s']);
  });
});

describe('withBadgeVersion via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('strips the Proxmox pve-manager wrapper format down to the dotted version', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'debian',
          osName: 'Proxmox VE',
          osVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE 9.1.9']);
  });

  it.each(['unknown', 'n/a', 'na', 'none', '-'] as const)(
    'treats sentinel version %s as empty so no suffix is appended',
    (sentinelVersion) => {
      const badges = getInfrastructureSystemIdentityBadges(
        makeResource(
          agentOnly({
            platform: 'linux',
            osName: 'TrueNAS SCALE',
            osVersion: sentinelVersion,
          }),
        ),
      );
      expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
    },
  );

  it('keeps the title unchanged when the version already appears in it (? title arm)', () => {
    // The agent OS-label branch builds a title containing the version; withBadgeVersion
    // then detects the version substring in the title and reuses it verbatim rather
    // than appending a duplicate.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'linux',
          osName: 'Ubuntu 24.04',
          osVersion: '24.04',
        }),
      ),
    );
    expect(badges[0]?.label).toBe('Ubuntu 24.04');
    expect(badges[0]?.title).toBe('Ubuntu 24.04 24.04');
  });
});

describe('getAgentSystemIdentityBadge via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('resolves an exact agent host profile id (exactProfile return arm)', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          hostProfile: 'unraid',
          osName: 'Unraid',
          osVersion: '7.3.0',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid 7.3.0']);
  });

  it('falls back to the badge title in the tooltip when osName is empty (hostProfile branch || arm)', () => {
    // `titleFromParts(osName || badge?.title || profileFamily || label, osVersion)`:
    // empty osName makes the `osName ||` operand falsy so evaluation proceeds to
    // `badge?.title`, but the badge still resolves.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          hostProfile: 'unraid',
          osName: '',
          osVersion: '',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid']);
    expect(badges[0]?.title).toBe('Unraid');
  });

  it('derives a known source from a raw platform key via normalizeSourcePlatformKey', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'linux',
          osName: 'truenas',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('falls back to the badge title in the tooltip when osName is empty (knownSource branch || arm)', () => {
    // `titleFromParts(osName || badge.title, osVersion)`: no hostProfile, empty osName,
    // but the platform key alone resolves a known source.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'truenas',
          osName: '',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
    expect(badges[0]?.title).toBe('TrueNAS');
  });

  it('uses the second identity value when the first does not match', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'unknown-os',
          osName: 'truenas',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('matches an agent host profile identity token in the osName', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'unknown-os',
          osName: 'my unraid server',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid']);
  });

  it('matches a source platform display token in the osName', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'linux',
          osName: 'Synology DSM',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Synology']);
  });

  it('derives the OS label from platform when osName is empty (osLabel branch)', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'freebsd',
          osName: '',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['FreeBSD']);
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
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'linux',
          osName,
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual([expected]);
  });

  it('excludes docker/generic from host identity and falls through to the platform badge', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: 'generic',
          osName: 'generic host',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Agent']);
  });
});

describe('getAvailabilitySystemIdentityBadge via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  const endpoint = (
    availability: Record<string, unknown>,
    overrides: Partial<Resource> = {},
  ): Resource =>
    makeResource({
      type: 'network-endpoint',
      platformType: 'generic',
      sourceType: 'api',
      sources: ['availability'],
      platformData: { sources: ['availability'], availability },
      ...overrides,
    });

  it('maps the icmp protocol to an uppercased label', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      endpoint({ protocol: 'icmp', address: 'host-a' }),
    );
    expect(badges.map((b) => b.label)).toEqual(['ICMP']);
    expect(badges[0]?.title).toBe('ICMP availability probe host-a');
  });

  it('maps tcp with a port to a TCP label and appends the port to the title', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      endpoint({ protocol: 'tcp', address: 'host-a', port: 22 }),
    );
    expect(badges.map((b) => b.label)).toEqual(['TCP']);
    expect(badges[0]?.title).toBe('TCP availability probe host-a:22');
  });

  it('drops the port suffix when the port is zero', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      endpoint({ protocol: 'tcp', address: 'host-a', port: 0 }),
    );
    expect(badges[0]?.title).toBe('TCP availability probe host-a');
  });

  it('maps http and https to uppercased labels without a port when none is set', () => {
    const http = getInfrastructureSystemIdentityBadges(endpoint({ protocol: 'http' }));
    expect(http.map((b) => b.label)).toEqual(['HTTP']);
    expect(http[0]?.title).toBe('HTTP availability probe');
    const https = getInfrastructureSystemIdentityBadges(endpoint({ protocol: 'https' }));
    expect(https.map((b) => b.label)).toEqual(['HTTPS']);
  });

  it('uppercases an unrecognized non-empty protocol', () => {
    const badges = getInfrastructureSystemIdentityBadges(endpoint({ protocol: 'snmp' }));
    expect(badges.map((b) => b.label)).toEqual(['SNMP']);
  });

  it('falls back to the Probe label when the protocol is empty', () => {
    const badges = getInfrastructureSystemIdentityBadges(endpoint({ address: 'host-a' }));
    expect(badges.map((b) => b.label)).toEqual(['Probe']);
    expect(badges[0]?.title).toBe('Probe availability probe host-a');
  });

  it('treats a raw availability source as an endpoint even without an availability facet', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['availability'],
        platformData: { sources: ['availability'] },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Probe']);
    expect(badges[0]?.title).toBe('Probe availability probe');
  });
});

describe('getStorageSystemIdentityBadge / getStoragePlatformSource via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('versions a TrueNAS storage identity and surfaces the topology in the title', () => {
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

  it('reads the storage platform from platformData.platform for a storage-typed resource', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        sources: ['pbs'],
        platformData: {
          sources: ['pbs'],
          storage: { type: 'pbs-datastore', topology: 'datastore' },
          platform: 'pbs',
          pbs: { version: 2 },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PBS 2']);
  });

  it('resolves the docker version arm when the storage platform is docker', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'docker',
        sourceType: 'agent',
        sources: ['docker'],
        platformData: {
          sources: ['docker'],
          storage: { platform: 'docker', type: 'docker-volume' },
          docker: { runtimeVersion: '24.0.7' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman 24.0.7']);
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

  it('returns null for an unrecognized storage platform and falls through', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          storage: { platform: 'mystery-nas', type: 'mystery-array' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Agent']);
  });

  it('skips platformData.platform for a non-storage resource type (ternary "" arm)', () => {
    // An agent-shaped resource carrying a storage facet with no platform: the ternary
    // takes the "" arm (type is 'agent') so storagePlatform resolves to null and the
    // storage identity badge is skipped, letting the caller fall through.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          storage: { type: 'mystery-array' },
          platform: 'should-be-ignored',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Agent']);
  });
});

describe('getSystemSourceVersion via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('versions PBS, PMG, vSphere, and K8s identities from their facet versions', () => {
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

  it('falls back to platformData.version for proxmox-pmg when the pmg facet omits version', () => {
    // PMG case: `getRecordVersion(pmg, 'version') || getRecordVersion(platformData, 'version')`
    // — the pmg facet has no version so the `||` evaluates the right operand.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'pmg',
        platformType: 'proxmox-pmg',
        sourceType: 'api',
        sources: ['pmg'],
        platformData: {
          sources: ['pmg'],
          pmg: { hostname: 'mailgw-1' },
          version: '9.1.2',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PMG 9.1.2']);
  });

  it('falls back to the platform facet version in the default case when the source facet omits it', () => {
    // Default case: `getRecordVersion(sourceRecord, ...) || getRecordVersion(platformRecord, ...)`.
    // A presentation-only platform (synology-dsm) routes here; its own facet lacks a
    // version, so the `||` evaluates the platform-record operand.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['synology-dsm'],
        platformData: {
          sources: ['synology-dsm'],
          'synology-dsm': { note: 'no version here' },
          platform: { version: '7.2.1' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Synology 7.2.1']);
  });

  it('drops the version suffix when neither the source facet nor the platform facet carries one', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'agent',
        sourceType: 'api',
        sources: ['microsoft-hyperv'],
        platformData: {
          sources: ['microsoft-hyperv'],
          'microsoft-hyperv': { note: 'no version here' },
          platform: { note: 'also no version' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Hyper-V']);
  });

  it('falls back to platformData.pveVersion for proxmox-pve when the proxmox facet omits it', () => {
    // PVE case: `getRecordVersion(proxmox,'pveVersion','version') || getRecordVersion(platformData,'pveVersion')`
    // — the proxmox facet has no version so the `||` evaluates the platformData operand.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        sources: ['proxmox'],
        platformData: {
          sources: ['proxmox'],
          proxmox: { hostname: 'pve-1' },
          pveVersion: '8.2.4',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE 8.2.4']);
  });

  it('falls back to the agent osVersion for proxmox-pve when no platform version is present', () => {
    // PVE case third operand: both version records are empty, so the `||` evaluates
    // getAgentPlatformVersion, which proceeds because the agent identity matches the
    // resolved source.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        sources: ['proxmox'],
        platformData: {
          sources: ['proxmox'],
          proxmox: { hostname: 'pve-1' },
          agent: { platform: 'proxmox-pve', osVersion: '8.2.4' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE 8.2.4']);
  });

  it('falls back to platformData.version for proxmox-pbs when the pbs facet omits version', () => {
    // PBS case: `getRecordVersion(pbs, 'version') || getRecordVersion(platformData, 'version')`.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'pbs',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        sources: ['pbs'],
        platformData: {
          sources: ['pbs'],
          pbs: { hostname: 'pbs-1' },
          version: '3.3.2',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PBS 3.3.2']);
  });
});

describe('getHostIdentityAgentProfile / getAgentIdentitySource via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('resolves an exact agent host profile id passed as the osName (exactProfile return arm)', () => {
    // getKnownHostIdentitySource feeds 'unraid' to getHostIdentityAgentProfile, which
    // resolves an exact manifest entry and returns it before the token-search fallback.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource(
        agentOnly({
          platform: '',
          osName: 'unraid',
        }),
      ),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid']);
  });

  it('skips a non-agent-profile hostProfile and falls back to platformData.osVersion for unraid', () => {
    // getAgentIdentitySource sees hostProfile 'proxmox-pve' (not an agent host profile),
    // so the `if (profile && getSourcePlatformManifestEntry(profile.id))` guard is false
    // and it falls through; getAgentPlatformVersion then returns '' for the 'unraid'
    // source because the resolved identity differs, so the unraid case falls back to
    // platformData.osVersion.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'storage',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent'],
        platformData: {
          sources: ['agent'],
          storage: { platform: 'unraid' },
          agent: { hostProfile: 'proxmox-pve', platform: 'linux' },
          osVersion: '6.9.0',
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Unraid 6.9.0']);
  });
});

describe('deriveSourceKeysFromFacets via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  it('derives the system source from a proxmox facet when no explicit sources are set', () => {
    // resource.sources is absent, so rawSources is built from platformData.sources
    // plus deriveSourceKeysFromFacets; the proxmox facet pushes 'proxmox' which then
    // resolves the PVE version from pveVersion.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        platformData: {
          sources: [],
          proxmox: { pveVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9' },
        },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['PVE 9.1.9']);
  });
});

describe('getDockerHostOsIdentityBadge via getInfrastructureSystemIdentityBadges — branch coverage', () => {
  const dockerHost = (docker: Resource['docker']): Resource =>
    makeResource({
      type: 'docker-host',
      platformType: 'docker',
      sourceType: 'agent',
      sources: ['docker'],
      platformData: { sources: ['docker'] },
      docker,
    });

  it('maps a docker OS string that matches a governed platform to the shared badge', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      dockerHost({ os: 'TrueNAS SCALE', runtime: 'docker' }),
    );
    expect(badges.map((b) => b.label)).toEqual(['TrueNAS']);
  });

  it('derives a host OS label from docker metadata when no known source matches', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      dockerHost({ os: 'Ubuntu 24.04.2 LTS', runtime: 'docker' }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Ubuntu']);
    expect(badges[0]?.title).toBe('Ubuntu 24.04.2 LTS');
  });

  it('falls back to the docker runtime badge when the docker OS string is unrecognized', () => {
    const badges = getInfrastructureSystemIdentityBadges(
      dockerHost({ os: 'acme-custom-os', runtime: 'docker' }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman']);
  });
});

describe('proxmoxLxcDockerBadge / proxmoxLxcDockerVmid via getInfrastructureSystemIdentityBadges — branch coverage', () => {
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

  it('produces no LXC badge when docker.hostSourceId is a non-LXC string (startsWith false arm)', () => {
    // A string hostSourceId that is not the LXC prefix takes the early-return arm, so
    // no LXC badge is emitted and the docker runtime badge is used instead.
    const badges = getInfrastructureSystemIdentityBadges(
      makeResource({
        type: 'docker-host',
        platformType: 'docker',
        sourceType: 'agent',
        sources: ['docker'],
        docker: { hostSourceId: 'docker://box-01', runtime: 'docker' },
      }),
    );
    expect(badges.map((b) => b.label)).toEqual(['Docker / Podman']);
  });
});

describe('getInfrastructureSystemIdentityBadges — end-to-end branch coverage', () => {
  it('falls back to the platform badge for a bare agent resource with no other identity', () => {
    const resource = makeResource({
      type: 'agent',
      platformType: 'agent',
      sourceType: 'agent',
      sources: [],
    });
    expect(getInfrastructureSystemIdentityBadges(resource).map((b) => b.label)).toEqual(['Agent']);
  });

  it('returns an empty badge set when nothing resolves', () => {
    const resource = makeResource({
      id: 'lonely',
      name: 'lonely',
      type: 'vm',
      platformType: undefined,
      sourceType: 'api',
      sources: [],
    });
    expect(getInfrastructureSystemIdentityBadges(resource)).toEqual([]);
  });
});

describe('getInfrastructureSystemIdentitySortLabel — branch coverage', () => {
  it('returns the first identity badge label when one resolves', () => {
    const resource = makeResource({
      type: 'k8s-node',
      platformType: 'kubernetes',
      sourceType: 'api',
      sources: ['kubernetes'],
      platformData: { sources: ['kubernetes'], kubernetes: { version: '1.28.4' } },
    });
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('K8s 1.28.4');
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

  it('returns "" when no identity, no platform badge, and no platformType resolve', () => {
    const resource = makeResource({
      type: 'storage',
      platformType: '' as Resource['platformType'],
      sourceType: 'agent',
      sources: [],
    });
    expect(getInfrastructureSystemIdentitySortLabel(resource)).toBe('');
  });
});
