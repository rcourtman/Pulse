import { createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
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
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import {
  buildInfrastructurePath,
  INFRASTRUCTURE_PATH,
  INFRASTRUCTURE_QUERY_PARAMS,
  parseInfrastructureLinkSearch,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';

export type GroupingMode = 'grouped' | 'flat';

type DeployCluster = {
  id: string;
  name: string;
};

export function useInfrastructurePageState() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const location = useLocation();
  const navigate = useNavigate();
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
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [hoveredResourceId, setHoveredResourceId] = createSignal<string | null>(null);
  const [highlightedResourceId, setHighlightedResourceId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledSourceParam, setHandledSourceParam] = createSignal<string | null>(null);
  const [handledQueryParam, setHandledQueryParam] = createSignal('');
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

  let highlightTimer: number | undefined;
  let pendingUrlSyncHandle: number | null = null;
  let pendingUrlSyncPath: string | null = null;

  const scheduleUrlSyncNavigate = (nextPath: string) => {
    pendingUrlSyncPath = nextPath;
    if (pendingUrlSyncHandle !== null) return;
    pendingUrlSyncHandle = window.setTimeout(() => {
      pendingUrlSyncHandle = null;
      const target = pendingUrlSyncPath;
      pendingUrlSyncPath = null;
      if (!target) return;
      const current = `${untrack(() => location.pathname)}${untrack(() => location.search)}`;
      if (current === target) return;
      navigate(target, { replace: true });
    }, 0);
  };

  const clearFilters = () => {
    setSelectedSource('');
    setSelectedStatus('');
    setSearchQuery('');
  };

  const handleNavigateToSettings = () => navigate('/settings');

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  createEffect(() => {
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
    if (!resourceId || resourceId === handledResourceId()) return;
    const matching = resources().some((resource) => resource.id === resourceId);
    if (!matching) return;
    setExpandedResourceId(resourceId);
    setHighlightedResourceId(resourceId);
    setHandledResourceId(resourceId);
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
    highlightTimer = window.setTimeout(() => {
      setHighlightedResourceId(null);
    }, 2000);
  });

  createEffect(() => {
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
    if (resourceId) return;
    if (handledResourceId() === null) return;

    if (expandedResourceId() !== null) {
      setExpandedResourceId(null);
    }
    if (highlightedResourceId() !== null) {
      setHighlightedResourceId(null);
    }
    setHandledResourceId(null);
  });

  createEffect(() => {
    const { source: sourceParam } = parseInfrastructureLinkSearch(location.search);
    if (!sourceParam) {
      const previous = (handledSourceParam() ?? '').trim();
      if (previous) {
        if (selectedSource() !== '') setSelectedSource('');
        setHandledSourceParam('');
      } else if (handledSourceParam() === null) {
        setHandledSourceParam('');
      }
      return;
    }
    if (sourceParam === handledSourceParam()) return;
    const normalized = normalizeSourcePlatformKey(sourceParam) ?? '';
    setSelectedSource(normalized);
    setHandledSourceParam(sourceParam);
  });

  createEffect(() => {
    const { query: nextSearch } = parseInfrastructureLinkSearch(location.search);
    const normalized = nextSearch ?? '';
    if (normalized !== handledQueryParam()) {
      if (normalized !== untrack(searchQuery)) {
        setSearchQuery(normalized);
      }
      setHandledQueryParam(normalized);
    }
  });

  createEffect(() => {
    if (location.pathname !== INFRASTRUCTURE_PATH) return;

    const parsed = parseInfrastructureLinkSearch(location.search);
    const urlSource = parsed.source ?? '';
    const urlQuery = parsed.query ?? '';
    const urlResource = parsed.resource ?? '';
    if ((handledSourceParam() ?? '') !== urlSource) return;
    if (handledQueryParam() !== urlQuery) return;
    if (urlResource && handledResourceId() !== urlResource && !initialLoadComplete()) return;

    const nextSource = selectedSource();
    const nextQuery = searchQuery().trim();
    const currentLinkedResource = parsed.resource;
    const selectedResourceId = expandedResourceId();
    const shouldPreserveIncomingResource =
      !selectedResourceId && Boolean(currentLinkedResource) && !initialLoadComplete();
    const nextResource = shouldPreserveIncomingResource
      ? currentLinkedResource
      : (selectedResourceId ?? '');

    const managedPath = buildInfrastructurePath({
      source: nextSource || null,
      query: nextQuery || null,
      resource: nextResource || null,
    });
    const managedUrl = new URL(managedPath, 'http://pulse.local');
    const currentParams = new URLSearchParams(location.search);
    const nextParams = new URLSearchParams(location.search);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.source);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.query);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.resource);
    managedUrl.searchParams.forEach((value, key) => {
      nextParams.set(key, value);
    });

    if (!areSearchParamsEquivalent(currentParams, nextParams)) {
      const nextSearch = nextParams.toString();
      const nextPath = nextSearch ? `${INFRASTRUCTURE_PATH}?${nextSearch}` : INFRASTRUCTURE_PATH;
      scheduleUrlSyncNavigate(nextPath);
    }
  });

  createEffect(() => {
    const hoveredId = hoveredResourceId();
    if (!hoveredId) return;
    const exists = filteredResources().some((resource) => resource.id === hoveredId);
    if (!exists) {
      setHoveredResourceId(null);
    }
  });

  onCleanup(() => {
    if (pendingUrlSyncHandle !== null) {
      window.clearTimeout(pendingUrlSyncHandle);
      pendingUrlSyncHandle = null;
      pendingUrlSyncPath = null;
    }
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
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
    expandedResourceId,
    setExpandedResourceId,
    hoveredResourceId,
    setHoveredResourceId,
    highlightedResourceId,
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
    handleNavigateToSettings,
  };
}
