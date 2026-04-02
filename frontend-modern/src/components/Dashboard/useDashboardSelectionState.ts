import { useLocation, useNavigate } from '@solidjs/router';
import { createEffect, createMemo, createSignal, onCleanup, untrack, type Accessor } from 'solid-js';

import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import {
  isSummarySeriesInGroupScope,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import type { WorkloadGuest } from '@/types/workloads';
import { capturePendingAppShellRestoreTop } from '@/utils/appShellScrollRestoration';
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

  const clearPinnedSummaryScope = () => {
    preserveScrollableAncestorVerticalOffset(tableWrapperRef(), () => {
      setSelectedGuestIdRaw(null);
      setSelectedWorkloadGroupIdRaw(null);
    });
    const nextPath = resolveDashboardSelectionNavigateTarget({
      pathname: location.pathname,
      search: location.search,
      resourceId: null,
      summaryGroupId: null,
    });
    if (nextPath) {
      routeStateNavigate.schedule(nextPath);
    }
  };

  const summaryInteraction = useSummaryPageInteractionState({
    clearPinnedScope: clearPinnedSummaryScope,
    hoveredSeriesId: hoveredWorkloadId,
    hoveredGroupScope: hoveredWorkloadGroupScope,
    focusedSeriesId: selectedGuestId,
    focusedGroupId: selectedWorkloadGroupId,
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

  onCleanup(() => {
    routeStateNavigate.cleanup();
  });

  return {
    activeSummaryScopeState: summaryInteraction.activeScopeState,
    activeSummaryWorkloadGroupScope: summaryInteraction.activeGroupScope,
    activeSummaryWorkloadId: summaryInteraction.activeSeriesId,
    chartHoverSync: summaryInteraction.chartHoverSync,
    clearPinnedSummaryScope,
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
    shouldShowPinnedSummaryScopeFallback: summaryInteraction.shouldShowPinnedScopeFallback,
    shouldShowJumpToActiveWorkloadRow: summaryInteraction.shouldShowJumpToActiveRow,
    tableBodyRef,
  } as const;
}
