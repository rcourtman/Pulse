import type { Resource } from '@/types/resource';
import type { KubernetesMetricCapabilities, PlatformData } from './resourceDetailMappers';
import { buildServiceDetailLinks, type ServiceDetailLink } from './serviceDetailLinks';

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
  'inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900/25 dark:text-cyan-400';
const UNSUPPORTED_BADGE_CLASS =
  'inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted';

const K8S_CAPABILITY_SPECS: ReadonlyArray<
  readonly [keyof KubernetesMetricCapabilities, boolean, string, string]
> = [
  ['nodeCpuMemory', true, 'K8s Node CPU/Memory', 'Node CPU and memory metrics are available.'],
  [
    'nodeTelemetry',
    true,
    'Node Telemetry (Agent)',
    'Linked Pulse agent provides node uptime, temperature, disk, network, and disk I/O.',
  ],
  ['podCpuMemory', true, 'Pod CPU/Memory', 'Pod CPU and memory metrics are available.'],
  ['podNetwork', true, 'Pod Network', 'Pod network throughput is available.'],
  ['podEphemeralDisk', true, 'Pod Ephemeral Disk', 'Pod ephemeral storage usage is available.'],
  [
    'podDiskIo',
    false,
    'Pod Disk I/O Unsupported',
    'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
  ],
];

export const buildKubernetesCapabilityBadges = (
  capabilities?: KubernetesMetricCapabilities,
): ResourceDetailDrawerOperationalBadge[] =>
  capabilities
    ? K8S_CAPABILITY_SPECS.filter(
        ([key, expectTrue]) => Boolean(capabilities[key]) === expectTrue,
      ).map(([, expectTrue, label, title]) => ({
        label,
        classes: expectTrue ? SUPPORTED_BADGE_CLASS : UNSUPPORTED_BADGE_CLASS,
        title,
      }))
    : [];

export const buildSourceHealthSummary = (
  sourceStatus: SourceStatusMap,
): ResourceDetailDrawerSummary | null => {
  const entries = Object.entries(sourceStatus);
  if (!entries.length) return null;
  let warning = 0;
  let unhealthy = 0;
  const parts = entries.map(([source, status]) => {
    const normalized = (status?.status || '').trim().toLowerCase();
    if (/^(degraded|warning|stale)$/.test(normalized)) warning += 1;
    else if (!/^(online|running|healthy|connected|ok)$/.test(normalized)) unhealthy += 1;
    return `${source}:${normalized || 'unknown'}`;
  });
  if (unhealthy > 0) {
    return {
      label: `${unhealthy}/${entries.length} unhealthy`,
      className: 'text-red-600 dark:text-red-400',
      title: parts.join(' • '),
    };
  }
  if (warning > 0) {
    return {
      label: `${warning}/${entries.length} degraded`,
      className: 'text-amber-600 dark:text-amber-400',
      title: parts.join(' • '),
    };
  }
  return null;
};

export const buildSourceSummary = (
  _mergedSources: string[],
  sourceStatus: SourceStatusMap,
): ResourceDetailDrawerSummary | null => buildSourceHealthSummary(sourceStatus);

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
  }

  if (options.networkInterfaceCount > 0) cards.push('network');

  if (options.hasAgentDetails) {
    if (options.diskCount > 0) cards.push('disks');
    if (options.raidCount > 0) cards.push('raid');
    if (options.temperatureRowCount > 0) cards.push('temperatures');
  }

  return cards;
};

export const buildHostDetailSummary = (hostDetailCards: string[]): string | null => {
  const labels = Array.from(new Set(hostDetailCards), (label) =>
    label === 'raid' ? 'RAID' : label.charAt(0).toUpperCase() + label.slice(1),
  );
  if (labels.length === 0) return null;
  return labels.length <= 2
    ? labels.join(' and ')
    : `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;
};

export const buildAccessSummary = (options: {
  hasWebInterface: boolean;
  links: ResourceDetailDrawerOperationalLink[];
}): string | null => {
  const parts: string[] = [];

  if (options.hasWebInterface) {
    parts.push('Web interface');
  }

  if (options.links.length > 0) {
    parts.push(`${options.links.length} link${options.links.length === 1 ? '' : 's'}`);
  }

  return parts.join(' · ') || null;
};

export const buildRelatedLinks = (
  resource: Resource,
  _displayName: string,
): ResourceDetailDrawerOperationalLink[] => {
  const links: ResourceDetailDrawerOperationalLink[] = [];
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
): boolean => badges.length > 0;
