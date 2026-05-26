import { useLocation } from '@solidjs/router';
import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import {
  isSummarySeriesInGroupScope,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import type { WorkloadGuest } from '@/types/workloads';
import { capturePendingAppShellRestoreTop } from '@/utils/appShellScrollRestoration';
import {
  workloadsHasHoveredWorkload,
  workloadsHasVisibleWorkloadGroupScope,
  resolveWorkloadResourceSelection,
} from './workloadSelectionModel';

interface UseWorkloadsSelectionStateOptions {
  clearAdditionalPageStateOnEscape?: () => void;
  filteredGuests: Accessor<WorkloadGuest[]>;
  summaryGroupScopes: Accessor<Map<string, SummarySeriesGroupScope>>;
}

export function useWorkloadSelectionState(options: UseWorkloadsSelectionStateOptions) {
  const location = useLocation();

  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [selectedWorkloadGroupId, setSelectedWorkloadGroupIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [hoveredWorkloadGroupScope, setHoveredWorkloadGroupScope] =
    createSignal<SummarySeriesGroupScope | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledWorkloadGroupId, setHandledWorkloadGroupId] = createSignal<string | null>(null);
  const [revealedGuestId, setRevealedGuestId] = createSignal<string | null>(null);

  const [tableWrapperRef, setTableWrapperRefSignal] = createSignal<HTMLDivElement | undefined>(
    undefined,
  );
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
  const focusedWorkloadGroupScope = createMemo<SummarySeriesGroupScope | null>(() => {
    const selectedGroupId = selectedWorkloadGroupId();
    if (!selectedGroupId) {
      return null;
    }
    return options.summaryGroupScopes().get(selectedGroupId) ?? null;
  });

  const clearPinnedSummaryScope = () => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(null);
      setSelectedWorkloadGroupIdRaw(null);
    });
  };

  const summaryInteraction = useSummaryPageInteractionState({
    clearPinnedScope: clearPinnedSummaryScope,
    hoveredSeriesId: hoveredWorkloadId,
    hoveredGroupScope: hoveredWorkloadGroupScope,
    focusedSeriesId: selectedGuestId,
    focusedGroupId: selectedWorkloadGroupId,
    focusedGroupScope: focusedWorkloadGroupScope,
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

  const setSelectedGuestIdState = (id: string | null) => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(id);
    });
  };

  const setSelectedGuestId = (id: string | null) => {
    capturePendingAppShellRestoreTop();
    const activeFocusedGroupScope = focusedWorkloadGroupScope();
    const nextGroupScope =
      activeFocusedGroupScope && !isSummarySeriesInGroupScope(activeFocusedGroupScope, id)
        ? null
        : activeFocusedGroupScope;
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(id);
      setSelectedWorkloadGroupIdRaw(nextGroupScope?.id ?? null);
    });
  };

  const setFocusedWorkloadGroupScopeState = (scope: SummarySeriesGroupScope | null) => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedWorkloadGroupIdRaw(scope?.id ?? null);
      if (scope && !isSummarySeriesInGroupScope(scope, selectedGuestId())) {
        setSelectedGuestIdRaw(null);
      }
    });
  };

  const setFocusedWorkloadGroupScope = (scope: SummarySeriesGroupScope | null) => {
    setFocusedWorkloadGroupScopeState(scope);
  };

  createEffect(() => {
    const selection = resolveWorkloadResourceSelection(location.search);
    if (!selection) {
      if (handledResourceId() !== null) {
        setSelectedGuestIdState(null);
        setHandledResourceId(null);
      }
      if (handledWorkloadGroupId() !== null) {
        setSelectedWorkloadGroupIdRaw(null);
        setHandledWorkloadGroupId(null);
      }
      return;
    }

    const { resourceId, summaryGroupId } = selection;
    if (summaryGroupId !== handledWorkloadGroupId()) {
      setSelectedWorkloadGroupIdRaw(summaryGroupId);
      setHandledWorkloadGroupId(summaryGroupId);
    }
    if (resourceId !== handledResourceId()) {
      setSelectedGuestIdState(resourceId);
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

  createEffect(() => {
    const groupScope = hoveredWorkloadGroupScope();
    if (!groupScope) {
      return;
    }
    if (!workloadsHasVisibleWorkloadGroupScope(options.filteredGuests(), groupScope)) {
      setHoveredWorkloadGroupScope(null);
    }
  });

  createEffect(() => {
    const selectedGroupId = selectedWorkloadGroupId();
    if (!selectedGroupId) {
      return;
    }
    if (!focusedWorkloadGroupScope()) {
      setFocusedWorkloadGroupScope(null);
    }
  });

  createEffect(() => {
    const selectedId = selectedGuestId();
    const groupScope = focusedWorkloadGroupScope();
    if (!selectedId || !groupScope) {
      return;
    }
    if (!isSummarySeriesInGroupScope(groupScope, selectedId)) {
      setSelectedGuestId(null);
    }
  });

  return {
    activeSummaryWorkloadGroupScope: summaryInteraction.activeGroupScope,
    activeSummaryWorkloadId: summaryInteraction.activeSeriesId,
    clearPinnedSummaryScope,
    focusedSummaryWorkloadGroupScope: focusedWorkloadGroupScope,
    hoveredWorkloadId,
    hoveredSummaryWorkloadGroupScope: hoveredWorkloadGroupScope,
    focusedSummaryWorkloadGroupId: selectedWorkloadGroupId,
    revealedGuestId,
    selectedGuestId,
    setClearSurfaceRootRef,
    setFocusedWorkloadGroupScope,
    setHoveredWorkloadGroupScope,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableRootRef,
    setTableWrapperRef,
    tableBodyRef,
  } as const;
}
