import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
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
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { buildWorkloadsHref } from './workloadsLink';
import { getPlatformBadge, getSourceBadge, getUnifiedSourceBadges } from './resourceBadges';

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

type ServiceSummary = {
  text: string;
  tone: ServiceSummaryTone;
};

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

const formatJobCount = (count: number): string => `${count} job${count === 1 ? '' : 's'}`;
const formatDatastoreCount = (count: number): string => `${count} datastore${count === 1 ? '' : 's'}`;
const formatNodeCount = (count: number): string => `${count} node${count === 1 ? '' : 's'}`;

const getServiceSummary = (resource: Resource): ServiceSummary | null => {
  const platformData = resource.platformData as { pbs?: PBSServiceData; pmg?: PMGServiceData } | undefined;
  if (!platformData) return null;

  if (resource.type === 'pbs' && platformData.pbs) {
    const pbs = platformData.pbs;
    const totalJobs =
      (pbs.backupJobCount || 0) +
      (pbs.syncJobCount || 0) +
      (pbs.verifyJobCount || 0) +
      (pbs.pruneJobCount || 0) +
      (pbs.garbageJobCount || 0);
    const parts: string[] = [];
    if ((pbs.datastoreCount || 0) > 0) {
      parts.push(formatDatastoreCount(pbs.datastoreCount || 0));
    }
    if (totalJobs > 0) {
      parts.push(formatJobCount(totalJobs));
    }
    if (parts.length === 0 && pbs.connectionHealth) {
      parts.push(pbs.connectionHealth);
    }
    if (parts.length === 0) return null;
    return { text: parts.join(' · '), tone: summarizeServiceHealthTone(pbs.connectionHealth) };
  }

  if (resource.type === 'pmg' && platformData.pmg) {
    const pmg = platformData.pmg;
    const parts: string[] = [];
    if ((pmg.queueTotal || 0) > 0) {
      parts.push(`Queue ${pmg.queueTotal}`);
    }
    if ((pmg.nodeCount || 0) > 0) {
      parts.push(formatNodeCount(pmg.nodeCount || 0));
    }
    if (parts.length === 0 && pmg.connectionHealth) {
      parts.push(pmg.connectionHealth);
    }
    const backlog = (pmg.queueDeferred || 0) + (pmg.queueHold || 0);
    const tone = backlog > 0 ? 'warning' : summarizeServiceHealthTone(pmg.connectionHealth);
    if (parts.length === 0) return null;
    return { text: parts.join(' · '), tone };
  }

  return null;
};

interface IODistributionStats {
  median: number;
  mad: number;
  max: number;
  p97: number;
  p99: number;
  count: number;
}

interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

const computeMedian = (values: number[]): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return (sorted[mid - 1] + sorted[mid]) / 2;
  }
  return sorted[mid];
};

const computePercentile = (values: number[], percentile: number): number => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const clamped = Math.max(0, Math.min(1, percentile));
  const index = Math.max(0, Math.min(sorted.length - 1, Math.ceil(clamped * sorted.length) - 1));
  return sorted[index];
};

