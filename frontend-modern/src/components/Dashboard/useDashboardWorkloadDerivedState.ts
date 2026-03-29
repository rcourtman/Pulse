import { createMemo, type Accessor } from 'solid-js';

import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import type { WorkloadSummarySnapshot } from '@/components/Workloads/WorkloadsSummary';
import { getNodeDisplayName } from '@/utils/nodes';
import { getCanonicalWorkloadId } from '@/utils/workloads';

import {
  getDiskUsagePercent,
  getWorkloadGroupKey,
  groupWorkloads,
  computeWorkloadStats,
  computeWorkloadIOEmphasis,
} from './workloadSelectors';
import {
  buildNodeByInstance,
  buildGuestParentNodeMap,
} from './workloadTopology';
import { useDashboardWorkloadViewportSync } from './useDashboardWorkloadViewportSync';
import { useGroupedTableWindowing } from './useGroupedTableWindowing';

type GroupingMode = 'grouped' | 'flat';

const DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT = 32;

const workloadMetricPercent = (value: number | null | undefined): number => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
  if (value <= 1) return Math.max(0, value * 100);
  return Math.max(0, value);
};

interface DashboardWorkloadDerivedStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  filteredGuests: Accessor<WorkloadGuest[]>;
  groupingMode: Accessor<GroupingMode>;
  guestSortComparator: Accessor<((a: WorkloadGuest, b: WorkloadGuest) => number) | null>;
  nodes: Accessor<Node[]>;
  selectedGuestId: Accessor<string | null>;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
}

