import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { Resource } from '@/types/resource';
import { getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent } from '@/types/resource';
import { formatBytes, formatUptime, formatSpeed } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { buildMetricKey } from '@/utils/metricsKeys';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { getHostStatusIndicator } from '@/utils/status';
import {
  splitHostAndServiceResources,
  sortResources,
  groupResources,
  computeIOScale,
  type IODistributionStats,
  type ResourceGroup,
} from '@/components/Infrastructure/infrastructureSelectors';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { buildWorkloadsHref } from './workloadsLink';
import { buildServiceDetailLinks } from './serviceDetailLinks';
import { getPlatformBadge, getSourceBadge, getUnifiedSourceBadges } from './resourceBadges';
import { useTableWindowing } from './useTableWindowing';

interface UnifiedResourceTableProps {
  resources: Resource[];
  expandedResourceId: string | null;
  highlightedResourceId?: string | null;
  hoveredResourceId?: string | null;
  onExpandedResourceChange: (id: string | null) => void;
  onHoverChange?: (id: string | null) => void;
  groupingMode?: 'grouped' | 'flat';
}

type SortKey = 'default' | 'name' | 'uptime' | 'cpu' | 'memory' | 'disk' | 'network' | 'diskio' | 'source' | 'temp';

type PBSServiceData = {
  datastoreCount?: number;
  backupJobCount?: number;
  syncJobCount?: number;
  verifyJobCount?: number;
  pruneJobCount?: number;
  garbageJobCount?: number;
  connectionHealth?: string;
};

type PMGServiceData = {
  nodeCount?: number;
  queueTotal?: number;
  queueDeferred?: number;
  queueHold?: number;
  connectionHealth?: string;
};

type ServiceSummaryTone = 'ok' | 'warning' | 'muted';

type PBSTableRow = {
  datastores: number | null;
  jobs: number | null;
  health: string | null;
  tone: ServiceSummaryTone;
};

type PMGTableRow = {
  queue: number | null;
  deferred: number | null;
  hold: number | null;
  nodes: number | null;
  health: string | null;
  tone: ServiceSummaryTone;
};

type HostTableHeaderItem = {
  type: 'header';
  group: ResourceGroup;
};

type HostTableResourceItem = {
  type: 'row';
  group: ResourceGroup;
  resource: Resource;
};

type HostTableItem = HostTableHeaderItem | HostTableResourceItem;

const HOST_TABLE_ESTIMATED_ROW_HEIGHT = 40;
const HOST_TABLE_WINDOW_SIZE = 137;

const isResourceOnline = (resource: Resource) => {
  const status = resource.status?.toLowerCase();
  return status !== 'offline' && status !== 'stopped';
};

const hasAlternateName = (resource: Resource) => {
  if (!resource.displayName || !resource.name) return false;
  const display = resource.displayName.trim().toLowerCase();
  const name = resource.name.trim().toLowerCase();
  return display !== name;
};

const summarizeServiceHealthTone = (value?: string): ServiceSummaryTone => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return 'muted';
  if (['offline', 'down', 'disconnected', 'error', 'failed'].includes(normalized)) return 'warning';
  if (['degraded', 'warning', 'stale'].includes(normalized)) return 'warning';
  if (['online', 'running', 'healthy', 'connected', 'ok'].includes(normalized)) return 'ok';
  return 'muted';
};

const getPBSTableRow = (resource: Resource): PBSTableRow | null => {
  if (resource.type !== 'pbs') return null;
  const platformData = resource.platformData as { pbs?: PBSServiceData; pmg?: PMGServiceData } | undefined;
  const pbs = platformData?.pbs;
  const totalJobs =
    (pbs?.backupJobCount || 0) +
    (pbs?.syncJobCount || 0) +
    (pbs?.verifyJobCount || 0) +
    (pbs?.pruneJobCount || 0) +
    (pbs?.garbageJobCount || 0);
  const health = pbs?.connectionHealth?.trim() || null;

  return {
    datastores: (pbs?.datastoreCount || 0) > 0 ? (pbs?.datastoreCount || 0) : null,
    jobs: totalJobs > 0 ? totalJobs : null,
    health,
    tone: summarizeServiceHealthTone(health || undefined),
  };
};

const getPMGTableRow = (resource: Resource): PMGTableRow | null => {
  if (resource.type !== 'pmg') return null;
  const platformData = resource.platformData as { pbs?: PBSServiceData; pmg?: PMGServiceData } | undefined;
  const pmg = platformData?.pmg;
  const health = pmg?.connectionHealth?.trim() || null;
  const backlog = (pmg?.queueDeferred || 0) + (pmg?.queueHold || 0);

  return {
    queue: (pmg?.queueTotal || 0) > 0 ? (pmg?.queueTotal || 0) : null,
    deferred: (pmg?.queueDeferred || 0) > 0 ? (pmg?.queueDeferred || 0) : null,
    hold: (pmg?.queueHold || 0) > 0 ? (pmg?.queueHold || 0) : null,
    nodes: (pmg?.nodeCount || 0) > 0 ? (pmg?.nodeCount || 0) : null,
    health,
    tone: backlog > 0 ? 'warning' : summarizeServiceHealthTone(health || undefined),
  };
};

interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

const getOutlierEmphasis = (value: number, stats: IODistributionStats): IOEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { className: 'text-gray-400 dark:text-gray-500', showOutlierHint: false };
  }

  // For tiny sets, avoid aggressive highlighting.
  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { className: 'text-gray-800 dark:text-gray-100 font-medium', showOutlierHint: true };
    }
    return { className: 'text-gray-500 dark:text-gray-400', showOutlierHint: false };
  }

  // Robust outlier score: only values meaningfully far from the cluster brighten.
  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { className: 'text-gray-900 dark:text-gray-50 font-semibold', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { className: 'text-gray-800 dark:text-gray-100 font-medium', showOutlierHint: true };
    }
    return { className: 'text-gray-500 dark:text-gray-400', showOutlierHint: false };
  }

  // Fallback when values are too uniform for MAD to separate:
  // only near-peak values should get emphasis.
  if (value >= stats.p99) return { className: 'text-gray-900 dark:text-gray-50 font-semibold', showOutlierHint: true };
  if (value >= stats.p97) return { className: 'text-gray-800 dark:text-gray-100 font-medium', showOutlierHint: true };
  if (value > 0) return { className: 'text-gray-500 dark:text-gray-400', showOutlierHint: false };
  return { className: 'text-gray-400 dark:text-gray-500', showOutlierHint: false };
};