const buildIODistribution = (values: number[]): IODistributionStats => {
  const valid = values.filter((value) => Number.isFinite(value) && value >= 0);
  if (valid.length === 0) {
    return { median: 0, mad: 0, max: 0, p97: 0, p99: 0, count: 0 };
  }

  const median = computeMedian(valid);
  const deviations = valid.map((value) => Math.abs(value - median));
  const mad = computeMedian(deviations);
  const max = Math.max(...valid, 0);
  const p97 = computePercentile(valid, 0.97);
  const p99 = computePercentile(valid, 0.99);

  return { median, mad, max, p97, p99, count: valid.length };
};

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
  const { isMobile } = useBreakpoint();
  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const setExpandedResourceId = (id: string | null) => props.onExpandedResourceChange(id);
  const rowRefs = new Map<string, HTMLTableRowElement>();

  createEffect(() => {
    const selectedId = props.expandedResourceId;
    if (!selectedId) return;
    const row = rowRefs.get(selectedId);
    if (row) {
      row.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }
  });

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

  const getSortValue = (resource: Resource, key: SortKey): number | string | null => {
    switch (key) {
      case 'name':
        return getDisplayName(resource);
      case 'uptime':
        return resource.uptime ?? 0;
      case 'cpu':
        return resource.cpu ? getCpuPercent(resource) : null;
      case 'memory':
        return resource.memory ? getMemoryPercent(resource) : null;
      case 'disk':
        return resource.disk ? getDiskPercent(resource) : null;
      case 'network':
        return resource.network ? (resource.network.rxBytes + resource.network.txBytes) : null;
      case 'diskio':
        return resource.diskIO ? (resource.diskIO.readRate + resource.diskIO.writeRate) : null;
      case 'source':
        return `${resource.platformType ?? ''}-${resource.sourceType ?? ''}`;
      case 'temp':
        return resource.temperature ?? null;
      default:
        return null;
    }
  };

  const defaultComparison = (a: Resource, b: Resource) => {
    const aOnline = isResourceOnline(a);
    const bOnline = isResourceOnline(b);
    if (aOnline !== bOnline) return aOnline ? -1 : 1;
    return getDisplayName(a).localeCompare(getDisplayName(b));
  };

  const compareValues = (valueA: number | string | null, valueB: number | string | null) => {
    const aEmpty = valueA === null || valueA === undefined || (typeof valueA === 'number' && Number.isNaN(valueA));
    const bEmpty = valueB === null || valueB === undefined || (typeof valueB === 'number' && Number.isNaN(valueB));

    if (aEmpty && bEmpty) return 0;
    if (aEmpty) return 1;
    if (bEmpty) return -1;

    if (typeof valueA === 'number' && typeof valueB === 'number') {
      if (valueA === valueB) return 0;
      return valueA < valueB ? -1 : 1;
    }

    const aStr = String(valueA).toLowerCase();
    const bStr = String(valueB).toLowerCase();

    if (aStr === bStr) return 0;
    return aStr < bStr ? -1 : 1;
  };

  const sortedResources = createMemo(() => {
    const resources = [...props.resources];
    const key = sortKey();
    const direction = sortDirection();

    return resources.sort((a, b) => {
      if (key === 'default') {
        return defaultComparison(a, b);
      }

      const valueA = getSortValue(a, key);
      const valueB = getSortValue(b, key);
      const comparison = compareValues(valueA, valueB);

      if (comparison !== 0) {
        return direction === 'asc' ? comparison : -comparison;
      }

      return defaultComparison(a, b);
    });
  });

  const groupedResources = createMemo(() => {
    const sorted = sortedResources();
    if (props.groupingMode !== 'grouped') {
      return [{ cluster: '', resources: sorted }];
    }
    const groups = new Map<string, Resource[]>();
    for (const resource of sorted) {
      const cluster = resource.clusterId || '';
      const list = groups.get(cluster);
      if (list) {
        list.push(resource);
      } else {
        groups.set(cluster, [resource]);
      }
    }
    const entries = Array.from(groups.entries()).map(([cluster, resources]) => ({ cluster, resources }));
    // Named clusters first (alphabetical), then standalone
    entries.sort((a, b) => {
      if (!a.cluster && b.cluster) return 1;
      if (a.cluster && !b.cluster) return -1;
      return a.cluster.localeCompare(b.cluster);
    });
    return entries;
  });

  const ioScale = createMemo(() => {
    const networkValues: number[] = [];
    const diskIOValues: number[] = [];

    for (const resource of props.resources) {
      const networkTotal = (resource.network?.rxBytes ?? 0) + (resource.network?.txBytes ?? 0);
      if (resource.network) {
        networkValues.push(networkTotal);
      }

      const diskIOTotal = (resource.diskIO?.readRate ?? 0) + (resource.diskIO?.writeRate ?? 0);
      if (resource.diskIO) {
        diskIOValues.push(diskIOTotal);
      }
    }

    return {
      network: buildIODistribution(networkValues),
      diskIO: buildIODistribution(diskIOValues),
    };
  });

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const toggleExpand = (resourceId: string) => {
    setExpandedResourceId(props.expandedResourceId === resourceId ? null : resourceId);
  };

  const thClassBase = 'px-1.5 sm:px-2 py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap';
  const thClass = `${thClassBase} text-center`;
  const tdClass = 'px-1.5 sm:px-2 py-1 align-middle';
  const resourceColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '110px', 'min-width': '110px', 'max-width': '150px' }
      : { 'min-width': '220px' }
  );
  const metricColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '70px', 'min-width': '70px', 'max-width': '90px' }
      : { 'min-width': '140px', 'max-width': '180px' }
  );
  const ioColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '110px', 'min-width': '110px', 'max-width': '130px' }
      : { width: '160px', 'min-width': '160px', 'max-width': '180px' }
  );
  const sourceColumnStyle = createMemo(() =>
    isMobile()
      ? { width: '100px', 'min-width': '100px', 'max-width': '120px' }
      : { width: '140px', 'min-width': '140px', 'max-width': '160px' }
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

  return (
    <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
      <div
        class="overflow-x-auto"
        style={{ '-webkit-overflow-scrolling': 'touch' }}
      >
        <table class="w-full border-collapse whitespace-nowrap" style={{ 'table-layout': 'fixed', 'min-width': '900px' }}>
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
              <th class={thClass} style={ioColumnStyle()} onClick={() => handleSort('network')}>
                Net I/O {renderSortIndicator('network')}
              </th>
              <th class={thClass} style={ioColumnStyle()} onClick={() => handleSort('diskio')}>
                Disk I/O {renderSortIndicator('diskio')}
              </th>
              <th class={thClass} style={sourceColumnStyle()} onClick={() => handleSort('source')}>
                Source {renderSortIndicator('source')}
              </th>
              <th class={thClass} style={uptimeColumnStyle()} onClick={() => handleSort('uptime')}>
                Uptime {renderSortIndicator('uptime')}
              </th>
              <th class={thClass} style={tempColumnStyle()} onClick={() => handleSort('temp')}>
                Temp {renderSortIndicator('temp')}
              </th>
            </tr>
          </thead>
          <tbody class="bg-white dark:bg-gray-800">
            <For each={groupedResources()}>
              {(group) => (
                <>
                  <Show when={props.groupingMode === 'grouped'}>
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
                  </Show>
                  <For each={group.resources}>
                    {(resource) => {
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
                      const serviceSummary = createMemo(() => getServiceSummary(resource));
                      const workloadsHref = createMemo(() => buildWorkloadsHref(resource));
                      const serviceSummaryClass = createMemo(() => {
                        const summary = serviceSummary();
                        if (!summary) return '';
                        if (summary.tone === 'ok') return 'text-emerald-600 dark:text-emerald-400';
                        if (summary.tone === 'warning') return 'text-amber-600 dark:text-amber-400';
                        return 'text-gray-500 dark:text-gray-400';
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
                                <div
                                  class={`transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}
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
                                <span
                                  class="font-medium text-[11px] text-gray-900 dark:text-gray-100 whitespace-nowrap select-text"
                                  title={displayName()}
                                >
                                  {displayName()}
                                </span>
                                <Show when={hasAlternateName(resource)}>
                                  <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                                    ({resource.name})
                                  </span>
                                </Show>
                              </div>
                              <Show when={serviceSummary()}>
                                {(summary) => (
                                  <div class={`ml-5 mt-0.5 text-[10px] font-medium ${serviceSummaryClass()}`}>
                                    {summary().text}
                                  </div>
                                )}
                              </Show>
                              <Show when={workloadsHref()}>
                                {(href) => (
                                  <div class="ml-5 mt-0.5">
                                    <a
                                      href={href()}
                                      class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700/60 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      View workloads
                                    </a>
                                  </div>
                                )}
                              </Show>
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

                            <td class={tdClass}>
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

                            <td class={tdClass}>
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

                            <td class={tdClass}>
                              <div class="flex flex-wrap items-center justify-center gap-1">
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

                            <td class={tdClass}>
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
                </>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </Card>
  );
};

export default UnifiedResourceTable;
