import type { Node } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import type { WorkloadIOEmphasis } from './guestRowModel';
import { computeIOScale } from '@/components/Infrastructure/infrastructureSelectors';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES } from '@/utils/status';
import { getNodeDisplayName } from '@/utils/nodes';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';
import { buildNodeByInstance, getKubernetesContextKey, workloadNodeScopeId } from './workloadTopology';

export interface FilterWorkloadsParams {
  guests: WorkloadGuest[];
  viewMode: ViewMode;
  statusMode: string;
  searchTerm: string;
  selectedNode: string | null;
  selectedHostHint: string | null;
  selectedPlatform?: string | null;
  selectedKubernetesContext: string | null;
  selectedKubernetesNamespace?: string | null;
  containerRuntime?: string | null;
}

export interface WorkloadStats {
  total: number;
  running: number;
  degraded: number;
  stopped: number;
  vms: number;
  containers: number;
  appContainers: number;
  pods: number;
}

type SortDirection = 'asc' | 'desc';

type SortValue = string | number | boolean | null | undefined;

export const filterWorkloads = ({
  guests: allGuests,
  viewMode,
  statusMode,
  searchTerm,
  selectedNode,
  selectedHostHint,
  selectedPlatform,
  selectedKubernetesContext,
  selectedKubernetesNamespace,
  containerRuntime,
}: FilterWorkloadsParams): WorkloadGuest[] => {
  let guests = allGuests;

  const nodeScope = selectedNode;
  if (nodeScope && viewMode !== 'pod') {
    guests = guests.filter((g) => workloadNodeScopeId(g) === nodeScope);
  }

  const hostHint = (selectedHostHint || '').trim().toLowerCase();
  if (!nodeScope && hostHint && viewMode !== 'pod') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) === 'pod') return false;
      const candidates = [g.node, g.instance, g.contextLabel];
      return candidates.some((candidate) => (candidate || '').toLowerCase().includes(hostHint));
    });
  }

  const k8sContext = selectedKubernetesContext;
  if (k8sContext && viewMode === 'pod') {
    guests = guests.filter(
      (g) => resolveWorkloadType(g) === 'pod' && getKubernetesContextKey(g) === k8sContext,
    );
  }

  const k8sNamespace = (selectedKubernetesNamespace || '').trim();
  if (k8sNamespace && viewMode === 'pod') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) !== 'pod') return false;
      return (g.namespace || '').trim() === k8sNamespace;
    });
  }

  if (viewMode !== 'all') {
    guests = guests.filter((g) => resolveWorkloadType(g) === viewMode);
  }

  const normalizedPlatform = normalizeSourcePlatformQueryValue(selectedPlatform);
  if (normalizedPlatform) {
    guests = guests.filter(
      (g) => normalizeSourcePlatformQueryValue(g.platformType || '') === normalizedPlatform,
    );
  }

  const normalizedRuntime = (containerRuntime || '').trim().toLowerCase();
  if (normalizedRuntime && viewMode === 'app-container') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) !== 'app-container') return false;
      const runtime = (g.containerRuntime || '').trim().toLowerCase();
      return runtime === normalizedRuntime;
    });
  }

  if (statusMode === 'running') {
    guests = guests.filter((g) => g.status === 'running');
  } else if (statusMode === 'degraded') {
    guests = guests.filter((g) => {
      const status = (g.status || '').toLowerCase();
      return (
        DEGRADED_HEALTH_STATUSES.has(status) ||
        (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status))
      );
    });
  } else if (statusMode === 'stopped') {
    guests = guests.filter((g) => g.status !== 'running');
  }

  const trimmedSearch = searchTerm.trim();
  if (trimmedSearch) {
    const searchParts = trimmedSearch
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t);

    const filters: string[] = [];
    const textSearches: string[] = [];

    searchParts.forEach((part) => {
      if (part.includes('>') || part.includes('<') || part.includes(':')) {
        filters.push(part);
      } else {
        textSearches.push(part.toLowerCase());
      }
    });

    if (filters.length > 0) {
      const filterString = filters.join(' AND ');
      const stack = parseFilterStack(filterString);
      if (stack.filters.length > 0) {
        guests = guests.filter((g) => evaluateFilterStack(g, stack));
      }
    }

    if (textSearches.length > 0) {
      guests = guests.filter((g) =>
        textSearches.some(
          (term) =>
            g.name.toLowerCase().includes(term) ||
            g.vmid.toString().includes(term) ||
            g.node.toLowerCase().includes(term) ||
            g.status.toLowerCase().includes(term),
        ),
      );
    }
  }

  return guests;
};

