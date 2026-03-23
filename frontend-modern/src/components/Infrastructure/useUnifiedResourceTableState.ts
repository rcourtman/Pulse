import { createMemo, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import {
  splitPrimaryAndServiceResources,
  sortResources,
  groupResources,
  computeIOScale,
  type ResourceGroup,
} from '@/components/Infrastructure/infrastructureSelectors';
import { useTableWindowing } from './useTableWindowing';
import { useUnifiedResourceTableViewportSync } from './useUnifiedResourceTableViewportSync';

export interface UnifiedResourceTableProps {
  resources: Resource[];
  expandedResourceId: string | null;
  highlightedResourceId?: string | null;
  hoveredResourceId?: string | null;
  onExpandedResourceChange: (id: string | null) => void;
  onHoverChange?: (id: string | null) => void;
  groupingMode?: 'grouped' | 'flat';
  onDeployCluster?: (clusterId: string, clusterName: string) => void;
}

type SortKey =
  | 'default'
  | 'name'
  | 'uptime'
  | 'cpu'
  | 'memory'
  | 'disk'
  | 'network'
  | 'diskio'
  | 'source'
  | 'temp';

type HostTableHeaderItem = {
  type: 'header';
  group: ResourceGroup;
};

type HostTableResourceItem = {
  type: 'row';
  group: ResourceGroup;
  resource: Resource;
};

export type HostTableItem = HostTableHeaderItem | HostTableResourceItem;

const HOST_TABLE_ESTIMATED_ROW_HEIGHT = 40;
const HOST_TABLE_WINDOW_SIZE = 137;

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
  const resourceLabelById = createMemo(() => {
    const map = new Map<string, string>();
    for (const resource of props.resources) {
      map.set(resource.id, getPreferredResourceDisplayName(resource));
    }
    return map;
  });
  const resolveResourceLabel = (resourceId: string): string | undefined =>
    resourceLabelById().get(resourceId);

  const groupedResources = createMemo<ResourceGroup[]>(() =>
    groupResources(sortedResources(), props.groupingMode ?? 'grouped'),
  );

  const hostTableItems = createMemo<HostTableItem[]>(() => {
    const items: HostTableItem[] = [];
    const showGroupHeaders = props.groupingMode === 'grouped';

    for (const group of groupedResources()) {
      if (showGroupHeaders) {
        items.push({ type: 'header', group });
      }
      for (const resource of group.resources) {
        items.push({ type: 'row', group, resource });
      }
    }

    return items;
  });

  const hostRowIndexById = createMemo(() => {
    const map = new Map<string, number>();
    hostTableItems().forEach((item, index) => {
      if (item.type === 'row') {
        map.set(item.resource.id, index);
      }
    });
    return map;
  });

  const hostRevealTargetIndex = createMemo<number | null>(() => {
    const targetId = props.expandedResourceId ?? props.highlightedResourceId ?? null;
    if (!targetId) return null;
    return hostRowIndexById().get(targetId) ?? null;
  });

  const hostWindowing = useTableWindowing({
    totalCount: () => hostTableItems().length,
    windowSize: HOST_TABLE_WINDOW_SIZE,
    revealIndex: hostRevealTargetIndex,
  });

  const visibleHostTableItems = createMemo(() => {
    if (!hostWindowing.isWindowed()) return hostTableItems();
    return hostTableItems().slice(hostWindowing.startIndex(), hostWindowing.endIndex());
  });

  const hostTopSpacerHeight = createMemo(() =>
    hostWindowing.isWindowed() ? hostWindowing.startIndex() * HOST_TABLE_ESTIMATED_ROW_HEIGHT : 0,
  );

  const hostBottomSpacerHeight = createMemo(() =>
    hostWindowing.isWindowed()
      ? Math.max(
          0,
          (hostTableItems().length - hostWindowing.endIndex()) * HOST_TABLE_ESTIMATED_ROW_HEIGHT,
        )
      : 0,
  );
  const viewportSync = useUnifiedResourceTableViewportSync({
    expandedResourceId: () => props.expandedResourceId,
    totalCount: () => hostTableItems().length,
    estimatedRowHeight: HOST_TABLE_ESTIMATED_ROW_HEIGHT,
    hostWindowing,
  });

  const sortedPBSResources = createMemo(() =>
    sortResources(
      serviceResources().filter((resource) => resource.type === 'pbs'),
      'default',
      'asc',
    ),
  );
  const sortedPMGResources = createMemo(() =>
    sortResources(
      serviceResources().filter((resource) => resource.type === 'pmg'),
      'default',
      'asc',
    ),
  );

  const ioScale = createMemo(() => computeIOScale(primaryResources()));

  const handleSort = (key: Exclude<SortKey, 'default'>) => {
    if (sortKey() === key) {
      if (sortDirection() === 'asc') {
        setSortDirection('desc');
      } else {
        setSortKey('default');
        setSortDirection('asc');
      }
    } else {
      setSortKey(key);
      setSortDirection(key === 'name' || key === 'source' ? 'asc' : 'desc');
    }
  };

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const toggleExpand = (resourceId: string) => {
    props.onExpandedResourceChange(props.expandedResourceId === resourceId ? null : resourceId);
  };

  const resourceColumnStyle = createMemo(() =>
    isMobile() ? { width: '100%', 'min-width': '120px' } : { 'min-width': '220px' },
  );
  const metricColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '70px', 'min-width': '65px' }
      : { 'min-width': '140px', 'max-width': '180px' },
  );
  const ioColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '180px', 'min-width': '180px' }
      : { width: '160px', 'min-width': '160px', 'max-width': '180px' },
  );
  const sourceColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '140px', 'min-width': '140px' }
      : { width: '160px', 'min-width': '160px' },
  );
  const uptimeColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '70px', 'min-width': '70px', 'max-width': '80px' }
      : { width: '80px', 'min-width': '80px', 'max-width': '80px' },
  );
  const tempColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '50px', 'min-width': '50px', 'max-width': '60px' }
      : { width: '60px', 'min-width': '60px', 'max-width': '70px' },
  );

  const showHostTable = createMemo(
    () => primaryResources().length > 0 || serviceResources().length === 0,
  );
  const serviceCountColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '80px', 'min-width': '80px', 'max-width': '90px' }
      : { width: '110px', 'min-width': '110px', 'max-width': '130px' },
  );
  const serviceQueueColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '88px', 'min-width': '88px', 'max-width': '104px' }
      : { width: '120px', 'min-width': '120px', 'max-width': '140px' },
  );
  const serviceHealthColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '100px', 'min-width': '100px', 'max-width': '120px' }
      : { width: '140px', 'min-width': '140px', 'max-width': '170px' },
  );
  const serviceActionColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '82px', 'min-width': '82px', 'max-width': '96px' }
      : { width: '120px', 'min-width': '120px', 'max-width': '140px' },
  );

  const getUnifiedSources = (resource: Resource): string[] => {
    const platformData = resource.platformData as { sources?: string[] } | undefined;
    return platformData?.sources ?? [];
  };

  return {
    isMobile,
    isVisible,
    handleSort,
    renderSortIndicator,
    resolveResourceLabel,
    visibleHostTableItems,
    hostTopSpacerHeight,
    hostBottomSpacerHeight,
    hostWindowing,
    sortedPBSResources,
    sortedPMGResources,
    ioScale,
    ...viewportSync,
    resourceColumnStyle,
    metricColumnStyle,
    ioColumnStyle,
    sourceColumnStyle,
    uptimeColumnStyle,
    tempColumnStyle,
    showHostTable,
    serviceCountColumnStyle,
    serviceQueueColumnStyle,
    serviceHealthColumnStyle,
    serviceActionColumnStyle,
    toggleExpand,
    getUnifiedSources,
  };
}

export type UnifiedResourceTableState = ReturnType<typeof useUnifiedResourceTableState>;
