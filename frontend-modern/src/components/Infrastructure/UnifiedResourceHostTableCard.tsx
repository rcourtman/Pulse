import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import type { Disk } from '@/types/api';
import { formatBytes, formatSpeed, formatUptime, normalizeDiskArray } from '@/utils/format';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';
import {
  GROUPED_TABLE_ROW_BADGE_CLASS,
  GROUPED_TABLE_ROW_META_CLASS,
  getGroupedTableRowCellClass,
  getInteractiveGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { TableCard } from '@/components/shared/TableCard';
import { hostOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
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
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemTitleBadges,
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
import { getAvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { getResourceHealthIssuePresentation } from './resourceHealthPresentation';
import { ClusterDeployBanner } from './ClusterDeployBanner';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import { UnifiedResourceSourceBadgeCell } from './UnifiedResourceSourceBadgeCell';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { resolveSummaryGroupMemberInteractionState } from '@/components/shared/summaryCardInteraction';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { shouldShowClusterGroupTypeLabel } from './unifiedResourceTableStateModel';
import { getOutlierEmphasis, isResourceOnline } from './unifiedResourceTableModel';
import { type Resource, getCpuPercent, getDiskPercent, getMemoryPercent } from '@/types/resource';

interface UnifiedResourceHostTableCardProps {
  tableProps: UnifiedResourceTableProps;
  table: UnifiedResourceTableState;
}

type ThermalPressurePresentation = {
  label: string;
  title: string;
  className: string;
};

const getThermalPressurePresentation = (resource: Resource): ThermalPressurePresentation | null => {
  const thermalState = resource.agent?.sensors?.thermalState;
  if (!thermalState) return null;

  const pressure = thermalState?.pressure?.trim().toLowerCase();
  if (!pressure) return null;

  const label =
    pressure === 'nominal' ? 'Nominal' : pressure === 'constrained' ? 'Constrained' : 'Unknown';
  const className =
    pressure === 'nominal'
      ? 'text-emerald-600 dark:text-emerald-400'
      : pressure === 'constrained'
        ? 'text-amber-600 dark:text-amber-400'
        : 'text-slate-500 dark:text-slate-400';
  const limits = Object.entries(thermalState.limitsPercent ?? {})
    .filter(([, value]) => Number.isFinite(value) && value < 100)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => `${key.replace(/_/g, ' ')} ${Math.round(value)}%`);
  const source = thermalState.source ? ` via ${thermalState.source}` : '';
  const limitSummary = limits.length > 0 ? `; limits: ${limits.join(', ')}` : '';

  return {
    label,
    className,
    title: `Thermal pressure ${label.toLowerCase()}${source}${limitSummary}`,
  };
};

export const UnifiedResourceHostTableCard: Component<UnifiedResourceHostTableCardProps> = (
  props,
) => {
  const { table, tableProps } = props;

  return (
    <Show when={table.showHostTable()}>
      <TableCard class="mb-0">
        <TableCardHeader
          title="Agent Infrastructure"
          showClearAction={table.showHostClearAction()}
          onClear={tableProps.clearPinnedSummaryScope}
        />
        <Table class={`whitespace-nowrap ${table.tableShellClass()}`}>
          <TableHeader>
            <TableRow class="bg-surface-alt text-muted border-b border-border">
              <TableHead
                class={`text-left pl-2 sm:pl-3 ${table.resourceColumn().className}`}
                width={table.resourceColumn().width}
                onClick={() => table.handleSort('name')}
              >
                {table.headerLabels().resource} {table.renderSortIndicator('name')}
              </TableHead>
              <TableHead
                class={table.metricColumn().className}
                width={table.metricColumn().width}
                onClick={() => table.handleSort('cpu')}
              >
                {table.headerLabels().cpu} {table.renderSortIndicator('cpu')}
              </TableHead>
              <TableHead
                class={table.metricColumn().className}
                width={table.metricColumn().width}
                onClick={() => table.handleSort('memory')}
              >
                {table.headerLabels().memory} {table.renderSortIndicator('memory')}
              </TableHead>
              <TableHead
                class={table.metricColumn().className}
                width={table.metricColumn().width}
                onClick={() => table.handleSort('disk')}
              >
                {table.headerLabels().disk} {table.renderSortIndicator('disk')}
              </TableHead>
              <TableHead
                classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                class={table.ioColumn().className}
                width={table.ioColumn().width}
                onClick={() => table.handleSort('network')}
              >
                {table.headerLabels().network} {table.renderSortIndicator('network')}
              </TableHead>
              <TableHead
                classList={{ hidden: table.isMobile() || !table.isHostDiskIoVisible() }}
                class={table.ioColumn().className}
                width={table.ioColumn().width}
                onClick={() => table.handleSort('diskio')}
              >
                {table.headerLabels().diskIo} {table.renderSortIndicator('diskio')}
              </TableHead>
              <TableHead
                classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                class={table.sourceColumn().className}
                width={table.sourceColumn().width}
                onClick={() => table.handleSort('source')}
              >
                {table.headerLabels().source} {table.renderSortIndicator('source')}
              </TableHead>
              <TableHead
                classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                class={table.uptimeColumn().className}
                width={table.uptimeColumn().width}
                onClick={() => table.handleSort('uptime')}
              >
                {table.headerLabels().uptime} {table.renderSortIndicator('uptime')}
              </TableHead>
              <TableHead
                classList={{ hidden: table.isMobile() || !table.isVisible('supplementary') }}
                class={table.tempColumn().className}
                width={table.tempColumn().width}
                onClick={() => table.handleSort('temp')}
              >
                {table.headerLabels().temp} {table.renderSortIndicator('temp')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody ref={table.setHostBodyRef}>
            <Show when={table.hostWindowing.isWindowed() && table.hostTopSpacerHeight() > 0}>
              <TableRow aria-hidden="true">
                <TableCell colspan={9} class="border-0 p-0" height={table.hostTopSpacerHeight()} />
              </TableRow>
            </Show>

            <For each={table.visibleHostTableItems()}>
              {(item) => {
                if (item.type === 'header') {
                  const group = item.group;
                  const groupSummaryScope = createMemo<SummarySeriesGroupScope | null>(() =>
                    table.buildHostSummaryGroupScope(group),
                  );
                  const isSummaryGroupHighlighted = createMemo(
                    () => tableProps.activeSummaryGroupScope?.id === groupSummaryScope()?.id,
                  );
                  const handleGroupHoverChange = (next: SummarySeriesGroupScope | null) => {
                    tableProps.onGroupHoverChange?.(next);
                  };
                  const handleGroupFocusToggle = () => {
                    const nextScope = groupSummaryScope();
                    tableProps.onGroupFocusChange?.(
                      nextScope && tableProps.focusedSummaryGroupId === nextScope.id
                        ? null
                        : (nextScope?.id ?? null),
                    );
                  };
                  const groupRowInteraction = createSummaryInteractiveRowPreviewHandlers({
                    onPreview: () => handleGroupHoverChange(groupSummaryScope()),
                    onPreviewClear: () => handleGroupHoverChange(null),
                  });
                  return (
                    <TableRow
                      class={getInteractiveGroupedTableRowClass()}
                      data-summary-group-id={groupSummaryScope()?.id ?? undefined}
                      data-summary-group-series-count={String(
                        groupSummaryScope()?.seriesIds.length ?? 0,
                      )}
                      data-summary-row-active={isSummaryGroupHighlighted() ? 'true' : 'false'}
                      onClick={handleGroupFocusToggle}
                      {...groupRowInteraction}
                    >
                      <TableCell colspan={9} class={getGroupedTableRowCellClass()}>
                        <div class="flex items-center gap-2">
                          <Show
                            when={group.cluster}
                            fallback={<span class="text-muted">Standalone</span>}
                          >
                            <span>{group.cluster}</span>
                            <Show when={shouldShowClusterGroupTypeLabel(group.cluster)}>
                              <span class={GROUPED_TABLE_ROW_BADGE_CLASS}>Cluster</span>
                            </Show>
                          </Show>
                          <span class={GROUPED_TABLE_ROW_META_CLASS}>
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
                const isExpanded = createMemo(() => tableProps.expandedResourceId === resource.id);
                const isHighlighted = createMemo(
                  () => tableProps.highlightedResourceId === resource.id,
                );
                const displayName = createMemo(() =>
                  getPreferredInfrastructureDisplayName(resource),
                );
                const summaryGroupMemberState = createMemo(() =>
                  resolveSummaryGroupMemberInteractionState({
                    seriesId: resource.id,
                    hoveredGroupScope: tableProps.hoveredSummaryGroupScope,
                    focusedGroupScope: tableProps.focusedSummaryGroupScope,
                  }),
                );
                const statusIndicator = createMemo(() =>
                  getAgentStatusIndicator({ status: resource.status }),
                );
                const healthIssue = createMemo(() => getResourceHealthIssuePresentation(resource));
                const metricsKey = createMemo(() => buildMetricKeyForUnifiedResource(resource));
                const alertResourceIdCandidates = createMemo(() =>
                  hostOverrideIdCandidates(resource),
                );
                const cpuThresholds = createMemo(() =>
                  table.getMetricThresholds('agent', 'cpu', alertResourceIdCandidates()),
                );
                const memoryThresholds = createMemo(() =>
                  table.getMetricThresholds('agent', 'memory', alertResourceIdCandidates()),
                );
                const diskThresholds = createMemo(() =>
                  table.getMetricThresholds('agent', 'disk', alertResourceIdCandidates()),
                );
                const temperatureThresholds = createMemo(() =>
                  table.getMetricThresholds('node', 'temperature', alertResourceIdCandidates()),
                );
                const detailControlsId = createMemo(() =>
                  buildSummaryDisclosureControlsId(resource.id),
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
                  if (!isResourceOnline(resource)) {
                    className += ' opacity-60';
                  }

                  return className;
                });
                const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                const unifiedSources = createMemo(() => table.getUnifiedSources(resource));
                const sourceBadges = createMemo(() => getUnifiedSourceBadges(unifiedSources()));
                const systemBadges = createMemo(() =>
                  getInfrastructureSystemIdentityBadges(resource),
                );
                const systemTitleBadges = createMemo(() =>
                  getInfrastructureSystemTitleBadges(systemBadges(), sourceBadges()),
                );
                const policyBadges = createMemo(() =>
                  getResourcePolicyTableBadges(resource.policy),
                );
                const availabilityProbe = createMemo(() =>
                  getAvailabilityProbePresentation(resource),
                );
                const thermalPressure = createMemo(() => getThermalPressurePresentation(resource));
                const resourceRowInteraction = createSummaryInteractiveRowPreviewHandlers({
                  onPreview: () => tableProps.onHoverChange?.(resource.id),
                  onPreviewClear: () => tableProps.onHoverChange?.(null),
                });

                return (
                  <>
                    <TableRow
                      data-row-id={resource.id}
                      data-summary-series-id={resource.id}
                      data-summary-group-member-active={
                        summaryGroupMemberState() !== 'default'
                          ? summaryGroupMemberState()
                          : undefined
                      }
                      data-summary-row-active={
                        (tableProps.hoveredResourceId === resource.id || isHighlighted()) &&
                        !isExpanded()
                          ? 'true'
                          : 'false'
                      }
                      class={`${rowClass()} h-8`}
                      onClick={() => table.toggleExpand(resource.id)}
                      {...resourceRowInteraction}
                    >
                      <TableCell
                        class={`pr-1.5 sm:pr-2 py-0.5 align-middle overflow-hidden pl-2 sm:pl-3 ${table.resourceColumn().className}`}
                        width={table.resourceColumn().width}
                      >
                        <div class="flex items-center gap-1.5 min-w-0">
                          <SummaryRowActionButton
                            kind="disclosure"
                            subjectLabel={displayName()}
                            expanded={isExpanded()}
                            controlsId={detailControlsId()}
                            onAction={() => table.toggleExpand(resource.id)}
                            onPreviewClear={() => tableProps.onHoverChange?.(null)}
                          />
                          <StatusDot
                            variant={statusIndicator().variant}
                            title={healthIssue()?.title || statusIndicator().label}
                            ariaLabel={healthIssue()?.title || statusIndicator().label}
                            size="xs"
                          />
                          <div class="flex min-w-0 flex-1 flex-col">
                            <div class="flex min-w-0 items-center gap-1">
                              <span
                                class="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                                title={displayName()}
                              >
                                {displayName()}
                              </span>
                              <Show when={healthIssue()}>
                                {(issue) => (
                                  <span
                                    class="hidden shrink-0 whitespace-nowrap rounded bg-amber-100 px-1 text-[9px] font-medium text-amber-700 dark:bg-amber-900 dark:text-amber-300 lg:inline"
                                    title={issue().title}
                                  >
                                    {issue().compactLabel}
                                  </span>
                                )}
                              </Show>
                              <Show when={availabilityProbe()}>
                                {(probe) => (
                                  <span
                                    class={`hidden shrink-0 whitespace-nowrap rounded px-1 text-[9px] font-medium lg:inline ${probe().toneClassName}`}
                                    title={probe().detailLabel}
                                  >
                                    {probe().methodLabel}
                                  </span>
                                )}
                              </Show>
                              <Show when={shouldShowResourceAlternateName(resource)}>
                                <span class="hidden min-w-0 max-w-[28%] shrink truncate text-[9px] text-muted lg:inline">
                                  ({resource.name})
                                </span>
                              </Show>
                              <For each={policyBadges()}>
                                {(badge) => (
                                  <span
                                    class={`${badge.className} max-w-[44%] shrink-0 overflow-hidden px-1`}
                                    title={badge.title}
                                  >
                                    <span class="min-w-0 truncate">{badge.label}</span>
                                  </span>
                                )}
                              </For>
                              <Show when={policyBadges().length === 0}>
                                <ResourceFacetSummary
                                  recentChanges={resource.recentChanges}
                                  counts={resource.facetCounts}
                                  maxVisibleBadges={1}
                                  class="hidden max-w-[48%] shrink-0 flex-nowrap overflow-hidden lg:flex"
                                />
                              </Show>
                            </div>
                          </div>
                        </div>
                      </TableCell>

                      <TableCell>
                        <Show
                          when={cpuPercentValue() !== null}
                          fallback={
                            <div class="flex justify-center">
                              <span class="text-xs" aria-hidden="true">
                                —
                              </span>
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
                            thresholds={cpuThresholds()}
                          />
                        </Show>
                      </TableCell>

                      <TableCell>
                        <Show
                          when={memoryPercentValue() !== null}
                          fallback={
                            <div class="flex justify-center">
                              <span class="text-xs" aria-hidden="true">
                                —
                              </span>
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
                              thresholds={memoryThresholds()}
                            />
                          </div>
                        </Show>
                      </TableCell>

                      <TableCell>
                        <Show
                          when={diskPercentValue() !== null}
                          fallback={
                            <div class="flex justify-center">
                              <span class="text-xs" aria-hidden="true">
                                —
                              </span>
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
                              thresholds={diskThresholds()}
                            />
                          </div>
                        </Show>
                      </TableCell>

                      <TableCell
                        classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                      >
                        <Show
                          when={resource.network}
                          fallback={
                            <Show
                              when={availabilityProbe()}
                              fallback={
                                <div class="text-center">
                                  <span class="text-xs text-slate-400" aria-hidden="true">
                                    —
                                  </span>
                                </div>
                              }
                            >
                              {(probe) => (
                                <div
                                  class="mx-auto inline-flex max-w-full min-w-0 items-baseline justify-center gap-1 whitespace-nowrap text-[11px] leading-tight"
                                  title={probe().detailLabel}
                                >
                                  <Show
                                    when={probe().targetLabel}
                                    fallback={
                                      <span
                                        class={`font-medium tabular-nums ${probe().toneClassName}`}
                                      >
                                        {probe().resultLabel}
                                      </span>
                                    }
                                  >
                                    {(targetLabel) => (
                                      <>
                                        <span class="min-w-0 truncate text-muted">
                                          {targetLabel()}:
                                        </span>
                                        <span
                                          class={`shrink-0 font-medium tabular-nums ${probe().toneClassName}`}
                                        >
                                          {probe().resultLabel}
                                        </span>
                                      </>
                                    )}
                                  </Show>
                                </div>
                              )}
                            </Show>
                          }
                        >
                          <div
                            class={
                              table.layoutMode() === 'wide'
                                ? 'grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums'
                                : 'grid w-full grid-cols-[0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[10px] leading-tight tabular-nums'
                            }
                          >
                            <span class="inline-flex w-3 justify-center text-emerald-500">↓</span>
                            <span
                              class={`min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${networkEmphasis().className}`}
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
                              class={`min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${networkEmphasis().className}`}
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
                        classList={{
                          hidden: table.isMobile() || !table.isHostDiskIoVisible(),
                        }}
                      >
                        <Show
                          when={resource.diskIO}
                          fallback={
                            <div class="text-center">
                              <span class="text-xs text-slate-400" aria-hidden="true">
                                —
                              </span>
                            </div>
                          }
                        >
                          <div
                            class={
                              table.layoutMode() === 'wide'
                                ? 'grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums'
                                : 'grid w-full grid-cols-[0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[10px] leading-tight tabular-nums'
                            }
                          >
                            <span class="inline-flex w-3 justify-center font-mono text-blue-500">
                              R
                            </span>
                            <span
                              class={`min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${diskIOEmphasis().className}`}
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
                              class={`min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${diskIOEmphasis().className}`}
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

                      <TableCell
                        classList={{ hidden: table.isMobile() || !table.isVisible('secondary') }}
                      >
                        <UnifiedResourceSourceBadgeCell
                          unifiedBadges={systemBadges()}
                          platformBadge={platformBadge()}
                          sourceBadge={sourceBadge()}
                          titleBadges={systemTitleBadges()}
                          layoutMode={table.layoutMode()}
                        />
                      </TableCell>

                      <TableCell
                        classList={{
                          hidden: table.isMobile() || !table.isVisible('supplementary'),
                        }}
                      >
                        <div class="flex justify-center">
                          <Show
                            when={resource.uptime}
                            fallback={
                              <span class="text-xs text-slate-400" aria-hidden="true">
                                —
                              </span>
                            }
                          >
                            <span class="text-xs text-base-content whitespace-nowrap">
                              {formatUptime(resource.uptime ?? 0)}
                            </span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell
                        classList={{
                          hidden: table.isMobile() || !table.isVisible('supplementary'),
                        }}
                      >
                        <div class="flex justify-center">
                          <Show
                            when={resource.temperature != null}
                            fallback={
                              <Show
                                when={thermalPressure()}
                                fallback={
                                  <span class="text-xs text-slate-400" aria-hidden="true">
                                    —
                                  </span>
                                }
                              >
                                {(thermal) => (
                                  <span
                                    class={`text-[11px] whitespace-nowrap font-medium ${thermal().className}`}
                                    title={thermal().title}
                                  >
                                    {thermal().label}
                                  </span>
                                )}
                              </Show>
                            }
                          >
                            <span
                              class={`text-xs whitespace-nowrap font-medium ${getTemperatureTextClass(
                                resource.temperature,
                                temperatureThresholds(),
                              )}`}
                            >
                              {formatTemperature(resource.temperature)}
                            </span>
                          </Show>
                        </div>
                      </TableCell>
                    </TableRow>
                    <Show when={isExpanded()}>
                      <InlineDetailTableRow
                        cellId={detailControlsId()}
                        cellClass="border-border-subtle shadow-inner"
                        colspan={9}
                        contentClass="px-4 py-4"
                        data-inline-detail-for={resource.id}
                      >
                        <ResourceDetailDrawer
                          resource={resource}
                          resolveResourceLabel={table.resolveResourceLabel}
                          onClose={() => tableProps.onExpandedResourceChange(null)}
                        />
                      </InlineDetailTableRow>
                    </Show>
                  </>
                );
              }}
            </For>

            <Show when={table.hostWindowing.isWindowed() && table.hostBottomSpacerHeight() > 0}>
              <TableRow aria-hidden="true">
                <TableCell
                  colspan={9}
                  class="border-0 p-0"
                  height={table.hostBottomSpacerHeight()}
                />
              </TableRow>
            </Show>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
};
