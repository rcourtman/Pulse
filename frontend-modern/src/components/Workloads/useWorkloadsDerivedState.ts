import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import type { Alert, Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getNodeDisplayName } from '@/utils/nodes';
import { getCanonicalWorkloadId } from '@/utils/workloads';

import {
  getWorkloadGroupLabel,
  groupWorkloads,
  computeWorkloadStats,
  computeWorkloadIOEmphasis,
} from './workloadSelectors';
import { buildNodeByInstance, buildGuestParentNodeMap } from './workloadTopology';
import { useWorkloadViewportSync } from './useWorkloadViewportSync';
import { useGroupedTableWindowing } from './useGroupedTableWindowing';

type GroupingMode = 'grouped' | 'flat';

const DESKTOP_WORKLOAD_ROW_HEIGHT = 32;
const DESKTOP_WORKLOAD_GROUP_HEADER_HEIGHT = 33;
const PHONE_WORKLOAD_ROW_HEIGHT = 37;
const PHONE_WORKLOAD_GROUP_HEADER_HEIGHT = 28;
const WORKLOADS_TABLE_DIVIDER_HEIGHT = 1;
const DESKTOP_WORKLOAD_WINDOW_SIZE = 140;
const PHONE_WORKLOAD_WINDOW_SIZE = 36;

interface WorkloadVirtualOffsetOptions {
  detailHeight: number;
  detailRowIndex: number | null;
  groupHeaderHeight: number;
  groupStartIndices: readonly number[];
  offset: number;
  rowHeight: number;
  totalRows: number;
}

export const resolveWorkloadGuestIndexAtVirtualOffset = ({
  detailHeight,
  detailRowIndex,
  groupHeaderHeight,
  groupStartIndices,
  offset,
  rowHeight,
  totalRows,
}: WorkloadVirtualOffsetOptions): number => {
  if (totalRows <= 1) return 0;

  const countGroupStartsAtOrBefore = (guestIndex: number) => {
    let low = 0;
    let high = groupStartIndices.length;
    while (low < high) {
      const middle = Math.floor((low + high) / 2);
      if (groupStartIndices[middle] <= guestIndex) low = middle + 1;
      else high = middle;
    }
    return low;
  };

  let low = 0;
  let high = totalRows - 1;
  let resolved = 0;
  while (low <= high) {
    const middle = Math.floor((low + high) / 2);
    const detailOffset = detailRowIndex !== null && middle > detailRowIndex ? detailHeight : 0;
    const guestTop =
      middle * rowHeight + countGroupStartsAtOrBefore(middle) * groupHeaderHeight + detailOffset;
    if (guestTop <= offset) {
      resolved = middle;
      low = middle + 1;
    } else {
      high = middle - 1;
    }
  }
  return resolved;
};

interface WorkloadsWorkloadDerivedStateOptions {
  activeAlerts: Accessor<Record<string, Alert>>;
  alertsEnabled: Accessor<boolean>;
  allGuests: Accessor<WorkloadGuest[]>;
  filteredGuests: Accessor<WorkloadGuest[]>;
  groupingMode: Accessor<GroupingMode>;
  guestSortComparator: Accessor<((a: WorkloadGuest, b: WorkloadGuest) => number) | null>;
  nodes: Accessor<Node[]>;
  selectedGuestId: Accessor<string | null>;
  revealedGuestId: Accessor<string | null>;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
  groupLabelBadges?: Accessor<Record<string, { label: string }>>;
}