export const getDiskUsagePercent = (guest: WorkloadGuest): number | null => {
  const disk = guest?.disk;
  if (!disk) return null;

  const clamp = (value: number) => Math.min(100, Math.max(0, value));

  if (typeof disk.usage === 'number' && Number.isFinite(disk.usage)) {
    const usageValue = disk.usage > 1 ? disk.usage : disk.usage * 100;
    return clamp(usageValue);
  }

  if (
    typeof disk.used === 'number' &&
    Number.isFinite(disk.used) &&
    typeof disk.total === 'number' &&
    Number.isFinite(disk.total) &&
    disk.total > 0
  ) {
    return clamp((disk.used / disk.total) * 100);
  }

  return null;
};

export const createWorkloadSortComparator = (
  sortKey: string,
  sortDirection: SortDirection,
): ((a: WorkloadGuest, b: WorkloadGuest) => number) | null => {
  if (!sortKey) {
    return null;
  }

  const tiebreak = (a: WorkloadGuest, b: WorkloadGuest): number => {
    const nameA = (a.name || '').toLowerCase();
    const nameB = (b.name || '').toLowerCase();
    if (nameA !== nameB) return nameA < nameB ? -1 : 1;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  };

  return (a: WorkloadGuest, b: WorkloadGuest): number => {
    let aVal: SortValue = null;
    let bVal: SortValue = null;

    if (sortKey === 'cpu') {
      aVal = a.cpu * 100;
      bVal = b.cpu * 100;
    } else if (sortKey === 'memory') {
      aVal = a.memory ? a.memory.usage || 0 : 0;
      bVal = b.memory ? b.memory.usage || 0 : 0;
    } else if (sortKey === 'disk') {
      aVal = getDiskUsagePercent(a);
      bVal = getDiskUsagePercent(b);
    } else if (sortKey === 'diskIo') {
      aVal = Math.max(0, a.diskRead || 0) + Math.max(0, a.diskWrite || 0);
      bVal = Math.max(0, b.diskRead || 0) + Math.max(0, b.diskWrite || 0);
    } else if (sortKey === 'netIo') {
      aVal = Math.max(0, a.networkIn || 0) + Math.max(0, a.networkOut || 0);
      bVal = Math.max(0, b.networkIn || 0) + Math.max(0, b.networkOut || 0);
    } else {
      aVal = (a as unknown as Record<string, SortValue>)[sortKey];
      bVal = (b as unknown as Record<string, SortValue>)[sortKey];
    }

    const aIsEmpty = aVal === null || aVal === undefined || aVal === '';
    const bIsEmpty = bVal === null || bVal === undefined || bVal === '';

    if (aIsEmpty && bIsEmpty) return tiebreak(a, b);
    if (aIsEmpty) return 1;
    if (bIsEmpty) return -1;

    if (typeof aVal === 'number' && typeof bVal === 'number') {
      if (aVal === bVal) return tiebreak(a, b);
      const comparison = aVal < bVal ? -1 : 1;
      return sortDirection === 'asc' ? comparison : -comparison;
    }

    const aStr = String(aVal).toLowerCase();
    const bStr = String(bVal).toLowerCase();

    if (aStr === bStr) return tiebreak(a, b);
    const comparison = aStr < bStr ? -1 : 1;
    return sortDirection === 'asc' ? comparison : -comparison;
  };
};

export const getWorkloadGroupKey = (guest: WorkloadGuest): string => {
  const type = resolveWorkloadType(guest);
  if (type === 'vm' || type === 'system-container') {
    return `${guest.instance}-${guest.node}`;
  }
  const context = guest.contextLabel || guest.node || guest.instance || guest.namespace || guest.id;
  return `${type}:${context}`;
};

export const getWorkloadGroupLabel = (
  groupKey: string,
  guests: WorkloadGuest[],
  groupNodeName?: string | null,
): { type: string; name: string } => {
  const normalizedGroupKey = guests.length > 0 ? getWorkloadGroupKey(guests[0]) : groupKey;
  const [prefix, ...rest] = normalizedGroupKey.split(':');
  const context = rest.length > 0 ? rest.join(':') : normalizedGroupKey;
  const normalizedNodeName = (groupNodeName || '').trim();
  if (normalizedNodeName) {
    return { type: '', name: normalizedNodeName };
  }
  if (prefix === 'app-container') return { type: 'App Containers', name: context };
  if (prefix === 'pod') return { type: 'Pods', name: context };
  if (prefix === 'vm') return { type: 'VM', name: context };
  if (prefix === 'system-container') return { type: 'Container', name: context };
  const first = guests[0];
  if (first) {
    const cluster = (first.clusterName || '').trim();
    const nodeName = (first.node || '').trim();
    if (nodeName && cluster) return { type: cluster, name: nodeName };
    if (nodeName) return { type: '', name: nodeName };
  }
  return { type: '', name: context };
};

