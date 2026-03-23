import type { Agent, Node, PBSInstance } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { buildMetricKey } from '@/utils/metricsKeys';
import { getNodeDisplayName } from '@/utils/nodes';
import { getCpuTemperature } from '@/utils/temperature';
import {
  getAgentLikeIdentityAliases,
  getNormalizedIdentityLookupVariants,
} from '@/utils/resourceIdentity';
import { asTrimmedString } from '@/utils/stringUtils';

export interface InfrastructureSummaryTableProps {
  nodes: Node[];
  pbsInstances?: PBSInstance[];
  vmCounts?: Record<string, number>;
  containerCounts?: Record<string, number>;
  storageCounts?: Record<string, number>;
  diskCounts?: Record<string, number>;
  agents?: Agent[];
  backupCounts?: Record<string, number>;
  currentTab: 'dashboard' | 'storage' | 'recovery';
  selectedNode: string | null;
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeClick: (nodeId: string, nodeType: 'pve' | 'pbs') => void;
}

export type TableItem = Node | PBSInstance;

export type CountSortKey =
  | 'vmCount'
  | 'containerCount'
  | 'storageCount'
  | 'diskCount'
  | 'backupCount';

export type InfrastructureSummarySortKey =
  | 'default'
  | 'name'
  | 'uptime'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'temperature'
  | CountSortKey;

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const getAgentLinkedNodeId = (agent: Agent): string | undefined => {
  const agentRecord = agent as unknown as Record<string, unknown>;
  const platformData = asRecord(agentRecord.platformData);
  const platformAgent = asRecord(platformData?.agent);

  return (
    asTrimmedString(agent.linkedNodeId) ||
    asTrimmedString(platformData?.linkedNodeId) ||
    asTrimmedString(platformAgent?.linkedNodeId)
  );
};

export const isPVE = (item: TableItem): item is Node =>
  (item as Node).pveVersion !== undefined || (item as Node & { type?: string }).type === 'agent';

export const isTemperatureMonitoringEnabled = (node: Node, globalEnabled = true): boolean => {
  if (
    node.temperatureMonitoringEnabled !== undefined &&
    node.temperatureMonitoringEnabled !== null
  ) {
    return node.temperatureMonitoringEnabled;
  }
  return globalEnabled;
};

export const isInfrastructureSummaryItemOnline = (item: TableItem) => {
  if (isPVE(item)) {
    return item.status === 'online' && (item.uptime || 0) > 0;
  }
  return item.status === 'healthy' || item.status === 'online';
};

export const getPbsTotals = (pbs: PBSInstance) =>
  (pbs.datastores ?? []).reduce(
    (acc, datastore) => {
      acc.used += datastore.used || 0;
      acc.total += datastore.total || 0;
      return acc;
    },
    { used: 0, total: 0 },
  );

export const getInfrastructureSummaryCpuPercent = (item: TableItem) => {
  if (isPVE(item)) {
    return Math.round((item.cpu || 0) * 100);
  }
  return Math.round(item.cpu || 0);
};

export const getInfrastructureSummaryMemoryPercent = (item: TableItem) => {
  if (isPVE(item)) {
    return Math.round(item.memory?.usage || 0);
  }
  if (!item.memoryTotal) return 0;
  return Math.round((item.memoryUsed / item.memoryTotal) * 100);
};

export const getInfrastructureSummaryDiskPercent = (item: TableItem) => {
  if (isPVE(item)) {
    if (!item.disk || item.disk.total === 0) return 0;
    return Math.round((item.disk.used / item.disk.total) * 100);
  }
  const totals = getPbsTotals(item);
  if (totals.total === 0) return 0;
  return Math.round((totals.used / totals.total) * 100);
};

export const getInfrastructureSummaryDiskSublabel = (item: TableItem) => {
  if (isPVE(item)) {
    if (!item.disk) return undefined;
    return `${formatBytes(item.disk.used)}/${formatBytes(item.disk.total)}`;
  }
  if (!item.datastores || item.datastores.length === 0) return undefined;
  const totals = getPbsTotals(item);
  return `${formatBytes(totals.used)}/${formatBytes(totals.total)}`;
};

export const getInfrastructureSummaryCpuTemperatureValue = (item: TableItem) => {
  if (!isPVE(item)) return null;
  const value = getCpuTemperature(item.temperature);
  return value !== null ? Math.round(value) : null;
};

export const getInfrastructureSummaryCountValue = (
  item: TableItem,
  key: CountSortKey,
  props: InfrastructureSummaryTableProps,
): number | null => {
  if (!isPVE(item)) {
    if (key === 'backupCount') {
      return props.backupCounts?.[item.name] ?? 0;
    }
    return null;
  }

  switch (key) {
    case 'vmCount':
      return props.vmCounts?.[item.id] ?? 0;
    case 'containerCount':
      return props.containerCounts?.[item.id] ?? 0;
    case 'storageCount':
      return props.storageCounts?.[item.id] ?? 0;
    case 'diskCount':
      return props.diskCounts?.[item.id] ?? 0;
    case 'backupCount':
      return props.backupCounts?.[item.id] ?? 0;
    default:
      return null;
  }
};

