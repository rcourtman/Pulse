import type { ResourceType as HistoryResourceType } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import type { Node } from '@/types/api';
import { getCpuTemperature } from '@/utils/temperature';

import type { GuestDrawerHistoryGroupConfig, GuestDrawerHistoryTarget } from './guestDrawerModel';

export interface NodeDrawerHistoryTarget extends GuestDrawerHistoryTarget {
  resourceType: Extract<HistoryResourceType, 'agent'>;
}

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

export const NODE_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] =
  HOST_METRICS_HISTORY_GROUPS;

export const getNodeDrawerHistoryTarget = (node: Node): NodeDrawerHistoryTarget | null => {
  const resourceId = stripAgentPrefix((node.linkedAgentId || node.id || node.name || '').trim());
  if (!resourceId) return null;
  return { resourceType: 'agent', resourceId };
};

export const getNodeDrawerCurrentMetrics = (node: Node): Record<string, number | undefined> => ({
  temperature: getCpuTemperature(node.temperature) ?? undefined,
});
