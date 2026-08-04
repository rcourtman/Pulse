import { createMemo, createSignal, onMount, type Accessor } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { blurFocusedTypeToSearch } from '@/hooks/useTypeToSearch';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { aiChatStore } from '@/stores/aiChat';
import type { ViewMode } from '@/types/workloads';

import {
  GUEST_COLUMNS,
  VIEW_MODE_COLUMNS,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadTableReadableMinWidth,
  getWorkloadTableLayoutMode,
  getWorkloadVisibleColumnsForLayout,
} from './guestRowModel';
import {
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
  DEFAULT_WORKLOADS_STATUS_MODE,
  type WorkloadsGroupingMode,
  type WorkloadsMetricDisplayMode,
  type WorkloadsSortKey,
  type WorkloadsStatusMode,
} from './workloadsFilterModel';
import {
  isWorkloadTableMetricHistoryRange,
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  type WorkloadTableMetricHistoryRange,
} from './workloadMetricHistoryModel';

interface WorkloadsControlsStateOptions {
  layoutWidth?: Accessor<number | null | undefined>;
  forcedGroupingMode?: WorkloadsGroupingMode;
  defaultSortKey?: WorkloadsSortKey;
  statusModeStorageScope?: string;
  // When a platform page owns the metric display mode (e.g. Proxmox
  // overview shares it across a top hosts table and the embedded workloads
  // surface), pass the accessor + change handler so the controls track the
  // page-level state instead of forking a local persistent signal.
  metricDisplayMode?: Accessor<WorkloadsMetricDisplayMode>;
  onMetricDisplayModeChange?: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;
  onMetricHistoryRangeChange?: (value: WorkloadTableMetricHistoryRange) => void;
  columnVisibilityStorageScope?: string;
  additionalDefaultHiddenColumnIds?: string[];
  columnLabelOverrides?: Partial<Record<string, string>>;
  setShowFilters: (value: boolean | ((current: boolean) => boolean)) => void;
  showFilters: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
}

const parseWorkloadsStatusMode = (raw: string | null | undefined): WorkloadsStatusMode =>
  raw === 'all' || raw === 'running' || raw === 'degraded' || raw === 'stopped'
    ? (raw as WorkloadsStatusMode)
    : DEFAULT_WORKLOADS_STATUS_MODE;

const workloadsStatusModeStorageKey = (scope: string | undefined): string => {
  const trimmedScope = (scope || '').trim();
  return trimmedScope ? `workloadsStatusMode:${trimmedScope}` : 'workloadsStatusMode';
};

const saveWorkloadsStatusMode = (scope: string | undefined, value: WorkloadsStatusMode): void => {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(workloadsStatusModeStorageKey(scope), value);
  } catch {
    // Ignore storage failures; the URL remains the canonical live state.
  }
};

const readSavedWorkloadsStatusMode = (scope: string | undefined): WorkloadsStatusMode => {
  if (typeof window === 'undefined') return DEFAULT_WORKLOADS_STATUS_MODE;
  try {
    const raw = window.localStorage.getItem(workloadsStatusModeStorageKey(scope));
    return parseWorkloadsStatusMode(raw);
  } catch {
    return DEFAULT_WORKLOADS_STATUS_MODE;
  }
};

