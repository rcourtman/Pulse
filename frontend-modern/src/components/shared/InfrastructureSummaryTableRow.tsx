import { Show, Suspense, createMemo, lazy } from 'solid-js';
import type { Component } from 'solid-js';
import type { Agent, Node, PBSInstance } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { getAlertStyles } from '@/utils/alerts';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { formatTemperature } from '@/utils/temperature';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import { getNodeStatusIndicator, getPBSStatusIndicator } from '@/utils/status';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  getInfrastructureSummaryCountValue,
  getInfrastructureSummaryCpuPercent,
  getInfrastructureSummaryCpuTemperatureValue,
  getInfrastructureSummaryDiskPercent,
  getInfrastructureSummaryDiskSublabel,
  getInfrastructureSummaryMemoryPercent,
  getInfrastructureSummaryMetricsKey,
  isInfrastructureSummaryItemOnline,
  isPVE,
  resolveInfrastructureSummaryLinkedAgent,
  type InfrastructureSummaryTableProps,
  type TableItem,
} from './infrastructureSummaryTableModel';
import type { InfrastructureSummaryTableState } from './useInfrastructureSummaryTableState';

const InfrastructureDetailsDrawer = lazy(() =>
  import('./InfrastructureDetailsDrawer').then((module) => ({
    default: module.InfrastructureDetailsDrawer,
  })),
);

const tdClass = 'px-2 py-1 align-middle';
const metricColumnStyle = { 'min-width': '140px', 'max-width': '180px' } as const;

interface InfrastructureSummaryTableRowProps {
  item: TableItem;
  table: InfrastructureSummaryTableState;
  tableProps: InfrastructureSummaryTableProps;
}

