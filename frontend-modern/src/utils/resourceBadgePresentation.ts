import type { PlatformType, SourceType, Resource, ResourceType } from '@/types/resource';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { getPlatformAgentRecord, getPlatformDataRecord } from '@/utils/agentResources';
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
  'unraid',
  'synology-dsm',
  'microsoft-hyperv',
  'kubernetes',
];

const knownHostIdentityPlatformPatterns: Array<{
  pattern: RegExp;
  source: KnownSourcePlatform;
}> = [
  { pattern: /\btrue\s*nas\b|\btruenas\b/i, source: 'truenas' },
  { pattern: /\bunraid\b/i, source: 'unraid' },
  { pattern: /\bsynology\b|\bdiskstation\b|\bdsm\b/i, source: 'synology-dsm' },
  { pattern: /\bhyper-?v\b/i, source: 'microsoft-hyperv' },
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

const getKnownHostIdentitySource = (...values: string[]): KnownSourcePlatform | null => {
  for (const value of values) {
    if (!value) continue;
    const normalized = normalizeSourcePlatformKey(value);
    if (
      normalized &&
      normalized !== 'agent' &&
      normalized !== 'docker' &&
      normalized !== 'generic'
    ) {
      return normalized;
    }
    const match = knownHostIdentityPlatformPatterns.find(({ pattern }) => pattern.test(value));
    if (match) return match.source;
  }
  return null;
};

const getHostOsLabel = (...values: string[]): string | null => {
  for (const value of values) {
    if (!value) continue;
    const match = hostOsLabelPatterns.find(({ pattern }) => pattern.test(value));
    if (match) return match.label;
  }
  return null;
};

const getAgentSystemIdentityBadge = (resource: Resource): ResourceBadge | null => {
  const agent = getPlatformAgentRecord(resource);
  if (!agent) return null;

  const platform = trimString(agent.platform);
  const osName = trimString(agent.osName);
  const osVersion = trimString(agent.osVersion);
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
): ResourceBadge | null => {
  const availability = platformData?.availability as
    | { address?: string; protocol?: string; port?: number }
    | undefined;
  const sources = (platformData?.sources as string[] | undefined) ?? [];
  const isAvailabilityEndpoint =
    resource.type === 'network-endpoint' ||
    Boolean(availability) ||
    sources.some((source) => source.trim().toLowerCase() === 'availability');

  if (!isAvailabilityEndpoint) return null;

  const protocol = trimString(availability?.protocol).toUpperCase();
  const address = trimString(availability?.address);
  const port = Number.isFinite(availability?.port) && availability?.port ? `:${availability.port}` : '';
  return {
    label: 'Availability',
    classes: `${baseBadge} ${availabilityBadgeClasses}`,
    title: titleFromParts(protocol || 'Availability', address ? `${address}${port}` : undefined),
  };
};

export function getInfrastructureSystemIdentityBadges(resource: Resource): ResourceBadge[] {
  const platformData = getPlatformDataRecord(resource) as
    | (Record<string, unknown> & { sources?: string[] })
    | undefined;
  const sources = normalizeUnifiedSourceKeys(platformData?.sources);
  const availabilityIdentityBadge = getAvailabilitySystemIdentityBadge(resource, platformData);
  if (availabilityIdentityBadge) {
    return [availabilityIdentityBadge];
  }

  const systemSource = firstSystemSource(sources, resource.platformType);
  if (systemSource) {
    return buildUnifiedSourceBadges([systemSource]);
  }

  const agentIdentityBadge = getAgentSystemIdentityBadge(resource);
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

  return getInfrastructurePlatformBadges(platformData?.sources);
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
