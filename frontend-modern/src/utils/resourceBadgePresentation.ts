import type { PlatformType, SourceType, Resource, ResourceType } from '@/types/resource';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { getPlatformAgentRecord, getPlatformDataRecord } from '@/utils/agentResources';
import {
  AGENT_HOST_PROFILE_IDS,
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES,
  SOURCE_PLATFORM_MANIFEST_ENTRIES,
  getAgentHostProfileFamily,
  getAgentHostProfileManifestEntry,
  getSourcePlatformManifestEntry,
} from '@/utils/platformSupportManifest';
import { normalizeSourcePlatformKey, type KnownSourcePlatform } from '@/utils/sourcePlatforms';
import { getSourceTypePresentation } from '@/utils/sourceTypePresentation';
import {
  canonicalResourceTypeForDisplay,
  getResourceTypePresentation,
} from '@/utils/resourceTypePresentation';

export interface ResourceBadge {
  label: string;
  classes: string;
  title?: string;
}

const baseBadge =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';

const typeClasses = 'bg-surface-alt text-base-content';
const availabilityBadgeClasses = 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300';

const PRIMARY_SYSTEM_SOURCE_PRIORITY: KnownSourcePlatform[] = [
  'proxmox-pve',
  'proxmox-pbs',
  'proxmox-pmg',
  'truenas',
  'vmware-vsphere',
  ...(AGENT_HOST_PROFILE_IDS.filter((profileId) =>
    getSourcePlatformManifestEntry(profileId),
  ) as KnownSourcePlatform[]),
  'synology-dsm',
  'microsoft-hyperv',
  'kubernetes',
];

const hostOsLabelPatterns: Array<{ pattern: RegExp; label: string }> = [
  { pattern: /\bqnap\b|\bqts\b|\bquts\b/i, label: 'QNAP' },
  { pattern: /\bubuntu\b/i, label: 'Ubuntu' },
  { pattern: /\bdebian\b/i, label: 'Debian' },
  { pattern: /\bproxmox\b/i, label: 'Proxmox' },
  { pattern: /\bfedora\b/i, label: 'Fedora' },
  { pattern: /\brocky\b/i, label: 'Rocky' },
  { pattern: /\balma\s*linux\b|\balmalinux\b/i, label: 'AlmaLinux' },
  { pattern: /\bcentos\b/i, label: 'CentOS' },
  { pattern: /\bred\s*hat\b|\brhel\b/i, label: 'RHEL' },
  { pattern: /\barch\b/i, label: 'Arch' },
  { pattern: /\balpine\b/i, label: 'Alpine' },
  { pattern: /\bopen\s*suse\b|\bopensuse\b/i, label: 'openSUSE' },
  { pattern: /\bsuse\b/i, label: 'SUSE' },
  { pattern: /\bfreebsd\b/i, label: 'FreeBSD' },
  { pattern: /\bwindows\b/i, label: 'Windows' },
  { pattern: /\bmac\s*os\b|\bmacos\b|\bdarwin\b/i, label: 'macOS' },
  { pattern: /\blinux\b/i, label: 'Linux' },
];

const trimString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const getResourceRecord = (resource: Resource): Record<string, unknown> =>
  resource as unknown as Record<string, unknown>;

const getFacetRecord = (
  resource: Resource,
  platformData: Record<string, unknown> | undefined,
  key: string,
): Record<string, unknown> | undefined =>
  asRecord(getResourceRecord(resource)[key]) || asRecord(platformData?.[key]);

const deriveSourceKeysFromFacets = (
  resource: Resource,
  platformData: Record<string, unknown> | undefined,
): string[] => {
  const sources: string[] = [];
  for (const [key, source] of [
    ['proxmox', 'proxmox'],
    ['pbs', 'pbs'],
    ['pmg', 'pmg'],
    ['vmware', 'vmware'],
    ['kubernetes', 'kubernetes'],
    ['docker', 'docker'],
    ['availability', 'availability'],
    ['agent', 'agent'],
  ] as const) {
    if (getFacetRecord(resource, platformData, key)) {
      sources.push(source);
    }
  }
  return sources;
};

