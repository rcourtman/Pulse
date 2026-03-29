import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { blurFocusedTypeToSearch } from '@/hooks/useTypeToSearch';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { aiChatStore } from '@/stores/aiChat';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import type { ViewMode } from '@/types/workloads';

import { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
import {
  DEFAULT_DASHBOARD_SORT_DIRECTION,
  DEFAULT_DASHBOARD_SORT_KEY,
  DEFAULT_DASHBOARD_STATUS_MODE,
  type DashboardGroupingMode,
  type DashboardSortKey,
  type DashboardStatusMode,
} from './dashboardFilterModel';

interface DashboardControlsStateOptions {
  containerRuntime: Accessor<string>;
  resetWorkloadRouteFilters: () => void;
  selectedHostHint: Accessor<string | null>;
  selectedPlatform: Accessor<string | null>;
  selectedKubernetesContext: Accessor<string | null>;
  selectedKubernetesNamespace: Accessor<string | null>;
  selectedNode: Accessor<string | null>;
  setShowFilters: (value: boolean | ((current: boolean) => boolean)) => void;
  showFilters: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
}

export function useDashboardControlsState(options: DashboardControlsStateOptions) {
  const { isMobile } = useBreakpoint();
  const [search, setSearch] = createSignal('');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);

  const [statusMode, setStatusMode] = usePersistentSignal<DashboardStatusMode>(
    'dashboardStatusMode',
    DEFAULT_DASHBOARD_STATUS_MODE,
    {
      deserialize: (raw) =>
        raw === 'all' || raw === 'running' || raw === 'degraded' || raw === 'stopped'
          ? (raw as DashboardStatusMode)
          : DEFAULT_DASHBOARD_STATUS_MODE,
    },
  );

  const [groupingMode, setGroupingMode] = usePersistentSignal<DashboardGroupingMode>(
    'dashboardGroupingMode',
    'grouped',
    {
      deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped'),
    },
  );

  const [workloadsSummaryRange, setWorkloadsSummaryRange] = usePersistentSignal(
    STORAGE_KEYS.WORKLOADS_SUMMARY_RANGE,
    '1h',
    {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    },
  );

  const [workloadsSummaryCollapsed, setWorkloadsSummaryCollapsed] = usePersistentSignal<boolean>(
    STORAGE_KEYS.WORKLOADS_SUMMARY_COLLAPSED,
    false,
    { deserialize: (raw) => raw === 'true' },
  );

  const [sortKey, setSortKey] = createSignal<DashboardSortKey | null>(
    DEFAULT_DASHBOARD_SORT_KEY,
  );
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>(
    DEFAULT_DASHBOARD_SORT_DIRECTION,
  );

  const relevantColumns = createMemo(() => {
    const base = VIEW_MODE_COLUMNS[options.viewMode()];
    if (!base) return null;
    if (groupingMode() === 'grouped' && base.has('node')) {
      const filtered = new Set(base);
      filtered.delete('node');
      return filtered;
    }
    return base;
  });

  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.DASHBOARD_HIDDEN_COLUMNS,
    GUEST_COLUMNS,
    ['os', 'ip'],
    relevantColumns,
  );

  const visibleColumns = columnVisibility.visibleColumns;
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const mobileEssentialColumns = new Set(['name', 'cpu', 'memory', 'disk', 'link']);
  const mobileVisibleColumns = createMemo(() =>
    isMobile()
      ? visibleColumns().filter((column) => mobileEssentialColumns.has(column.id))
      : visibleColumns(),
  );
  const mobileVisibleColumnIds = createMemo(() =>
    isMobile() ? mobileVisibleColumns().map((column) => column.id) : visibleColumnIds(),
  );
  const totalColumns = createMemo(() => mobileVisibleColumns().length);

  const handleSort = (key: DashboardSortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
      return;
    }

    setSortKey(key);
    if (
      key === 'cpu' ||
      key === 'memory' ||
      key === 'disk' ||
      key === 'diskIo' ||
      key === 'netIo' ||
      key === 'uptime'
    ) {
      setSortDirection('desc');
    } else {
      setSortDirection(DEFAULT_DASHBOARD_SORT_DIRECTION);
    }
  };

  createEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;

      const hasActiveFilters =
        search().trim() ||
        sortKey() !== DEFAULT_DASHBOARD_SORT_KEY ||
        sortDirection() !== DEFAULT_DASHBOARD_SORT_DIRECTION ||
        options.selectedNode() !== null ||
        options.selectedHostHint() !== null ||
        options.selectedPlatform() !== null ||
        options.selectedKubernetesContext() !== null ||
        options.selectedKubernetesNamespace() !== null ||
        options.containerRuntime().trim() !== '' ||
        options.viewMode() !== 'all' ||
        statusMode() !== DEFAULT_DASHBOARD_STATUS_MODE;

      if (!hasActiveFilters) {
        options.setShowFilters(!options.showFilters());
        return;
      }

      setSearch('');
      setIsSearchLocked(false);
      setSortKey(DEFAULT_DASHBOARD_SORT_KEY);
      setSortDirection(DEFAULT_DASHBOARD_SORT_DIRECTION);
      options.resetWorkloadRouteFilters();
      setStatusMode(DEFAULT_DASHBOARD_STATUS_MODE);
      blurFocusedTypeToSearch();
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  const handleBeforeAutoFocus = () => {
    if (aiChatStore.focusInput()) return true;
    if (!options.showFilters()) options.setShowFilters(true);
    return false;
  };

  const handleTagClick = (tag: string) => {
    const currentSearch = search().trim();
    const tagFilter = `tags:${tag}`;

    if (currentSearch.includes(tagFilter)) {
      let newSearch = currentSearch;

      if (currentSearch === tagFilter) {
        newSearch = '';
      } else if (currentSearch.startsWith(tagFilter + ',')) {
        newSearch = currentSearch.replace(tagFilter + ',', '').trim();
      } else if (currentSearch.endsWith(', ' + tagFilter)) {
        newSearch = currentSearch.replace(', ' + tagFilter, '').trim();
      } else if (currentSearch.includes(', ' + tagFilter + ',')) {
        newSearch = currentSearch.replace(', ' + tagFilter + ',', ',').trim();
      } else if (currentSearch.includes(tagFilter + ', ')) {
        newSearch = currentSearch.replace(tagFilter + ', ', '').trim();
      }

      setSearch(newSearch);
      if (!newSearch) {
        setIsSearchLocked(false);
      }
      return;
    }

    if (!currentSearch || isSearchLocked()) {
      setSearch(tagFilter);
      setIsSearchLocked(false);
    } else {
      setSearch(`${currentSearch}, ${tagFilter}`);
    }

    if (!options.showFilters()) {
      options.setShowFilters(true);
    }
  };

  const dashboardFilterColumnVisibility = createMemo(() => ({
    availableColumns: columnVisibility.availableToggles(),
    isColumnHidden: columnVisibility.isHiddenByUser,
    onColumnToggle: columnVisibility.toggle,
    onColumnReset: columnVisibility.resetToDefaults,
  }));

  return {
    columnVisibility,
    dashboardFilterColumnVisibility,
    groupingMode,
    handleBeforeAutoFocus,
    handleSort,
    handleTagClick,
    isMobile,
    isSearchLocked,
    mobileVisibleColumnIds,
    mobileVisibleColumns,
    search,
    setGroupingMode,
    setSearch,
    setSortDirection,
    setSortKey,
    setStatusMode,
    sortDirection,
    sortKey,
    statusMode,
    totalColumns,
    visibleColumns,
    workloadsSummaryCollapsed,
    workloadsSummaryRange,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
  } as const;
}