export const InfrastructureSummaryTableRow: Component<InfrastructureSummaryTableRowProps> = (
  props,
) => {
  const isPVEItem = isPVE(props.item);
  const isPBSItem = !isPVEItem;
  const node = () => (isPVEItem ? (props.item as Node) : null);
  const pbs = () => (isPBSItem ? (props.item as PBSInstance) : null);
  const online = createMemo(() => isInfrastructureSummaryItemOnline(props.item));
  const nodeId = () => (isPVEItem ? node()!.id : pbs()!.name);
  const resourceId = () => (isPVEItem ? node()!.id || node()!.name : pbs()!.id || pbs()!.name);
  const isSelected = createMemo(() => props.tableProps.selectedNode === nodeId());
  const isExpanded = createMemo(() => props.table.isExpandedNode(nodeId()));

  const statusIndicator = createMemo(() =>
    isPVEItem ? getNodeStatusIndicator(node() as Node) : getPBSStatusIndicator(pbs() as PBSInstance),
  );
  const cpuPercentValue = createMemo(() => getInfrastructureSummaryCpuPercent(props.item));
  const memoryPercentValue = createMemo(() => getInfrastructureSummaryMemoryPercent(props.item));
  const diskPercentValue = createMemo(() => getInfrastructureSummaryDiskPercent(props.item));
  const diskSublabel = createMemo(() => getInfrastructureSummaryDiskSublabel(props.item));
  const cpuTemperatureValue = createMemo(() => getInfrastructureSummaryCpuTemperatureValue(props.item));
  const uptimeValue = createMemo(() => props.item.uptime ?? 0);
  const displayName = () => (isPVEItem ? getNodeDisplayName(node() as Node) : pbs()!.name);
  const showActualName = () => isPVEItem && hasAlternateDisplayName(node() as Node);
  const linkedAgentForDrawer = (): Agent | undefined =>
    isPVEItem && node()
      ? resolveInfrastructureSummaryLinkedAgent(node() as Node, props.tableProps.agents)
      : undefined;
  const metricsKey = createMemo(() => getInfrastructureSummaryMetricsKey(props.item));
  const alertStyles = createMemo(() =>
    getAlertStyles(resourceId(), props.table.activeAlerts, props.table.alertsEnabled()),
  );
  const showAlertHighlight = createMemo(
    () => alertStyles().hasUnacknowledgedAlert && online(),
  );

  const rowStyle = createMemo(() => {
    const styles: Record<string, string> = {};
    const shadows: string[] = [];

    if (showAlertHighlight()) {
      const color = alertStyles().severity === 'critical' ? '#ef4444' : '#eab308';
      shadows.push(`inset 4px 0 0 0 ${color}`);
    }

    if (isSelected()) {
      shadows.push('0 0 0 1px rgba(59, 130, 246, 0.5)');
      shadows.push('0 2px 4px -1px rgba(0, 0, 0, 0.1)');
    }

    if (shadows.length > 0) {
      styles['box-shadow'] = shadows.join(', ');
    }

    return styles;
  });

  const rowClass = createMemo(() => {
    const baseHover =
      'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';

    if (isSelected()) {
      return `${baseHover} z-10 bg-blue-50 dark:bg-blue-900`;
    }

    if (showAlertHighlight()) {
      const alertBg =
        alertStyles().severity === 'critical'
          ? 'bg-red-50 dark:bg-red-950'
          : 'bg-yellow-50 dark:bg-yellow-950';
      return `${baseHover} ${alertBg}`;
    }

    let className = baseHover;
    if (props.tableProps.selectedNode && props.tableProps.selectedNode !== nodeId()) {
      className += ' opacity-50 hover:opacity-80';
    }
    if (!online()) {
      className += ' opacity-60';
    }
    return className;
  });

  return (
    <>
      <TableRow
        class={rowClass()}
        style={{ ...rowStyle(), 'min-height': '36px' }}
        onClick={() => props.tableProps.onNodeClick(nodeId(), isPVEItem ? 'pve' : 'pbs')}
      >
        <TableCell
          class={`pr-2 py-1 align-middle overflow-hidden ${showAlertHighlight() ? 'pl-4' : 'pl-3'}`}
        >
          <div class="flex items-center gap-1.5 min-w-0">
            <Show when={isPVEItem}>
              <div
                class={`cursor-pointer transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}
                onClick={(event) => props.table.toggleNodeExpand(nodeId(), event)}
              >
                <svg
                  class="w-3.5 h-3.5 hover:text-base-content"
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
            </Show>
            <StatusDot
              variant={statusIndicator().variant}
              title={statusIndicator().label}
              ariaLabel={statusIndicator().label}
              size="xs"
            />
            <span
              class="font-medium text-[11px] text-base-content whitespace-nowrap select-text"
              title={displayName()}
            >
              {displayName()}
            </span>
            <Show when={showActualName()}>
              <span class="text-[9px] text-muted whitespace-nowrap">({node()!.name})</span>
            </Show>
            <div class="hidden xl:flex items-center gap-1.5 ml-1 flex-shrink min-w-0 overflow-hidden">
              <Show when={isPVEItem}>
                <span class="text-[9px] px-1 py-0 rounded font-medium bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400">
                  PVE
                </span>
              </Show>
              <Show when={isPVEItem && node()!.pveVersion}>
                <span class="text-[9px] text-muted whitespace-nowrap">
                  v{node()!.pveVersion.split('/')[1] || node()!.pveVersion}
                </span>
              </Show>
              <Show when={isPVEItem && node()!.isClusterMember !== undefined}>
                <span
                  class={`text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap ${
                    node()!.isClusterMember
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-400'
                      : 'bg-surface-alt text-muted'
                  }`}
                >
                  {node()!.isClusterMember ? node()!.clusterName : 'Standalone'}
                </span>
              </Show>
              <Show when={isPVEItem && node()!.linkedAgentId}>
                <span
                  class="text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-400"
                  title="Pulse agent installed for enhanced metrics"
                >
                  +Agent
                </span>
              </Show>
              <Show
                when={
                  isPVEItem &&
                  online() &&
                  node()!.pendingUpdates !== undefined &&
                  node()!.pendingUpdates > 0
                }
              >
                <span
                  class={`text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap ${
                    (node()!.pendingUpdates ?? 0) >= 10
                      ? 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400'
                      : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-400'
                  }`}
                  title={`${node()!.pendingUpdates} pending apt update${node()!.pendingUpdates !== 1 ? 's' : ''}`}
                >
                  {node()!.pendingUpdates} updates
                </span>
              </Show>
              <Show when={isPBSItem}>
                <span class="text-[9px] px-1 py-0 rounded font-medium bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-400">
                  PBS
                </span>
              </Show>
              <Show when={isPBSItem && pbs()!.version}>
                <span class="text-[9px] text-muted whitespace-nowrap">v{pbs()!.version}</span>
              </Show>
            </div>
          </div>
        </TableCell>

        <TableCell class={tdClass}>
          <div class="flex justify-center">
            <span
              class={`text-xs whitespace-nowrap ${
                isPVEItem && uptimeValue() < 3600 ? 'text-orange-500' : 'text-muted'
              }`}
            >
              <Show when={online() && uptimeValue()} fallback="-">
                <Show when={props.table.isMobile()} fallback={formatUptime(uptimeValue())}>
                  {formatUptime(uptimeValue(), true)}
                </Show>
              </Show>
            </span>
          </div>
        </TableCell>

        <TableCell
          class={tdClass}
          style={props.table.isMobile() ? { 'min-width': '80px' } : metricColumnStyle}
        >
          <div class="h-5">
            <EnhancedCPUBar
              usage={cpuPercentValue()}
              loadAverage={isPVEItem ? node()!.loadAverage?.[0] : undefined}
              cores={props.table.isMobile() ? undefined : isPVEItem ? node()!.cpuInfo?.cores : undefined}
              model={isPVEItem ? node()!.cpuInfo?.model : undefined}
              resourceId={metricsKey()}
            />
          </div>
        </TableCell>

        <TableCell
          class={tdClass}
          style={props.table.isMobile() ? { 'min-width': '80px' } : metricColumnStyle}
        >
          <div class="h-5">
            <Show
              when={isPVEItem}
              fallback={
                <ResponsiveMetricCell
                  value={memoryPercentValue()}
                  type="memory"
                  resourceId={metricsKey()}
                  sublabel={
                    pbs()!.memoryTotal
                      ? `${formatBytes(pbs()!.memoryUsed)}/${formatBytes(pbs()!.memoryTotal)}`
                      : undefined
                  }
                  isRunning={online()}
                  showMobile={false}
                />
              }
            >
              <StackedMemoryBar
                used={node()!.memory?.used || 0}
                total={node()!.memory?.total || 0}
                balloon={node()!.memory?.balloon || 0}
                swapUsed={node()!.memory?.swapUsed || 0}
                swapTotal={node()!.memory?.swapTotal || 0}
                resourceId={metricsKey()}
              />
            </Show>
          </div>
        </TableCell>

        <TableCell
          class={tdClass}
          style={props.table.isMobile() ? { 'min-width': '80px' } : metricColumnStyle}
        >
          <div class="h-5">
            <Show
              when={isPVEItem}
              fallback={
                <ResponsiveMetricCell
                  value={diskPercentValue()}
                  type="disk"
                  resourceId={metricsKey()}
                  sublabel={diskSublabel()}
                  isRunning={online()}
                  showMobile={false}
                />
              }
            >
              <StackedDiskBar
                aggregateDisk={{
                  total: node()!.disk?.total || 0,
                  used: node()!.disk?.used || 0,
                  free: (node()!.disk?.total || 0) - (node()!.disk?.used || 0),
                  usage: node()!.disk?.total ? node()!.disk!.used / node()!.disk!.total : 0,
                }}
              />
            </Show>
          </div>
        </TableCell>

        <Show when={props.table.hasAnyTemperatureData()}>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <Show
                when={
                  online() &&
                  isPVEItem &&
                  cpuTemperatureValue() !== null &&
                  (node()!.temperature?.hasCPU ??
                    node()!.temperature?.hasGPU ??
                    node()!.temperature?.available) &&
                  props.table.isTemperatureMonitoringEnabled(node()!)
                }
                fallback={<span class="text-xs text-muted">-</span>}
              >
                {(() => {
                  const value = cpuTemperatureValue() as number;
                  const temperature = node()!.temperature;
                  const cpuMinValue =
                    typeof temperature?.cpuMin === 'number' && temperature.cpuMin > 0
                      ? temperature.cpuMin
                      : null;
                  const cpuMaxValue =
                    typeof temperature?.cpuMaxRecord === 'number' && temperature.cpuMaxRecord > 0
                      ? temperature.cpuMaxRecord
                      : null;
                  const hasMinMax = cpuMinValue !== null && cpuMaxValue !== null;
                  const gpus = temperature?.gpu ?? [];
                  const hasGPU = gpus.length > 0;

                  if (hasMinMax || hasGPU) {
                    const min =
                      typeof cpuMinValue === 'number' ? Math.round(cpuMinValue) : undefined;
                    const max =
                      typeof cpuMaxValue === 'number' ? Math.round(cpuMaxValue) : undefined;

                    return (
                      <div
                        title={`Min: ${min !== undefined ? formatTemperature(min) : '-'}, Max: ${max !== undefined ? formatTemperature(max) : '-'}${hasGPU ? `\nGPU: ${gpus.map((gpu) => formatTemperature(gpu.edge ?? gpu.junction ?? gpu.mem)).join(', ')}` : ''}`}
                      >
                        <TemperatureGauge
                          value={value}
                          min={min}
                          max={max}
                          critical={props.table.temperatureThreshold()}
                          warning={Math.max(0, props.table.temperatureThreshold() - 5)}
                        />
                      </div>
                    );
                  }

                  return (
                    <TemperatureGauge
                      value={value}
                      critical={props.table.temperatureThreshold()}
                      warning={Math.max(0, props.table.temperatureThreshold() - 5)}
                    />
                  );
                })()}
              </Show>
            </div>
          </TableCell>
        </Show>

        <Show when={props.tableProps.currentTab === 'dashboard'}>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <span class={online() ? 'text-xs text-base-content' : 'text-xs text-muted'}>
                {online()
                  ? (getInfrastructureSummaryCountValue(props.item, 'vmCount', props.tableProps) ??
                    '-')
                  : '-'}
              </span>
            </div>
          </TableCell>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <span class={online() ? 'text-xs text-base-content' : 'text-xs text-muted'}>
                {online()
                  ? (getInfrastructureSummaryCountValue(
                      props.item,
                      'containerCount',
                      props.tableProps,
                    ) ?? '-')
                  : '-'}
              </span>
            </div>
          </TableCell>
        </Show>

        <Show when={props.tableProps.currentTab === 'storage'}>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <span class={online() ? 'text-xs text-base-content' : 'text-xs text-muted'}>
                {online()
                  ? (getInfrastructureSummaryCountValue(
                      props.item,
                      'storageCount',
                      props.tableProps,
                    ) ?? '-')
                  : '-'}
              </span>
            </div>
          </TableCell>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <span class={online() ? 'text-xs text-base-content' : 'text-xs text-muted'}>
                {online()
                  ? (getInfrastructureSummaryCountValue(props.item, 'diskCount', props.tableProps) ??
                    '-')
                  : '-'}
              </span>
            </div>
          </TableCell>
        </Show>

        <Show when={props.tableProps.currentTab === 'recovery'}>
          <TableCell class={tdClass}>
            <div class="flex justify-center">
              <span class={online() ? 'text-xs text-base-content' : 'text-xs text-muted'}>
                {online()
                  ? (getInfrastructureSummaryCountValue(
                      props.item,
                      'backupCount',
                      props.tableProps,
                    ) ?? '-')
                  : '-'}
              </span>
            </div>
          </TableCell>
        </Show>

        <TableCell class="px-0 py-1 align-middle text-center">
          <Show when={isPVEItem ? node()!.guestURL || node()!.host : pbs()!.guestURL || pbs()!.host}>
            <a
              href={
                isPVEItem
                  ? node()!.guestURL || node()!.host || `https://${node()!.name}:8006`
                  : pbs()!.guestURL || pbs()!.host || `https://${pbs()!.name}:8007`
              }
              target="_blank"
              rel="noopener noreferrer"
              onClick={(event) => event.stopPropagation()}
              class="inline-flex justify-center items-center text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
              title={`Open ${displayName()} web interface`}
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                />
              </svg>
            </a>
          </Show>
        </TableCell>
      </TableRow>

      <Show when={isExpanded() && isPVEItem}>
        <TableRow>
          <TableCell
            colspan={props.table.totalColumnCount()}
            class="bg-surface-alt px-4 py-4 border-b border-border-subtle shadow-inner"
          >
            <Suspense fallback={<div class="flex justify-center p-4">Loading stats...</div>}>
              <InfrastructureDetailsDrawer node={node()!} agent={linkedAgentForDrawer()} />
            </Suspense>
          </TableCell>
        </TableRow>
      </Show>
    </>
  );
};
