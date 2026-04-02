import { useLocation, useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';

import {
  findInlineDetailElement,
  preserveScrollableAncestorVerticalOffset,
  revealInlineDetailInViewport,
} from '@/components/shared/contextualFocus';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import {
  isSummarySeriesInGroupScope,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import type { WorkloadGuest } from '@/types/workloads';
import { createRouteStateNavigateScheduler } from '@/utils/routeStateNavigation';
import {
  dashboardHasHoveredWorkload,
  dashboardHasVisibleWorkloadGroupScope,
  resolveDashboardResourceSelection,
  resolveDashboardSelectionNavigateTarget,
} from './dashboardSelectionModel';

interface UseDashboardSelectionStateOptions {
  filteredGuests: Accessor<WorkloadGuest[]>;
  summaryGroupScopes: Accessor<Map<string, SummarySeriesGroupScope>>;
}

export function useDashboardSelectionState(options: UseDashboardSelectionStateOptions) {
  const location = useLocation();
  const navigate = useNavigate();
  const routeStateNavigate = createRouteStateNavigateScheduler(
    navigate,
    () => `${untrack(() => location.pathname)}${untrack(() => location.search)}`,
  );

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

  const summaryInteraction = useSummaryPageInteractionState({
    hoveredSeriesId: hoveredWorkloadId,
    hoveredGroupScope: hoveredWorkloadGroupScope,
    focusedSeriesId: selectedGuestId,
    focusedGroupScope: focusedWorkloadGroupScope,
    revealActiveSeries: setRevealedGuestId,
  });

  const setTableWrapperRef = (element: HTMLDivElement | undefined) => {
    setTableWrapperRefSignal(element);
    summaryInteraction.setTableRootRef(element);
  };

  const setSelectedGuestIdState = (id: string | null) => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(id);
    });
  };

  const setSelectedGuestId = (id: string | null) => {
    const activeFocusedGroupScope = focusedWorkloadGroupScope();
    const nextGroupScope =
      activeFocusedGroupScope && !isSummarySeriesInGroupScope(activeFocusedGroupScope, id)
        ? null
        : activeFocusedGroupScope;
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(id);
      setSelectedWorkloadGroupIdRaw(nextGroupScope?.id ?? null);
    });
    const nextPath = resolveDashboardSelectionNavigateTarget({
      pathname: location.pathname,
      search: location.search,
      resourceId: id,
      summaryGroupId: nextGroupScope?.id ?? null,
    });
    if (nextPath) {
      routeStateNavigate.schedule(nextPath);
    }
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
    const nextGroupId = scope?.id ?? null;
    const nextSelectedGuestId =
      scope && !isSummarySeriesInGroupScope(scope, selectedGuestId()) ? null : selectedGuestId();
    setFocusedWorkloadGroupScopeState(scope);
    const nextPath = resolveDashboardSelectionNavigateTarget({
      pathname: location.pathname,
      search: location.search,
      resourceId: nextSelectedGuestId,
      summaryGroupId: nextGroupId,
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
    if (!dashboardHasHoveredWorkload(options.filteredGuests(), hoveredId)) {
      setHoveredWorkloadId(null);
    }
  });

  createEffect(() => {
    const revealedId = revealedGuestId();
    if (!revealedId) return;
    if (!dashboardHasHoveredWorkload(options.filteredGuests(), revealedId)) {
      setRevealedGuestId(null);
    }
  });

  createEffect(() => {
    const groupScope = hoveredWorkloadGroupScope();
    if (!groupScope) {
      return;
    }
    if (!dashboardHasVisibleWorkloadGroupScope(options.filteredGuests(), groupScope)) {
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

  createEffect(() => {
    const selectedId = selectedGuestId();
    const root = tableWrapperRef();
    if (!selectedId || !root || typeof window === 'undefined') {
      return;
    }

    let rafId: number | undefined;
    let timeoutId: number | undefined;
    let settled = false;

    const cleanup = () => {
      settled = true;
      if (rafId !== undefined) {
        window.cancelAnimationFrame(rafId);
      }
      if (timeoutId !== undefined) {
        window.clearTimeout(timeoutId);
      }
    };

    const attemptReveal = (remainingFrames: number) => {
      if (settled) {
        return;
      }

      const row = root.querySelector<HTMLElement>(`[data-summary-series-id="${selectedId}"]`);
      const detail = findInlineDetailElement(root, selectedId);
      if (row && detail) {
        const didScroll = revealInlineDetailInViewport({ row, detail });
        if (!didScroll || remainingFrames <= 0) {
          cleanup();
          return;
        }
      } else if (remainingFrames <= 0) {
        cleanup();
        return;
      }

      rafId = window.requestAnimationFrame(() => attemptReveal(remainingFrames - 1));
    };

    rafId = window.requestAnimationFrame(() => attemptReveal(24));
    timeoutId = window.setTimeout(cleanup, 1500);
  });

  onCleanup(() => {
    routeStateNavigate.cleanup();
  });

  return {
    activeSummaryWorkloadGroupScope: summaryInteraction.activeGroupScope,
    activeSummaryWorkloadId: summaryInteraction.activeSeriesId,
    chartHoverSync: summaryInteraction.chartHoverSync,
    focusedSummaryWorkloadGroupScope: focusedWorkloadGroupScope,
    hoveredWorkloadId,
    hoveredSummaryWorkloadGroupScope: hoveredWorkloadGroupScope,
    jumpToActiveWorkloadRow: summaryInteraction.jumpToActiveRow,
    focusedSummaryWorkloadGroupId: selectedWorkloadGroupId,
    revealedGuestId,
    selectedGuestId,
    setChartHoverSync: summaryInteraction.setChartHoverSync,
    setFocusedWorkloadGroupScope,
    setHoveredWorkloadGroupScope,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableWrapperRef,
    shouldShowJumpToActiveWorkloadRow: summaryInteraction.shouldShowJumpToActiveRow,
    tableBodyRef,
  } as const;
}
