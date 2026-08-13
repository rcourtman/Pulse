import type { ResourceType as HistoryResourceType } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
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

// Docker hosts report the same agent telemetry shape as PVE nodes and
// standalone machines, so all host drawers consume one shared group catalog.
export const DOCKER_HOST_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] =
  HOST_METRICS_HISTORY_GROUPS;

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
