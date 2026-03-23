import { createEffect, createMemo, createSignal } from 'solid-js';
import type { TimeRange } from '@/api/charts';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useKioskMode } from '@/hooks/useKioskMode';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import {
  tokenizeSearch,
  filterResources,
  collectAvailableSources,
  collectAvailableStatuses,
  buildStatusOptions,
} from '@/components/Infrastructure/infrastructureSelectors';
import { useInfrastructurePageRouteState } from './useInfrastructurePageRouteState';

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
  const [deployCluster, setDeployCluster] = createSignal<DeployCluster | null>(null);
  const [filtersOpen, setFiltersOpen] = createSignal(false);

  const hasResources = createMemo(() => resources().length > 0);
  const showNoResources = createMemo(() => initialLoadComplete() && !hasResources() && !error());
  const activeFilterCount = createMemo(
    () => (selectedSource() !== '' ? 1 : 0) + (selectedStatus() !== '' ? 1 : 0),
  );
  const availableSources = createMemo(() => collectAvailableSources(resources()));
  const availableStatuses = createMemo(() => collectAvailableStatuses(resources()));
  const statusOptions = createMemo(() => buildStatusOptions(availableStatuses()));
  const hasActiveFilters = createMemo(
    () => selectedSource() !== '' || selectedStatus() !== '' || searchQuery().trim().length > 0,
  );
  const searchTerms = createMemo(() => tokenizeSearch(searchQuery()));
  const filteredResources = createMemo(() =>
    filterResources(
      resources(),
      selectedSource() !== '' ? new Set([selectedSource()]) : new Set(),
      selectedStatus() !== '' ? new Set([selectedStatus()]) : new Set(),
      searchTerms(),
    ),
  );
  const hasFilteredResources = createMemo(() => filteredResources().length > 0);
  const routeState = useInfrastructurePageRouteState({
    resources,
    filteredResources,
    initialLoadComplete,
    selectedSource,
    setSelectedSource,
    searchQuery,
    setSearchQuery,
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
    ...routeState,
    isMobile,
    deployCluster,
    setDeployCluster,
    filtersOpen,
    setFiltersOpen,
    activeFilterCount,
    kioskMode,
    availableSources,
    statusOptions,
    hasActiveFilters,
    clearFilters,
    filteredResources,
    hasFilteredResources,
  };
}
