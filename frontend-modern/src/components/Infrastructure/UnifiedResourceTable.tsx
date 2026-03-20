import { Component, For, Show, createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import { getCpuPercent, getMemoryPercent, getDiskPercent } from '@/types/resource';
import { formatBytes, formatUptime, formatSpeed, normalizeDiskArray } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
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
import { getServiceHealthSummaryPresentation } from '@/utils/serviceHealthPresentation';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import {
  getResourcePolicyBadges,
  shouldShowResourceAlternateName,
} from '@/utils/resourcePolicyPresentation';
import type { Disk } from '@/types/api';
import type { IODistributionStats } from '@/components/Infrastructure/infrastructureSelectors';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { buildWorkloadsHref } from './workloadsLink';
import { buildServiceDetailLinks } from './serviceDetailLinks';
import { ClusterDeployBanner } from './ClusterDeployBanner';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import {
  useUnifiedResourceTableState,
  type UnifiedResourceTableProps,
} from './useUnifiedResourceTableState';

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

type PBSTableRow = {
  datastores: number | null;
  jobs: number | null;
  health: string | null;
  tone: ReturnType<typeof getServiceHealthSummaryPresentation>['tone'];
};

type PMGTableRow = {
  queue: number | null;
  deferred: number | null;
  hold: number | null;
  nodes: number | null;
  health: string | null;
  tone: ReturnType<typeof getServiceHealthSummaryPresentation>['tone'];
};

const isResourceOnline = (resource: Resource) => {
  const status = resource.status?.toLowerCase();
  return status !== 'offline' && status !== 'stopped';
};

const getPBSTableRow = (resource: Resource): PBSTableRow | null => {
  if (resource.type !== 'pbs') return null;
  const platformData = resource.platformData as
    | { pbs?: PBSServiceData; pmg?: PMGServiceData }
    | undefined;
  const pbs = platformData?.pbs;
  const totalJobs =
    (pbs?.backupJobCount || 0) +
    (pbs?.syncJobCount || 0) +
    (pbs?.verifyJobCount || 0) +
    (pbs?.pruneJobCount || 0) +
    (pbs?.garbageJobCount || 0);
  const health = pbs?.connectionHealth?.trim() || null;

  return {
    datastores: (pbs?.datastoreCount || 0) > 0 ? pbs?.datastoreCount || 0 : null,
    jobs: totalJobs > 0 ? totalJobs : null,
    health,
    tone: getServiceHealthSummaryPresentation(resource.status, health).tone,
  };
};

const getPMGTableRow = (resource: Resource): PMGTableRow | null => {
  if (resource.type !== 'pmg') return null;
  const platformData = resource.platformData as
    | { pbs?: PBSServiceData; pmg?: PMGServiceData }
    | undefined;
  const pmg = platformData?.pmg;
  const health = pmg?.connectionHealth?.trim() || null;
  const backlog = (pmg?.queueDeferred || 0) + (pmg?.queueHold || 0);

  return {
    queue: (pmg?.queueTotal || 0) > 0 ? pmg?.queueTotal || 0 : null,
    deferred: (pmg?.queueDeferred || 0) > 0 ? pmg?.queueDeferred || 0 : null,
    hold: (pmg?.queueHold || 0) > 0 ? pmg?.queueHold || 0 : null,
    nodes: (pmg?.nodeCount || 0) > 0 ? pmg?.nodeCount || 0 : null,
    health,
    tone:
      backlog > 0 ? 'warning' : getServiceHealthSummaryPresentation(resource.status, health).tone,
  };
};

interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

const getOutlierEmphasis = (value: number, stats: IODistributionStats): IOEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { className: 'text-muted', showOutlierHint: false };
  }

  // For tiny sets, avoid aggressive highlighting.
  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  // Robust outlier score: only values meaningfully far from the cluster brighten.
  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { className: 'text-base-content font-semibold', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  // Fallback when values are too uniform for MAD to separate:
  // only near-peak values should get emphasis.
  if (value >= stats.p99)
    return { className: 'text-base-content font-semibold', showOutlierHint: true };
  if (value >= stats.p97)
    return { className: 'text-base-content font-medium', showOutlierHint: true };
  if (value > 0) return { className: 'text-muted', showOutlierHint: false };
  return { className: 'text-muted', showOutlierHint: false };
};

