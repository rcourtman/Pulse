import type { ResourceType as HistoryResourceType } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import type { Node } from '@/types/api';
import { getCpuTemperature } from '@/utils/temperature';

import type { GuestDrawerHistoryGroupConfig, GuestDrawerHistoryTarget } from './guestDrawerModel';

export interface NodeDrawerHistoryTarget extends GuestDrawerHistoryTarget {
  resourceType: Extract<HistoryResourceType, 'agent' | 'node'>;
}

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

export const NODE_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] =
  HOST_METRICS_HISTORY_GROUPS;

// The PVE node API exposes utilization and network history, while host disk
// throughput is available only from an attached Unified Agent. Keep thermals
// because Pulse can collect and persist them through the node temperature path.
export const PVE_API_NODE_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] =
  HOST_METRICS_HISTORY_GROUPS.filter((group) => group.id !== 'disk-io');

export const getNodeDrawerHistoryTarget = (node: Node): NodeDrawerHistoryTarget | null => {
  const linkedAgentId = (node.linkedAgentId || '').trim();
  if (linkedAgentId) {
    const resourceId = stripAgentPrefix(linkedAgentId);
    return resourceId ? { resourceType: 'agent', resourceId } : null;
  }

  const resourceId = (node.id || node.name || '').trim();
  if (!resourceId) return null;
  return { resourceType: 'node', resourceId };
};

export const getNodeDrawerHistoryGroups = (node: Node): GuestDrawerHistoryGroupConfig[] =>
  node.linkedAgentId?.trim() ? NODE_DRAWER_HISTORY_GROUPS : PVE_API_NODE_HISTORY_GROUPS;

export const getNodeDrawerCurrentMetrics = (node: Node): Record<string, number | undefined> => ({
  temperature: getCpuTemperature(node.temperature) ?? undefined,
});
