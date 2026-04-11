import type { Resource } from '@/types/resource';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import {
  sortResources,
  type ResourceGroup,
} from '@/components/Infrastructure/infrastructureSelectors';

export type UnifiedResourceTableSortKey =
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

export type UnifiedResourceTableSortDirection = 'asc' | 'desc';

export type HostTableHeaderItem = {
  type: 'header';
  group: ResourceGroup;
};

export type HostTableResourceItem = {
  type: 'row';
  group: ResourceGroup;
  resource: Resource;
};

export type HostTableItem = HostTableHeaderItem | HostTableResourceItem;

export const HOST_TABLE_ESTIMATED_ROW_HEIGHT = 40;
export const HOST_TABLE_WINDOW_SIZE = 137;

export const buildResourceLabelById = (resources: Resource[]): Map<string, string> => {
  const map = new Map<string, string>();
  for (const resource of resources) {
    map.set(resource.id, getPreferredInfrastructureDisplayName(resource));
  }
  return map;
};

export const buildHostTableItems = (
  groupedResources: ResourceGroup[],
  groupingMode: 'grouped' | 'flat' | undefined,
): HostTableItem[] => {
  const items: HostTableItem[] = [];
  const showGroupHeaders = groupingMode === 'grouped';

  for (const group of groupedResources) {
    if (showGroupHeaders) {
      items.push({ type: 'header', group });
    }
    for (const resource of group.resources) {
      items.push({ type: 'row', group, resource });
    }
  }

  return items;
};

export const buildHostRowIndexById = (items: HostTableItem[]): Map<string, number> => {
  const map = new Map<string, number>();
  items.forEach((item, index) => {
    if (item.type === 'row') {
      map.set(item.resource.id, index);
    }
  });
  return map;
};

export const getHostRevealTargetIndex = (
  rowIndexById: Map<string, number>,
  expandedResourceId: string | null,
  highlightedResourceId: string | null | undefined,
  revealedResourceId?: string | null,
): number | null => {
  const targetId = expandedResourceId ?? revealedResourceId ?? highlightedResourceId ?? null;
  if (!targetId) return null;
  return rowIndexById.get(targetId) ?? null;
};

export const getVisibleHostTableItems = (
  items: HostTableItem[],
  isWindowed: boolean,
  startIndex: number,
  endIndex: number,
): HostTableItem[] => {
  if (!isWindowed) return items;
  return items.slice(startIndex, endIndex);
};

export const getHostSpacerHeights = (
  totalCount: number,
  startIndex: number,
  endIndex: number,
  isWindowed: boolean,
  estimatedRowHeight: number = HOST_TABLE_ESTIMATED_ROW_HEIGHT,
): { top: number; bottom: number } => {
  if (!isWindowed) {
    return { top: 0, bottom: 0 };
  }

  return {
    top: startIndex * estimatedRowHeight,
    bottom: Math.max(0, (totalCount - endIndex) * estimatedRowHeight),
  };
};

export const getNextUnifiedResourceTableSortState = (
  currentKey: UnifiedResourceTableSortKey,
  currentDirection: UnifiedResourceTableSortDirection,
  nextKey: Exclude<UnifiedResourceTableSortKey, 'default'>,
): {
  key: UnifiedResourceTableSortKey;
  direction: UnifiedResourceTableSortDirection;
} => {
  if (currentKey === nextKey) {
    if (currentDirection === 'asc') {
      return { key: nextKey, direction: 'desc' };
    }
    return { key: 'default', direction: 'asc' };
  }

  return {
    key: nextKey,
    direction: nextKey === 'name' || nextKey === 'source' ? 'asc' : 'desc',
  };
};

export const getUnifiedResourceTableSortIndicator = (
  activeKey: UnifiedResourceTableSortKey,
  activeDirection: UnifiedResourceTableSortDirection,
  key: UnifiedResourceTableSortKey,
): '▲' | '▼' | null => {
  if (activeKey !== key) return null;
  return activeDirection === 'asc' ? '▲' : '▼';
};

export const sortServiceResources = (
  services: Resource[],
  type: 'pbs' | 'pmg',
): Resource[] =>
  sortResources(
    services.filter((resource) => resource.type === type),
    'default',
    'asc',
  );

export const shouldShowUnifiedResourceHostTable = (
  primaryResourceCount: number,
  serviceResourceCount: number,
): boolean => primaryResourceCount > 0 || serviceResourceCount === 0;

export const getUnifiedSources = (resource: Resource): string[] => {
  const platformData = resource.platformData as { sources?: string[] } | undefined;
  return platformData?.sources ?? [];
};

export interface UnifiedResourceTableColumnPresentation {
  className: string;
  width?: string | number;
}

export type UnifiedResourceTableColumnPresentations = {
  resourceColumn: UnifiedResourceTableColumnPresentation;
  metricColumn: UnifiedResourceTableColumnPresentation;
  ioColumn: UnifiedResourceTableColumnPresentation;
  sourceColumn: UnifiedResourceTableColumnPresentation;
  uptimeColumn: UnifiedResourceTableColumnPresentation;
  tempColumn: UnifiedResourceTableColumnPresentation;
  serviceCountColumn: UnifiedResourceTableColumnPresentation;
  serviceQueueColumn: UnifiedResourceTableColumnPresentation;
  serviceHealthColumn: UnifiedResourceTableColumnPresentation;
  serviceActionColumn: UnifiedResourceTableColumnPresentation;
};

const buildUnifiedResourceTableColumnPresentation = (
  className: string,
  width?: string | number,
): UnifiedResourceTableColumnPresentation => ({
  className,
  width,
});

export const getUnifiedResourceTableShellClass = (isMobile: boolean): string =>
  `table-fixed ${isMobile ? 'min-w-full' : 'min-w-[max-content]'}`;

export const getUnifiedResourceTableColumnPresentations = (
  isMobile: boolean,
): UnifiedResourceTableColumnPresentations => ({
  resourceColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('w-full min-w-[120px]', '100%')
    : buildUnifiedResourceTableColumnPresentation('min-w-[220px]'),
  metricColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[65px]', 70)
    : buildUnifiedResourceTableColumnPresentation('min-w-[140px] max-w-[180px]'),
  ioColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[180px]', 180)
    : buildUnifiedResourceTableColumnPresentation('min-w-[160px] max-w-[180px]', 160),
  sourceColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[140px]', 140)
    : buildUnifiedResourceTableColumnPresentation('min-w-[160px]', 160),
  uptimeColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[70px] max-w-[80px]', 70)
    : buildUnifiedResourceTableColumnPresentation('min-w-[80px] max-w-[80px]', 80),
  tempColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[50px] max-w-[60px]', 50)
    : buildUnifiedResourceTableColumnPresentation('min-w-[60px] max-w-[70px]', 60),
  serviceCountColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[80px] max-w-[90px]', 80)
    : buildUnifiedResourceTableColumnPresentation('min-w-[110px] max-w-[130px]', 110),
  serviceQueueColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[88px] max-w-[104px]', 88)
    : buildUnifiedResourceTableColumnPresentation('min-w-[120px] max-w-[140px]', 120),
  serviceHealthColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[100px] max-w-[120px]', 100)
    : buildUnifiedResourceTableColumnPresentation('min-w-[140px] max-w-[170px]', 140),
  serviceActionColumn: isMobile
    ? buildUnifiedResourceTableColumnPresentation('min-w-[82px] max-w-[96px]', 82)
    : buildUnifiedResourceTableColumnPresentation('min-w-[120px] max-w-[140px]', 120),
});
