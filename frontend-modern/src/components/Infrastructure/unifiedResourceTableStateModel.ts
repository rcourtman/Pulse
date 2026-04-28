import type { Resource } from '@/types/resource';
import type { ColumnPriority } from '@/hooks/useBreakpoint';
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
export const UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH = 1024;
export const UNIFIED_RESOURCE_TABLE_MOBILE_LAYOUT_WIDTH = 700;
export const UNIFIED_RESOURCE_TABLE_TABLET_DISK_IO_LAYOUT_WIDTH = 800;
export const UNIFIED_RESOURCE_TABLE_COMPACT_LAYOUT_WIDTH = 900;
export const UNIFIED_RESOURCE_TABLE_WIDE_LAYOUT_WIDTH = 1160;
export const UNIFIED_RESOURCE_TABLE_COLUMN_BREAKPOINTS: Record<ColumnPriority, number> = {
  essential: 0,
  primary: 640,
  secondary: UNIFIED_RESOURCE_TABLE_MOBILE_LAYOUT_WIDTH,
  supplementary: UNIFIED_RESOURCE_TABLE_COMPACT_LAYOUT_WIDTH,
  detailed: UNIFIED_RESOURCE_TABLE_WIDE_LAYOUT_WIDTH,
};
export const UNIFIED_RESOURCE_SERVICE_COLUMN_BREAKPOINTS: Record<ColumnPriority, number> = {
  essential: 0,
  primary: 500,
  secondary: 580,
  supplementary: 640,
  detailed: 900,
};

export type UnifiedResourceTableLayoutMode = 'mobile' | 'tablet' | 'compact' | 'wide';

export const normalizeUnifiedResourceTableLayoutWidth = (
  width: number | null | undefined,
  fallback: number = UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH,
): number => {
  if (typeof width === 'number' && Number.isFinite(width) && width > 0) {
    return Math.round(width);
  }
  if (Number.isFinite(fallback) && fallback > 0) {
    return Math.round(fallback);
  }
  return UNIFIED_RESOURCE_TABLE_DEFAULT_LAYOUT_WIDTH;
};

export const getUnifiedResourceTableLayoutMode = (
  layoutWidth: number,
): UnifiedResourceTableLayoutMode => {
  const width = normalizeUnifiedResourceTableLayoutWidth(layoutWidth);

  if (width < UNIFIED_RESOURCE_TABLE_MOBILE_LAYOUT_WIDTH) return 'mobile';
  if (width < UNIFIED_RESOURCE_TABLE_COMPACT_LAYOUT_WIDTH) return 'tablet';
  if (width < UNIFIED_RESOURCE_TABLE_WIDE_LAYOUT_WIDTH) return 'compact';
  return 'wide';
};

export const shouldUseUnifiedResourceTableMobileLayout = (layoutWidth: number): boolean =>
  getUnifiedResourceTableLayoutMode(layoutWidth) === 'mobile';

export const isUnifiedResourceTableColumnVisible = (
  priority: ColumnPriority,
  layoutWidth: number,
): boolean =>
  normalizeUnifiedResourceTableLayoutWidth(layoutWidth) >=
  UNIFIED_RESOURCE_TABLE_COLUMN_BREAKPOINTS[priority];

export const isUnifiedResourceServiceColumnVisible = (
  priority: ColumnPriority,
  layoutWidth: number,
): boolean =>
  normalizeUnifiedResourceTableLayoutWidth(layoutWidth) >=
  UNIFIED_RESOURCE_SERVICE_COLUMN_BREAKPOINTS[priority];

export const isUnifiedResourceHostDiskIoVisible = (layoutWidth: number): boolean =>
  normalizeUnifiedResourceTableLayoutWidth(layoutWidth) >=
  UNIFIED_RESOURCE_TABLE_TABLET_DISK_IO_LAYOUT_WIDTH;

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