export const UnifiedResourceTable: Component<UnifiedResourceTableProps> = (props) => {
  const {
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
    registerRowRef,
    setHostBodyRef,
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
  } = useUnifiedResourceTableState(props);

  const setExpandedResourceId = (id: string | null) => props.onExpandedResourceChange(id);

  return (
    <div class="space-y-4">
      <Show when={showHostTable()}>
        <Card padding="none" tone="card" class="mb-0 overflow-hidden">
          <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
            Agent Infrastructure
          </div>
          <div class="overflow-x-auto">
            <Table
              class="whitespace-nowrap min-w-[max-content]"
              style={{ 'table-layout': 'fixed', 'min-width': isMobile() ? '100%' : 'max-content' }}
            >
              <TableHeader>
                <TableRow class="bg-surface-alt text-muted border-b border-border">
                  <TableHead
                    class="text-left pl-2 sm:pl-3"
                    style={resourceColumnStyle()}
                    onClick={() => handleSort('name')}
                  >
                    Resource {renderSortIndicator('name')}
                  </TableHead>
                  <TableHead style={metricColumnStyle()} onClick={() => handleSort('cpu')}>
                    CPU {renderSortIndicator('cpu')}
                  </TableHead>
                  <TableHead style={metricColumnStyle()} onClick={() => handleSort('memory')}>
                    Memory {renderSortIndicator('memory')}
                  </TableHead>
                  <TableHead style={metricColumnStyle()} onClick={() => handleSort('disk')}>
                    Disk {renderSortIndicator('disk')}
                  </TableHead>
                  <TableHead
                    classList={{ hidden: isMobile() || !isVisible('secondary') }}
                    style={ioColumnStyle()}
                    onClick={() => handleSort('network')}
                  >
                    Net I/O {renderSortIndicator('network')}
                  </TableHead>
                  <TableHead
                    classList={{ hidden: isMobile() || !isVisible('supplementary') }}
                    style={ioColumnStyle()}
                    onClick={() => handleSort('diskio')}
                  >
                    Disk I/O {renderSortIndicator('diskio')}
                  </TableHead>
                  <TableHead
                    classList={{ hidden: isMobile() || !isVisible('secondary') }}
                    style={sourceColumnStyle()}
                    onClick={() => handleSort('source')}
                  >
                    Source {renderSortIndicator('source')}
                  </TableHead>
                  <TableHead
                    classList={{ hidden: isMobile() || !isVisible('supplementary') }}
                    style={uptimeColumnStyle()}
                    onClick={() => handleSort('uptime')}
                  >
                    Uptime {renderSortIndicator('uptime')}
                  </TableHead>
                  <TableHead
                    classList={{ hidden: isMobile() || !isVisible('supplementary') }}
                    style={tempColumnStyle()}
                    onClick={() => handleSort('temp')}
                  >
                    Temp {renderSortIndicator('temp')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody ref={setHostBodyRef}>
                <Show when={hostWindowing.isWindowed() && hostTopSpacerHeight() > 0}>
                  <TableRow aria-hidden="true">
                    <TableCell
                      colspan={9}
                      style={{ height: `${hostTopSpacerHeight()}px`, padding: '0', border: '0' }}
                    />
                  </TableRow>
                </Show>

                <For each={visibleHostTableItems()}>
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
                            <Show when={props.onDeployCluster}>
                              <ClusterDeployBanner
                                group={group}
                                onDeploy={props.onDeployCluster!}
                              />
                            </Show>
                          </TableCell>
                        </TableRow>
                      );
                    }

                    const resource = item.resource;
                    const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                    const isHighlighted = createMemo(
                      () => props.highlightedResourceId === resource.id,
                    );
                    const displayName = createMemo(() => getPreferredResourceDisplayName(resource));
                    const statusIndicator = createMemo(() =>
                      getAgentStatusIndicator({ status: resource.status }),
                    );
                    const metricsKey = createMemo(() => buildMetricKeyForUnifiedResource(resource));

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
                      getOutlierEmphasis(networkTotal(), ioScale().network),
                    );
                    const diskIOTotal = createMemo(
                      () => (resource.diskIO?.readRate ?? 0) + (resource.diskIO?.writeRate ?? 0),
                    );
                    const diskIOEmphasis = createMemo(() =>
                      getOutlierEmphasis(diskIOTotal(), ioScale().diskIO),
                    );

                    const rowClass = createMemo(() => {
                      const baseHover = `cursor-pointer transition-all duration-200 relative group hover:bg-surface-hover`;

                      if (isExpanded()) {
                        return `cursor-pointer transition-all duration-200 relative z-10 group bg-blue-50 dark:bg-blue-900`;
                      }

                      let className = baseHover;
                      if (isHighlighted()) {
                        className +=
                          ' bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600';
                      }
                      if (props.hoveredResourceId === resource.id && !isHighlighted()) {
                        className += ' bg-surface-hover';
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
                    const policyBadges = createMemo(() => getResourcePolicyBadges(resource.policy));
                    const workloadsHref = createMemo(() => buildWorkloadsHref(resource));

                    return (
                      <>
                        <TableRow
                          ref={(el) => registerRowRef(resource.id, el)}
                          class={rowClass()}
                          style={{ 'min-height': '32px' }}
                          onClick={() => toggleExpand(resource.id)}
                          onMouseEnter={() => props.onHoverChange?.(resource.id)}
                          onMouseLeave={() => props.onHoverChange?.(null)}
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
                                resourceId={isMobile() ? undefined : metricsKey()}
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

                          <TableCell classList={{ hidden: isMobile() || !isVisible('secondary') }}>
                            <Show
                              when={resource.network}
                              fallback={
                                <div class="text-center">
                                  <span class="text-xs text-slate-400">—</span>
                                </div>
                              }
                            >
                              <div class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums">
                                <span class="inline-flex w-3 justify-center text-emerald-500">
                                  ↓
                                </span>
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
                                <span class="inline-flex w-3 justify-center text-orange-400">
                                  ↑
                                </span>
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
                            classList={{ hidden: isMobile() || !isVisible('supplementary') }}
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

                          <TableCell classList={{ hidden: isMobile() || !isVisible('secondary') }}>
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
                            classList={{ hidden: isMobile() || !isVisible('supplementary') }}
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
                            classList={{ hidden: isMobile() || !isVisible('supplementary') }}
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
                                resolveResourceLabel={resolveResourceLabel}
                                onClose={() => setExpandedResourceId(null)}
                              />
                            </TableCell>
                          </TableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>

                <Show when={hostWindowing.isWindowed() && hostBottomSpacerHeight() > 0}>
                  <TableRow aria-hidden="true">
                    <TableCell
                      colspan={9}
                      style={{ height: `${hostBottomSpacerHeight()}px`, padding: '0', border: '0' }}
                    />
                  </TableRow>
                </Show>
              </TableBody>
            </Table>
          </div>
        </Card>
      </Show>

      <Show when={sortedPBSResources().length > 0 || sortedPMGResources().length > 0}>
        <Card padding="none" tone="card" class="mb-0 overflow-hidden">
          <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
            Service Infrastructure
          </div>

          <Show when={sortedPBSResources().length > 0}>
            <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
              PBS Services
            </div>
            <div class="overflow-x-auto">
              <Table
                class="whitespace-nowrap min-w-[max-content]"
                style={{
                  'table-layout': 'fixed',
                  'min-width': isMobile() ? '100%' : 'max-content',
                }}
              >
                <TableHeader>
                  <TableRow class="bg-surface-alt text-muted border-b border-border">
                    <TableHead class="text-left pl-2 sm:pl-3" style={resourceColumnStyle()}>
                      Resource
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('primary') && !isMobile() }}
                      style={serviceCountColumnStyle()}
                    >
                      Datastores
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                      style={serviceCountColumnStyle()}
                    >
                      Jobs
                    </TableHead>
                    <TableHead style={serviceHealthColumnStyle()}>Health</TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                      style={sourceColumnStyle()}
                    >
                      Source
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
                      style={uptimeColumnStyle()}
                    >
                      Uptime
                    </TableHead>
                    <TableHead style={serviceActionColumnStyle()}>Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <For each={sortedPBSResources()}>
                    {(resource) => {
                      const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                      const isHighlighted = createMemo(
                        () => props.highlightedResourceId === resource.id,
                      );
                      const displayName = createMemo(() => getPreferredResourceDisplayName(resource));
                      const serviceLink = createMemo(
                        () => buildServiceDetailLinks(resource)[0] ?? null,
                      );
                      const statusIndicator = createMemo(() =>
                        getAgentStatusIndicator({ status: resource.status }),
                      );
                      const pbsRow = createMemo(() => getPBSTableRow(resource));
                      const platformBadge = createMemo(() =>
                        getPlatformBadge(resource.platformType),
                      );
                      const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                      const unifiedSourceBadges = createMemo(() =>
                        getUnifiedSourceBadges(getUnifiedSources(resource)),
                      );
                      const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                      const healthClass = createMemo(
                        () =>
                          getServiceHealthSummaryPresentation(resource.status, pbsRow()?.health)
                            .textClass,
                      );

                      const rowClass = createMemo(() => {
                        const baseBorder = 'border-b border-border-subtle';
                        const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                        if (isExpanded()) {
                          return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900 ${baseBorder}`;
                        }

                        let className = baseHover;
                        if (isHighlighted()) {
                          className +=
                            ' bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600';
                        }
                        if (props.hoveredResourceId === resource.id) {
                          className += ' bg-blue-100 dark:bg-blue-800';
                        }
                        if (!isResourceOnline(resource)) {
                          className += ' opacity-60';
                        }

                        return className;
                      });

                      return (
                        <>
                          <TableRow
                            ref={(el) => registerRowRef(resource.id, el)}
                            class={rowClass()}
                            style={{ 'min-height': '32px' }}
                            onClick={() => toggleExpand(resource.id)}
                            onMouseEnter={() => props.onHoverChange?.(resource.id)}
                            onMouseLeave={() => props.onHoverChange?.(null)}
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
                                <span
                                  class="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                                  title={displayName()}
                                >
                                  {displayName()}
                                </span>
                                <Show when={shouldShowResourceAlternateName(resource)}>
                                  <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-muted lg:inline">
                                    ({resource.name})
                                  </span>
                                </Show>
                                <ResourceFacetSummary
                                  recentChanges={resource.recentChanges}
                                  counts={resource.facetCounts}
                                  class="mt-0.5"
                                />
                              </div>
                            </TableCell>

                            <TableCell classList={{ hidden: !isVisible('primary') && !isMobile() }}>
                              <div class="flex justify-center">
                                <Show
                                  when={pbsRow()?.datastores != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class="text-xs text-base-content">
                                    {pbsRow()!.datastores}
                                  </span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                            >
                              <div class="flex justify-center">
                                <Show
                                  when={pbsRow()?.jobs != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class="text-xs text-base-content">{pbsRow()!.jobs}</span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell>
                              <div class="flex justify-center">
                                <Show
                                  when={pbsRow()?.health}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class={`text-xs font-medium ${healthClass()}`}>
                                    {pbsRow()!.health}
                                  </span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                            >
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
                              classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
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

                            <TableCell>
                              <div class="flex justify-center">
                                <Show
                                  when={serviceLink()}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  {(link) => (
                                    <a
                                      href={link().href}
                                      class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800"
                                      title={link().label}
                                      aria-label={link().ariaLabel}
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      {link().compactLabel}
                                    </a>
                                  )}
                                </Show>
                              </div>
                            </TableCell>
                          </TableRow>
                          <Show when={isExpanded()}>
                            <TableRow>
                              <TableCell
                                colspan={7}
                                class="bg-surface-alt px-4 py-4 border-b border-border-subtle shadow-inner"
                              >
                                <ResourceDetailDrawer
                                  resource={resource}
                                  resolveResourceLabel={resolveResourceLabel}
                                  onClose={() => setExpandedResourceId(null)}
                                />
                              </TableCell>
                            </TableRow>
                          </Show>
                        </>
                      );
                    }}
                  </For>
                </TableBody>
              </Table>
            </div>
          </Show>

          <Show when={sortedPMGResources().length > 0}>
            <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
              PMG Services
            </div>
            <div class="overflow-x-auto">
              <Table
                class="whitespace-nowrap min-w-[max-content]"
                style={{
                  'table-layout': 'fixed',
                  'min-width': isMobile() ? '100%' : 'max-content',
                }}
              >
                <TableHeader>
                  <TableRow class="bg-surface-alt text-muted border-b border-border">
                    <TableHead class="text-left pl-2 sm:pl-3" style={resourceColumnStyle()}>
                      Resource
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('primary') && !isMobile() }}
                      style={serviceQueueColumnStyle()}
                    >
                      Queue
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                      style={serviceQueueColumnStyle()}
                    >
                      Deferred
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
                      style={serviceQueueColumnStyle()}
                    >
                      Hold
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                      style={serviceCountColumnStyle()}
                    >
                      Nodes
                    </TableHead>
                    <TableHead style={serviceHealthColumnStyle()}>Health</TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                      style={sourceColumnStyle()}
                    >
                      Source
                    </TableHead>
                    <TableHead
                      classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
                      style={uptimeColumnStyle()}
                    >
                      Uptime
                    </TableHead>
                    <TableHead style={serviceActionColumnStyle()}>Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <For each={sortedPMGResources()}>
                    {(resource) => {
                      const isExpanded = createMemo(() => props.expandedResourceId === resource.id);
                      const isHighlighted = createMemo(
                        () => props.highlightedResourceId === resource.id,
                      );
                      const displayName = createMemo(() => getPreferredResourceDisplayName(resource));
                      const serviceLink = createMemo(
                        () => buildServiceDetailLinks(resource)[0] ?? null,
                      );
                      const statusIndicator = createMemo(() =>
                        getAgentStatusIndicator({ status: resource.status }),
                      );
                      const pmgRow = createMemo(() => getPMGTableRow(resource));
                      const platformBadge = createMemo(() =>
                        getPlatformBadge(resource.platformType),
                      );
                      const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                      const unifiedSourceBadges = createMemo(() =>
                        getUnifiedSourceBadges(getUnifiedSources(resource)),
                      );
                      const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                      const healthClass = createMemo(
                        () =>
                          getServiceHealthSummaryPresentation(resource.status, pmgRow()?.health)
                            .textClass,
                      );
                      const queueClass = createMemo(() =>
                        (pmgRow()?.deferred || 0) + (pmgRow()?.hold || 0) > 0
                          ? 'text-amber-600 dark:text-amber-400'
                          : 'text-base-content',
                      );

                      const rowClass = createMemo(() => {
                        const baseBorder = 'border-b border-border-subtle';
                        const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                        if (isExpanded()) {
                          return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900 ${baseBorder}`;
                        }

                        let className = baseHover;
                        if (isHighlighted()) {
                          className +=
                            ' bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600';
                        }
                        if (props.hoveredResourceId === resource.id) {
                          className += ' bg-blue-100 dark:bg-blue-800';
                        }
                        if (!isResourceOnline(resource)) {
                          className += ' opacity-60';
                        }

                        return className;
                      });

                      return (
                        <>
                          <TableRow
                            ref={(el) => registerRowRef(resource.id, el)}
                            class={rowClass()}
                            style={{ 'min-height': '32px' }}
                            onClick={() => toggleExpand(resource.id)}
                            onMouseEnter={() => props.onHoverChange?.(resource.id)}
                            onMouseLeave={() => props.onHoverChange?.(null)}
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
                                <span
                                  class="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                                  title={displayName()}
                                >
                                  {displayName()}
                                </span>
                                <Show when={shouldShowResourceAlternateName(resource)}>
                                  <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-muted lg:inline">
                                    ({resource.name})
                                  </span>
                                </Show>
                                <ResourceFacetSummary
                                  recentChanges={resource.recentChanges}
                                  counts={resource.facetCounts}
                                  class="mt-0.5"
                                />
                              </div>
                            </TableCell>

                            <TableCell classList={{ hidden: !isVisible('primary') && !isMobile() }}>
                              <div class="flex justify-center">
                                <Show
                                  when={pmgRow()?.queue != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class={`text-xs font-medium ${queueClass()}`}>
                                    {pmgRow()!.queue}
                                  </span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                            >
                              <div class="flex justify-center">
                                <Show
                                  when={pmgRow()?.deferred != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class="text-xs text-base-content">
                                    {pmgRow()!.deferred}
                                  </span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
                            >
                              <div class="flex justify-center">
                                <Show
                                  when={pmgRow()?.hold != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class="text-xs text-base-content">{pmgRow()!.hold}</span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                            >
                              <div class="flex justify-center">
                                <Show
                                  when={pmgRow()?.nodes != null}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class="text-xs text-base-content">{pmgRow()!.nodes}</span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell>
                              <div class="flex justify-center">
                                <Show
                                  when={pmgRow()?.health}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  <span class={`text-xs font-medium ${healthClass()}`}>
                                    {pmgRow()!.health}
                                  </span>
                                </Show>
                              </div>
                            </TableCell>

                            <TableCell
                              classList={{ hidden: !isVisible('secondary') && !isMobile() }}
                            >
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
                              classList={{ hidden: !isVisible('supplementary') && !isMobile() }}
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

                            <TableCell>
                              <div class="flex justify-center">
                                <Show
                                  when={serviceLink()}
                                  fallback={<span class="text-xs text-slate-400">—</span>}
                                >
                                  {(link) => (
                                    <a
                                      href={link().href}
                                      class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800"
                                      title={link().label}
                                      aria-label={link().ariaLabel}
                                      onClick={(event) => event.stopPropagation()}
                                    >
                                      {link().compactLabel}
                                    </a>
                                  )}
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
                                  resolveResourceLabel={resolveResourceLabel}
                                  onClose={() => setExpandedResourceId(null)}
                                />
                              </TableCell>
                            </TableRow>
                          </Show>
                        </>
                      );
                    }}
                  </For>
                </TableBody>
              </Table>
            </div>
          </Show>
        </Card>
      </Show>
    </div>
  );
};

export default UnifiedResourceTable;