const titleFromParts = (...parts: Array<string | undefined>): string | undefined => {
  const title = parts
    .map((part) => (part || '').trim())
    .filter(Boolean)
    .join(' ');
  return title || undefined;
};

const normalizeUnifiedSourceKeys = (sources?: string[] | null): KnownSourcePlatform[] => {
  if (!sources || sources.length === 0) return [];
  const normalized = sources
    .map((source) => normalizeSourcePlatformKey(source))
    .filter((source): source is KnownSourcePlatform => Boolean(source));
  return Array.from(new Set(normalized));
};

const buildUnifiedSourceBadges = (sources: KnownSourcePlatform[]): ResourceBadge[] =>
  sources.map((source) => {
    const sharedBadge = getSourcePlatformBadge(source);
    return {
      label: sharedBadge?.label ?? source,
      classes: sharedBadge?.classes ?? `${baseBadge} ${typeClasses}`,
      title: sharedBadge?.title ?? source,
    };
  });

export function getPlatformBadge(platformType?: PlatformType): ResourceBadge | null {
  if (!platformType) return null;
  if (platformType === 'availability') {
    return {
      label: 'Availability',
      classes: `${baseBadge} ${availabilityBadgeClasses}`,
      title: 'Availability',
    };
  }
  const sharedBadge = getSourcePlatformBadge(platformType);
  if (!sharedBadge) return null;
  return {
    label: sharedBadge.label,
    classes: sharedBadge.classes,
    title: sharedBadge.title,
  };
}

export function getSourceBadge(sourceType?: SourceType): ResourceBadge | null {
  if (!sourceType) return null;
  const presentation = getSourceTypePresentation(sourceType);
  return {
    label: presentation?.label ?? sourceType,
    classes: `${baseBadge} ${presentation?.badgeClasses ?? typeClasses}`,
    title: sourceType,
  };
}

export function getTypeBadge(resourceType?: ResourceType | string): ResourceBadge | null {
  if (!resourceType) return null;
  const normalizedType = canonicalResourceTypeForDisplay(resourceType);
  const presentation = getResourceTypePresentation(resourceType);
  if (!presentation) return null;
  return {
    label: presentation.label,
    classes: `${baseBadge} ${presentation.badgeClasses || typeClasses}`,
    title: normalizedType,
  };
}

export function getUnifiedSourceBadges(sources?: string[] | null): ResourceBadge[] {
  return buildUnifiedSourceBadges(normalizeUnifiedSourceKeys(sources));
}

export function getInfrastructurePlatformBadges(sources?: string[] | null): ResourceBadge[] {
  const normalized = normalizeUnifiedSourceKeys(sources);
  if (normalized.length <= 1) {
    return buildUnifiedSourceBadges(normalized);
  }

  const platformSources = normalized.filter((source) => source !== 'agent');
  return buildUnifiedSourceBadges(platformSources.length > 0 ? platformSources : normalized);
}

const firstSystemSource = (
  sources: KnownSourcePlatform[],
  platformType?: PlatformType,
): KnownSourcePlatform | null => {
  const sourceSet = new Set(sources);
  const normalizedPlatform = normalizeSourcePlatformKey(platformType);
  if (normalizedPlatform) {
    sourceSet.add(normalizedPlatform);
  }

  return PRIMARY_SYSTEM_SOURCE_PRIORITY.find((source) => sourceSet.has(source)) ?? null;
};

const textContainsToken = (value: string, token: string): boolean => {
  const normalizedToken = token.trim().toLowerCase();
  return Boolean(normalizedToken) && value.includes(normalizedToken);
};

const getHostIdentityAgentProfile = (value: string) => {
  const exactProfile = getAgentHostProfileManifestEntry(value);
  if (exactProfile) return exactProfile;

  const normalized = value.trim().toLowerCase();
  if (!normalized) return null;

  return (
    SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES.find((profile) =>
      [profile.id, profile.family, ...profile.hostIdentityTokens].some((token) =>
        textContainsToken(normalized, token),
      ),
    ) ?? null
  );
};

