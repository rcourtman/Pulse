import type { Node } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import type { WorkloadIOEmphasis } from './GuestRow';
import { computeIOScale } from '@/components/Infrastructure/infrastructureSelectors';
import { parseFilterStack, evaluateFilterStack } from '@/utils/searchQuery';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES } from '@/utils/status';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';

export interface FilterWorkloadsParams {
  guests: WorkloadGuest[];
  viewMode: ViewMode;
  statusMode: string;
  searchTerm: string;
  selectedNode: string | null;
  selectedHostHint: string | null;
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
  docker: number;
  k8s: number;
}

type SortDirection = 'asc' | 'desc';

type SortValue = string | number | boolean | null | undefined;

export const workloadNodeScopeId = (guest: WorkloadGuest): string =>
  `${(guest.instance || '').trim()}-${(guest.node || '').trim()}`;

export const getKubernetesContextKey = (guest: WorkloadGuest): string => {
  const candidates = [guest.contextLabel, guest.instance, guest.node];
  for (const value of candidates) {
    const trimmed = (value || '').trim();
    if (trimmed.length > 0) {
      return trimmed;
    }
  }
  return '';
};

export const filterWorkloads = ({
  guests: allGuests,
  viewMode,
  statusMode,
  searchTerm,
  selectedNode,
  selectedHostHint,
  selectedKubernetesContext,
  selectedKubernetesNamespace,
  containerRuntime,
}: FilterWorkloadsParams): WorkloadGuest[] => {
  let guests = allGuests;

  const nodeScope = selectedNode;
  if (nodeScope && viewMode !== 'k8s') {
    guests = guests.filter((g) => workloadNodeScopeId(g) === nodeScope);
  }

  const hostHint = (selectedHostHint || '').trim().toLowerCase();
  if (!nodeScope && hostHint && viewMode !== 'k8s') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) === 'k8s') return false;
      const candidates = [g.node, g.instance, g.contextLabel];
      return candidates.some((candidate) => (candidate || '').toLowerCase().includes(hostHint));
    });
  }

  const k8sContext = selectedKubernetesContext;
  if (k8sContext && viewMode === 'k8s') {
    guests = guests.filter(
      (g) => resolveWorkloadType(g) === 'k8s' && getKubernetesContextKey(g) === k8sContext,
    );
  }

  const k8sNamespace = (selectedKubernetesNamespace || '').trim();
  if (k8sNamespace && viewMode === 'k8s') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) !== 'k8s') return false;
      return (g.namespace || '').trim() === k8sNamespace;
    });
  }

  if (viewMode !== 'all') {
    guests = guests.filter((g) => resolveWorkloadType(g) === viewMode);
  }

  const normalizedRuntime = (containerRuntime || '').trim().toLowerCase();
  if (normalizedRuntime && viewMode === 'docker') {
    guests = guests.filter((g) => {
      if (resolveWorkloadType(g) !== 'docker') return false;
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
  if (type === 'vm' || type === 'lxc') {
    return `${guest.instance}-${guest.node}`;
  }
  const context = guest.contextLabel || guest.node || guest.instance || guest.namespace || guest.id;
  return `${type}:${context}`;
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
  const containers = guests.filter((g) => resolveWorkloadType(g) === 'lxc').length;
  const docker = guests.filter((g) => resolveWorkloadType(g) === 'docker').length;
  const k8s = guests.filter((g) => resolveWorkloadType(g) === 'k8s').length;

  return {
    total: guests.length,
    running,
    degraded,
    stopped,
    vms,
    containers,
    docker,
    k8s,
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

export const buildNodeByInstance = (nodes: Node[]): Record<string, Node> => {
  const map: Record<string, Node> = {};
  nodes.forEach((node) => {
    map[node.id] = node;
    const legacyKey = `${node.instance}-${node.name}`;
    if (!map[legacyKey]) {
      map[legacyKey] = node;
    }
  });
  return map;
};

export const buildGuestParentNodeMap = (
  guests: WorkloadGuest[],
  nodeMap: Record<string, Node>,
): Record<string, Node | undefined> => {
  const mapping: Record<string, Node | undefined> = {};

  guests.forEach((guest) => {
    const canonicalGuestId = getCanonicalWorkloadId(guest);

    if (guest.id) {
      const lastDash = guest.id.lastIndexOf('-');
      if (lastDash > 0) {
        const nodeId = guest.id.slice(0, lastDash);
        if (nodeMap[nodeId]) {
          mapping[canonicalGuestId] = nodeMap[nodeId];
          return;
        }
      }
    }

    const compositeKey = `${guest.instance}-${guest.node}`;
    if (nodeMap[compositeKey]) {
      mapping[canonicalGuestId] = nodeMap[compositeKey];
    }
  });

  return mapping;
};
