import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { ColumnPriority } from '@/hooks/useBreakpoint';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import {
  buildInfrastructureSummaryGroupScope,
  splitPrimaryAndServiceResources,
  sortResources,
  groupResources,
  computeIOScale,
} from '@/components/Infrastructure/infrastructureSelectors';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { useTableWindowing } from './useTableWindowing';
import { useUnifiedResourceTableViewportSync } from './useUnifiedResourceTableViewportSync';
import {
  buildHostRowIndexById,
  buildHostTableItems,
  buildResourceLabelById,
  getHostRevealTargetIndex,
  getHostSpacerHeights,
  getNextUnifiedResourceTableSortState,
  getUnifiedResourceTableColumnPresentations,
  getUnifiedResourceTableHeaderLabels,
  getUnifiedResourceTableLayoutMode,
  isUnifiedResourceServiceColumnVisible,
  isUnifiedResourceTableColumnVisible,
  getUnifiedResourceTableShellClass,
  getUnifiedResourceTableSortIndicator,
  getUnifiedSources,
  getVisibleHostTableItems,
  HOST_TABLE_ESTIMATED_ROW_HEIGHT,
  HOST_TABLE_WINDOW_SIZE,
  normalizeUnifiedResourceTableLayoutWidth,
  shouldShowUnifiedResourceHostTable,
  shouldUseUnifiedResourceTableMobileLayout,
  sortServiceResources,
  type HostTableItem,
  type UnifiedResourceTableSortKey as SortKey,
} from './unifiedResourceTableStateModel';

export interface UnifiedResourceTableProps {
  resources: Resource[];
  expandedResourceId: string | null;
  clearPinnedSummaryScope?: () => void;
  highlightedResourceId?: string | null;
  revealedResourceId?: string | null;
  hoveredResourceId?: string | null;
  activeSummaryGroupScope?: SummarySeriesGroupScope | null;
  hoveredSummaryGroupScope?: SummarySeriesGroupScope | null;
  focusedSummaryGroupScope?: SummarySeriesGroupScope | null;
  focusedSummaryGroupId?: string | null;
  onExpandedResourceChange: (id: string | null) => void;
  onHoverChange?: (id: string | null) => void;
  onGroupFocusChange?: (groupId: string | null) => void;
  onGroupHoverChange?: (scope: SummarySeriesGroupScope | null) => void;
  groupingMode?: 'grouped' | 'flat';
  onDeployCluster?: (clusterId: string, clusterName: string) => void;
  setTableRootRef?: (element: HTMLDivElement | undefined) => void;
}