export const sortServiceResources = (services: Resource[], type: 'pbs' | 'pmg'): Resource[] =>
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
  serviceResourceColumn: UnifiedResourceTableColumnPresentation;
  metricColumn: UnifiedResourceTableColumnPresentation;
  ioColumn: UnifiedResourceTableColumnPresentation;
  sourceColumn: UnifiedResourceTableColumnPresentation;
  serviceSourceColumn: UnifiedResourceTableColumnPresentation;
  uptimeColumn: UnifiedResourceTableColumnPresentation;
  tempColumn: UnifiedResourceTableColumnPresentation;
  serviceCountColumn: UnifiedResourceTableColumnPresentation;
  serviceQueueColumn: UnifiedResourceTableColumnPresentation;
  serviceHealthColumn: UnifiedResourceTableColumnPresentation;
  serviceActionColumn: UnifiedResourceTableColumnPresentation;
};

export type UnifiedResourceTableHeaderLabels = {
  resource: string;
  cpu: string;
  memory: string;
  disk: string;
  network: string;
  diskIo: string;
  source: string;
  uptime: string;
  temp: string;
  datastores: string;
  activity: string;
  queue: string;
  deferred: string;
  hold: string;
  nodes: string;
  health: string;
  action: string;
};

const buildUnifiedResourceTableColumnPresentation = (
  className: string,
  width?: string | number,
): UnifiedResourceTableColumnPresentation => ({
  className,
  width,
});

export const getUnifiedResourceTableShellClass = (
  layoutMode: UnifiedResourceTableLayoutMode,
): string => `table-fixed min-w-full${layoutMode === 'wide' ? '' : ' text-[11px] sm:text-xs'}`;

export const getUnifiedResourceTableHeaderLabels = (
  layoutMode: UnifiedResourceTableLayoutMode,
): UnifiedResourceTableHeaderLabels => {
  if (layoutMode === 'wide') {
    return {
      resource: 'Resource',
      cpu: 'CPU',
      memory: 'Memory',
      disk: 'Disk',
      network: 'Net I/O',
      diskIo: 'Disk I/O',
      source: 'Platform',
      uptime: 'Uptime',
      temp: 'Temp',
      datastores: 'Datastores',
      activity: 'Activity',
      queue: 'Queue',
      deferred: 'Deferred',
      hold: 'Hold',
      nodes: 'Nodes',
      health: 'Health',
      action: 'Action',
    };
  }

  if (layoutMode === 'compact') {
    return {
      resource: 'Resource',
      cpu: 'CPU',
      memory: 'Mem',
      disk: 'Disk',
      network: 'Net I/O',
      diskIo: 'Disk I/O',
      source: 'Platform',
      uptime: 'Up',
      temp: 'Temp',
      datastores: 'Stores',
      activity: 'Activity',
      queue: 'Queue',
      deferred: 'Def',
      hold: 'Hold',
      nodes: 'Nodes',
      health: 'Health',
      action: 'Open',
    };
  }

  return {
    resource: 'Resource',
    cpu: 'CPU',
    memory: 'Mem',
    disk: 'Disk',
    network: 'Net',
    diskIo: 'I/O',
    source: 'Plat',
    uptime: 'Up',
    temp: 'C',
    datastores: 'Store',
    activity: 'Jobs',
    queue: 'Queue',
    deferred: 'Def',
    hold: 'Hold',
    nodes: 'Nodes',
    health: 'Health',
    action: 'Open',
  };
};

