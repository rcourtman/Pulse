import type { ResourceType as HistoryResourceType } from '@/api/charts';
import type { Node } from '@/types/api';
import { getCpuTemperature } from '@/utils/temperature';

import type { GuestDrawerHistoryGroupConfig, GuestDrawerHistoryTarget } from './guestDrawerModel';

export interface NodeDrawerHistoryTarget extends GuestDrawerHistoryTarget {
  resourceType: Extract<HistoryResourceType, 'agent'>;
}

const stripAgentPrefix = (value: string): string =>
  value.startsWith('agent:') ? value.slice('agent:'.length) : value;

export const NODE_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
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

export const getNodeDrawerHistoryTarget = (node: Node): NodeDrawerHistoryTarget | null => {
  const resourceId = stripAgentPrefix((node.linkedAgentId || node.id || node.name || '').trim());
  if (!resourceId) return null;
  return { resourceType: 'agent', resourceId };
};

export const getNodeDrawerHistoryFallbackMetrics = (
  node: Node,
): Record<string, number | undefined> => ({
  temperature: getCpuTemperature(node.temperature) ?? undefined,
});
