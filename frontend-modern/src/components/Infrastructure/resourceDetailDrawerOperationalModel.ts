import type { Resource } from '@/types/resource';
import type { KubernetesMetricCapabilities, PlatformData } from './resourceDetailMappers';
import { buildServiceDetailLinks, type ServiceDetailLink } from './serviceDetailLinks';
import { buildWorkloadsHref } from './workloadsLink';

export type ResourceDetailDrawerOperationalBadge = {
  label: string;
  classes: string;
  title: string;
};

export type ResourceDetailDrawerSummary = {
  label: string;
  className: string;
  title: string;
};

export type ResourceDetailDrawerOperationalLink = ServiceDetailLink;

type SourceStatusMap = NonNullable<PlatformData['sourceStatus']>;

const SUPPORTED_BADGE_CLASS =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400';
const UNSUPPORTED_BADGE_CLASS =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted';

export const buildKubernetesCapabilityBadges = (
  capabilities?: KubernetesMetricCapabilities,
): ResourceDetailDrawerOperationalBadge[] => {
  if (!capabilities) return [];

  const badges: ResourceDetailDrawerOperationalBadge[] = [];

  if (capabilities.nodeCpuMemory) {
    badges.push({
      label: 'K8s Node CPU/Memory',
      classes: SUPPORTED_BADGE_CLASS,
      title: 'Node CPU and memory metrics are available.',
    });
  }
  if (capabilities.nodeTelemetry) {
    badges.push({
      label: 'Node Telemetry (Agent)',
      classes: SUPPORTED_BADGE_CLASS,
      title: 'Linked Pulse agent provides node uptime, temperature, disk, network, and disk I/O.',
    });
  }
  if (capabilities.podCpuMemory) {
    badges.push({
      label: 'Pod CPU/Memory',
      classes: SUPPORTED_BADGE_CLASS,
      title: 'Pod CPU and memory metrics are available.',
    });
  }
  if (capabilities.podNetwork) {
    badges.push({
      label: 'Pod Network',
      classes: SUPPORTED_BADGE_CLASS,
      title: 'Pod network throughput is available.',
    });
  }
  if (capabilities.podEphemeralDisk) {
    badges.push({
      label: 'Pod Ephemeral Disk',
      classes: SUPPORTED_BADGE_CLASS,
      title: 'Pod ephemeral storage usage is available.',
    });
  }
  if (!capabilities.podDiskIo) {
    badges.push({
      label: 'Pod Disk I/O Unsupported',
      classes: UNSUPPORTED_BADGE_CLASS,
      title:
        'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
    });
  }

  return badges;
};

export const buildSourceHealthSummary = (
  sourceStatus: SourceStatusMap,
): ResourceDetailDrawerSummary | null => {
  const entries = Object.entries(sourceStatus);
  if (entries.length === 0) return null;

  let warning = 0;
  let unhealthy = 0;
  const parts: string[] = [];

  for (const [source, status] of entries) {
    const normalized = (status?.status || '').trim().toLowerCase();
    parts.push(`${source}:${normalized || 'unknown'}`);
    if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) {
      continue;
    } else if (['degraded', 'warning', 'stale'].includes(normalized)) {
      warning += 1;
    } else {
      unhealthy += 1;
    }
  }

  const total = entries.length;
  if (unhealthy > 0) {
    return {
      label: `${unhealthy}/${total} unhealthy`,
      className: 'text-red-600 dark:text-red-400',
      title: parts.join(' • '),
    };
  }
  if (warning > 0) {
    return {
      label: `${warning}/${total} degraded`,
      className: 'text-amber-600 dark:text-amber-400',
      title: parts.join(' • '),
    };
  }
  return null;
};

export const buildSourceSummary = (
  mergedSources: string[],
  sourceStatus: SourceStatusMap,
): ResourceDetailDrawerSummary | null => {
  const health = buildSourceHealthSummary(sourceStatus);
  if (health) return health;
  return null;
};

export const buildHostDetailCards = (options: {
  hasProxmoxNode: boolean;
  hasAgentDetails: boolean;
  networkInterfaceCount: number;
  diskCount: number;
  raidCount: number;
  temperatureRowCount: number;
}): string[] => {
  const cards: string[] = [];

  if (options.hasProxmoxNode) {
    cards.push('system', 'hardware', 'storage');
  }

  if (options.hasAgentDetails) {
    cards.push('system', 'hardware');
    if (options.networkInterfaceCount > 0) cards.push('network');
    if (options.diskCount > 0) cards.push('disks');
    if (options.raidCount > 0) cards.push('raid');
    if (options.temperatureRowCount > 0) cards.push('temperatures');
  }

  return cards;
};

export const buildHostDetailSummary = (hostDetailCards: string[]): string | null => {
  const labels = Array.from(new Set(hostDetailCards));
  if (labels.length === 0) return null;
  const presentation: Record<string, string> = {
    system: 'System',
    hardware: 'Hardware',
    storage: 'Storage',
    network: 'Network',
    disks: 'Disks',
    raid: 'RAID',
    temperatures: 'Temperatures',
  };
  const displayLabels = labels.map((label) => presentation[label] ?? label);

  const categories =
    displayLabels.length === 1
      ? displayLabels[0]
      : displayLabels.length === 2
        ? `${displayLabels[0]} and ${displayLabels[1]}`
        : `${displayLabels.slice(0, -1).join(', ')}, and ${displayLabels[displayLabels.length - 1]}`;

  return categories;
};

export const buildRelatedLinks = (
  resource: Resource,
  displayName: string,
): ResourceDetailDrawerOperationalLink[] => {
  const links: ResourceDetailDrawerOperationalLink[] = [];
  const workloads = buildWorkloadsHref(resource);
  const workloadSearch = workloads ? new URLSearchParams(workloads.split('?')[1] ?? '') : null;
  const scopedWorkloadType = workloadSearch?.get('type')?.trim() ?? '';
  if (workloads && scopedWorkloadType) {
    links.push({
      href: workloads,
      label: 'Open in Workloads',
      compactLabel: 'Workloads',
      ariaLabel: `Open related workloads for ${displayName}`,
    });
  }
  links.push(...buildServiceDetailLinks(resource));

  const seen = new Set<string>();
  return links.filter((link) => {
    if (seen.has(link.href)) return false;
    seen.add(link.href);
    return true;
  });
};

export const hasRuntimeOperationalContext = (
  badges: ResourceDetailDrawerOperationalBadge[],
  links: ResourceDetailDrawerOperationalLink[],
): boolean => badges.length > 0 || links.length > 0;