export function useUnifiedResourceTableState(props: UnifiedResourceTableProps) {
  const breakpoint = useBreakpoint();
  const [tableContainerWidth, setTableContainerWidth] = createSignal<number | null>(null);
  const [tableRootElement, setTableRootElement] = createSignal<HTMLDivElement | null>(null);
  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const updateTableContainerWidth = (width: number | null | undefined) => {
    if (typeof width === 'number' && Number.isFinite(width) && width > 0) {
      setTableContainerWidth(Math.round(width));
    }
  };

  const setTableRootRef = (element: HTMLDivElement | undefined) => {
    setTableRootElement(element ?? null);
    props.setTableRootRef?.(element);
    if (element) {
      updateTableContainerWidth(element.clientWidth);
    } else {
      setTableContainerWidth(null);
    }
  };

  createEffect(() => {
    const element = tableRootElement();
    if (!element || typeof ResizeObserver === 'undefined') return;

    const observer = new ResizeObserver((entries) => {
      const entryWidth = entries[0]?.contentRect.width;
      updateTableContainerWidth(entryWidth ?? element.clientWidth);
    });
    observer.observe(element);
    onCleanup(() => observer.disconnect());
  });

  const fallbackViewportWidth = () => {
    const widthAccessor = breakpoint.width as (() => number) | undefined;
    const width = widthAccessor?.();
    return normalizeUnifiedResourceTableLayoutWidth(
      width,
      typeof window !== 'undefined' ? window.innerWidth : undefined,
    );
  };
  const tableLayoutWidth = createMemo(() =>
    normalizeUnifiedResourceTableLayoutWidth(tableContainerWidth(), fallbackViewportWidth()),
  );
  const layoutMode = createMemo(() => getUnifiedResourceTableLayoutMode(tableLayoutWidth()));
  const isMobile = createMemo(() => shouldUseUnifiedResourceTableMobileLayout(tableLayoutWidth()));
  const isVisible = (priority: ColumnPriority) =>
    isUnifiedResourceTableColumnVisible(priority, tableLayoutWidth());
  const isServiceVisible = (priority: ColumnPriority) =>
    isUnifiedResourceServiceColumnVisible(priority, tableLayoutWidth());

  const split = createMemo(() => splitPrimaryAndServiceResources(props.resources));
  const primaryResources = createMemo(() => split().primaryResources);
  const serviceResources = createMemo(() => split().services);
  const primaryResourceIds = createMemo(
    () => new Set(primaryResources().map((resource) => resource.id)),
  );
  const serviceResourceIds = createMemo(
    () => new Set(serviceResources().map((resource) => resource.id)),
  );

  const sortedResources = createMemo(() =>
    sortResources(primaryResources(), sortKey(), sortDirection()),
  );
  const resourceLabelById = createMemo(() => buildResourceLabelById(props.resources));
  const resolveResourceLabel = (resourceId: string): string | undefined =>
    resourceLabelById().get(resourceId);

  const groupedResources = createMemo(() =>
    groupResources(sortedResources(), props.groupingMode ?? 'grouped'),
  );
  const hostTableItems = createMemo<HostTableItem[]>(() =>
    buildHostTableItems(groupedResources(), props.groupingMode),
  );
  const hostRowIndexById = createMemo(() => buildHostRowIndexById(hostTableItems()));
  const hostRevealTargetIndex = createMemo<number | null>(() =>
    getHostRevealTargetIndex(
      hostRowIndexById(),
      props.expandedResourceId,
      props.highlightedResourceId,
      props.revealedResourceId,
    ),
  );

  const hostWindowing = useTableWindowing({
    totalCount: () => hostTableItems().length,
    windowSize: HOST_TABLE_WINDOW_SIZE,
    revealIndex: hostRevealTargetIndex,
  });

  const visibleHostTableItems = createMemo(() =>
    getVisibleHostTableItems(
      hostTableItems(),
      hostWindowing.isWindowed(),
      hostWindowing.startIndex(),
      hostWindowing.endIndex(),
    ),
  );
  const hostSpacerHeights = createMemo(() =>
    getHostSpacerHeights(
      hostTableItems().length,
      hostWindowing.startIndex(),
      hostWindowing.endIndex(),
      hostWindowing.isWindowed(),
      HOST_TABLE_ESTIMATED_ROW_HEIGHT,
    ),
  );
  const viewportSync = useUnifiedResourceTableViewportSync({
    totalCount: () => hostTableItems().length,
    estimatedRowHeight: HOST_TABLE_ESTIMATED_ROW_HEIGHT,
    hostWindowing,
  });

  const sortedPBSResources = createMemo(() => sortServiceResources(serviceResources(), 'pbs'));
  const sortedPMGResources = createMemo(() => sortServiceResources(serviceResources(), 'pmg'));

  const ioScale = createMemo(() => computeIOScale(primaryResources()));

  const handleSort = (key: Exclude<SortKey, 'default'>) => {
    const nextSort = getNextUnifiedResourceTableSortState(sortKey(), sortDirection(), key);
    setSortKey(nextSort.key);
    setSortDirection(nextSort.direction);
  };

  const renderSortIndicator = (key: SortKey) =>
    getUnifiedResourceTableSortIndicator(sortKey(), sortDirection(), key);

  const toggleExpand = (resourceId: string) => {
    props.onExpandedResourceChange(props.expandedResourceId === resourceId ? null : resourceId);
  };

  const tableShellClass = createMemo(() => getUnifiedResourceTableShellClass(layoutMode()));
  const columnPresentations = createMemo(() =>
    getUnifiedResourceTableColumnPresentations(layoutMode()),
  );
  const headerLabels = createMemo(() => getUnifiedResourceTableHeaderLabels(layoutMode()));
  const showHostTable = createMemo(() =>
    shouldShowUnifiedResourceHostTable(primaryResources().length, serviceResources().length),
  );
  const showHostClearAction = createMemo(() =>
    Boolean(
      props.focusedSummaryGroupId ||
      (props.expandedResourceId && primaryResourceIds().has(props.expandedResourceId)),
    ),
  );
  const showServiceClearAction = createMemo(() =>
    Boolean(props.expandedResourceId && serviceResourceIds().has(props.expandedResourceId)),
  );

  return {
    layoutMode,
    isMobile,
    isVisible,
    isServiceVisible,
    handleSort,
    renderSortIndicator,
    resolveResourceLabel,
    setTableRootRef,
    visibleHostTableItems,
    hostTopSpacerHeight: () => hostSpacerHeights().top,
    hostBottomSpacerHeight: () => hostSpacerHeights().bottom,
    hostWindowing,
    sortedPBSResources,
    sortedPMGResources,
    ioScale,
    ...viewportSync,
    tableShellClass,
    headerLabels,
    resourceColumn: () => columnPresentations().resourceColumn,
    serviceResourceColumn: () => columnPresentations().serviceResourceColumn,
    metricColumn: () => columnPresentations().metricColumn,
    ioColumn: () => columnPresentations().ioColumn,
    sourceColumn: () => columnPresentations().sourceColumn,
    serviceSourceColumn: () => columnPresentations().serviceSourceColumn,
    uptimeColumn: () => columnPresentations().uptimeColumn,
    tempColumn: () => columnPresentations().tempColumn,
    showHostTable,
    showHostClearAction,
    showServiceClearAction,
    serviceCountColumn: () => columnPresentations().serviceCountColumn,
    serviceQueueColumn: () => columnPresentations().serviceQueueColumn,
    serviceHealthColumn: () => columnPresentations().serviceHealthColumn,
    serviceActionColumn: () => columnPresentations().serviceActionColumn,
    toggleExpand,
    buildHostSummaryGroupScope: buildInfrastructureSummaryGroupScope,
    getUnifiedSources,
  };
}

export type UnifiedResourceTableState = ReturnType<typeof useUnifiedResourceTableState>;
