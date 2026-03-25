import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import type { Disk } from '@/types/api';
import { formatBytes, formatSpeed, formatUptime, normalizeDiskArray } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  getPlatformBadge,
  getSourceBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import { getAgentStatusIndicator } from '@/utils/status';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import {
  getResourcePolicyTableBadges,
  shouldShowResourceAlternateName,
} from '@/utils/resourcePolicyPresentation';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { buildWorkloadsHref } from './workloadsLink';
import { ClusterDeployBanner } from './ClusterDeployBanner';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { getOutlierEmphasis, isResourceOnline } from './unifiedResourceTableModel';
import { getCpuPercent, getDiskPercent, getMemoryPercent } from '@/types/resource';

interface UnifiedResourceHostTableCardProps {
  tableProps: UnifiedResourceTableProps;
  table: UnifiedResourceTableState;
}

export const UnifiedResourceHostTableCard: Component<UnifiedResourceHostTableCardProps> = (
  props,
) => {
  const { table, tableProps } = props;

  return (
    <Show when={table.showHostTable()}>
      <Card padding="none" tone="card" class="mb-0 overflow-hidden">
        <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          Agent Infrastructure
        </div>
        <div class="overflow-x-auto">
          <Table
            class="whitespace-nowrap min-w-[max-content]"
            style={{
              'table-layout': 'fixed',
              'min-width': table.isMobile() ? '100%' : 'max-content',
            }}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt text-muted border-b border-border">
                <TableHead
                  class="text-left pl-2 sm:pl-3"
                  style={table.resourceColumnStyle()}
                  onClick={() => table.handleSort('name')}
                >
                  Resource {table.renderSortIndicator('name')}
                </TableHead>
                <TableHead
                  style={table.metricColumnStyle()}
                  onClick={() => table.handleSort('cpu')}
                >
                  CPU {table.renderSortIndicator('cpu')}
                </TableHead>
                <TableHead
                  style={table.metricColumnStyle()}
                  onClick={() => table.handleSort('memory')}
                >
                  Memory {table.renderSortIndicator('memory')}
                </TableHead>
                <TableHead
                  style={table.metricColumnStyle()}
                  onClick={() => table.handleSort('disk')}
                >
                  Disk {table.renderSortIndicator('disk')}
                </TableHead>
                <TableHead
                  classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                  style={table.ioColumnStyle()}
                  onClick={() => table.handleSort('network')}
                >
                  Net I/O {table.renderSortIndicator('network')}
                </TableHead>
                <TableHead
                  classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                  style={table.ioColumnStyle()}
                  onClick={() => table.handleSort('diskio')}
                >
                  Disk I/O {table.renderSortIndicator('diskio')}
                </TableHead>
                <TableHead
                  classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                  style={table.sourceColumnStyle()}
                  onClick={() => table.handleSort('source')}
                >
                  Source {table.renderSortIndicator('source')}
                </TableHead>
                <TableHead
                  classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                  style={table.uptimeColumnStyle()}
                  onClick={() => table.handleSort('uptime')}
                >
                  Uptime {table.renderSortIndicator('uptime')}
                </TableHead>
                <TableHead
                  classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                  style={table.tempColumnStyle()}
                  onClick={() => table.handleSort('temp')}
                >
                  Temp {table.renderSortIndicator('temp')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody ref={table.setHostBodyRef}>
              <Show when={table.hostWindowing.isWindowed() && table.hostTopSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={9}
                    style={{
                      height: `${table.hostTopSpacerHeight()}px`,
                      padding: '0',
                      border: '0',
                    }}
                  />
                </TableRow>
              </Show>

              <For each={table.visibleHostTableItems()}>
                {(item) => {
                  if (item.type === 'header') {
                    const group = item.group;
                    return (
                      <TableRow class="bg-surface-alt">
                        <TableCell
                          colspan={9}
                          class="py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-base-content"
                        >
                          <div class="flex items-center gap-2">
                            <Show
                              when={group.cluster}
                              fallback={<span class="text-muted">Standalone</span>}
                            >
                              <span>{group.cluster}</span>
                              <span class="inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                                Cluster
                              </span>
                            </Show>
                            <span class="text-[10px] text-muted font-normal">
                              {group.resources.length}{' '}
                              {group.resources.length === 1 ? 'resource' : 'resources'}
                            </span>
                          </div>
                          <Show when={tableProps.onDeployCluster}>
                            <ClusterDeployBanner
                              group={group}
                              onDeploy={tableProps.onDeployCluster!}
                            />
                          </Show>
                        </TableCell>
                      </TableRow>
                    );
                  }

                  const resource = item.resource;
                  const isExpanded = createMemo(
                    () => tableProps.expandedResourceId === resource.id,
                  );
                  const isHighlighted = createMemo(
                    () => tableProps.highlightedResourceId === resource.id,
                  );
                  const displayName = createMemo(() =>
                    getPreferredInfrastructureDisplayName(resource),
                  );
                  const statusIndicator = createMemo(() =>
                    getAgentStatusIndicator({ status: resource.status }),
                  );
                  const metricsKey = createMemo(() =>
                    buildMetricKeyForUnifiedResource(resource),
                  );

                  const cpuPercentValue = createMemo(() =>
                    resource.cpu ? Math.round(getCpuPercent(resource)) : null,
                  );
                  const memoryPercentValue = createMemo(() =>
                    resource.memory ? Math.round(getMemoryPercent(resource)) : null,
                  );
                  const diskPercentValue = createMemo(() =>
                    resource.disk ? Math.round(getDiskPercent(resource)) : null,
                  );

                  const memorySublabel = createMemo(() => {
                    if (
                      !resource.memory ||
                      resource.memory.used === undefined ||
                      resource.memory.total === undefined
                    )
                      return undefined;
                    return `${formatBytes(resource.memory.used)}/${formatBytes(resource.memory.total)}`;
                  });

                  const diskSublabel = createMemo(() => {
                    if (
                      !resource.disk ||
                      resource.disk.used === undefined ||
                      resource.disk.total === undefined
                    )
                      return undefined;
                    return `${formatBytes(resource.disk.used)}/${formatBytes(resource.disk.total)}`;
                  });
                  const networkTotal = createMemo(
                    () => (resource.network?.rxBytes ?? 0) + (resource.network?.txBytes ?? 0),
                  );
                  const networkEmphasis = createMemo(() =>
                    getOutlierEmphasis(networkTotal(), table.ioScale().network),
                  );
                  const diskIOTotal = createMemo(
                    () => (resource.diskIO?.readRate ?? 0) + (resource.diskIO?.writeRate ?? 0),
                  );
                  const diskIOEmphasis = createMemo(() =>
                    getOutlierEmphasis(diskIOTotal(), table.ioScale().diskIO),
                  );

                  const rowClass = createMemo(() => {
                    const baseHover =
                      'cursor-pointer transition-all duration-200 relative group hover:bg-surface-hover';

                    if (isExpanded()) {
                      return 'cursor-pointer transition-all duration-200 relative z-10 group bg-blue-50 dark:bg-blue-900';
                    }

                    let className = baseHover;
                    if (isHighlighted()) {
                      className +=
                        ' bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600';
                    }
                    if (tableProps.hoveredResourceId === resource.id && !isHighlighted()) {
                      className += ' bg-surface-hover';
                    }
                    if (!isResourceOnline(resource)) {
                      className += ' opacity-60';
                    }

                    return className;
                  });
                  const platformBadge = createMemo(() =>
                    getPlatformBadge(resource.platformType),
                  );
                  const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                  const unifiedSourceBadges = createMemo(() =>
                    getUnifiedSourceBadges(table.getUnifiedSources(resource)),
                  );
                  const hasUnifiedSources = createMemo(
                    () => unifiedSourceBadges().length > 0,
                  );
                  const policyBadges = createMemo(() =>
                    getResourcePolicyTableBadges(resource.policy),
                  );
                  const workloadsHref = createMemo(() => buildWorkloadsHref(resource));

                  return (
                    <>
                      <TableRow
                        ref={(el) => table.registerRowRef(resource.id, el)}
                        class={rowClass()}
                        style={{ 'min-height': '32px' }}
                        onClick={() => table.toggleExpand(resource.id)}
                        onMouseEnter={() => tableProps.onHoverChange?.(resource.id)}
                        onMouseLeave={() => tableProps.onHoverChange?.(null)}
                      >
                        <TableCell class="pr-1.5 sm:pr-2 py-0.5 align-middle overflow-hidden pl-2 sm:pl-3">
                          <div class="flex items-center gap-1.5 min-w-0">
                            <div
                              class={`shrink-0 transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}
                            >
                              <svg
                                class="w-3.5 h-3.5 text-muted group-hover:text-base-content"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M9 5l7 7-7 7"
                                />
                              </svg>
                            </div>
                            <StatusDot
                              variant={statusIndicator().variant}
                              title={statusIndicator().label}
                              ariaLabel={statusIndicator().label}
                              size="xs"
                            />
                            <div class="flex min-w-0 flex-1 flex-col">
                              <div class="flex min-w-0 items-baseline gap-1">
                                <span
                                  class="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                                  title={displayName()}
                                >
                                  {displayName()}
                                </span>
                                <Show when={shouldShowResourceAlternateName(resource)}>
                                  <span class="hidden min-w-0 max-w-[28%] shrink truncate text-[9px] text-muted lg:inline">
                                    ({resource.name})
                                  </span>
                                </Show>
                              </div>
                              <Show when={policyBadges().length > 0}>
                                <div class="mt-0.5 flex flex-wrap gap-1">
                                  <For each={policyBadges()}>
                                    {(badge) => (
                                      <span class={badge.className} title={badge.title}>
                                        {badge.label}
                                      </span>
                                    )}
                                  </For>
                                </div>
                              </Show>
                              <ResourceFacetSummary
                                recentChanges={resource.recentChanges}
                                counts={resource.facetCounts}
                                class="mt-0.5"
                              />
                            </div>
                            <Show when={workloadsHref()}>
                              {(href) => (
                                <a
                                  href={href()}
                                  class="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-blue-600 transition-colors hover:bg-blue-100 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 dark:text-blue-300 dark:hover:bg-blue-800 dark:hover:text-blue-200"
                                  title="View related workloads"
                                  aria-label={`View workloads for ${displayName()}`}
                                  onClick={(event) => event.stopPropagation()}
                                >
                                  <svg
                                    class="h-3.5 w-3.5"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      d="M14 5h5m0 0v5m0-5-8 8"
                                    />
                                    <path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      d="M5 10v9h9"
                                    />
                                  </svg>
                                </a>
                              )}
                            </Show>
                          </div>
                        </TableCell>

                        <TableCell>
                          <Show
                            when={cpuPercentValue() !== null}
                            fallback={
                              <div class="flex justify-center">
                                <span class="text-xs">—</span>
                              </div>
                            }
                          >
                            <ResponsiveMetricCell
                              class="w-full"
                              value={cpuPercentValue() ?? 0}
                              type="cpu"
                              resourceId={table.isMobile() ? undefined : metricsKey()}
                              isRunning={isResourceOnline(resource)}
                              showMobile={false}
                            />
                          </Show>
                        </TableCell>

                        <TableCell>
                          <Show
                            when={memoryPercentValue() !== null}
                            fallback={
                              <div class="flex justify-center">
                                <span class="text-xs">—</span>
                              </div>
                            }
                          >
                            <div class="w-full" title={memorySublabel()}>
                              <StackedMemoryBar
                                used={resource.memory?.used ?? 0}
                                total={resource.memory?.total ?? 0}
                                percentOnly={
                                  !resource.memory?.total && resource.memory?.current != null
                                    ? resource.memory.current
                                    : undefined
                                }
                                swapUsed={resource.agent?.memory?.swapUsed ?? 0}
                                swapTotal={resource.agent?.memory?.swapTotal ?? 0}
                              />
                            </div>
                          </Show>
                        </TableCell>

                        <TableCell>
                          <Show
                            when={diskPercentValue() !== null}
                            fallback={
                              <div class="flex justify-center">
                                <span class="text-xs">—</span>
                              </div>
                            }
                          >
                            <div class="w-full" title={diskSublabel()}>
                              <StackedDiskBar
                                disks={normalizeDiskArray(resource.agent?.disks)}
                                aggregateDisk={
                                  resource.disk
                                    ? ({
                                        total: resource.disk.total ?? 0,
                                        used: resource.disk.used ?? 0,
                                        free: resource.disk.free ?? 0,
                                        usage: resource.disk.current ?? 0,
                                      } as Disk)
                                    : undefined
                                }
                              />
                            </div>
                          </Show>
                        </TableCell>

                        <TableCell classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}>
                          <Show
                            when={resource.network}
                            fallback={
                              <div class="text-center">
                                <span class="text-xs text-slate-400">—</span>
                              </div>
                            }
                          >
                            <div class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums">
                              <span class="inline-flex w-3 justify-center text-emerald-500">↓</span>
                              <span
                                class={`min-w-0 whitespace-nowrap ${networkEmphasis().className}`}
                                title={
                                  networkEmphasis().showOutlierHint
                                    ? `${formatSpeed(resource.network!.rxBytes)} (Top outlier)`
                                    : formatSpeed(resource.network!.rxBytes)
                                }
                              >
                                {formatSpeed(resource.network!.rxBytes)}
                              </span>
                              <span class="inline-flex w-3 justify-center text-orange-400">↑</span>
                              <span
                                class={`min-w-0 whitespace-nowrap ${networkEmphasis().className}`}
                                title={
                                  networkEmphasis().showOutlierHint
                                    ? `${formatSpeed(resource.network!.txBytes)} (Top outlier)`
                                    : formatSpeed(resource.network!.txBytes)
                                }
                              >
                                {formatSpeed(resource.network!.txBytes)}
                              </span>
                            </div>
                          </Show>
                        </TableCell>

                        <TableCell
                          classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                        >
                          <Show
                            when={resource.diskIO}
                            fallback={
                              <div class="text-center">
                                <span class="text-xs text-slate-400">—</span>
                              </div>
                            }
                          >
                            <div class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums">
                              <span class="inline-flex w-3 justify-center font-mono text-blue-500">
                                R
                              </span>
                              <span
                                class={`min-w-0 whitespace-nowrap ${diskIOEmphasis().className}`}
                                title={
                                  diskIOEmphasis().showOutlierHint
                                    ? `${formatSpeed(resource.diskIO!.readRate)} (Top outlier)`
                                    : formatSpeed(resource.diskIO!.readRate)
                                }
                              >
                                {formatSpeed(resource.diskIO!.readRate)}
                              </span>
                              <span class="inline-flex w-3 justify-center font-mono text-amber-500">
                                W
                              </span>
                              <span
                                class={`min-w-0 whitespace-nowrap ${diskIOEmphasis().className}`}
                                title={
                                  diskIOEmphasis().showOutlierHint
                                    ? `${formatSpeed(resource.diskIO!.writeRate)} (Top outlier)`
                                    : formatSpeed(resource.diskIO!.writeRate)
                                }
                              >
                                {formatSpeed(resource.diskIO!.writeRate)}
                              </span>
                            </div>
                          </Show>
                        </TableCell>

                        <TableCell classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}>
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
                        </TableCell>

                        <TableCell
                          classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                        >
                          <div class="flex justify-center">
                            <Show
                              when={resource.uptime}
                              fallback={<span class="text-xs text-slate-400">—</span>}
                            >
                              <span class="text-xs text-base-content whitespace-nowrap">
                                {formatUptime(resource.uptime ?? 0)}
                              </span>
                            </Show>
                          </div>
                        </TableCell>

                        <TableCell
                          classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                        >
                          <div class="flex justify-center">
                            <Show
                              when={resource.temperature != null}
                              fallback={<span class="text-xs text-slate-400">—</span>}
                            >
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
                        </TableCell>
                      </TableRow>
                      <Show when={isExpanded()}>
                        <TableRow>
                          <TableCell
                            colspan={9}
                            class="bg-surface-alt px-4 py-4 border-b border-border-subtle shadow-inner"
                          >
                            <ResourceDetailDrawer
                              resource={resource}
                              resolveResourceLabel={table.resolveResourceLabel}
                              onClose={() => tableProps.onExpandedResourceChange(null)}
                            />
                          </TableCell>
                        </TableRow>
                      </Show>
                    </>
                  );
                }}
              </For>

              <Show when={table.hostWindowing.isWindowed() && table.hostBottomSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={9}
                    style={{
                      height: `${table.hostBottomSpacerHeight()}px`,
                      padding: '0',
                      border: '0',
                    }}
                  />
                </TableRow>
              </Show>
            </TableBody>
          </Table>
        </div>
      </Card>
    </Show>
  );
};