export function useWorkloadsDerivedState(options: WorkloadsWorkloadDerivedStateOptions) {
  const filteredGuests = createMemo<WorkloadGuest[]>(() => options.filteredGuests() ?? []);
  const phoneRowGeometry = typeof window !== 'undefined' && window.innerWidth < 640;
  const [estimatedRowHeight, setEstimatedRowHeight] = createSignal(
    phoneRowGeometry ? PHONE_WORKLOAD_ROW_HEIGHT : DESKTOP_WORKLOAD_ROW_HEIGHT,
  );
  const [estimatedGroupHeaderHeight, setEstimatedGroupHeaderHeight] = createSignal(
    phoneRowGeometry ? PHONE_WORKLOAD_GROUP_HEADER_HEIGHT : DESKTOP_WORKLOAD_GROUP_HEADER_HEIGHT,
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
    const badges = options.groupLabelBadges?.() ?? {};
    const badge = badges[groupKey] ?? badges[groupKey.toLowerCase()];
    return getWorkloadGroupLabel(
      groupKey,
      guests,
      node ? getNodeDisplayName(node) : null,
      badge?.label,
    );
  };

  const groupedGuests = createMemo(() =>
    groupWorkloads(filteredGuests(), options.groupingMode(), options.guestSortComparator()),
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

  const groupStartIndices = createMemo(() => {
    if (options.groupingMode() !== 'grouped') return [];
    const groups = groupedGuests();
    let guestIndex = 0;
    return sortedGroupKeys().map((groupKey) => {
      const start = guestIndex;
      guestIndex += (groups[groupKey] || []).length;
      return start;
    });
  });

  // Group headers are real rows in the virtual table. Account for every
  // header in either a spacer or the mounted window so scroll height cannot
  // grow and shrink as groups cross the window boundary.
  const countGroupStartsBefore = (guestIndex: number) => {
    const starts = groupStartIndices();
    let low = 0;
    let high = starts.length;
    while (low < high) {
      const middle = Math.floor((low + high) / 2);
      if (starts[middle] < guestIndex) low = middle + 1;
      else high = middle;
    }
    return low;
  };

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

  const [selectedDetailHeight, setSelectedDetailHeight] = createSignal(0);
  const selectedGuestIndex = createMemo<number | null>(() => {
    const selectedId = options.selectedGuestId();
    return selectedId ? (guestGlobalIndexById().get(selectedId) ?? null) : null;
  });

  createEffect(() => {
    options.selectedGuestId();
    setSelectedDetailHeight(0);
  });

  const guestIndexAtVirtualOffset = (offset: number, rowHeight: number) =>
    resolveWorkloadGuestIndexAtVirtualOffset({
      detailHeight: selectedDetailHeight(),
      detailRowIndex: selectedGuestIndex(),
      groupHeaderHeight: estimatedGroupHeaderHeight(),
      groupStartIndices: groupStartIndices(),
      offset,
      rowHeight,
      totalRows: filteredGuests().length,
    });

  const revealGuestIndex = createMemo<number | null>(() => {
    const selectedId = options.selectedGuestId();
    if (selectedId) {
      return guestGlobalIndexById().get(selectedId) ?? null;
    }
    const revealedId = options.revealedGuestId();
    if (!revealedId) return null;
    return guestGlobalIndexById().get(revealedId) ?? null;
  });

  const groupedWindowing = useGroupedTableWindowing({
    totalRowCount: () => filteredGuests().length,
    windowSize: phoneRowGeometry ? PHONE_WORKLOAD_WINDOW_SIZE : DESKTOP_WORKLOAD_WINDOW_SIZE,
    enableThreshold: phoneRowGeometry ? PHONE_WORKLOAD_WINDOW_SIZE : DESKTOP_WORKLOAD_WINDOW_SIZE,
    revealIndex: revealGuestIndex,
    rowIndexAtOffset: guestIndexAtVirtualOffset,
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

  const topSpacerHeight = createMemo(() => {
    if (!groupedWindowing.isWindowed() || groupedWindowing.startIndex() <= 0) return 0;
    const detailHeight =
      selectedGuestIndex() !== null && selectedGuestIndex()! < groupedWindowing.startIndex()
        ? selectedDetailHeight()
        : 0;
    const virtualHeight =
      groupedWindowing.startIndex() * estimatedRowHeight() +
      countGroupStartsBefore(groupedWindowing.startIndex()) * estimatedGroupHeaderHeight() +
      detailHeight;

    // The table's divide-y rule adds one border when a leading spacer is
    // mounted. Keep that border inside the virtual height instead of letting
    // the document grow by one pixel after the first scroll.
    return Math.max(0, virtualHeight - WORKLOADS_TABLE_DIVIDER_HEIGHT);
  });

  const bottomSpacerHeight = createMemo(() => {
    if (!groupedWindowing.isWindowed()) return 0;
    const detailHeight =
      selectedGuestIndex() !== null && selectedGuestIndex()! >= groupedWindowing.endIndex()
        ? selectedDetailHeight()
        : 0;
    return Math.max(
      0,
      (filteredGuests().length - groupedWindowing.endIndex()) * estimatedRowHeight() +
        (groupStartIndices().length - countGroupStartsBefore(groupedWindowing.endIndex())) *
          estimatedGroupHeaderHeight() +
        detailHeight,
    );
  });

  const workloadViewport = useWorkloadViewportSync({
    expandedDetailActive: () => selectedGuestIndex() !== null && selectedDetailHeight() > 0,
    filteredGuestCount: () => filteredGuests().length,
    groupedWindowing,
    onExpandedDetailHeightChange: (height) => {
      if (Math.abs(height - selectedDetailHeight()) > 0.5) setSelectedDetailHeight(height);
    },
    onRowGeometryChange: (geometry) => {
      if (geometry.rowHeight) setEstimatedRowHeight(geometry.rowHeight);
      if (geometry.groupHeaderHeight) setEstimatedGroupHeaderHeight(geometry.groupHeaderHeight);
    },
    rowHeight: estimatedRowHeight,
    selectedGuestId: options.selectedGuestId,
    tableBodyRef: options.tableBodyRef,
  });

  const totalStats = createMemo(() => computeWorkloadStats(filteredGuests()));
  const inventoryStats = createMemo(() => computeWorkloadStats(options.allGuests()));
  const workloadIOEmphasis = createMemo(() => computeWorkloadIOEmphasis(filteredGuests()));

  return {
    bottomSpacerHeight,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestParentNodeMap,
    inventoryStats,
    isScrollToTopVisible: workloadViewport.isScrollToTopVisible,
    nodeByInstance,
    topSpacerHeight,
    scrollToTop: workloadViewport.scrollToTop,
    totalStats,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
  } as const;
}
