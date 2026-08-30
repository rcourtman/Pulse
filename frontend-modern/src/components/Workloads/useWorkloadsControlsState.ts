import { createEffect, createMemo, createSignal, onMount, type Accessor } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { recordWorkloadHistoryActivity } from '@/api/workloadHistoryActivity';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { blurFocusedTypeToSearch } from '@/hooks/useTypeToSearch';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { aiChatStore } from '@/stores/aiChat';
import type { ViewMode, WorkloadType } from '@/types/workloads';

import {
  GUEST_COLUMNS,
  VIEW_MODE_COLUMNS,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadTableLayoutMode,
  getWorkloadVisibleColumnsForLayout,
  resolveWorkloadColumnViewMode,
} from './guestRowModel';
import {
  DEFAULT_WORKLOADS_SORT_DIRECTION,
  DEFAULT_WORKLOADS_SORT_KEY,
  DEFAULT_WORKLOADS_METRIC_DISPLAY_MODE,
  DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
  DEFAULT_WORKLOADS_STATUS_MODE,
  type WorkloadsGroupingMode,
  type WorkloadsMetricDisplayMode,
  type WorkloadsMetricHoverMode,
  type WorkloadsSortKey,
  type WorkloadsStatusMode,
} from './workloadsFilterModel';
import {
  isWorkloadTableMetricHistoryRange,
  WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE,
  type WorkloadTableMetricHistoryRange,
} from './workloadMetricHistoryModel';
import {
  buildWorkloadColumnLayoutEntries,
  parseWorkloadColumnLayoutParam,
  resolveWorkloadColumnLayoutEntries,
  serializeWorkloadColumnLayout,
  toggleWorkloadColumnLayoutEntry,
  workloadColumnLayoutIds,
  workloadColumnLayoutWidths,
  WORKLOAD_COLUMNS_URL_PARAM,
  type WorkloadColumnLayoutEntry,
} from './workloadColumnLayoutUrl';
import {
  hasWorkloadColumnWidths,
  isWorkloadManualSizingSupported,
  normalizeWorkloadColumnWidths,
  pruneWorkloadColumnWidths,
  seedWorkloadColumnWidthsFromDefaults,
  snapshotWorkloadColumnWidths,
  sumWorkloadColumnWidths,
  WORKLOAD_COLUMN_FALLBACK_WIDTH,
  withWorkloadColumnWidth,
  withoutWorkloadColumnWidth,
  workloadColumnWidthsStorageKey,
  type WorkloadColumnWidths,
} from './workloadColumnWidths';

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
  metricHoverMode?: Accessor<WorkloadsMetricHoverMode>;
  onMetricHoverModeChange?: (value: WorkloadsMetricHoverMode) => void;
  metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;
  onMetricHistoryRangeChange?: (value: WorkloadTableMetricHistoryRange) => void;
  columnVisibilityStorageScope?: string;
  additionalDefaultHiddenColumnIds?: string[];
  // False when no workload in the current set has an availability check
  // linked, which suppresses the Availability column instead of rendering an
  // empty cell on every row. Omitted means "assume data exists".
  hasAvailabilityData?: Accessor<boolean>;
  columnLabelOverrides?: Partial<Record<string, string>>;
  excludedWorkloadTypes?: readonly WorkloadType[];
  setShowFilters: (value: boolean | ((current: boolean) => boolean)) => void;
  showFilters: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
  routeStateEnabled?: Accessor<boolean>;
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
  const routeStateEnabled = options.routeStateEnabled ?? (() => true);
  const [activeRouteSearch, setActiveRouteSearch] = createSignal(location.search);
  createEffect(() => {
    if (!routeStateEnabled()) return;
    setActiveRouteSearch(location.search);
  });
  const breakpoint = useBreakpoint();
  const workloadTableLayoutMode = createMemo(() => {
    const measuredWidth = options.layoutWidth?.();
    return typeof measuredWidth === 'number' && measuredWidth > 0
      ? getWorkloadTableLayoutModeForContainer(measuredWidth)
      : getWorkloadTableLayoutMode(breakpoint.width());
  });
  const isMobile = createMemo(() =>
    ['narrow', 'phone', 'mobile'].includes(workloadTableLayoutMode()),
  );
  const [isSearchLocked, setIsSearchLocked] = createSignal(false);

  const updateSearchParam = (mutate: (params: URLSearchParams) => void): void => {
    if (!routeStateEnabled()) return;
    const params = new URLSearchParams(activeRouteSearch());
    mutate(params);
    const query = params.toString();
    // Router propagation is asynchronous. Keep the local URL-backed controls
    // responsive in the same event turn, then let the location effect confirm
    // the canonical value after navigation completes.
    setActiveRouteSearch(query ? `?${query}` : '');
    navigate(`${location.pathname}${query ? `?${query}` : ''}`, { replace: true });
  };

  const search: Accessor<string> = () => new URLSearchParams(activeRouteSearch()).get('q') ?? '';
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
    parseWorkloadsStatusMode(new URLSearchParams(activeRouteSearch()).get('status'));
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
    if (!routeStateEnabled()) return;
    const params = new URLSearchParams(activeRouteSearch());
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

  const [internalMetricHoverMode, setInternalMetricHoverMode] =
    usePersistentSignal<WorkloadsMetricHoverMode>(
      STORAGE_KEYS.WORKLOADS_METRIC_HOVER_MODE,
      DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
      {
        deserialize: (raw) =>
          raw === 'details' || raw === 'history' ? raw : DEFAULT_WORKLOADS_METRIC_HOVER_MODE,
      },
    );
  const workloadMetricHoverMode: Accessor<WorkloadsMetricHoverMode> =
    options.metricHoverMode ?? internalMetricHoverMode;
  const setWorkloadMetricHoverMode = (value: WorkloadsMetricHoverMode): void => {
    if (value === 'details' && value !== workloadMetricHoverMode()) {
      recordWorkloadHistoryActivity('details_selected');
    }
    if (options.onMetricHoverModeChange) {
      options.onMetricHoverModeChange(value);
      return;
    }
    setInternalMetricHoverMode(value);
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
    if (value !== workloadMetricHistoryRange()) {
      recordWorkloadHistoryActivity('range_change');
    }
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

  // The Availability cell renders nothing at all until an availability check
  // is linked to that workload, so a table where no row has one shows an empty
  // column under an "Avail" header and tells the reader nothing. Gate it on the
  // live data rather than on a persisted preference: dropping it from the
  // relevant set leaves the user's own show/hide choice untouched, and the
  // column comes back on its own the first time a check is linked.
  const hasAvailabilityData = createMemo(() =>
    options.hasAvailabilityData ? options.hasAvailabilityData() : true,
  );

  const relevantColumns = createMemo(() => {
    const columnViewMode = resolveWorkloadColumnViewMode(
      options.viewMode(),
      options.excludedWorkloadTypes,
    );
    const base = VIEW_MODE_COLUMNS[columnViewMode];
    const dropNode = effectiveGroupingMode() === 'grouped';
    const dropAvailability = !hasAvailabilityData();
    if (!base) {
      if (!dropAvailability) return null;
      const all = new Set(GUEST_COLUMNS.map((column) => column.id));
      all.delete('availability');
      return all;
    }
    if ((dropNode && base.has('node')) || (dropAvailability && base.has('availability'))) {
      const filtered = new Set(base);
      if (dropNode) filtered.delete('node');
      if (dropAvailability) filtered.delete('availability');
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
    // One-time hides for users who already have a saved column preference for
    // this scope. `backup` reads exclusively from `resource.proxmox.lastBackup`,
    // so on the scopes that opt into hiding it the column says the same thing
    // on every row forever. The migration only fires where the id is already in
    // that scope's default-hidden set, so Proxmox keeps the column visible.
    // `tags` was on this list while vSphere shipped adapter provenance strings
    // in place of real tags; the adapter now reads vCenter's tagging API, so no
    // scope default-hides it and retiring the preference would be wrong.
    ['aiContext', 'backup'],
  );

  const visibleColumns = columnVisibility.visibleColumns;
  const [columnWidths, setColumnWidths] = usePersistentSignal<WorkloadColumnWidths>(
    workloadColumnWidthsStorageKey(
      STORAGE_KEYS.WORKLOADS_COLUMN_WIDTHS,
      options.columnVisibilityStorageScope,
    ),
    {},
    {
      serialize: (widths) => JSON.stringify(widths),
      deserialize: (raw) => {
        try {
          return normalizeWorkloadColumnWidths(JSON.parse(raw));
        } catch {
          return {};
        }
      },
    },
  );
  const [pendingColumnWidths, setPendingColumnWidths] = createSignal<WorkloadColumnWidths | null>(
    null,
  );
  const manualColumnSizingSupported = createMemo(() =>
    isWorkloadManualSizingSupported(workloadTableLayoutMode()),
  );
  const renderableColumnIds = createMemo(() => {
    const relevant = relevantColumns();
    return new Set(
      workloadColumns
        .filter((column) => !relevant || relevant.has(column.id))
        .map((column) => column.id),
    );
  });
  const urlColumnLayout = createMemo<WorkloadColumnLayoutEntry[]>(() => {
    if (!routeStateEnabled()) return [];
    return resolveWorkloadColumnLayoutEntries(
      parseWorkloadColumnLayoutParam(
        new URLSearchParams(activeRouteSearch()).get(WORKLOAD_COLUMNS_URL_PARAM),
      ),
      renderableColumnIds(),
    );
  });
  const hasUrlColumnLayout = createMemo(
    () => manualColumnSizingSupported() && urlColumnLayout().length > 0,
  );
  const manualColumnSizingActive = createMemo(
    () =>
      manualColumnSizingSupported() &&
      (pendingColumnWidths() !== null ||
        hasUrlColumnLayout() ||
        hasWorkloadColumnWidths(columnWidths())),
  );
  const activeColumnWidths = createMemo<WorkloadColumnWidths>(() => {
    if (!manualColumnSizingActive()) return {};
    const pending = pendingColumnWidths();
    if (pending) return pending;
    if (hasUrlColumnLayout()) return workloadColumnLayoutWidths(urlColumnLayout());
    return columnWidths();
  });
  const workloadTableVisibleColumns = createMemo(() => {
    if (hasUrlColumnLayout()) {
      const byId = new Map(workloadColumns.map((column) => [column.id, column] as const));
      return workloadColumnLayoutIds(urlColumnLayout())
        .map((columnId) => byId.get(columnId))
        .filter((column): column is (typeof workloadColumns)[number] => !!column);
    }
    return getWorkloadVisibleColumnsForLayout(visibleColumns(), workloadTableLayoutMode(), {
      manualSizing: manualColumnSizingActive(),
    });
  });
  const workloadTableVisibleColumnIds = createMemo(() =>
    workloadTableVisibleColumns().map((column) => column.id),
  );
  const totalColumns = createMemo(() => workloadTableVisibleColumns().length);
  const workloadTableManualWidth = createMemo<number | null>(() =>
    sumWorkloadColumnWidths(activeColumnWidths(), workloadTableVisibleColumnIds()),
  );

  const beginWorkloadColumnResize = (measuredWidths: Readonly<Record<string, number>>): void => {
    if (!manualColumnSizingSupported()) return;
    const measured = snapshotWorkloadColumnWidths(columnWidths(), measuredWidths);
    setPendingColumnWidths(seedWorkloadColumnWidthsFromDefaults(measured, visibleColumns()));
  };
  const previewWorkloadColumnWidth = (columnId: string, width: number): void => {
    setPendingColumnWidths((current) =>
      current ? withWorkloadColumnWidth(current, columnId, width) : current,
    );
  };
  const writeColumnLayoutParam = (entries: readonly WorkloadColumnLayoutEntry[] | null): void => {
    updateSearchParam((params) => {
      if (!entries || entries.length === 0) params.delete(WORKLOAD_COLUMNS_URL_PARAM);
      else params.set(WORKLOAD_COLUMNS_URL_PARAM, serializeWorkloadColumnLayout(entries));
    });
  };
  const publishColumnLayout = (
    widths: WorkloadColumnWidths,
    orderedIds: readonly string[],
  ): void => {
    writeColumnLayoutParam(buildWorkloadColumnLayoutEntries(orderedIds, widths));
  };
  const commitWorkloadColumnResize = (): void => {
    const pending = pendingColumnWidths();
    if (!pending) return;
    const renderedColumnIds = workloadTableVisibleColumnIds();
    const committed = pruneWorkloadColumnWidths(pending, renderedColumnIds);
    if (!hasUrlColumnLayout()) setColumnWidths(committed);
    setPendingColumnWidths(null);
    publishColumnLayout(committed, renderedColumnIds);
  };
  const cancelWorkloadColumnResize = (): void => {
    setPendingColumnWidths(null);
  };
  const clearWorkloadColumnWidth = (columnId: string): void => {
    const renderedColumnIds = workloadTableVisibleColumnIds();
    const current = activeColumnWidths();
    setPendingColumnWidths(null);
    const linkOwnsLayout = hasUrlColumnLayout();
    const without = withoutWorkloadColumnWidth(current, columnId);
    if (!hasWorkloadColumnWidths(without)) {
      if (!linkOwnsLayout) setColumnWidths({});
      writeColumnLayoutParam(null);
      return;
    }
    const column = workloadColumns.find((candidate) => candidate.id === columnId);
    const restored = column ? seedWorkloadColumnWidthsFromDefaults(without, [column]) : without;
    if (!linkOwnsLayout) setColumnWidths(restored);
    publishColumnLayout(restored, renderedColumnIds);
  };
  const resetWorkloadColumnWidths = (): void => {
    setPendingColumnWidths(null);
    setColumnWidths({});
    writeColumnLayoutParam(null);
  };
  const toggleWorkloadColumn = (columnId: string): void => {
    if (!hasUrlColumnLayout()) {
      columnVisibility.toggle(columnId);
      return;
    }
    const column = workloadColumns.find((candidate) => candidate.id === columnId);
    const defaultWidth =
      column?.width && /^\d+(?:\.\d+)?px$/.test(column.width)
        ? Number.parseFloat(column.width)
        : WORKLOAD_COLUMN_FALLBACK_WIDTH;
    writeColumnLayoutParam(
      toggleWorkloadColumnLayoutEntry(urlColumnLayout(), columnId, defaultWidth),
    );
  };

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

  const workloadsFilterColumnVisibility = createMemo(() => ({
    availableColumns: getWorkloadVisibleColumnsForLayout(
      columnVisibility.availableToggles(),
      workloadTableLayoutMode(),
      { manualSizing: manualColumnSizingActive() },
    ),
    isColumnHidden: (columnId: string) =>
      hasUrlColumnLayout()
        ? !workloadTableVisibleColumnIds().includes(columnId)
        : columnVisibility.isHiddenByUser(columnId),
    onColumnToggle: toggleWorkloadColumn,
    onColumnReset: columnVisibility.resetToDefaults,
    hasColumnWidthOverrides: manualColumnSizingActive(),
    onColumnWidthsReset: resetWorkloadColumnWidths,
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
    workloadMetricHoverMode,
    workloadTableVisibleColumnIds,
    workloadTableVisibleColumns,
    workloadTableLayoutMode,
    workloadColumnWidths: activeColumnWidths,
    workloadManualColumnSizing: manualColumnSizingActive,
    workloadManualColumnSizingSupported: manualColumnSizingSupported,
    workloadTableManualWidth,
    beginWorkloadColumnResize,
    previewWorkloadColumnWidth,
    commitWorkloadColumnResize,
    cancelWorkloadColumnResize,
    clearWorkloadColumnWidth,
    resetWorkloadColumnWidths,
    setWorkloadMetricHistoryRange,
    setWorkloadMetricDisplayMode,
    setWorkloadMetricHoverMode,
  } as const;
}
