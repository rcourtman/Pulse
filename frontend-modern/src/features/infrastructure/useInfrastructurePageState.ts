import { createEffect, createMemo, createSignal } from 'solid-js';
import type { TimeRange } from '@/api/charts';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { infrastructureHasVisibleSummaryGroupScope } from '@/components/Infrastructure/infrastructureSelectors';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
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
  const summaryInteraction = useSummaryPageInteractionState({
    hoveredSeriesId: routeState.hoveredResourceId,
    hoveredGroupScope: hoveredResourceGroupScope,
    focusedSeriesId: routeState.expandedResourceId,
    revealActiveSeries: routeState.setRevealedResourceId,
  });

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
    activeSummaryResourceId: summaryInteraction.activeSeriesId,
    activeSummaryResourceGroupScope: summaryInteraction.activeGroupScope,
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
    setChartHoverSync: summaryInteraction.setChartHoverSync,
    setHoveredResourceGroupScope,
    setSummaryTableRootRef: summaryInteraction.setTableRootRef,
    shouldShowJumpToActiveResourceRow: summaryInteraction.shouldShowJumpToActiveRow,
  };
}