export const UnifiedResourceTable: Component<UnifiedResourceTableProps> = (props) => {
  const { isMobile, isVisible } = useBreakpoint();
  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const setExpandedResourceId = (id: string | null) => props.onExpandedResourceChange(id);
  const rowRefs = new Map<string, HTMLTableRowElement>();
  const [hostBodyRef, setHostBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

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

  const split = createMemo(() => splitHostAndServiceResources(props.resources));
  const hostResources = createMemo(() => split().hosts);
  const serviceResources = createMemo(() => split().services);

  const sortedResources = createMemo(() =>
    sortResources(hostResources(), sortKey(), sortDirection()),
  );

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

  const syncHostWindowToViewport = () => {
    if (!hostWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = hostBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollTop = Math.max(0, -rect.top);
    hostWindowing.onScroll(scrollTop, window.innerHeight, HOST_TABLE_ESTIMATED_ROW_HEIGHT);
  };

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

  const ioScale = createMemo(() => computeIOScale(hostResources()));

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const toggleExpand = (resourceId: string) => {
    setExpandedResourceId(props.expandedResourceId === resourceId ? null : resourceId);
  };

  createEffect(() => {
    const selectedId = props.expandedResourceId;
    if (!selectedId) return;
    hostWindowing.startIndex();
    hostWindowing.endIndex();
    const row = rowRefs.get(selectedId);
    if (row) {
      row.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }
  });

  createEffect(() => {
    if (typeof window === 'undefined') return;
    hostTableItems().length;
    if (!hostWindowing.isWindowed()) return;
    if (!hostBodyRef()) return;

    const handleViewportChange = () => {
      syncHostWindowToViewport();
    };

    handleViewportChange();
    window.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      window.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });

  const thClassBase = 'px-1.5 sm:px-2 py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap';
  const thClass = `${thClassBase} text-center`;
  const tdClass = 'px-1.5 sm:px-2 py-1 align-middle';
  const resourceColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '220px', 'min-width': '180px' }
      : { 'min-width': '220px' }
  );
  const metricColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '80px', 'min-width': '80px' }
      : { 'min-width': '140px', 'max-width': '180px' }
  );
  const ioColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '180px', 'min-width': '180px' }
      : { width: '160px', 'min-width': '160px', 'max-width': '180px' }
  );
  const sourceColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '140px', 'min-width': '140px' }
      : { width: '160px', 'min-width': '160px' }
  );
  const uptimeColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '70px', 'min-width': '70px', 'max-width': '80px' }
      : { width: '80px', 'min-width': '80px', 'max-width': '80px' }
  );
  const tempColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '50px', 'min-width': '50px', 'max-width': '60px' }
      : { width: '60px', 'min-width': '60px', 'max-width': '70px' }
  );

  const getUnifiedSources = (resource: Resource): string[] => {
    const platformData = resource.platformData as { sources?: string[] } | undefined;
    return platformData?.sources ?? [];
  };

  const showHostTable = createMemo(() =>
    hostResources().length > 0 || serviceResources().length === 0,
  );
  const staticThClass = 'px-1.5 sm:px-2 py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap text-center';
  const serviceCountColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '80px', 'min-width': '80px', 'max-width': '90px' }
      : { width: '110px', 'min-width': '110px', 'max-width': '130px' }
  );
  const serviceQueueColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '88px', 'min-width': '88px', 'max-width': '104px' }
      : { width: '120px', 'min-width': '120px', 'max-width': '140px' }
  );
  const serviceHealthColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '100px', 'min-width': '100px', 'max-width': '120px' }
      : { width: '140px', 'min-width': '140px', 'max-width': '170px' }
  );
  const serviceActionColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '82px', 'min-width': '82px', 'max-width': '96px' }
      : { width: '120px', 'min-width': '120px', 'max-width': '140px' }
  );

  return (
    <div class="space-y-4">
      <Show when={showHostTable()}>
        <Card padding="none" tone="glass" class="mb-0 overflow-hidden">
          <div class="border-b border-gray-200/70 bg-gray-50/70 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-600 dark:border-gray-700 dark:bg-gray-900/40 dark:text-slate-300">
            Host Infrastructure
          </div>
	      <div
	        class="overflow-x-auto"
	        style={{ '-webkit-overflow-scrolling': 'touch' }}
	      >
	        <table class="w-full border-collapse whitespace-nowrap" style={{ 'table-layout': 'fixed', 'min-width': isMobile() ? '1080px' : '600px' }}>
	          <thead>
	            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
	              <th class={`${thClassBase} text-left pl-2 sm:pl-3`} style={resourceColumnStyle()} onClick={() => handleSort('name')}>
	                Resource {renderSortIndicator('name')}
	              </th>
	              <th class={thClass} style={metricColumnStyle()} onClick={() => handleSort('cpu')}>
	                CPU {renderSortIndicator('cpu')}
	              </th>
	              <th class={thClass} style={metricColumnStyle()} onClick={() => handleSort('memory')}>
	                Memory {renderSortIndicator('memory')}
	              </th>
	              <th class={thClass} style={metricColumnStyle()} onClick={() => handleSort('disk')}>
	                Disk {renderSortIndicator('disk')}
	              </th>
	              <th class={thClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={ioColumnStyle()} onClick={() => handleSort('network')}>
	                Net I/O {renderSortIndicator('network')}
	              </th>
	              <th class={thClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={ioColumnStyle()} onClick={() => handleSort('diskio')}>
	                Disk I/O {renderSortIndicator('diskio')}
	              </th>
	              <th class={thClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={sourceColumnStyle()} onClick={() => handleSort('source')}>
	                Source {renderSortIndicator('source')}
	              </th>
	              <th class={thClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={uptimeColumnStyle()} onClick={() => handleSort('uptime')}>
	                Uptime {renderSortIndicator('uptime')}
	              </th>
	              <th class={thClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={tempColumnStyle()} onClick={() => handleSort('temp')}>
	                Temp {renderSortIndicator('temp')}
	              </th>
	            </tr>
	          </thead>
	          <tbody ref={setHostBodyRef} class="bg-white dark:bg-gray-800">
            <Show when={hostWindowing.isWindowed() && hostTopSpacerHeight() > 0}>
              <tr aria-hidden="true">
                <td colspan={9} style={{ height: `${hostTopSpacerHeight()}px`, padding: '0', border: '0' }} />
              </tr>
            </Show>

            <For each={visibleHostTableItems()}>
              {(item) => {
                if (item.type === 'header') {
                  const group = item.group;
                  return (
                    <tr class="bg-gray-50 dark:bg-gray-900/40">
                      <td
                        colspan={9}
                        class="py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-100"
                      >
                        <div class="flex items-center gap-2">
                          <Show
                            when={group.cluster}
                            fallback={
                              <span class="text-slate-500 dark:text-slate-400">Standalone</span>
                            }
                          >
                            <span>{group.cluster}</span>
                            <span class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                              Cluster
                            </span>
                          </Show>
                          <span class="text-[10px] text-slate-400 dark:text-slate-500 font-normal">
                            {group.resources.length} {group.resources.length === 1 ? 'resource' : 'resources'}
                          </span>
                        </div>
                      </td>
                    </tr>
                  );
                }

                const resource = item.resource;
                const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                const isHighlighted = createMemo(() => props.highlightedResourceId === resource.id);
                const displayName = createMemo(() => getDisplayName(resource));
                const statusIndicator = createMemo(() => getHostStatusIndicator({ status: resource.status }));
                const metricsKey = createMemo(() => buildMetricKey('host', resource.id));

                const cpuPercentValue = createMemo(() => (resource.cpu ? Math.round(getCpuPercent(resource)) : null));
                const memoryPercentValue = createMemo(() => (resource.memory ? Math.round(getMemoryPercent(resource)) : null));
                const diskPercentValue = createMemo(() => (resource.disk ? Math.round(getDiskPercent(resource)) : null));

                const memorySublabel = createMemo(() => {
                  if (!resource.memory || resource.memory.used === undefined || resource.memory.total === undefined) return undefined;
                  return `${formatBytes(resource.memory.used)}/${formatBytes(resource.memory.total)}`;
                });

                const diskSublabel = createMemo(() => {
                  if (!resource.disk || resource.disk.used === undefined || resource.disk.total === undefined) return undefined;
                  return `${formatBytes(resource.disk.used)}/${formatBytes(resource.disk.total)}`;
                });
                const networkTotal = createMemo(() =>
                  (resource.network?.rxBytes ?? 0) + (resource.network?.txBytes ?? 0),
                );
                const networkEmphasis = createMemo(() =>
                  getOutlierEmphasis(networkTotal(), ioScale().network),
                );
                const diskIOTotal = createMemo(() =>
                  (resource.diskIO?.readRate ?? 0) + (resource.diskIO?.writeRate ?? 0),
                );
                const diskIOEmphasis = createMemo(() =>
                  getOutlierEmphasis(diskIOTotal(), ioScale().diskIO),
                );

                const rowClass = createMemo(() => {
                  const baseBorder = 'border-b border-gray-100 dark:border-gray-700/50';
                  const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                  if (isExpanded()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900/20 ${baseBorder}`;
                  }

                  let className = baseHover;
                  if (isHighlighted()) {
                    className += ' bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-300 dark:ring-blue-600';
                  }
                  if (props.hoveredResourceId === resource.id) {
                    className += ' bg-blue-50/60 dark:bg-blue-900/30';
                  }
                  if (!isResourceOnline(resource)) {
                    className += ' opacity-60';
                  }

                  return className;
                });
                const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                const unifiedSourceBadges = createMemo(() =>
                  getUnifiedSourceBadges(getUnifiedSources(resource)),
                );
                const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                const workloadsHref = createMemo(() => buildWorkloadsHref(resource));

                return (
                  <>
                    <tr
                      ref={(el) => {
                        if (el) {
                          rowRefs.set(resource.id, el);
                        } else {
                          rowRefs.delete(resource.id);
                        }
                      }}
                      class={rowClass()}
                      style={{ 'min-height': '36px' }}
                      onClick={() => toggleExpand(resource.id)}
                      onMouseEnter={() => props.onHoverChange?.(resource.id)}
                      onMouseLeave={() => props.onHoverChange?.(null)}
                    >
                      <td class="pr-1.5 sm:pr-2 py-1 align-middle overflow-hidden pl-2 sm:pl-3">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <div
                            class={`shrink-0 transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}
                          >
                            <svg class="w-3.5 h-3.5 text-gray-500 group-hover:text-gray-700 dark:group-hover:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                            </svg>
                          </div>
                          <StatusDot
                            variant={statusIndicator().variant}
                            title={statusIndicator().label}
                            ariaLabel={statusIndicator().label}
                            size="xs"
                          />
                          <div class="flex min-w-0 flex-1 items-baseline gap-1">
                            <span
                              class="block min-w-0 flex-1 truncate font-medium text-[11px] text-gray-900 dark:text-gray-100 select-text"
                              title={displayName()}
                            >
                              {displayName()}
                            </span>
                            <Show when={hasAlternateName(resource)}>
                              <span class="hidden min-w-0 max-w-[28%] shrink truncate text-[9px] text-gray-500 dark:text-gray-400 lg:inline">
                                ({resource.name})
                              </span>
                            </Show>
                          </div>
                          <Show when={workloadsHref()}>
                            {(href) => (
                              <a
                                href={href()}
                                class="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-blue-600 transition-colors hover:bg-blue-100 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 dark:text-blue-300 dark:hover:bg-blue-900/40 dark:hover:text-blue-200"
                                title="View related workloads"
                                aria-label={`View workloads for ${displayName()}`}
                                onClick={(event) => event.stopPropagation()}
                              >
                                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                  <path stroke-linecap="round" stroke-linejoin="round" d="M14 5h5m0 0v5m0-5-8 8" />
                                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 10v9h9" />
                                </svg>
                              </a>
                            )}
                          </Show>
                        </div>
	                      </td>
	
	                      <td class={tdClass}>
	                        <Show when={cpuPercentValue() !== null} fallback={<div class="flex justify-center"><span class="text-xs text-gray-400">—</span></div>}>
	                          <ResponsiveMetricCell
                            class="w-full"
                            value={cpuPercentValue() ?? 0}
                            type="cpu"
                            resourceId={isMobile() ? undefined : metricsKey()}
                            isRunning={isResourceOnline(resource)}
                            showMobile={false}
                          />
	                        </Show>
	                      </td>
	
	                      <td class={tdClass}>
	                        <Show when={memoryPercentValue() !== null} fallback={<div class="flex justify-center"><span class="text-xs text-gray-400">—</span></div>}>
	                          <ResponsiveMetricCell
                            class="w-full"
                            value={memoryPercentValue() ?? 0}
                            type="memory"
                            sublabel={memorySublabel()}
                            resourceId={isMobile() ? undefined : metricsKey()}
                            isRunning={isResourceOnline(resource)}
                            showMobile={false}
                          />
	                        </Show>
	                      </td>
	
	                      <td class={tdClass}>
	                        <Show when={diskPercentValue() !== null} fallback={<div class="flex justify-center"><span class="text-xs text-gray-400">—</span></div>}>
	                          <ResponsiveMetricCell
	                            class="w-full"
	                            value={diskPercentValue() ?? 0}
                            type="disk"
                            sublabel={diskSublabel()}
                            resourceId={isMobile() ? undefined : metricsKey()}
                            isRunning={isResourceOnline(resource)}
                            showMobile={false}
                          />
	                        </Show>
	                      </td>
	
	                      <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                        <Show when={resource.network} fallback={<div class="text-center"><span class="text-xs text-gray-400">—</span></div>}>
	                          <div class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums">
	                            <span class="inline-flex w-3 justify-center text-emerald-500">↓</span>
	                            <span
                              class={`min-w-0 whitespace-nowrap ${networkEmphasis().className}`}
                              title={networkEmphasis().showOutlierHint ? `${formatSpeed(resource.network!.rxBytes)} (Top outlier)` : formatSpeed(resource.network!.rxBytes)}
                            >
                              {formatSpeed(resource.network!.rxBytes)}
                            </span>
                            <span class="inline-flex w-3 justify-center text-orange-400">↑</span>
                            <span
                              class={`min-w-0 whitespace-nowrap ${networkEmphasis().className}`}
                              title={networkEmphasis().showOutlierHint ? `${formatSpeed(resource.network!.txBytes)} (Top outlier)` : formatSpeed(resource.network!.txBytes)}
                            >
                              {formatSpeed(resource.network!.txBytes)}
                            </span>
                          </div>
	                        </Show>
	                      </td>
	
	                      <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                        <Show when={resource.diskIO} fallback={<div class="text-center"><span class="text-xs text-gray-400">—</span></div>}>
	                          <div class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums">
	                            <span class="inline-flex w-3 justify-center font-mono text-blue-500">R</span>
	                            <span
                              class={`min-w-0 whitespace-nowrap ${diskIOEmphasis().className}`}
                              title={diskIOEmphasis().showOutlierHint ? `${formatSpeed(resource.diskIO!.readRate)} (Top outlier)` : formatSpeed(resource.diskIO!.readRate)}
                            >
                              {formatSpeed(resource.diskIO!.readRate)}
                            </span>
                            <span class="inline-flex w-3 justify-center font-mono text-amber-500">W</span>
                            <span
                              class={`min-w-0 whitespace-nowrap ${diskIOEmphasis().className}`}
                              title={diskIOEmphasis().showOutlierHint ? `${formatSpeed(resource.diskIO!.writeRate)} (Top outlier)` : formatSpeed(resource.diskIO!.writeRate)}
                            >
                              {formatSpeed(resource.diskIO!.writeRate)}
                            </span>
                          </div>
	                        </Show>
	                      </td>
	
	                      <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                        <div class="flex items-center justify-center gap-1">
	                          <Show
	                            when={hasUnifiedSources()}
	                            fallback={
                              <>
                                <Show when={platformBadge()}>
                                  {(badge) => (
                                    <span class={badge().classes} title={badge().title}>
                                      {badge().label}
                                    </span>
                                  )}
                                </Show>
                                <Show when={sourceBadge()}>
                                  {(badge) => (
                                    <span class={badge().classes} title={badge().title}>
                                      {badge().label}
                                    </span>
                                  )}
                                </Show>
                              </>
                            }
                          >
                            <For each={unifiedSourceBadges()}>
                              {(badge) => (
                                <span class={badge.classes} title={badge.title}>
                                  {badge.label}
                                </span>
                              )}
                            </For>
                          </Show>
	                        </div>
	                      </td>
	
	                      <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                        <div class="flex justify-center">
	                          <Show when={resource.uptime} fallback={<span class="text-xs text-gray-400">—</span>}>
	                            <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
	                              {formatUptime(resource.uptime ?? 0)}
                            </span>
                          </Show>
	                        </div>
	                      </td>
	
	                      <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                        <div class="flex justify-center">
	                          <Show when={resource.temperature != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                            <span
	                              class={`text-xs whitespace-nowrap font-medium ${
                                (resource.temperature ?? 0) >= 80
                                  ? 'text-red-600 dark:text-red-400'
                                  : (resource.temperature ?? 0) >= 65
                                    ? 'text-amber-600 dark:text-amber-400'
                                    : 'text-emerald-600 dark:text-emerald-400'
                              }`}
                            >
                              {formatTemperature(resource.temperature)}
                            </span>
                          </Show>
                        </div>
                      </td>
                    </tr>
                    <Show when={isExpanded()}>
                      <tr>
                        <td colspan={9} class="bg-gray-50/50 dark:bg-gray-900/20 px-4 py-4 border-b border-gray-100 dark:border-gray-700 shadow-inner">
                          <ResourceDetailDrawer resource={resource} onClose={() => setExpandedResourceId(null)} />
                        </td>
                      </tr>
                    </Show>
                  </>
                );
              }}
            </For>

            <Show when={hostWindowing.isWindowed() && hostBottomSpacerHeight() > 0}>
              <tr aria-hidden="true">
                <td colspan={9} style={{ height: `${hostBottomSpacerHeight()}px`, padding: '0', border: '0' }} />
              </tr>
            </Show>
          </tbody>
        </table>
      </div>
        </Card>
      </Show>

      <Show when={sortedPBSResources().length > 0 || sortedPMGResources().length > 0}>
        <Card padding="none" tone="glass" class="mb-0 overflow-hidden">
          <div class="border-b border-gray-200/70 bg-gray-50/70 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-600 dark:border-gray-700 dark:bg-gray-900/40 dark:text-slate-300">
            Service Infrastructure
          </div>

	          <Show when={sortedPBSResources().length > 0}>
	            <div class="border-b border-gray-100 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500 dark:border-gray-700 dark:text-slate-300">
	              PBS Services
	            </div>
	            <div class="overflow-x-auto" style={{ '-webkit-overflow-scrolling': 'touch' }}>
	              <table class="w-full border-collapse whitespace-nowrap" style={{ 'table-layout': 'fixed', 'min-width': isMobile() ? '660px' : '500px' }}>
	                <thead>
	                  <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
	                    <th class={`${staticThClass} text-left pl-2 sm:pl-3`} style={resourceColumnStyle()}>
	                      Resource
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('primary') && !isMobile() }} style={serviceCountColumnStyle()}>
	                      Datastores
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={serviceCountColumnStyle()}>
	                      Jobs
	                    </th>
	                    <th class={staticThClass} style={serviceHealthColumnStyle()}>
	                      Health
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={sourceColumnStyle()}>
	                      Source
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={uptimeColumnStyle()}>
	                      Uptime
	                    </th>
	                    <th class={staticThClass} style={serviceActionColumnStyle()}>
	                      Action
	                    </th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800">
                  <For each={sortedPBSResources()}>
                    {(resource) => {
                      const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                      const isHighlighted = createMemo(() => props.highlightedResourceId === resource.id);
                      const displayName = createMemo(() => getDisplayName(resource));
                      const serviceLink = createMemo(() => buildServiceDetailLinks(resource)[0] ?? null);
                      const statusIndicator = createMemo(() => getHostStatusIndicator({ status: resource.status }));
                      const pbsRow = createMemo(() => getPBSTableRow(resource));
                      const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                      const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                      const unifiedSourceBadges = createMemo(() =>
                        getUnifiedSourceBadges(getUnifiedSources(resource)),
                      );
                      const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                      const healthClass = createMemo(() => {
                        const tone = pbsRow()?.tone ?? 'muted';
                        if (tone === 'ok') return 'text-emerald-600 dark:text-emerald-400';
                        if (tone === 'warning') return 'text-amber-600 dark:text-amber-400';
                        return 'text-gray-500 dark:text-gray-400';
                      });

                      const rowClass = createMemo(() => {
                        const baseBorder = 'border-b border-gray-100 dark:border-gray-700/50';
                        const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                        if (isExpanded()) {
                          return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900/20 ${baseBorder}`;
                        }

                        let className = baseHover;
                        if (isHighlighted()) {
                          className += ' bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-300 dark:ring-blue-600';
                        }
                        if (props.hoveredResourceId === resource.id) {
                          className += ' bg-blue-50/60 dark:bg-blue-900/30';
                        }
                        if (!isResourceOnline(resource)) {
                          className += ' opacity-60';
                        }

                        return className;
                      });

                      return (
                        <>
                          <tr
                            ref={(el) => {
                              if (el) {
                                rowRefs.set(resource.id, el);
                              } else {
                                rowRefs.delete(resource.id);
                              }
                            }}
                            class={rowClass()}
                            style={{ 'min-height': '36px' }}
                            onClick={() => toggleExpand(resource.id)}
                            onMouseEnter={() => props.onHoverChange?.(resource.id)}
                            onMouseLeave={() => props.onHoverChange?.(null)}
                          >
                            <td class="pr-1.5 sm:pr-2 py-1 align-middle overflow-hidden pl-2 sm:pl-3">
                              <div class="flex items-center gap-1.5 min-w-0">
                                <div class={`shrink-0 transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}>
                                  <svg class="w-3.5 h-3.5 text-gray-500 group-hover:text-gray-700 dark:group-hover:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                  </svg>
                                </div>
                                <StatusDot
                                  variant={statusIndicator().variant}
                                  title={statusIndicator().label}
                                  ariaLabel={statusIndicator().label}
                                  size="xs"
                                />
                                <span class="block min-w-0 flex-1 truncate font-medium text-[11px] text-gray-900 dark:text-gray-100 select-text" title={displayName()}>
                                  {displayName()}
                                </span>
                                <Show when={hasAlternateName(resource)}>
                                  <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-gray-500 dark:text-gray-400 lg:inline">
                                    ({resource.name})
                                  </span>
                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('primary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pbsRow()?.datastores != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300">{pbsRow()!.datastores}</span>
	                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pbsRow()?.jobs != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300">{pbsRow()!.jobs}</span>
	                                </Show>
                              </div>
                            </td>

                            <td class={tdClass}>
                              <div class="flex justify-center">
                                <Show when={pbsRow()?.health} fallback={<span class="text-xs text-gray-400">—</span>}>
                                  <span class={`text-xs font-medium ${healthClass()}`}>{pbsRow()!.health}</span>
                                </Show>
                              </div>
	                            </td>
	
	                            <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                              <div class="flex items-center justify-center gap-1">
	                                <Show
	                                  when={hasUnifiedSources()}
	                                  fallback={
                                    <>
                                      <Show when={platformBadge()}>
                                        {(badge) => (
                                          <span class={badge().classes} title={badge().title}>
                                            {badge().label}
                                          </span>
                                        )}
                                      </Show>
                                      <Show when={sourceBadge()}>
                                        {(badge) => (
                                          <span class={badge().classes} title={badge().title}>
                                            {badge().label}
                                          </span>
                                        )}
                                      </Show>
                                    </>
                                  }
                                >
                                  <For each={unifiedSourceBadges()}>
                                    {(badge) => (
                                      <span class={badge.classes} title={badge.title}>
                                        {badge.label}
                                      </span>
                                    )}
                                  </For>
                                </Show>
	                              </div>
	                            </td>
	
	                            <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={resource.uptime} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
	                                    {formatUptime(resource.uptime ?? 0)}
                                  </span>
                                </Show>
                              </div>
                            </td>

                            <td class={tdClass}>
                              <div class="flex justify-center">
                                <Show when={serviceLink()} fallback={<span class="text-xs text-gray-400">—</span>}>
                                  {(link) => (
                                    <a
                                      href={link().href}
                                      class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
                                      title={link().label}
                                      aria-label={link().ariaLabel}
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      {link().compactLabel}
                                    </a>
                                  )}
                                </Show>
                              </div>
                            </td>
                          </tr>
                          <Show when={isExpanded()}>
                            <tr>
                              <td colspan={7} class="bg-gray-50/50 dark:bg-gray-900/20 px-4 py-4 border-b border-gray-100 dark:border-gray-700 shadow-inner">
                                <ResourceDetailDrawer resource={resource} onClose={() => setExpandedResourceId(null)} />
                              </td>
                            </tr>
                          </Show>
                        </>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>

          <Show when={sortedPMGResources().length > 0}>
	            <div class="border-b border-gray-100 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500 dark:border-gray-700 dark:text-slate-300">
	              PMG Services
	            </div>
	            <div class="overflow-x-auto" style={{ '-webkit-overflow-scrolling': 'touch' }}>
	              <table class="w-full border-collapse whitespace-nowrap" style={{ 'table-layout': 'fixed', 'min-width': isMobile() ? '840px' : '500px' }}>
	                <thead>
	                  <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
	                    <th class={`${staticThClass} text-left pl-2 sm:pl-3`} style={resourceColumnStyle()}>
	                      Resource
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('primary') && !isMobile() }} style={serviceQueueColumnStyle()}>
	                      Queue
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={serviceQueueColumnStyle()}>
	                      Deferred
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={serviceQueueColumnStyle()}>
	                      Hold
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={serviceCountColumnStyle()}>
	                      Nodes
	                    </th>
	                    <th class={staticThClass} style={serviceHealthColumnStyle()}>
	                      Health
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }} style={sourceColumnStyle()}>
	                      Source
	                    </th>
	                    <th class={staticThClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }} style={uptimeColumnStyle()}>
	                      Uptime
	                    </th>
	                    <th class={staticThClass} style={serviceActionColumnStyle()}>
	                      Action
	                    </th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800">
                  <For each={sortedPMGResources()}>
                    {(resource) => {
                      const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                      const isHighlighted = createMemo(() => props.highlightedResourceId === resource.id);
                      const displayName = createMemo(() => getDisplayName(resource));
                      const serviceLink = createMemo(() => buildServiceDetailLinks(resource)[0] ?? null);
                      const statusIndicator = createMemo(() => getHostStatusIndicator({ status: resource.status }));
                      const pmgRow = createMemo(() => getPMGTableRow(resource));
                      const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                      const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                      const unifiedSourceBadges = createMemo(() =>
                        getUnifiedSourceBadges(getUnifiedSources(resource)),
                      );
                      const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                      const healthClass = createMemo(() => {
                        const tone = pmgRow()?.tone ?? 'muted';
                        if (tone === 'ok') return 'text-emerald-600 dark:text-emerald-400';
                        if (tone === 'warning') return 'text-amber-600 dark:text-amber-400';
                        return 'text-gray-500 dark:text-gray-400';
                      });
                      const queueClass = createMemo(() =>
                        (pmgRow()?.deferred || 0) + (pmgRow()?.hold || 0) > 0
                          ? 'text-amber-600 dark:text-amber-400'
                          : 'text-gray-700 dark:text-gray-300',
                      );

                      const rowClass = createMemo(() => {
                        const baseBorder = 'border-b border-gray-100 dark:border-gray-700/50';
                        const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                        if (isExpanded()) {
                          return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900/20 ${baseBorder}`;
                        }

                        let className = baseHover;
                        if (isHighlighted()) {
                          className += ' bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-300 dark:ring-blue-600';
                        }
                        if (props.hoveredResourceId === resource.id) {
                          className += ' bg-blue-50/60 dark:bg-blue-900/30';
                        }
                        if (!isResourceOnline(resource)) {
                          className += ' opacity-60';
                        }

                        return className;
                      });

                      return (
                        <>
                          <tr
                            ref={(el) => {
                              if (el) {
                                rowRefs.set(resource.id, el);
                              } else {
                                rowRefs.delete(resource.id);
                              }
                            }}
                            class={rowClass()}
                            style={{ 'min-height': '36px' }}
                            onClick={() => toggleExpand(resource.id)}
                            onMouseEnter={() => props.onHoverChange?.(resource.id)}
                            onMouseLeave={() => props.onHoverChange?.(null)}
                          >
                            <td class="pr-1.5 sm:pr-2 py-1 align-middle overflow-hidden pl-2 sm:pl-3">
                              <div class="flex items-center gap-1.5 min-w-0">
                                <div class={`shrink-0 transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}>
                                  <svg class="w-3.5 h-3.5 text-gray-500 group-hover:text-gray-700 dark:group-hover:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                  </svg>
                                </div>
                                <StatusDot
                                  variant={statusIndicator().variant}
                                  title={statusIndicator().label}
                                  ariaLabel={statusIndicator().label}
                                  size="xs"
                                />
                                <span class="block min-w-0 flex-1 truncate font-medium text-[11px] text-gray-900 dark:text-gray-100 select-text" title={displayName()}>
                                  {displayName()}
                                </span>
                                <Show when={hasAlternateName(resource)}>
                                  <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-gray-500 dark:text-gray-400 lg:inline">
                                    ({resource.name})
                                  </span>
                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('primary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pmgRow()?.queue != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class={`text-xs font-medium ${queueClass()}`}>{pmgRow()!.queue}</span>
	                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pmgRow()?.deferred != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300">{pmgRow()!.deferred}</span>
	                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pmgRow()?.hold != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300">{pmgRow()!.hold}</span>
	                                </Show>
                              </div>
                            </td>

	                            <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={pmgRow()?.nodes != null} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300">{pmgRow()!.nodes}</span>
	                                </Show>
                              </div>
                            </td>

                            <td class={tdClass}>
                              <div class="flex justify-center">
                                <Show when={pmgRow()?.health} fallback={<span class="text-xs text-gray-400">—</span>}>
                                  <span class={`text-xs font-medium ${healthClass()}`}>{pmgRow()!.health}</span>
                                </Show>
	                              </div>
	                            </td>
	
	                            <td class={tdClass} classList={{ hidden: !isVisible('secondary') && !isMobile() }}>
	                              <div class="flex items-center justify-center gap-1">
	                                <Show
	                                  when={hasUnifiedSources()}
	                                  fallback={
                                    <>
                                      <Show when={platformBadge()}>
                                        {(badge) => (
                                          <span class={badge().classes} title={badge().title}>
                                            {badge().label}
                                          </span>
                                        )}
                                      </Show>
                                      <Show when={sourceBadge()}>
                                        {(badge) => (
                                          <span class={badge().classes} title={badge().title}>
                                            {badge().label}
                                          </span>
                                        )}
                                      </Show>
                                    </>
                                  }
                                >
                                  <For each={unifiedSourceBadges()}>
                                    {(badge) => (
                                      <span class={badge.classes} title={badge.title}>
                                        {badge.label}
                                      </span>
                                    )}
                                  </For>
                                </Show>
	                              </div>
	                            </td>
	
	                            <td class={tdClass} classList={{ hidden: !isVisible('supplementary') && !isMobile() }}>
	                              <div class="flex justify-center">
	                                <Show when={resource.uptime} fallback={<span class="text-xs text-gray-400">—</span>}>
	                                  <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
	                                    {formatUptime(resource.uptime ?? 0)}
                                  </span>
                                </Show>
                              </div>
                            </td>

                            <td class={tdClass}>
                              <div class="flex justify-center">
                                <Show when={serviceLink()} fallback={<span class="text-xs text-gray-400">—</span>}>
                                  {(link) => (
                                    <a
                                      href={link().href}
                                      class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
                                      title={link().label}
                                      aria-label={link().ariaLabel}
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      {link().compactLabel}
                                    </a>
                                  )}
                                </Show>
                              </div>
                            </td>
                          </tr>
                          <Show when={isExpanded()}>
                            <tr>
                              <td colspan={9} class="bg-gray-50/50 dark:bg-gray-900/20 px-4 py-4 border-b border-gray-100 dark:border-gray-700 shadow-inner">
                                <ResourceDetailDrawer resource={resource} onClose={() => setExpandedResourceId(null)} />
                              </td>
                            </tr>
                          </Show>
                        </>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </Card>
      </Show>
    </div>
  );
};

export default UnifiedResourceTable;
