import { useLocation } from '@solidjs/router';
import {
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
  untrack,
  type Accessor,
} from 'solid-js';

import { ResourceAPI } from '@/api/resources';
import { getCanonicalWorkloadId } from '@/utils/workloads';

import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import type { WorkloadGuest } from '@/types/workloads';
import {
  workloadsHasHoveredWorkload,
  resolveWorkloadResourceSelection,
} from './workloadSelectionModel';

import { createGuestDrawerViewState } from './useGuestDrawerState';

interface UseWorkloadsSelectionStateOptions {
  clearAdditionalPageStateOnEscape?: () => void;
  filteredGuests: Accessor<WorkloadGuest[]>;
  routeStateEnabled?: Accessor<boolean>;
}

export function useWorkloadSelectionState(options: UseWorkloadsSelectionStateOptions) {
  const location = useLocation();

  const selectedGuestDrawerState = createGuestDrawerViewState();
  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  let referenceLookup: AbortController | undefined;
  const [revealedGuestId, setRevealedGuestId] = createSignal<string | null>(null);

  const [tableWrapperRef, setTableWrapperRefSignal] = createSignal<HTMLDivElement | undefined>(
    undefined,
  );
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const clearPinnedSummaryScope = () => {
    referenceLookup?.abort();
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      selectedGuestDrawerState.reset();
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
  const applySelectedGuestId = (id: string | null) => {
    if (id !== selectedGuestId()) selectedGuestDrawerState.reset();
    setSelectedGuestIdRaw(id);
  };

  const setSelectedGuestId = (id: string | null) => {
    referenceLookup?.abort();
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => applySelectedGuestId(id));
  };

  const routeReference = createMemo(() => resolveWorkloadResourceSelection(location.search));
  createEffect(
    on(
      [() => location.pathname, routeReference, () => options.routeStateEnabled?.() ?? true],
      ([, reference, enabled]) => {
        if (!enabled) return;
        referenceLookup?.abort();
        // A route selects a destination. The shared focus bridge reveals it;
        // retaining the previous scroll offset would undo that reveal.
        applySelectedGuestId(reference);
        if (!reference) return;

        // Exact current IDs need no translation. Historical/source references
        // are resolved by the registry, which owns succession and ambiguity.
        if (
          untrack(options.filteredGuests).some(
            (guest) => getCanonicalWorkloadId(guest) === reference,
          )
        )
          return;
        const controller = new AbortController();
        referenceLookup = controller;
        onCleanup(() => controller.abort());
        void ResourceAPI.getResource(reference, { signal: controller.signal })
          .then((resource) => {
            if (
              !controller.signal.aborted &&
              selectedGuestId() === reference &&
              resource.id?.trim()
            ) {
              applySelectedGuestId(resource.id);
            }
          })
          .catch(() => {
            // An unresolved or refused reference must not select another row.
          });
      },
    ),
  );

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
    selectedGuestDrawerState,
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
