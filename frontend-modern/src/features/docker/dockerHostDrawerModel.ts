import type { ResourceType as HistoryResourceType } from '@/api/charts';
import type { Resource } from '@/types/resource';

import type {
  GuestDrawerHistoryGroupConfig,
  GuestDrawerHistoryTarget,
} from '@/components/Workloads/guestDrawerModel';

export interface DockerHostDrawerHistoryTarget extends GuestDrawerHistoryTarget {
  resourceType: Extract<HistoryResourceType, 'agent'>;
}

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

// Docker hosts report the same agent telemetry shape as PVE nodes, so the
// drawer history backend is the same and the chart groups mirror
// NODE_DRAWER_HISTORY_GROUPS.
export const DOCKER_HOST_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'utilization',
    label: 'Utilization',
    unit: '%',
    series: [
      { metric: 'cpu', label: 'CPU', unit: '%', color: '#8b5cf6' },
      { metric: 'memory', label: 'Memory', unit: '%', color: '#f59e0b' },
      { metric: 'disk', label: 'Disk', unit: '%', color: '#10b981' },
    ],
  },
  {
    id: 'network',
    label: 'Network I/O',
    unit: 'B/s',
    series: [
      { metric: 'netin', label: 'In', unit: 'B/s', color: '#10b981' },
      { metric: 'netout', label: 'Out', unit: 'B/s', color: '#fb923c' },
    ],
  },
  {
    id: 'disk-io',
    label: 'Disk I/O',
    unit: 'B/s',
    series: [
      { metric: 'diskread', label: 'Read', unit: 'B/s', color: '#3b82f6' },
      { metric: 'diskwrite', label: 'Write', unit: 'B/s', color: '#f59e0b' },
    ],
  },
  {
    id: 'thermals',
    label: 'Thermals',
    unit: 'C',
    series: [{ metric: 'temperature', label: 'CPU', unit: 'C', color: '#ef4444' }],
  },
];

export const getDockerHostDrawerHistoryTarget = (
  host: Resource,
): DockerHostDrawerHistoryTarget | null => {
  const candidate = host.agent?.agentId || host.id || host.name || '';
  const resourceId = stripAgentPrefix(candidate.trim());
  if (!resourceId) return null;
  return { resourceType: 'agent', resourceId };
};

export const getDockerHostDrawerHistoryFallbackMetrics = (
  host: Resource,
): Record<string, number | undefined> => {
  const finite = (value: number | undefined): number | undefined =>
    typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  const temperature = finite(host.temperature) ?? finite(host.docker?.temperature);
  return { temperature };
};
