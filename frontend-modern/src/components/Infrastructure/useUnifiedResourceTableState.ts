import { createMemo, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
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
  getUnifiedResourceTableColumnStyles,
  getUnifiedResourceTableSortIndicator,
  getUnifiedSources,
  getVisibleHostTableItems,
  HOST_TABLE_ESTIMATED_ROW_HEIGHT,
  HOST_TABLE_WINDOW_SIZE,
  shouldShowUnifiedResourceHostTable,
  sortServiceResources,
  type HostTableItem,
  type UnifiedResourceTableSortKey as SortKey,
} from './unifiedResourceTableStateModel';

export interface UnifiedResourceTableProps {
  resources: Resource[];
  expandedResourceId: string | null;
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
  const { isMobile, isVisible } = useBreakpoint();
  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const split = createMemo(() => splitPrimaryAndServiceResources(props.resources));
  const primaryResources = createMemo(() => split().primaryResources);
  const serviceResources = createMemo(() => split().services);

  const sortedResources = createMemo(() =>
    sortResources(primaryResources(), sortKey(), sortDirection()),
  );
  const resourceLabelById = createMemo(() => buildResourceLabelById(props.resources));
  const resolveResourceLabel = (resourceId: string): string | undefined =>
    resourceLabelById().get(resourceId);

  const groupedResources = createMemo(() => groupResources(sortedResources(), props.groupingMode ?? 'grouped'));
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

  const columnStyles = createMemo(() => getUnifiedResourceTableColumnStyles(isMobile()));
  const showHostTable = createMemo(() =>
    shouldShowUnifiedResourceHostTable(primaryResources().length, serviceResources().length),
  );

  return {
    isMobile,
    isVisible,
    handleSort,
    renderSortIndicator,
    resolveResourceLabel,
    visibleHostTableItems,
    hostTopSpacerHeight: () => hostSpacerHeights().top,
    hostBottomSpacerHeight: () => hostSpacerHeights().bottom,
    hostWindowing,
    sortedPBSResources,
    sortedPMGResources,
    ioScale,
    ...viewportSync,
    resourceColumnStyle: () => columnStyles().resourceColumnStyle,
    metricColumnStyle: () => columnStyles().metricColumnStyle,
    ioColumnStyle: () => columnStyles().ioColumnStyle,
    sourceColumnStyle: () => columnStyles().sourceColumnStyle,
    uptimeColumnStyle: () => columnStyles().uptimeColumnStyle,
    tempColumnStyle: () => columnStyles().tempColumnStyle,
    showHostTable,
    serviceCountColumnStyle: () => columnStyles().serviceCountColumnStyle,
    serviceQueueColumnStyle: () => columnStyles().serviceQueueColumnStyle,
    serviceHealthColumnStyle: () => columnStyles().serviceHealthColumnStyle,
    serviceActionColumnStyle: () => columnStyles().serviceActionColumnStyle,
    toggleExpand,
    buildHostSummaryGroupScope: buildInfrastructureSummaryGroupScope,
    getUnifiedSources,
  };
}

export type UnifiedResourceTableState = ReturnType<typeof useUnifiedResourceTableState>;