const getHostIdentityPlatform = (value: string) => {
  const exactPlatform = getSourcePlatformManifestEntry(value);
  if (exactPlatform) return exactPlatform;

  const normalized = value.trim().toLowerCase();
  if (!normalized) return null;

  return (
    SOURCE_PLATFORM_MANIFEST_ENTRIES.filter(
      (platform) => platform.id !== 'agent' && platform.id !== 'docker',
    ).find((platform) =>
      [platform.id, platform.family, ...platform.aliases, ...platform.displayTokens].some((token) =>
        textContainsToken(normalized, token),
      ),
    ) ?? null
  );
};

const getKnownHostIdentitySource = (...values: string[]): KnownSourcePlatform | null => {
  for (const value of values) {
    if (!value) continue;
    const profile = getHostIdentityAgentProfile(value);
    if (profile && getSourcePlatformManifestEntry(profile.id)) {
      return profile.id as KnownSourcePlatform;
    }
    const normalized = normalizeSourcePlatformKey(value);
    if (
      normalized &&
      normalized !== 'agent' &&
      normalized !== 'docker' &&
      normalized !== 'generic'
    ) {
      return normalized;
    }
    const manifestPlatform = getHostIdentityPlatform(value);
    if (manifestPlatform && manifestPlatform.id !== 'agent' && manifestPlatform.id !== 'docker') {
      return manifestPlatform.id as KnownSourcePlatform;
    }
  }
  return null;
};

const getAgentRecord = (resource: Resource): Record<string, unknown> | undefined =>
  getPlatformAgentRecord(resource) ?? asRecord(getResourceRecord(resource).agent);

const hasExplicitAgentHostProfile = (resource: Resource): boolean => {
  const agent = getAgentRecord(resource);
  const hostProfile = trimString(agent?.hostProfile);
  return Boolean(hostProfile && getAgentHostProfileManifestEntry(hostProfile));
};

const hasPrimarySystemFacet = (
  resource: Resource,
  platformData: Record<string, unknown> | undefined,
): boolean =>
  ['proxmox', 'pbs', 'pmg', 'vmware', 'kubernetes'].some((key) =>
    Boolean(getFacetRecord(resource, platformData, key)),
  );

const getHostOsLabel = (...values: string[]): string | null => {
  for (const value of values) {
    if (!value) continue;
    const match = hostOsLabelPatterns.find(({ pattern }) => pattern.test(value));
    if (match) return match.label;
  }
  return null;
};

const getAgentSystemIdentityBadge = (resource: Resource): ResourceBadge | null => {
  const agent = getAgentRecord(resource);
  if (!agent) return null;

  const hostProfile = trimString(agent.hostProfile);
  const platform = trimString(agent.platform);
  const osName = trimString(agent.osName);
  const osVersion = trimString(agent.osVersion);
  if (hostProfile) {
    const badge = getSourcePlatformBadge(hostProfile);
    const profileFamily = getAgentHostProfileFamily(hostProfile);
    const label = badge?.label ?? profileFamily;
    if (label) {
      return {
        label,
        classes: badge?.classes ?? `${baseBadge} ${typeClasses}`,
        title: titleFromParts(osName || badge?.title || profileFamily || label, osVersion) ?? label,
      };
    }
  }

  const knownSource = getKnownHostIdentitySource(platform, osName);
  if (knownSource) {
    const badge = getSourcePlatformBadge(knownSource);
    if (badge) {
      return {
        label: badge.label,
        classes: badge.classes,
        title: titleFromParts(osName || badge.title, osVersion) ?? badge.title,
      };
    }
  }

  const osLabel = getHostOsLabel(osName, platform);
  if (osLabel) {
    return {
      label: osLabel,
      classes: `${baseBadge} ${typeClasses}`,
      title: titleFromParts(osName || osLabel, osVersion),
    };
  }

  return null;
};