const getInfrastructureSummarySortValue = (
  item: TableItem,
  key: InfrastructureSummarySortKey,
  props: InfrastructureSummaryTableProps,
): number | string | null => {
  switch (key) {
    case 'name':
      return isPVE(item) ? getNodeDisplayName(item) : item.name;
    case 'uptime':
      return item.uptime ?? 0;
    case 'cpu':
      return getInfrastructureSummaryCpuPercent(item);
    case 'memory':
      return getInfrastructureSummaryMemoryPercent(item);
    case 'disk':
      return getInfrastructureSummaryDiskPercent(item);
    case 'temperature':
      return getInfrastructureSummaryCpuTemperatureValue(item);
    case 'vmCount':
    case 'containerCount':
    case 'storageCount':
    case 'diskCount':
    case 'backupCount':
      return getInfrastructureSummaryCountValue(item, key, props);
    default:
      return null;
  }
};

const defaultInfrastructureSummaryComparison = (left: TableItem, right: TableItem) => {
  const leftIsPVE = isPVE(left);
  const rightIsPVE = isPVE(right);
  if (leftIsPVE !== rightIsPVE) return leftIsPVE ? -1 : 1;

  const leftOnline = isInfrastructureSummaryItemOnline(left);
  const rightOnline = isInfrastructureSummaryItemOnline(right);
  if (leftOnline !== rightOnline) return leftOnline ? -1 : 1;

  const leftName = leftIsPVE ? getNodeDisplayName(left) : left.name;
  const rightName = rightIsPVE ? getNodeDisplayName(right) : right.name;

  return leftName.localeCompare(rightName);
};

const compareInfrastructureSummaryValues = (
  left: number | string | null,
  right: number | string | null,
) => {
  const leftEmpty =
    left === null || left === undefined || (typeof left === 'number' && Number.isNaN(left));
  const rightEmpty =
    right === null || right === undefined || (typeof right === 'number' && Number.isNaN(right));

  if (leftEmpty && rightEmpty) return 0;
  if (leftEmpty) return 1;
  if (rightEmpty) return -1;

  if (typeof left === 'number' && typeof right === 'number') {
    if (left === right) return 0;
    return left < right ? -1 : 1;
  }

  const leftString = String(left).toLowerCase();
  const rightString = String(right).toLowerCase();
  if (leftString === rightString) return 0;
  return leftString < rightString ? -1 : 1;
};

export const getInfrastructureSummaryDefaultSortDirection = (
  key: Exclude<InfrastructureSummarySortKey, 'default'>,
) => {
  switch (key) {
    case 'name':
      return 'asc';
    default:
      return 'desc';
  }
};

export const sortInfrastructureSummaryItems = (
  props: InfrastructureSummaryTableProps,
  sortKey: InfrastructureSummarySortKey,
  sortDirection: 'asc' | 'desc',
) => {
  const items: TableItem[] = [];

  if (props.nodes) items.push(...props.nodes);
  if (props.pbsInstances) items.push(...props.pbsInstances);

  return items.sort((left, right) => {
    if (sortKey === 'default') {
      return defaultInfrastructureSummaryComparison(left, right);
    }

    const valueLeft = getInfrastructureSummarySortValue(left, sortKey, props);
    const valueRight = getInfrastructureSummarySortValue(right, sortKey, props);
    const comparison = compareInfrastructureSummaryValues(valueLeft, valueRight);

    if (comparison !== 0) {
      return sortDirection === 'asc' ? comparison : -comparison;
    }

    return defaultInfrastructureSummaryComparison(left, right);
  });
};

export const getInfrastructureSummaryMetricsKey = (item: TableItem) => {
  if (isPVE(item) && item.linkedAgentId) {
    return buildMetricKey('agent', item.linkedAgentId);
  }

  const resourceId = isPVE(item) ? item.id || item.name : item.id || item.name;
  return buildMetricKey('node', resourceId);
};

export const resolveInfrastructureSummaryLinkedAgent = (
  node: Node,
  agents: Agent[] = [],
): Agent | undefined => {
  const linkedAgentId = node.linkedAgentId;
  if (linkedAgentId) {
    const byId = agents.find((agent) => getAgentLikeIdentityAliases(agent).includes(linkedAgentId));
    if (byId) return byId;
  }

  const nodeName = (node.name || '').trim();
  return agents.find((agent) => {
    const linkedNodeId = getAgentLinkedNodeId(agent);
    if (linkedNodeId === node.id) {
      return true;
    }

    if (!nodeName) {
      return false;
    }

    const aliases = getAgentLikeIdentityAliases(agent).flatMap((value) =>
      getNormalizedIdentityLookupVariants(value),
    );
    return aliases.includes(nodeName.toLowerCase());
  });
};
