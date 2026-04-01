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

type UnifiedResourceTableColumnStyle = Record<string, string>;

export type UnifiedResourceTableColumnStyles = {
  resourceColumnStyle: UnifiedResourceTableColumnStyle;
  metricColumnStyle: UnifiedResourceTableColumnStyle;
  ioColumnStyle: UnifiedResourceTableColumnStyle;
  sourceColumnStyle: UnifiedResourceTableColumnStyle;
  uptimeColumnStyle: UnifiedResourceTableColumnStyle;
  tempColumnStyle: UnifiedResourceTableColumnStyle;
  serviceCountColumnStyle: UnifiedResourceTableColumnStyle;
  serviceQueueColumnStyle: UnifiedResourceTableColumnStyle;
  serviceHealthColumnStyle: UnifiedResourceTableColumnStyle;
  serviceActionColumnStyle: UnifiedResourceTableColumnStyle;
};

export const getUnifiedResourceTableColumnStyles = (
  isMobile: boolean,
): UnifiedResourceTableColumnStyles => ({
  resourceColumnStyle: isMobile ? { width: '100%', 'min-width': '120px' } : { 'min-width': '220px' },
  metricColumnStyle: isMobile
    ? { width: '70px', 'min-width': '65px' }
    : { 'min-width': '140px', 'max-width': '180px' },
  ioColumnStyle: isMobile
    ? { width: '180px', 'min-width': '180px' }
    : { width: '160px', 'min-width': '160px', 'max-width': '180px' },
  sourceColumnStyle: isMobile
    ? { width: '140px', 'min-width': '140px' }
    : { width: '160px', 'min-width': '160px' },
  uptimeColumnStyle: isMobile
    ? { width: '70px', 'min-width': '70px', 'max-width': '80px' }
    : { width: '80px', 'min-width': '80px', 'max-width': '80px' },
  tempColumnStyle: isMobile
    ? { width: '50px', 'min-width': '50px', 'max-width': '60px' }
    : { width: '60px', 'min-width': '60px', 'max-width': '70px' },
  serviceCountColumnStyle: isMobile
    ? { width: '80px', 'min-width': '80px', 'max-width': '90px' }
    : { width: '110px', 'min-width': '110px', 'max-width': '130px' },
  serviceQueueColumnStyle: isMobile
    ? { width: '88px', 'min-width': '88px', 'max-width': '104px' }
    : { width: '120px', 'min-width': '120px', 'max-width': '140px' },
  serviceHealthColumnStyle: isMobile
    ? { width: '100px', 'min-width': '100px', 'max-width': '120px' }
    : { width: '140px', 'min-width': '140px', 'max-width': '170px' },
  serviceActionColumnStyle: isMobile
    ? { width: '82px', 'min-width': '82px', 'max-width': '96px' }
    : { width: '120px', 'min-width': '120px', 'max-width': '140px' },
});
