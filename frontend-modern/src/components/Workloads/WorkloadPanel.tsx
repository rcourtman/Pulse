import { createMemo, Index, Show } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import { MetricMiniSparkline } from '@/components/Workloads/MetricMiniSparkline';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import {
  GROUPED_TABLE_ROW_BADGE_CLASS,
  getGroupedTableRowCellClass,
  getInteractiveGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';
import { NodeGroupHeader } from '@/components/shared/NodeGroupHeader';
import { createSummaryInteractiveRowPreviewHandlers } from '@/components/shared/summaryInteractionA11y';
import { buildSummaryDisclosureControlsId } from '@/components/shared/summaryInteractionA11y';
import { TableBody, TableCell, TableRow } from '@/components/shared/Table';
import type { Node } from '@/types/api';
import { getAlertStyles } from '@/utils/alerts';
import { formatSpeed, formatUptime } from '@/utils/format';
import { isNodeOnline } from '@/utils/status';
import { formatTemperature, getCpuTemperature, getTemperatureTextClass } from '@/utils/temperature';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  resolveSummaryGroupMemberInteractionState,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';

import { GuestDrawer } from './GuestDrawer';
import { GuestRow } from './GuestRow';
import { NodeDrawer } from './NodeDrawer';
import { buildWorkloadSummaryGroupScope } from './workloadSelectors';
import type { WorkloadsState } from './useWorkloadsState';
import type { WorkloadTableMetric } from './workloadMetricHistoryModel';

type WorkloadPanelProps = Pick<
  WorkloadsState,
  | 'activeAlerts'
  | 'alertsEnabled'
  | 'bottomSpacerHeight'
  | 'getGroupLabel'
  | 'groupedGuests'
  | 'groupedWindowing'
  | 'guestMetadata'
  | 'guestParentNodeMap'
  | 'groupingMode'
  | 'handleCustomUrlUpdate'
  | 'handleTagClick'
  | 'activeSummaryWorkloadGroupScope'
  | 'activeSummaryWorkloadId'
  | 'focusedSummaryWorkloadGroupScope'
  | 'focusedSummaryWorkloadGroupId'
  | 'hoveredSummaryWorkloadGroupScope'
  | 'isMobile'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setFocusedWorkloadGroupScope'
  | 'setHoveredWorkloadGroupScope'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
  | 'setTableBodyRef'
  | 'topSpacerHeight'
  | 'totalColumns'
  | 'visibleGroupKeys'
  | 'windowedGroupedGuests'
  | 'workloadIOEmphasis'
  | 'workloadMetricDisplayMode'
  | 'workloadMetricHistory'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
>;

const GROUP_NODE_METRIC_CELL_CLASS = 'px-1.5 sm:px-2 py-0.5 align-middle';
const GROUP_NODE_NAME_CELL_CLASS = getGroupedTableRowCellClass(
  '!py-1 !pl-2 sm:!pl-3 !pr-1.5 sm:!pr-2',
);

const getGroupNodeColumnCellClass = (_columnId: string, isNameColumn: boolean): string => {
  if (isNameColumn) return GROUP_NODE_NAME_CELL_CLASS;
  return GROUP_NODE_METRIC_CELL_CLASS;
};

const normalizeMetricPercent = (value: number | null | undefined): number => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
  return value <= 1 ? Math.max(0, value * 100) : Math.max(0, value);
};

const formatMetricPercent = (value: number | null | undefined): string =>
  `${Math.round(normalizeMetricPercent(value))}%`;

const getUsedPercent = (used?: number | null, total?: number | null): number => {
  if (typeof used !== 'number' || typeof total !== 'number' || total <= 0) return 0;
  return normalizeMetricPercent((used / total) * 100);
};

const getGroupNodeMetricKey = (node: Node): string => node.linkedAgentId || node.id || node.name;

