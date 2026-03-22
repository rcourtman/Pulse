import { useLocation } from '@solidjs/router';
import { createEffect, createSignal, type Accessor } from 'solid-js';

import type { WorkloadGuest } from '@/types/workloads';
import {
  dashboardHasHoveredWorkload,
  resolveDashboardResourceSelection,
} from './dashboardSelectionModel';

interface UseDashboardSelectionStateOptions {
  filteredGuests: Accessor<WorkloadGuest[]>;
  setSelectedNode: (value: string | null) => void;
}

export function useDashboardSelectionState(options: UseDashboardSelectionStateOptions) {
  const location = useLocation();

  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);

  let tableRef: HTMLDivElement | undefined;
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const setTableWrapperRef = (element: HTMLDivElement | undefined) => {
    tableRef = element;
  };

  const setSelectedGuestId = (id: string | null) => {
    let scroller: HTMLElement | null = tableRef ?? null;
    while (scroller) {
      const { overflowY } = getComputedStyle(scroller);
      if (
        (overflowY === 'auto' || overflowY === 'scroll') &&
        scroller.scrollHeight > scroller.clientHeight
      ) {
        break;
      }
      scroller = scroller.parentElement;
    }

    const scrollTop = scroller?.scrollTop ?? 0;
    setSelectedGuestIdRaw(id);

    if (scroller) scroller.scrollTop = scrollTop;
    requestAnimationFrame(() => {
      if (scroller) scroller.scrollTop = scrollTop;
    });
  };

  createEffect(() => {
    const selection = resolveDashboardResourceSelection(location.search);
    if (!selection) {
      if (handledResourceId() !== null) {
        setHandledResourceId(null);
      }
      return;
    }

    const { resourceId, selectedNode } = selection;
    if (resourceId === handledResourceId()) return;

    setSelectedGuestId(resourceId);
    if (selectedNode) {
      options.setSelectedNode(selectedNode);
    }
    setHandledResourceId(resourceId);
  });

  createEffect(() => {
    const hoveredId = hoveredWorkloadId();
    if (!hoveredId) return;
    if (!dashboardHasHoveredWorkload(options.filteredGuests(), hoveredId)) {
      setHoveredWorkloadId(null);
    }
  });

  return {
    hoveredWorkloadId,
    selectedGuestId,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableWrapperRef,
    tableBodyRef,
  } as const;
}