export const getUnifiedResourceTableColumnPresentations = (
  layoutMode: UnifiedResourceTableLayoutMode,
  layoutWidth?: number,
): UnifiedResourceTableColumnPresentations => {
  if (layoutMode === 'mobile') {
    // Mobile widths are percentages so visible columns fill the viewport
    // without horizontal scroll. Hidden columns keep placeholder widths that
    // never render.
    return {
      resourceColumn: buildUnifiedResourceTableColumnPresentation('', '40%'),
      serviceResourceColumn: buildUnifiedResourceTableColumnPresentation('', '28%'),
      metricColumn: buildUnifiedResourceTableColumnPresentation('', '20%'),
      ioColumn: buildUnifiedResourceTableColumnPresentation('', '20%'),
      sourceColumn: buildUnifiedResourceTableColumnPresentation('', '20%'),
      serviceSourceColumn: buildUnifiedResourceTableColumnPresentation('', '10%'),
      uptimeColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
      tempColumn: buildUnifiedResourceTableColumnPresentation('', '15%'),
      serviceCountColumn: buildUnifiedResourceTableColumnPresentation('', '11%'),
      serviceQueueColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
      serviceHealthColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
      serviceActionColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
    };
  }

  if (layoutMode === 'tablet') {
    if (typeof layoutWidth === 'number' && isUnifiedResourceHostDiskIoVisible(layoutWidth)) {
      return {
        resourceColumn: buildUnifiedResourceTableColumnPresentation('', '28%'),
        serviceResourceColumn: buildUnifiedResourceTableColumnPresentation('', '24%'),
        metricColumn: buildUnifiedResourceTableColumnPresentation('', '12%'),
        ioColumn: buildUnifiedResourceTableColumnPresentation('', '12%'),
        sourceColumn: buildUnifiedResourceTableColumnPresentation('', '12%'),
        serviceSourceColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
        uptimeColumn: buildUnifiedResourceTableColumnPresentation('', '7%'),
        tempColumn: buildUnifiedResourceTableColumnPresentation('', '6%'),
        serviceCountColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
        serviceQueueColumn: buildUnifiedResourceTableColumnPresentation('', '7.5%'),
        serviceHealthColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
        serviceActionColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
      };
    }

    return {
      resourceColumn: buildUnifiedResourceTableColumnPresentation('', '34%'),
      serviceResourceColumn: buildUnifiedResourceTableColumnPresentation('', '24%'),
      metricColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
      ioColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
      sourceColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
      serviceSourceColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
      uptimeColumn: buildUnifiedResourceTableColumnPresentation('', '7%'),
      tempColumn: buildUnifiedResourceTableColumnPresentation('', '6%'),
      serviceCountColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
      serviceQueueColumn: buildUnifiedResourceTableColumnPresentation('', '7.5%'),
      serviceHealthColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
      serviceActionColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
    };
  }

  if (layoutMode === 'compact') {
    return {
      resourceColumn: buildUnifiedResourceTableColumnPresentation('', '16%'),
      serviceResourceColumn: buildUnifiedResourceTableColumnPresentation('', '18%'),
      metricColumn: buildUnifiedResourceTableColumnPresentation('', '10%'),
      ioColumn: buildUnifiedResourceTableColumnPresentation('', '12.5%'),
      sourceColumn: buildUnifiedResourceTableColumnPresentation('', '13%'),
      serviceSourceColumn: buildUnifiedResourceTableColumnPresentation('', '9.5%'),
      uptimeColumn: buildUnifiedResourceTableColumnPresentation('', '7.5%'),
      tempColumn: buildUnifiedResourceTableColumnPresentation('', '6.5%'),
      serviceCountColumn: buildUnifiedResourceTableColumnPresentation('', '9.5%'),
      serviceQueueColumn: buildUnifiedResourceTableColumnPresentation('', '9.5%'),
      serviceHealthColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
      serviceActionColumn: buildUnifiedResourceTableColumnPresentation('', '12%'),
    };
  }

  return {
    resourceColumn: buildUnifiedResourceTableColumnPresentation('', '18%'),
    serviceResourceColumn: buildUnifiedResourceTableColumnPresentation('', '18%'),
    metricColumn: buildUnifiedResourceTableColumnPresentation('', '10.5%'),
    ioColumn: buildUnifiedResourceTableColumnPresentation('', '12.5%'),
    sourceColumn: buildUnifiedResourceTableColumnPresentation('', '10%'),
    serviceSourceColumn: buildUnifiedResourceTableColumnPresentation('', '10%'),
    uptimeColumn: buildUnifiedResourceTableColumnPresentation('', '8%'),
    tempColumn: buildUnifiedResourceTableColumnPresentation('', '7.5%'),
    serviceCountColumn: buildUnifiedResourceTableColumnPresentation('', '8.5%'),
    serviceQueueColumn: buildUnifiedResourceTableColumnPresentation('', '8.5%'),
    serviceHealthColumn: buildUnifiedResourceTableColumnPresentation('', '14%'),
    serviceActionColumn: buildUnifiedResourceTableColumnPresentation('', '16%'),
  };
};