const getGroupNodePveVersion = (node: Node): string => {
  const version = (node.pveVersion || '').trim();
  if (!version || version.toLowerCase() === 'unknown') return '';
  return (
    version.match(/pve-manager\/([^/\s]+)/i)?.[1] || version.match(/\d+(?:\.\d+)+/)?.[0] || version
  );
};

const renderGroupNodeEmptyCell = () => (
  <div class="text-center">
    <span class="text-xs text-slate-400" aria-hidden="true">
      —
    </span>
  </div>
);

export function WorkloadPanel(props: WorkloadPanelProps) {
  const renderGroupNodeSparkline = (
    node: Node,
    metric: WorkloadTableMetric,
    valueLabel: string,
    title: string,
    unit = '%',
  ) => (
    <MetricMiniSparkline
      series={props.workloadMetricHistory.getNodeMetricSeries(node, metric)}
      valueLabel={valueLabel}
      title={title}
      unit={unit}
    />
  );

  const renderGroupNodeRatePair = (
    leftLabel: string,
    leftClass: string,
    leftValue: number | null | undefined,
    rightLabel: string,
    rightClass: string,
    rightValue: number | null | undefined,
  ) => {
    const left = Math.max(0, leftValue ?? 0);
    const right = Math.max(0, rightValue ?? 0);
    if (left <= 0 && right <= 0) return renderGroupNodeEmptyCell();

    return (
      <div class="grid w-full min-w-0 grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 overflow-hidden text-[11px] tabular-nums">
        <span class={`inline-flex w-3 justify-center ${leftClass}`}>{leftLabel}</span>
        <span
          class="block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-muted"
          title={formatSpeed(left)}
        >
          {formatSpeed(left)}
        </span>
        <span class={`inline-flex w-3 justify-center ${rightClass}`}>{rightLabel}</span>
        <span
          class="block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-muted"
          title={formatSpeed(right)}
        >
          {formatSpeed(right)}
        </span>
      </div>
    );
  };

  const renderGroupNodeInfoCell = (node: Node) => {
    const version = getGroupNodePveVersion(node);
    const temperature = getCpuTemperature(node.temperature);
    if (!version && temperature === null) return renderGroupNodeEmptyCell();

    return (
      <div class="flex items-center justify-center gap-2 text-[10px] font-medium text-muted">
        <Show when={version}>
          <span title="Proxmox VE version">PVE {version}</span>
        </Show>
        <Show when={temperature !== null}>
          <span class={getTemperatureTextClass(temperature)} title="CPU temperature">
            {formatTemperature(temperature)}
          </span>
        </Show>
      </div>
    );
  };

  const renderGroupNodeColumnCell = (columnId: string, node: Node) => {
    switch (columnId) {
      case 'type':
        return (
          <div class="flex justify-center">
            <span
              class="inline-flex items-center px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300"
              title="Proxmox VE node"
            >
              PVE
            </span>
          </div>
        );
      case 'info':
      case 'vmid':
        return renderGroupNodeInfoCell(node);
      case 'cpu':
        if (props.workloadMetricDisplayMode() === 'sparklines') {
          return renderGroupNodeSparkline(
            node,
            'cpu',
            formatMetricPercent(node.cpu),
            `${node.name} CPU history`,
          );
        }
        return (
          <div class="h-4">
            <EnhancedCPUBar
              usage={normalizeMetricPercent(node.cpu)}
              loadAverage={node.loadAverage?.[0]}
              cores={props.isMobile() ? undefined : node.cpuInfo?.cores}
              model={node.cpuInfo?.model}
              resourceId={getGroupNodeMetricKey(node)}
            />
          </div>
        );
      case 'memory':
        if (props.workloadMetricDisplayMode() === 'sparklines') {
          return renderGroupNodeSparkline(
            node,
            'memory',
            formatMetricPercent(getUsedPercent(node.memory?.used, node.memory?.total)),
            `${node.name} memory history`,
          );
        }
        return (
          <div class="h-4">
            <StackedMemoryBar
              used={node.memory?.used || 0}
              total={node.memory?.total || 0}
              balloon={node.memory?.balloon || 0}
              swapUsed={node.memory?.swapUsed || 0}
              swapTotal={node.memory?.swapTotal || 0}
              resourceId={getGroupNodeMetricKey(node)}
            />
          </div>
        );
      case 'disk':
        if (props.workloadMetricDisplayMode() === 'sparklines') {
          return renderGroupNodeSparkline(
            node,
            'disk',
            formatMetricPercent(node.disk?.usage),
            `${node.name} disk usage history`,
          );
        }
        return (
          <div class="h-4">
            <StackedDiskBar
              aggregateDisk={{
                total: node.disk?.total || 0,
                used: node.disk?.used || 0,
                free:
                  node.disk?.free ?? Math.max(0, (node.disk?.total || 0) - (node.disk?.used || 0)),
                usage: node.disk?.usage || 0,
              }}
            />
          </div>
        );
      case 'uptime':
        return (
          <div class="flex justify-center">
            <Show when={(node.uptime ?? 0) > 0} fallback={renderGroupNodeEmptyCell()}>
              <span
                class={`text-xs whitespace-nowrap ${
                  node.uptime > 0 && node.uptime < 3600 ? 'text-orange-500' : 'text-muted'
                }`}
              >
                {formatUptime(node.uptime, props.isMobile())}
              </span>
            </Show>
          </div>
        );
      case 'node':
      case 'context':
        return (
          <div class="flex justify-center">
            <span class="text-xs text-muted truncate max-w-[120px]" title={node.name}>
              {node.name}
            </span>
          </div>
        );
      case 'os':
        return (
          <div class="flex justify-center">
            <span class="text-xs text-muted">PVE</span>
          </div>
        );
      case 'netIo':
        if (props.workloadMetricDisplayMode() === 'sparklines') {
          return renderGroupNodeSparkline(
            node,
            'netIo',
            `${formatSpeed(node.networkIn ?? 0)} / ${formatSpeed(node.networkOut ?? 0)}`,
            `${node.name} network I/O history`,
            'B/s',
          );
        }
        return renderGroupNodeRatePair(
          '↓',
          'text-emerald-500',
          node.networkIn,
          '↑',
          'text-orange-400',
          node.networkOut,
        );
      case 'diskIo':
        if (props.workloadMetricDisplayMode() === 'sparklines') {
          return renderGroupNodeSparkline(
            node,
            'diskIo',
            `${formatSpeed(node.diskRead ?? 0)} / ${formatSpeed(node.diskWrite ?? 0)}`,
            `${node.name} disk I/O history`,
            'B/s',
          );
        }
        return renderGroupNodeRatePair(
          'R',
          'font-mono text-blue-500',
          node.diskRead,
          'W',
          'font-mono text-amber-500',
          node.diskWrite,
        );
      default:
        return renderGroupNodeEmptyCell();
    }
  };

  return (
    <TableBody ref={props.setTableBodyRef} class="divide-y divide-border">
      <Show when={props.groupedWindowing.isWindowed() && props.topSpacerHeight() > 0}>
        <TableRow aria-hidden="true">
          <TableCell colspan={props.totalColumns()} class="p-0 border-0">
            <svg
              aria-hidden="true"
              width="1"
              height={String(props.topSpacerHeight())}
              class="block w-px pointer-events-none"
            />
          </TableCell>
        </TableRow>
      </Show>
      <Index each={props.visibleGroupKeys()} fallback={<></>}>
        {(groupKey) => {
          const groupGuests = () => props.windowedGroupedGuests()[groupKey()] || [];
          const fullGroupGuests = () => props.groupedGuests()[groupKey()] || [];
          const node = () => props.nodeByInstance()[groupKey()];
          const groupSummaryScope = createMemo<SummarySeriesGroupScope | null>(() => {
            if (props.groupingMode() !== 'grouped') {
              return null;
            }
            return buildWorkloadSummaryGroupScope(
              groupKey(),
              fullGroupGuests(),
              props.getGroupLabel(groupKey(), fullGroupGuests()),
            );
          });
          const isSummaryGroupHighlighted = createMemo(
            () => props.activeSummaryWorkloadGroupScope()?.id === groupKey(),
          );
          const shouldShowNodeDrawer = createMemo(
            () =>
              Boolean(node()) &&
              props.focusedSummaryWorkloadGroupId() === groupKey() &&
              props.selectedGuestId() === null,
          );
          const handleGroupHoverChange = (next: SummarySeriesGroupScope | null) => {
            props.setHoveredWorkloadGroupScope(next);
          };
          const handleGroupFocusToggle = () => {
            const scope = groupSummaryScope();
            const selectedGuestId = props.selectedGuestId();
            const nextFocusedScope =
              scope &&
              props.focusedSummaryWorkloadGroupId() === scope.id &&
              selectedGuestId === null
                ? null
                : scope;
            if (nextFocusedScope && selectedGuestId !== null) {
              props.setSelectedGuestId(null);
            }
            props.setFocusedWorkloadGroupScope(nextFocusedScope);
          };
          const groupRowInteraction = createSummaryInteractiveRowPreviewHandlers({
            onPreview: () => handleGroupHoverChange(groupSummaryScope()),
            onPreviewClear: () => handleGroupHoverChange(null),
          });

          return (
            <>
              <Show when={props.groupingMode() === 'grouped'}>
                <Show
                  when={node()}
                  fallback={
                    <TableRow
                      class={getInteractiveGroupedTableRowClass()}
                      data-summary-group-id={groupKey()}
                      data-summary-group-series-count={String(
                        groupSummaryScope()?.seriesIds.length ?? 0,
                      )}
                      data-summary-row-active={isSummaryGroupHighlighted() ? 'true' : 'false'}
                      onClick={handleGroupFocusToggle}
                      {...groupRowInteraction}
                    >
                      <TableCell
                        colspan={props.totalColumns()}
                        class={getGroupedTableRowCellClass()}
                      >
                        {(() => {
                          const label = props.getGroupLabel(groupKey(), fullGroupGuests());
                          return (
                            <div class="flex items-center gap-3">
                              <span>{label.name}</span>
                              <Show when={label.type}>
                                <span class={GROUPED_TABLE_ROW_BADGE_CLASS}>{label.type}</span>
                              </Show>
                            </div>
                          );
                        })()}
                      </TableCell>
                    </TableRow>
                  }
                >
                  <NodeGroupHeader
                    node={node()!}
                    renderAs="tr"
                    colspan={props.totalColumns()}
                    columns={props.workloadTableVisibleColumns()}
                    columnCellClass={getGroupNodeColumnCellClass}
                    renderColumnCell={renderGroupNodeColumnCell}
                    showFactsInName={!props.workloadTableVisibleColumnIds().includes('info')}
                    trClass="cursor-pointer select-none duration-150"
                    trProps={{
                      'aria-expanded': shouldShowNodeDrawer() ? 'true' : 'false',
                      'data-summary-group-id': groupKey(),
                      'data-summary-group-series-count': String(
                        groupSummaryScope()?.seriesIds.length ?? 0,
                      ),
                      'data-summary-row-active': isSummaryGroupHighlighted() ? 'true' : 'false',
                      onClick: handleGroupFocusToggle,
                      ...groupRowInteraction,
                    }}
                  />
                </Show>
                <Show when={shouldShowNodeDrawer()}>
                  <TableRow data-inline-node-detail-for={groupKey()}>
                    <TableCell
                      colspan={props.totalColumns()}
                      class="p-0 border-b border-border bg-surface-alt"
                    >
                      <div
                        class="px-2 sm:px-4 py-3 sm:py-4"
                        onClick={(event) => event.stopPropagation()}
                      >
                        <NodeDrawer node={node()!} />
                      </div>
                    </TableCell>
                  </TableRow>
                </Show>
              </Show>
              <Index each={groupGuests()} fallback={<></>}>
                {(guest) => {
                  const guestId = createMemo(() => getCanonicalWorkloadId(guest()));
                  const detailControlsId = createMemo(() =>
                    buildSummaryDisclosureControlsId(guestId()),
                  );
                  const metadata = () =>
                    props.guestMetadata()[guestId()] ||
                    props.guestMetadata()[`${guest().instance}:${guest().node}:${guest().vmid}`];
                  const parentNode = () => node() ?? props.guestParentNodeMap()[guestId()];
                  const parentNodeOnline = () => {
                    const pn = parentNode();
                    return pn ? isNodeOnline(pn) : true;
                  };

                  return (
                    <ComponentErrorBoundary name="GuestRow">
                      <GuestRow
                        guest={guest()}
                        alertStyles={getAlertStyles(
                          guestId(),
                          props.activeAlerts,
                          props.alertsEnabled(),
                        )}
                        customUrl={metadata()?.customUrl}
                        onTagClick={props.handleTagClick}
                        activeSearch={props.search()}
                        parentNodeOnline={parentNodeOnline()}
                        onCustomUrlUpdate={props.handleCustomUrlUpdate}
                        isGroupedView={props.groupingMode() === 'grouped'}
                        visibleColumnIds={props.workloadTableVisibleColumnIds()}
                        workloadTableLayoutMode={props.workloadTableLayoutMode()}
                        onClick={() =>
                          props.setSelectedGuestId(
                            props.selectedGuestId() === guestId() ? null : guestId(),
                          )
                        }
                        isExpanded={props.selectedGuestId() === guestId()}
                        isSummaryHighlighted={props.activeSummaryWorkloadId() === guestId()}
                        summaryGroupMemberState={resolveSummaryGroupMemberInteractionState({
                          seriesId: guestId(),
                          hoveredGroupScope: props.hoveredSummaryWorkloadGroupScope(),
                          focusedGroupScope: props.focusedSummaryWorkloadGroupScope(),
                        })}
                        ioEmphasis={props.workloadIOEmphasis()}
                        metricDisplayMode={props.workloadMetricDisplayMode()}
                        metricHistory={props.workloadMetricHistory}
                        onHoverChange={props.setHoveredWorkloadId}
                      />
                      <Show when={props.selectedGuestId() === guestId()}>
                        <TableRow data-inline-detail-for={guestId()}>
                          <TableCell
                            id={detailControlsId()}
                            colspan={props.totalColumns()}
                            class="p-0 border-b border-border bg-surface-alt"
                          >
                            <div
                              class="px-2 sm:px-4 py-3 sm:py-4"
                              onClick={(event) => event.stopPropagation()}
                            >
                              <GuestDrawer
                                guest={guest()}
                                onClose={() => props.setSelectedGuestId(null)}
                                customUrl={metadata()?.customUrl}
                                onCustomUrlChange={props.handleCustomUrlUpdate}
                              />
                            </div>
                          </TableCell>
                        </TableRow>
                      </Show>
                    </ComponentErrorBoundary>
                  );
                }}
              </Index>
            </>
          );
        }}
      </Index>
      <Show when={props.groupedWindowing.isWindowed() && props.bottomSpacerHeight() > 0}>
        <TableRow aria-hidden="true">
          <TableCell colspan={props.totalColumns()} class="p-0 border-0">
            <svg
              aria-hidden="true"
              width="1"
              height={String(props.bottomSpacerHeight())}
              class="block w-px pointer-events-none"
            />
          </TableCell>
        </TableRow>
      </Show>
    </TableBody>
  );
}