const getAvailabilitySystemIdentityBadge = (
  resource: Resource,
  platformData: Record<string, unknown> | undefined,
  rawSources: string[],
): ResourceBadge | null => {
  const availability = (asRecord(platformData?.availability) ||
    asRecord(getResourceRecord(resource).availability)) as
    | { address?: string; protocol?: string; port?: number }
    | undefined;
  const isAvailabilityEndpoint =
    resource.type === 'network-endpoint' ||
    Boolean(availability) ||
    rawSources.some((source) => source.trim().toLowerCase() === 'availability');

  if (!isAvailabilityEndpoint) return null;

  const protocol = trimString(availability?.protocol);
  const normalizedProtocol = protocol.toLowerCase();
  const protocolLabel =
    normalizedProtocol === 'icmp'
      ? 'ICMP'
      : normalizedProtocol === 'tcp'
        ? 'TCP'
        : normalizedProtocol === 'http' || normalizedProtocol === 'https'
          ? normalizedProtocol.toUpperCase()
          : protocol.toUpperCase() || 'Probe';
  const address = trimString(availability?.address);
  const port =
    Number.isFinite(availability?.port) && availability?.port ? `:${availability.port}` : '';
  return {
    label: protocolLabel,
    classes: `${baseBadge} ${availabilityBadgeClasses}`,
    title: titleFromParts(
      `${protocolLabel} availability probe`,
      address ? `${address}${port}` : undefined,
    ),
  };
};

export function getInfrastructureSystemIdentityBadges(resource: Resource): ResourceBadge[] {
  const platformData = getPlatformDataRecord(resource) as
    | (Record<string, unknown> & { sources?: string[] })
    | undefined;
  const rawSources = [
    ...(Array.isArray(platformData?.sources) ? platformData.sources : []),
    ...deriveSourceKeysFromFacets(resource, platformData),
  ];
  const sources = normalizeUnifiedSourceKeys(rawSources);
  const availabilityIdentityBadge = getAvailabilitySystemIdentityBadge(
    resource,
    platformData,
    rawSources,
  );
  if (availabilityIdentityBadge) {
    return [availabilityIdentityBadge];
  }

  const agentIdentityBadge = getAgentSystemIdentityBadge(resource);
  if (
    agentIdentityBadge &&
    hasExplicitAgentHostProfile(resource) &&
    !hasPrimarySystemFacet(resource, platformData)
  ) {
    return [agentIdentityBadge];
  }

  const systemSource = firstSystemSource(sources, resource.platformType);
  if (systemSource) {
    return buildUnifiedSourceBadges([systemSource]);
  }

  if (agentIdentityBadge) {
    return [agentIdentityBadge];
  }

  if (
    resource.platformType === 'docker' ||
    resource.type === 'docker-host' ||
    sources.includes('docker')
  ) {
    return buildUnifiedSourceBadges(['docker']);
  }

  const platformBadge = getPlatformBadge(resource.platformType);
  if (platformBadge) {
    return [platformBadge];
  }

  return getInfrastructurePlatformBadges(rawSources);
}

export function getInfrastructureSystemIdentitySortLabel(resource: Resource): string {
  return (
    getInfrastructureSystemIdentityBadges(resource)[0]?.label ||
    getPlatformBadge(resource.platformType)?.label ||
    resource.platformType ||
    ''
  );
}

export function dedupeResourceBadges(
  badges: Array<ResourceBadge | null | undefined>,
): ResourceBadge[] {
  const seen = new Set<string>();
  return badges.filter((badge): badge is ResourceBadge => {
    if (!badge) return false;
    const normalizedLabel = badge.label.trim().toLowerCase();
    if (!normalizedLabel || seen.has(normalizedLabel)) return false;
    seen.add(normalizedLabel);
    return true;
  });
}

export function getContainerRuntimeBadge(
  platformType?: PlatformType,
  platformData?: Record<string, unknown> | null,
): ResourceBadge | null {
  if (platformType !== 'docker' || !platformData) return null;

  const docker = (platformData as { docker?: { runtime?: string } } | undefined)?.docker;
  const raw = (docker?.runtime || '').trim();
  if (!raw) return null;

  const normalized = raw.toLowerCase();
  const label = normalized === 'podman' ? 'Podman' : normalized === 'docker' ? 'Docker' : raw;

  return {
    label,
    classes: `${baseBadge} ${typeClasses}`,
    title: `Runtime: ${label}`,
  };
}