export function useWorkloadsControlsState(options: WorkloadsControlsStateOptions) {
  const location = useLocation();
  const navigate = useNavigate();
  const breakpoint = useBreakpoint();
  const workloadTableLayoutMode = createMemo(() => {
    const measuredWidth = options.layoutWidth?.();
    return typeof measuredWidth === 'number' && measuredWidth > 0
      ? getWorkloadTableLayoutModeForContainer(measuredWidth)
      : getWorkloadTableLayoutMode(breakpoint.width());
  });
  const isMobile = createMemo(() => workloadTableLayoutMode() === 'mobile');
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);

  const updateSearchParam = (mutate: (params: URLSearchParams) => void): void => {
    const params = new URLSearchParams(location.search);
    mutate(params);
    const query = params.toString();
    navigate(`${location.pathname}${query ? `?${query}` : ''}`, { replace: true });
  };

  const search: Accessor<string> = () => new URLSearchParams(location.search).get('q') ?? '';
  const setSearch = (value: string): void => {
    updateSearchParam((params) => {
      if (value === '') {
        params.delete('q');
      } else {
        params.set('q', value);
      }
    });
  };

  const statusMode: Accessor<WorkloadsStatusMode> = () =>
    parseWorkloadsStatusMode(new URLSearchParams(location.search).get('status'));
  const setStatusMode = (value: WorkloadsStatusMode): void => {
    saveWorkloadsStatusMode(options.statusModeStorageScope, value);
    updateSearchParam((params) => {
      if (value === DEFAULT_WORKLOADS_STATUS_MODE) {
        params.delete('status');
      } else {
        params.set('status', value);
      }
    });
  };

  onMount(() => {
    if (typeof window === 'undefined') return;
    const params = new URLSearchParams(location.search);
    if (params.has('status')) return;
    const saved = readSavedWorkloadsStatusMode(options.statusModeStorageScope);
    if (saved !== DEFAULT_WORKLOADS_STATUS_MODE) {
      params.set('status', saved);
      navigate(`${location.pathname}?${params.toString()}`, { replace: true });
    }
  });

  const [groupingMode, setGroupingMode] = usePersistentSignal<WorkloadsGroupingMode>(
    'workloadsGroupingMode',
    'grouped',
    {
      deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped'),
    },
  );
  const effectiveGroupingMode = createMemo<WorkloadsGroupingMode>(
    () => options.forcedGroupingMode ?? groupingMode(),
  );

  const [internalMetricDisplayMode, setInternalMetricDisplayMode] =
    usePersistentSignal<WorkloadsMetricDisplayMode>(
      STORAGE_KEYS.WORKLOADS_METRIC_DISPLAY_MODE,
      DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
      {
        deserialize: (raw) =>
          raw === 'bars' || raw === 'sparklines' ? raw : DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
      },
    );
  const workloadMetricDisplayMode: Accessor<WorkloadsMetricDisplayMode> =
    options.metricDisplayMode ?? internalMetricDisplayMode;
  const setWorkloadMetricDisplayMode = (value: WorkloadsMetricDisplayMode): void => {
    if (options.onMetricDisplayModeChange) {
      options.onMetricDisplayModeChange(value);
      return;
    }
    setInternalMetricDisplayMode(value);
  };

  const [internalMetricHistoryRange, setInternalMetricHistoryRange] =
    usePersistentSignal<WorkloadTableMetricHistoryRange>(
      STORAGE_KEYS.WORKLOADS_METRIC_HISTORY_RANGE,
      WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
      {
        deserialize: (raw) =>
          isWorkloadTableMetricHistoryRange(raw) ? raw : WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
      },
    );
  const workloadMetricHistoryRange: Accessor<WorkloadTableMetricHistoryRange> =
    options.metricHistoryRange ?? internalMetricHistoryRange;
  const setWorkloadMetricHistoryRange = (value: WorkloadTableMetricHistoryRange): void => {
    if (options.onMetricHistoryRangeChange) {
      options.onMetricHistoryRangeChange(value);
      return;
    }
    setInternalMetricHistoryRange(value);
  };

  const defaultSortKey = options.defaultSortKey ?? DEFAULT_WORKLOADS_SORT_KEY;
  const [sortKey, setSortKey] = createSignal<WorkloadsSortKey | null>(defaultSortKey);
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>(
    DEFAULT_WORKLOADS_SORT_DIRECTION,
  );

  const relevantColumns = createMemo(() => {
    const base = VIEW_MODE_COLUMNS[options.viewMode()];
    if (!base) return null;
    if (effectiveGroupingMode() === 'grouped' && base.has('node')) {
      const filtered = new Set(base);
      filtered.delete('node');
      return filtered;
    }
    return base;
  });

  const workloadColumns = GUEST_COLUMNS.map((column) => {
    const label = options.columnLabelOverrides?.[column.id]?.trim();
    return label ? { ...column, label } : column;
  });
  const columnStorageKey = options.columnVisibilityStorageScope?.trim()
    ? `${STORAGE_KEYS.WORKLOADS_HIDDEN_COLUMNS}:${options.columnVisibilityStorageScope.trim()}`
    : STORAGE_KEYS.WORKLOADS_HIDDEN_COLUMNS;
  const defaultHiddenColumnIds = Array.from(
    new Set(['os', 'ip', ...(options.additionalDefaultHiddenColumnIds ?? [])]),
  );
  const columnVisibility = useColumnVisibility(
    columnStorageKey,
    workloadColumns,
    defaultHiddenColumnIds,
    relevantColumns,
    {},
    ['aiContext'],
  );

  // Columns the user explicitly chose to show. Keeping that intent separate
  // from the responsive defaults makes the choice stable across resizing:
  // automatic columns may disappear as space contracts, while an explicit
  // choice remains readable via horizontal scrolling when necessary.
  const [forcedColumnIds, setForcedColumnIds] = usePersistentSignal<string[]>(
    `${columnStorageKey}:forced`,
    [],
    {
      serialize: (arr) => JSON.stringify(arr),
      deserialize: (str) => {
        try {
          const parsed: unknown = JSON.parse(str);
          if (!Array.isArray(parsed)) return [];
          return parsed.filter((value): value is string => typeof value === 'string');
        } catch {
          return [];
        }
      },
    },
  );
  const forcedColumnIdSet = createMemo(() => new Set(forcedColumnIds()));
  const layoutHiddenColumnIds = createMemo(() => {
    const layoutVisible = new Set(
      getWorkloadVisibleColumnsForLayout(workloadColumns, workloadTableLayoutMode()).map(
        (column) => column.id,
      ),
    );
    return new Set(
      workloadColumns.filter((column) => !layoutVisible.has(column.id)).map((c) => c.id),
    );
  });

  const visibleColumns = columnVisibility.visibleColumns;
  const workloadTableVisibleColumns = createMemo(() => {
    const layoutVisible = new Set(
      getWorkloadVisibleColumnsForLayout(visibleColumns(), workloadTableLayoutMode()).map(
        (column) => column.id,
      ),
    );
    const forced = forcedColumnIdSet();
    return visibleColumns().filter(
      (column) => layoutVisible.has(column.id) || forced.has(column.id),
    );
  });
  const workloadTableVisibleColumnIds = createMemo(() =>
    workloadTableVisibleColumns().map((column) => column.id),
  );
  const workloadTableMinimumWidth = createMemo(() =>
    getWorkloadTableReadableMinWidth(
      workloadTableVisibleColumns(),
      workloadTableLayoutMode(),
      forcedColumnIdSet(),
    ),
  );
  const totalColumns = createMemo(() => workloadTableVisibleColumns().length);

  const handleSort = (key: WorkloadsSortKey) => {
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
      setSortDirection(DEFAULT_WORKLOADS_SORT_DIRECTION);
    }
  };

  const resetWorkloadsControls = () => {
    setSearch('');
    setIsSearchLocked(false);
    setSortKey(defaultSortKey);
    setSortDirection(DEFAULT_WORKLOADS_SORT_DIRECTION);
    setStatusMode(DEFAULT_WORKLOADS_STATUS_MODE);
    blurFocusedTypeToSearch();
  };

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

  // Menu checkbox state mirrors what the table actually renders. Showing a
  // column records explicit intent; hiding it removes that pin and applies the
  // same preference at every responsive stage.
  const isColumnHiddenForMenu = (id: string): boolean =>
    columnVisibility.isHiddenByUser(id) ||
    (layoutHiddenColumnIds().has(id) && !forcedColumnIdSet().has(id));

  const handleColumnToggle = (id: string): void => {
    if (!isColumnHiddenForMenu(id)) {
      setForcedColumnIds(forcedColumnIds().filter((existing) => existing !== id));
      columnVisibility.hide(id);
      return;
    }
    columnVisibility.show(id);
    if (!forcedColumnIdSet().has(id)) setForcedColumnIds([...forcedColumnIds(), id]);
  };

  const handleColumnReset = (): void => {
    setForcedColumnIds([]);
    columnVisibility.resetToDefaults();
  };

  const workloadsFilterColumnVisibility = createMemo(() => ({
    availableColumns: columnVisibility.availableToggles(),
    isColumnHidden: isColumnHiddenForMenu,
    onColumnToggle: handleColumnToggle,
    onColumnReset: handleColumnReset,
    showReset: forcedColumnIds().length > 0,
  }));

  return {
    columnVisibility,
    workloadsFilterColumnVisibility,
    groupingMode: effectiveGroupingMode,
    handleBeforeAutoFocus,
    handleSort,
    handleTagClick,
    isMobile,
    isSearchLocked,
    resetWorkloadsControls,
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
    workloadMetricHistoryRange,
    workloadMetricDisplayMode,
    workloadTableVisibleColumnIds,
    workloadTableVisibleColumns,
    workloadTableLayoutMode,
    workloadTableMinimumWidth,
    setWorkloadMetricHistoryRange,
    setWorkloadMetricDisplayMode,
  } as const;
}
