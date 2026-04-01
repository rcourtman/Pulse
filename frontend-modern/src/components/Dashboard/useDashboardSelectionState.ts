import { useLocation, useNavigate } from '@solidjs/router';
import { createEffect, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';

import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import type { WorkloadGuest } from '@/types/workloads';
import { createRouteStateNavigateScheduler } from '@/utils/routeStateNavigation';
import {
  dashboardHasHoveredWorkload,
  resolveDashboardResourceSelection,
  resolveDashboardSelectionNavigateTarget,
} from './dashboardSelectionModel';

interface UseDashboardSelectionStateOptions {
  filteredGuests: Accessor<WorkloadGuest[]>;
  setSelectedNode: (value: string | null) => void;
}

export function useDashboardSelectionState(options: UseDashboardSelectionStateOptions) {
  const location = useLocation();
  const navigate = useNavigate();
  const routeStateNavigate = createRouteStateNavigateScheduler(
    navigate,
    () => `${untrack(() => location.pathname)}${untrack(() => location.search)}`,
  );

  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);

  let tableRef: HTMLDivElement | undefined;
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const setTableWrapperRef = (element: HTMLDivElement | undefined) => {
    tableRef = element;
  };

  const setSelectedGuestIdState = (id: string | null) => {
    preserveScrollableAncestorVerticalOffset(tableRef, () => {
      setSelectedGuestIdRaw(id);
    });
  };

  const setSelectedGuestId = (id: string | null) => {
    setSelectedGuestIdState(id);
    const nextPath = resolveDashboardSelectionNavigateTarget({
      pathname: location.pathname,
      search: location.search,
      resourceId: id,
    });
    if (nextPath) {
      routeStateNavigate.schedule(nextPath);
    }
  };

  createEffect(() => {
    const selection = resolveDashboardResourceSelection(location.search);
    if (!selection) {
      if (handledResourceId() !== null) {
        setSelectedGuestIdState(null);
        setHandledResourceId(null);
      }
      return;
    }

    const { resourceId, selectedNode } = selection;
    if (resourceId === handledResourceId()) return;

    setSelectedGuestIdState(resourceId);
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

  onCleanup(() => {
    routeStateNavigate.cleanup();
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
