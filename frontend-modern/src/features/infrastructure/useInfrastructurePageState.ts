import { createEffect, createMemo, createSignal } from 'solid-js';
import type { TimeRange } from '@/api/charts';
import { preserveScrollableAncestorVerticalOffset } from '@/components/shared/contextualFocus';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { capturePendingAppShellRestoreTop } from '@/utils/appShellScrollRestoration';
import {
  buildInfrastructureSummaryGroupScope,
  groupResources,
  infrastructureHasVisibleSummaryGroupScope,
} from '@/components/Infrastructure/infrastructureSelectors';
import {
  isSummarySeriesInGroupScope,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import { useInfrastructurePageRouteState } from './useInfrastructurePageRouteState';
import { buildInfrastructurePageFilterDerivation } from './infrastructurePageModel';

export type GroupingMode = 'grouped' | 'flat';

type DeployCluster = {
  id: string;
  name: string;
};

export function useInfrastructurePageState() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const kioskMode = useKioskMode();
  const { isMobile } = useBreakpoint();

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  const [selectedSource, setSelectedSource] = createSignal('');
  const [selectedStatus, setSelectedStatus] = createSignal('');
  const [searchQuery, setSearchQuery] = createSignal('');
  const [infrastructureSummaryRange, setInfrastructureSummaryRange] =
    usePersistentSignal<TimeRange>(STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_RANGE, '1h', {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    });
  const [summaryCollapsed, setSummaryCollapsed] = usePersistentSignal<boolean>(
    STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_COLLAPSED,
    false,
    { deserialize: (raw) => raw === 'true' },
  );
  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'infrastructureGroupingMode',
    'grouped',
    { deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped') },
  );
  const [hoveredResourceGroupScope, setHoveredResourceGroupScope] =
    createSignal<SummarySeriesGroupScope | null>(null);
  const [deployCluster, setDeployCluster] = createSignal<DeployCluster | null>(null);
  const [filtersOpen, setFiltersOpen] = createSignal(false);
  const [tableRootRef, setTableRootRef] = createSignal<HTMLDivElement | undefined>(undefined);

  const hasResources = createMemo(() => resources().length > 0);
  const showNoResources = createMemo(() => initialLoadComplete() && !hasResources() && !error());
  const filterDerivation = createMemo(() =>
    buildInfrastructurePageFilterDerivation(
      resources(),
      selectedSource(),
      selectedStatus(),
      searchQuery(),
    ),
  );
  const activeFilterCount = createMemo(() => filterDerivation().activeFilterCount);
  const availableSources = createMemo(() => filterDerivation().availableSources);
  const sourceOptions = createMemo(() => filterDerivation().sourceOptions);
  const statusOptions = createMemo(() => filterDerivation().statusOptions);
  const hasActiveFilters = createMemo(() => filterDerivation().hasActiveFilters);
  const filteredResources = createMemo(() => filterDerivation().filteredResources);
  const hasFilteredResources = createMemo(() => filterDerivation().hasFilteredResources);
  const routeState = useInfrastructurePageRouteState({
    resources,
    filteredResources,
    initialLoadComplete,
    selectedSource,
    setSelectedSource,
    searchQuery,
    setSearchQuery,
  });
  const focusedResourceGroupScope = createMemo<SummarySeriesGroupScope | null>(() => {
    if (groupingMode() !== 'grouped') {
      return null;
    }
    const focusedGroupId = routeState.focusedResourceGroupId();
    if (!focusedGroupId) {
      return null;
    }
    const groups = groupResources(filteredResources(), groupingMode());
    for (const group of groups) {
      const scope = buildInfrastructureSummaryGroupScope(group);
      if (scope?.id === focusedGroupId) {
        return scope;
      }
    }
    return null;
  });
  const clearPinnedSummaryScope = () => {
    capturePendingAppShellRestoreTop();
    preserveTableScrollAnchor(() => {
      routeState.setExpandedResourceId(null);
      routeState.setFocusedResourceGroupId(null);
    });
  };
  const summaryInteraction = useSummaryPageInteractionState({
    clearPinnedScope: clearPinnedSummaryScope,
    hoveredSeriesId: routeState.hoveredResourceId,
    hoveredGroupScope: hoveredResourceGroupScope,
    focusedSeriesId: routeState.expandedResourceId,
    focusedGroupId: routeState.focusedResourceGroupId,
    focusedGroupScope: focusedResourceGroupScope,
    revealActiveSeries: routeState.setRevealedResourceId,
  });
  const setSummaryTableRootRef = (element: HTMLDivElement | undefined) => {
    setTableRootRef(element);
    summaryInteraction.setTableRootRef(element);
  };

  const preserveTableScrollAnchor = (apply: () => void) => {
    preserveScrollableAncestorVerticalOffset(tableRootRef(), apply);
  };

  const setExpandedResourceId = (resourceId: string | null) => {
    capturePendingAppShellRestoreTop();
    const groupScope = focusedResourceGroupScope();
    if (groupScope && resourceId && !isSummarySeriesInGroupScope(groupScope, resourceId)) {
      preserveTableScrollAnchor(() => {
        routeState.setFocusedResourceGroupId(null);
      });
    }
    preserveTableScrollAnchor(() => {
      routeState.setExpandedResourceId(resourceId);
    });
  };

  const setFocusedResourceGroupId = (groupId: string | null) => {
    capturePendingAppShellRestoreTop();
    preserveTableScrollAnchor(() => {
      routeState.setFocusedResourceGroupId(groupId);
    });
  };

  const clearFilters = () => {
    setSelectedSource('');
    setSelectedStatus('');
    setSearchQuery('');
  };

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  createEffect(() => {
    if (groupingMode() === 'grouped') {
      return;
    }
    if (hoveredResourceGroupScope()) {
      setHoveredResourceGroupScope(null);
    }
    if (routeState.focusedResourceGroupId()) {
      routeState.setFocusedResourceGroupId(null);
    }
  });

  createEffect(() => {
    const groupScope = hoveredResourceGroupScope();
    if (!groupScope) {
      return;
    }
    if (!infrastructureHasVisibleSummaryGroupScope(filteredResources(), groupScope)) {
      setHoveredResourceGroupScope(null);
    }
  });

  createEffect(() => {
    const focusedGroupId = routeState.focusedResourceGroupId();
    if (!focusedGroupId) {
      return;
    }
    if (!focusedResourceGroupScope()) {
      routeState.setFocusedResourceGroupId(null);
    }
  });

  createEffect(() => {
    const expandedResourceId = routeState.expandedResourceId();
    const groupScope = focusedResourceGroupScope();
    if (!expandedResourceId || !groupScope) {
      return;
    }
    if (!isSummarySeriesInGroupScope(groupScope, expandedResourceId)) {
      routeState.setExpandedResourceId(null);
    }
  });

  return {
    loading,
    error,
    refetch,
    initialLoadComplete,
    showNoResources,
    selectedSource,
    setSelectedSource,
    selectedStatus,
    setSelectedStatus,
    searchQuery,
    setSearchQuery,
    infrastructureSummaryRange,
    setInfrastructureSummaryRange,
    summaryCollapsed,
    setSummaryCollapsed,
    groupingMode,
    setGroupingMode,
    activeSummaryScopeState: summaryInteraction.activeScopeState,
    activeSummaryResourceId: summaryInteraction.activeSeriesId,
    activeSummaryResourceGroupScope: summaryInteraction.activeGroupScope,
    clearPinnedSummaryScope,
    focusedSummaryResourceGroupScope: focusedResourceGroupScope,
    focusedSummaryResourceGroupId: routeState.focusedResourceGroupId,
    hoveredSummaryResourceGroupScope: hoveredResourceGroupScope,
    ...routeState,
    chartHoverSync: summaryInteraction.chartHoverSync,
    isMobile,
    jumpToActiveResourceRow: summaryInteraction.jumpToActiveRow,
    deployCluster,
    setDeployCluster,
    filtersOpen,
    setFiltersOpen,
    activeFilterCount,
    kioskMode,
    availableSources,
    sourceOptions,
    statusOptions,
    hasActiveFilters,
    clearFilters,
    filteredResources,
    hasFilteredResources,
    setExpandedResourceId,
    setChartHoverSync: summaryInteraction.setChartHoverSync,
    setHoveredResourceGroupScope,
    setSummaryTableRootRef,
    shouldShowJumpToActiveResourceRow: summaryInteraction.shouldShowJumpToActiveRow,
    setFocusedResourceGroupId,
  };
}