export const buildWorkloadSummaryGroupScope = (
  groupId: string,
  guests: WorkloadGuest[],
  label: { type: string; name: string },
): SummarySeriesGroupScope | null => {
  const seriesIds = Array.from(
    new Set(guests.map((guest) => getCanonicalWorkloadId(guest)).filter(Boolean)),
  );
  if (seriesIds.length === 0) {
    return null;
  }

  const summaryLabelParts = [label.name.trim(), label.type.trim()].filter(Boolean);
  const scopeLabel = summaryLabelParts.join(' · ');
  const workloadCountLabel = `${guests.length} workload${guests.length === 1 ? '' : 's'}`;

  return {
    id: groupId.trim(),
    label: scopeLabel ? `${scopeLabel} (${workloadCountLabel})` : workloadCountLabel,
    seriesIds,
  };
};

export const groupWorkloads = (
  guests: WorkloadGuest[],
  mode: 'grouped' | 'flat',
  comparator: ((a: WorkloadGuest, b: WorkloadGuest) => number) | null,
): Record<string, WorkloadGuest[]> => {
  if (mode === 'flat') {
    const groups: Record<string, WorkloadGuest[]> = { '': guests };
    if (comparator) {
      groups[''] = groups[''].sort(comparator);
    }
    return groups;
  }

  const groups: Record<string, WorkloadGuest[]> = {};
  guests.forEach((guest) => {
    const nodeId = getWorkloadGroupKey(guest);
    if (!groups[nodeId]) {
      groups[nodeId] = [];
    }
    groups[nodeId].push(guest);
  });

  if (comparator) {
    Object.keys(groups).forEach((nodeId) => {
      groups[nodeId] = groups[nodeId].sort(comparator);
    });
  }

  return groups;
};

export const buildWorkloadSummaryGroupScopeMap = ({
  guests,
  nodes,
  groupingMode,
  sortComparator,
}: {
  guests: WorkloadGuest[];
  nodes: Node[];
  groupingMode: 'grouped' | 'flat';
  sortComparator: ((a: WorkloadGuest, b: WorkloadGuest) => number) | null;
}): Map<string, SummarySeriesGroupScope> => {
  if (groupingMode !== 'grouped') {
    return new Map<string, SummarySeriesGroupScope>();
  }

  const grouped = groupWorkloads(guests, groupingMode, sortComparator);
  const nodeByInstance = buildNodeByInstance(nodes);
  const scopes = new Map<string, SummarySeriesGroupScope>();
  for (const [groupKey, groupGuests] of Object.entries(grouped)) {
    const node = nodeByInstance[groupKey];
    const scope = buildWorkloadSummaryGroupScope(
      groupKey,
      groupGuests,
      getWorkloadGroupLabel(groupKey, groupGuests, node ? getNodeDisplayName(node) : null),
    );
    if (scope) {
      scopes.set(scope.id, scope);
    }
  }
  return scopes;
};

export const computeWorkloadStats = (guests: WorkloadGuest[]): WorkloadStats => {
  const running = guests.filter((g) => g.status === 'running').length;
  const degraded = guests.filter((g) => {
    const status = (g.status || '').toLowerCase();
    return (
      DEGRADED_HEALTH_STATUSES.has(status) ||
      (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status))
    );
  }).length;
  const stopped = guests.length - running - degraded;
  const vms = guests.filter((g) => resolveWorkloadType(g) === 'vm').length;
  const containers = guests.filter((g) => resolveWorkloadType(g) === 'system-container').length;
  const appContainers = guests.filter((g) => resolveWorkloadType(g) === 'app-container').length;
  const pods = guests.filter((g) => resolveWorkloadType(g) === 'pod').length;

  return {
    total: guests.length,
    running,
    degraded,
    stopped,
    vms,
    containers,
    appContainers,
    pods,
  };
};

export const computeWorkloadIOEmphasis = (guests: WorkloadGuest[]): WorkloadIOEmphasis => {
  const resources = guests.map((guest) => ({
    network: {
      rxBytes: Math.max(0, guest.networkIn ?? 0),
      txBytes: Math.max(0, guest.networkOut ?? 0),
    },
    diskIO: {
      readRate: Math.max(0, guest.diskRead ?? 0),
      writeRate: Math.max(0, guest.diskWrite ?? 0),
    },
  }));

  const { network, diskIO } = computeIOScale(resources as Parameters<typeof computeIOScale>[0]);
  return { network, diskIO };
};
