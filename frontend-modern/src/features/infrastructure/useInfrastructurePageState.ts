import { createEffect, createMemo, createSignal } from 'solid-js';
import type { TimeRange } from '@/api/charts';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { buildSummaryScopePresentation } from '@/components/shared/summaryScopePresentation';
import {
  buildInfrastructureSummaryGroupScope,
  groupResources,
  infrastructureHasVisibleSummaryGroupScope,
} from '@/components/Infrastructure/infrastructureSelectors';
import {
  isSummarySeriesInGroupScope,
  resolveSummaryScopeState,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
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
  const summaryInteraction = useSummaryPageInteractionState({
    hoveredSeriesId: routeState.hoveredResourceId,
    hoveredGroupScope: hoveredResourceGroupScope,
    focusedSeriesId: routeState.expandedResourceId,
    focusedGroupScope: focusedResourceGroupScope,
    revealActiveSeries: routeState.setRevealedResourceId,
  });
  const setExpandedResourceId = (resourceId: string | null) => {
    const groupScope = focusedResourceGroupScope();
    if (groupScope && resourceId && !isSummarySeriesInGroupScope(groupScope, resourceId)) {
      routeState.setFocusedResourceGroupId(null);
    }
    routeState.setExpandedResourceId(resourceId);
  };

  const clearPinnedSummaryScope = () => {
    routeState.setExpandedResourceId(null);
    routeState.setFocusedResourceGroupId(null);
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

  const resourceNamesById = createMemo(() => {
    const names = new Map<string, string>();
    for (const resource of filteredResources()) {
      if (!resource.id?.trim()) {
        continue;
      }
      names.set(resource.id.trim(), getPreferredInfrastructureDisplayName(resource));
    }
    return names;
  });
  const pinnedSummaryScopeState = createMemo(() =>
    resolveSummaryScopeState({
      focusedSeriesId: routeState.expandedResourceId(),
      focusedGroupScope: focusedResourceGroupScope(),
    }),
  );
  const summaryScopePresentation = createMemo(() =>
    buildSummaryScopePresentation({
      allLabel: 'All resources',
      resolveEntityLabel: (seriesId) => resourceNamesById().get(seriesId) ?? seriesId,
      state: summaryInteraction.activeScopeState(),
    }),
  );
  const pinnedSummaryScopePresentation = createMemo(() =>
    buildSummaryScopePresentation({
      allLabel: 'All resources',
      resolveEntityLabel: (seriesId) => resourceNamesById().get(seriesId) ?? seriesId,
      state: pinnedSummaryScopeState(),
    }),
  );
  const hasPinnedSummaryScope = createMemo(
    () => pinnedSummaryScopeState().source === 'pinned',
  );

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
    hasPinnedSummaryScope,
    hoveredSummaryResourceGroupScope: hoveredResourceGroupScope,
    pinnedSummaryScopePresentation,
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
    setSummaryTableRootRef: summaryInteraction.setTableRootRef,
    summaryScopePresentation,
    shouldShowJumpToActiveResourceRow: summaryInteraction.shouldShowJumpToActiveRow,
  };
}