export function useDashboardWorkloadDerivedState(
  options: DashboardWorkloadDerivedStateOptions,
) {
  const filteredGuests = createMemo<WorkloadGuest[]>(() => options.filteredGuests() ?? []);

  const workloadsSummaryVisibleIds = createMemo<string[]>(() =>
    filteredGuests().map((guest) => getCanonicalWorkloadId(guest)),
  );

  const workloadsSummaryFallbackCounts = createMemo(() => {
    const guests = filteredGuests();
    const running = guests.filter(
      (guest) => guest.status === 'running' || guest.status === 'online',
    ).length;
    return {
      total: guests.length,
      running,
      stopped: Math.max(0, guests.length - running),
    };
  });

  const workloadsSummaryFallbackSnapshots = createMemo<WorkloadSummarySnapshot[]>(() =>
    filteredGuests().map((guest) => {
      const guestId = getCanonicalWorkloadId(guest);
      const memoryUsage = workloadMetricPercent(guest.memory?.usage);
      let diskUsage = workloadMetricPercent(guest.disk?.usage);
      if (
        (!diskUsage || diskUsage <= 0) &&
        typeof guest.disk?.used === 'number' &&
        typeof guest.disk?.total === 'number' &&
        Number.isFinite(guest.disk.used) &&
        Number.isFinite(guest.disk.total) &&
        guest.disk.total > 0
      ) {
        const selectorDiskUsage = getDiskUsagePercent(guest);
        const rawDiskUsage = (guest.disk.used / guest.disk.total) * 100;
        diskUsage = rawDiskUsage > 100 ? rawDiskUsage : (selectorDiskUsage ?? rawDiskUsage);
      }

      return {
        id: guestId,
        name: guest.name || guestId,
        cpu: workloadMetricPercent(guest.cpu),
        memory: memoryUsage,
        disk: Math.max(0, diskUsage),
        network: Math.max(0, guest.networkIn || 0) + Math.max(0, guest.networkOut || 0),
      };
    }),
  );

  const nodeByInstance = createMemo(() => buildNodeByInstance(options.nodes()));
  const guestParentNodeMap = createMemo(() =>
    buildGuestParentNodeMap(options.allGuests(), nodeByInstance()),
  );

  const getGroupLabel = (
    groupKey: string,
    guests: WorkloadGuest[],
  ): { type: string; name: string } => {
    const node = nodeByInstance()[groupKey];
    if (node) return { type: '', name: getNodeDisplayName(node) };
    const normalizedGroupKey = guests.length > 0 ? getWorkloadGroupKey(guests[0]) : groupKey;
    const [prefix, ...rest] = normalizedGroupKey.split(':');
    const context = rest.length > 0 ? rest.join(':') : normalizedGroupKey;
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

  const groupedGuests = createMemo(() =>
    groupWorkloads(
      filteredGuests(),
      options.groupingMode(),
      options.guestSortComparator(),
    ),
  );

  const sortedGroupKeys = createMemo(() => {
    const groups = groupedGuests();
    const nodes = nodeByInstance();
    return Object.keys(groups).sort((a, b) => {
      const nodeA = nodes[a];
      const nodeB = nodes[b];
      const labelA = nodeA ? getNodeDisplayName(nodeA) : getGroupLabel(a, groups[a]).name;
      const labelB = nodeB ? getNodeDisplayName(nodeB) : getGroupLabel(b, groups[b]).name;
      return labelA.localeCompare(labelB) || a.localeCompare(b);
    });
  });

  const guestGlobalIndexById = createMemo(() => {
    const indexById = new Map<string, number>();
    const groups = groupedGuests();
    let globalIndex = 0;

    for (const groupKey of sortedGroupKeys()) {
      const guests = groups[groupKey] || [];
      for (const guest of guests) {
        indexById.set(getCanonicalWorkloadId(guest), globalIndex);
        globalIndex += 1;
      }
    }

    return indexById;
  });

  const revealGuestIndex = createMemo<number | null>(() => {
    const selectedId = options.selectedGuestId();
    if (!selectedId) return null;
    return guestGlobalIndexById().get(selectedId) ?? null;
  });

  const groupedWindowing = useGroupedTableWindowing({
    totalRowCount: () => filteredGuests().length,
    revealIndex: revealGuestIndex,
  });

  const groupStartIndexByKey = createMemo(() => {
    const starts = new Map<string, number>();
    const groups = groupedGuests();
    let globalIndex = 0;

    for (const groupKey of sortedGroupKeys()) {
      starts.set(groupKey, globalIndex);
      globalIndex += (groups[groupKey] || []).length;
    }

    return starts;
  });

  const windowedGroupedGuests = createMemo<Record<string, WorkloadGuest[]>>(() => {
    const groups = groupedGuests();
    if (!groupedWindowing.isWindowed()) {
      return groups;
    }

    const starts = groupStartIndexByKey();
    const result: Record<string, WorkloadGuest[]> = {};
    for (const groupKey of sortedGroupKeys()) {
      const guests = groups[groupKey] || [];
      const groupStart = starts.get(groupKey) ?? 0;
      const visible = groupedWindowing.getVisibleSlice(groupKey, guests, groupStart);
      if (visible.length > 0) {
        result[groupKey] = visible;
      }
    }

    return result;
  });

  const visibleGroupKeys = createMemo(() => {
    const keys = sortedGroupKeys();
    if (!groupedWindowing.isWindowed()) return keys;
    const groups = windowedGroupedGuests();
    return keys.filter((groupKey) => (groups[groupKey] || []).length > 0);
  });

  const topSpacerHeight = createMemo(() =>
    groupedWindowing.isWindowed()
      ? groupedWindowing.startIndex() * DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT
      : 0,
  );

  const bottomSpacerHeight = createMemo(() =>
    groupedWindowing.isWindowed()
      ? Math.max(
          0,
          (filteredGuests().length - groupedWindowing.endIndex()) * 32,
        )
      : 0,
  );

  useDashboardWorkloadViewportSync({
    filteredGuestCount: () => filteredGuests().length,
    groupedWindowing,
    rowHeight: DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT,
    tableBodyRef: options.tableBodyRef,
  });

  const totalStats = createMemo(() => computeWorkloadStats(filteredGuests()));
  const workloadIOEmphasis = createMemo(() => computeWorkloadIOEmphasis(filteredGuests()));

  return {
    bottomSpacerHeight,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestParentNodeMap,
    nodeByInstance,
    topSpacerHeight,
    totalStats,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
    workloadsSummaryFallbackCounts,
    workloadsSummaryFallbackSnapshots,
    workloadsSummaryVisibleIds,
  } as const;
}
