import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { getDisplayName, getCpuPercent, getMemoryPercent, getDiskPercent } from '@/types/resource';
import { formatBytes, formatUptime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { buildMetricKey } from '@/utils/metricsKeys';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { getHostStatusIndicator } from '@/utils/status';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { getPlatformBadge, getSourceBadge, getUnifiedSourceBadges } from './resourceBadges';

interface UnifiedResourceTableProps {
  resources: Resource[];
  expandedResourceId: string | null;
  highlightedResourceId?: string | null;
  onExpandedResourceChange: (id: string | null) => void;
}

type SortKey = 'default' | 'name' | 'uptime' | 'cpu' | 'memory' | 'disk' | 'source';

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
      case 'source':
        return `${resource.platformType ?? ''}-${resource.sourceType ?? ''}`;
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
        <table class="w-full border-collapse whitespace-nowrap" style={{ 'table-layout': 'fixed', 'min-width': '600px' }}>
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
              <th class={thClass} style={sourceColumnStyle()} onClick={() => handleSort('source')}>
                Source {renderSortIndicator('source')}
              </th>
              <th class={thClass} style={uptimeColumnStyle()} onClick={() => handleSort('uptime')}>
                Uptime {renderSortIndicator('uptime')}
              </th>
            </tr>
          </thead>
          <tbody class="bg-white dark:bg-gray-800">
            <For each={sortedResources()}>
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

                const rowClass = createMemo(() => {
                  const baseHover = 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';

                  if (isExpanded()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900/20`;
                  }

                  let className = baseHover;
                  if (isHighlighted()) {
                    className += ' bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-300 dark:ring-blue-600';
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
                    </tr>
                    <Show when={isExpanded()}>
                      <tr>
                        <td colspan={6} class="bg-gray-50/50 dark:bg-gray-900/20 px-4 py-4 border-b border-gray-100 dark:border-gray-700 shadow-inner">
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
    </Card>
  );
};

export default UnifiedResourceTable;
