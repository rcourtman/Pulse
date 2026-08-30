import type { ResourceType as HistoryResourceType } from '@/api/charts';
import {
  GPU_METRICS_HISTORY_GROUPS,
  HOST_METRICS_HISTORY_GROUPS,
} from '@/components/shared/hostMetricsHistoryModel';
import type { HostGPUSensor, Node } from '@/types/api';
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

export const getNodeDrawerHistoryGroups = (node: Node): GuestDrawerHistoryGroupConfig[] => {
  const baseGroups = node.linkedAgentId?.trim()
    ? NODE_DRAWER_HISTORY_GROUPS
    : PVE_API_NODE_HISTORY_GROUPS;
  return (node.sensors?.gpu?.length ?? 0) > 0
    ? [...baseGroups, ...GPU_METRICS_HISTORY_GROUPS]
    : baseGroups;
};

const maxGPUValue = (
  node: Node,
  select: (gpu: HostGPUSensor) => number | undefined,
): number | undefined => {
  let maximum: number | undefined;
  for (const gpu of node.sensors?.gpu ?? []) {
    const value = select(gpu);
    if (typeof value !== 'number' || !Number.isFinite(value)) continue;
    maximum = maximum === undefined ? value : Math.max(maximum, value);
  }
  return maximum;
};

export const getNodeDrawerCurrentMetrics = (node: Node): Record<string, number | undefined> => {
  const metrics: Record<string, number | undefined> = {
    temperature: getCpuTemperature(node.temperature) ?? undefined,
  };
  if ((node.sensors?.gpu?.length ?? 0) === 0) return metrics;

  metrics.gpu = maxGPUValue(node, (gpu) => {
    const value = gpu.utilizationPercent;
    return typeof value === 'number' && value >= 0 && value <= 100 ? value : undefined;
  });
  metrics.gpu_memory = maxGPUValue(node, (gpu) => {
    const used = gpu.memoryUsedBytes;
    const total = gpu.memoryTotalBytes;
    if (typeof used !== 'number' || typeof total !== 'number' || used < 0 || total <= 0) {
      return undefined;
    }
    return Math.min(100, Math.max(0, (used / total) * 100));
  });
  metrics.gpu_temperature = maxGPUValue(node, (gpu) => {
    const value = gpu.temperatureCelsius;
    return typeof value === 'number' && value > 0 && value <= 150 ? value : undefined;
  });
  return metrics;
};
