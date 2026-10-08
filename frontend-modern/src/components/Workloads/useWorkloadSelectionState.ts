import { useLocation } from '@solidjs/router';
import { createEffect, createSignal, type Accessor } from 'solid-js';

import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import type { WorkloadGuest } from '@/types/workloads';
import {
  workloadsHasHoveredWorkload,
  resolveWorkloadResourceSelection,
} from './workloadSelectionModel';

interface UseWorkloadsSelectionStateOptions {
  clearAdditionalPageStateOnEscape?: () => void;
  filteredGuests: Accessor<WorkloadGuest[]>;
  routeStateEnabled?: Accessor<boolean>;
}

export function useWorkloadSelectionState(options: UseWorkloadsSelectionStateOptions) {
  const location = useLocation();

  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [revealedGuestId, setRevealedGuestId] = createSignal<string | null>(null);

  const [tableWrapperRef, setTableWrapperRefSignal] = createSignal<HTMLDivElement | undefined>(
    undefined,
  );
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const clearPinnedSummaryScope = () => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(null);
    });
  };

  const summaryInteraction = useSummaryPageInteractionState({
    clearPinnedScope: clearPinnedSummaryScope,
    hoveredSeriesId: hoveredWorkloadId,
    focusedSeriesId: selectedGuestId,
    onEscapeClear: () => {
      clearPinnedSummaryScope();
      options.clearAdditionalPageStateOnEscape?.();
    },
    revealActiveSeries: setRevealedGuestId,
  });

  const setTableWrapperRef = (element: HTMLDivElement | undefined) => {
    setTableWrapperRefSignal(element);
  };

  const setTableRootRef = (element: HTMLDivElement | undefined) => {
    summaryInteraction.setTableRootRef(element);
  };

  const setClearSurfaceRootRef = (element: HTMLDivElement | undefined) => {
    summaryInteraction.setClearSurfaceRootRef(element);
  };

  // Row focus is local: it never navigates, so it must not stage an app-shell
  // restore that the next unrelated route change would replay.
  const setSelectedGuestId = (id: string | null) => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(id);
    });
  };

  createEffect(() => {
    if (options.routeStateEnabled && !options.routeStateEnabled()) return;
    const resourceId = resolveWorkloadResourceSelection(location.search);
    if (!resourceId) {
      if (handledResourceId() !== null) {
        setSelectedGuestId(null);
        setHandledResourceId(null);
      }
      return;
    }

    if (resourceId !== handledResourceId()) {
      setSelectedGuestId(resourceId);
      setHandledResourceId(resourceId);
    }
  });

  createEffect(() => {
    const hoveredId = hoveredWorkloadId();
    if (!hoveredId) return;
    if (!workloadsHasHoveredWorkload(options.filteredGuests(), hoveredId)) {
      setHoveredWorkloadId(null);
    }
  });

  createEffect(() => {
    const revealedId = revealedGuestId();
    if (!revealedId) return;
    if (!workloadsHasHoveredWorkload(options.filteredGuests(), revealedId)) {
      setRevealedGuestId(null);
    }
  });

  return {
    activeSummaryWorkloadId: summaryInteraction.activeSeriesId,
    clearPinnedSummaryScope,
    hoveredWorkloadId,
    revealedGuestId,
    selectedGuestId,
    setClearSurfaceRootRef,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableRootRef,
    setTableWrapperRef,
    tableBodyRef,
  } as const;
}
