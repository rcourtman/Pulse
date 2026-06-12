import {
  For,
  Show,
  createMemo,
  createSignal,
  type Accessor,
  type Component,
  type JSX,
} from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { getAlertStyles } from '@/utils/alerts';
import { StatusDot } from '@/components/shared/StatusDot';
import { WebInterfaceNameLink } from '@/components/shared/WebInterfaceNameLink';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { NodeDrawer } from '@/components/Workloads/NodeDrawer';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { MetricMiniSparkline } from '@/components/Workloads/MetricMiniSparkline';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getNodeExternalUrl } from '@/utils/nodes';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatUptime, normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { useWorkloadTableMetricHistory } from '@/components/Workloads/useWorkloadTableMetricHistory';
import { getWorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import {
  PlatformTableEmptyState,
  PlatformTableShell,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
import { type WorkloadsMetricDisplayMode } from '@/components/Workloads/workloadsFilterModel';
import { type WorkloadTableMetricHistoryRange } from '@/components/Workloads/workloadMetricHistoryModel';
import type { Disk, Node as LegacyNode } from '@/types/api';
import type { Resource } from '@/types/resource';
import { nodeFromResource } from '@/utils/resourceStateAdapters';
import {
  getResourceClusterLabel,
  getResourceNodeName,
  getResourceVersion,
} from './proxmoxPageModel';
import {
  getProxmoxHostColumnWidthStyle,
  getProxmoxHostTableMinWidthClass,
  getProxmoxHostVisibleColumnsForLayout,
  type ProxmoxHostTableColumn,
  type ProxmoxHostTableColumnId,
} from './proxmoxHostTableModel';

// Proxmox Overview mirrors the v5 Dashboard layout: a dedicated nodes table on
// top, the canonical Workloads filter + guest table below. The nodes table
// uses the canonical metric primitives (ResponsiveMetricCell / StackedMemoryBar
// / StackedDiskBar / TemperatureGauge) so the bars, severity coloring, and
// sparkline overlays match the rest of the platform-first surfaces. v5's
// NodeSummaryTable didn't have a search box or status chip strip — node lists
// are short and the Workloads filter below covers the place where filtering
// actually matters, so this table renders the rows directly. The bars /
// sparklines toggle in the workloads filter is shared by the page-level
// metricDisplayMode signal so the hosts table swaps to MetricMiniSparkline
// whenever the user flips the toggle.

const formatNodeUptime = (seconds: number | undefined): { label: string; warn: boolean } => {
  if (!seconds || seconds <= 0) return { label: '—', warn: false };
  // Full "26d 4h" precision matches v5 and the guest rows below; <1h keeps
  // the v5 "recently restarted" highlight.
  return { label: formatUptime(seconds), warn: seconds < 3_600 };
};

type GuestCounts = { vms: number; containers: number };

const countGuestsForNode = (guests: Resource[], nodeName: string): GuestCounts => {
  const counts: GuestCounts = { vms: 0, containers: 0 };
  for (const guest of guests) {
    if (getResourceNodeName(guest) !== nodeName) continue;
    if (guest.type === 'vm') counts.vms += 1;
    else if (guest.type === 'system-container' || guest.type === 'oci-container') {
      counts.containers += 1;
    }
  }
  return counts;
};

const VMS_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-sky-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-sky-700 dark:bg-sky-900/40 dark:text-sky-300';
const CTS_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-violet-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-violet-700 dark:bg-violet-900/40 dark:text-violet-300';
const ZERO_BADGE =
  'inline-flex min-w-[2rem] justify-center items-center rounded-md bg-surface-alt px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-muted';

// Shim a canonical Resource into the legacy Node shape that
// `useWorkloadTableMetricHistory.getNodeMetricSeries` uses for its chart-key
// candidate lookups. The lookup only reads `id`, `linkedAgentId`, `name`, and
// `instance`, so a minimal projection is enough; everything else is left at
// its harmless default. Field-cast through a Partial → unknown → LegacyNode
// chain because the legacy Node type carries dozens of optional shape fields
// the table doesn't need to satisfy here.
const projectResourceToLegacyNode = (resource: Resource): LegacyNode => {
  const proxmoxMeta = resource.proxmox ?? {};
  const projected: Partial<LegacyNode> & {
    id: string;
    name: string;
    instance: string;
    linkedAgentId?: string;
  } = {
    id: resource.id,
    name: resource.name,
    instance: proxmoxMeta.instance ?? '',
    linkedAgentId: resource.agent?.agentId ?? undefined,
  };
  return projected as unknown as LegacyNode;
};

const formatPercentLabel = (value: number | null | undefined): string => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—';
  const normalized = value <= 1 ? value * 100 : value;
  return `${Math.round(Math.max(0, normalized))}%`;
};

type HostSortKey = ProxmoxHostTableColumnId;

const TEXT_SORT_KEYS = new Set<ProxmoxHostTableColumnId>(['node', 'version', 'cluster']);

const getHostSortValue = (
  node: Resource,
  guests: Resource[],
  key: HostSortKey,
): string | number | null => {
  switch (key) {
    case 'node':
      return asTrimmedString(node.name) || node.id;
    case 'version':
      return asTrimmedString(getResourceVersion(node)) || null;
    case 'cluster':
      return getResourceClusterLabel(node);
    case 'cpu':
      return node.cpu?.current ?? null;
    case 'memory': {
      const total = node.memory?.total ?? 0;
      if (total > 0) return ((node.memory?.used ?? 0) / total) * 100;
      return typeof node.memory?.current === 'number' ? node.memory.current : null;
    }
    case 'disk':
      return node.disk?.current ?? null;
    case 'temp':
      return typeof node.temperature === 'number' && node.temperature > 0 ? node.temperature : null;
    case 'uptime':
      return node.uptime ?? null;
    case 'vms':
      return countGuestsForNode(guests, getResourceNodeName(node)).vms;
    case 'cts':
      return countGuestsForNode(guests, getResourceNodeName(node)).containers;
    default:
      key satisfies never;
      return null;
  }
};

const isEmptyHostSortValue = (value: string | number | null): boolean =>
  value === null || (typeof value === 'number' && Number.isNaN(value));

const compareHostSortValues = (a: string | number | null, b: string | number | null): number => {
  const aEmpty = isEmptyHostSortValue(a);
  const bEmpty = isEmptyHostSortValue(b);
  if (aEmpty && bEmpty) return 0;
  if (aEmpty) return 1;
  if (bEmpty) return -1;
  if (typeof a === 'number' && typeof b === 'number') return a === b ? 0 : a < b ? -1 : 1;
  return String(a).localeCompare(String(b), undefined, { sensitivity: 'base' });
};

export const ProxmoxNodesTable: Component<{
  nodes: Resource[];
  guests: Resource[];
  metricDisplayMode?: Accessor<WorkloadsMetricDisplayMode>;
  metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const breakpoint = useBreakpoint();
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const [selectedNodeId, setSelectedNodeId] = createSignal<string | null>(null);
  const layoutMode = createMemo(() => getWorkloadTableLayoutMode(breakpoint.width()));
  const visibleColumns = createMemo(() => getProxmoxHostVisibleColumnsForLayout(layoutMode()));
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const displayMode = () => props.metricDisplayMode?.() ?? 'bars';
  const isSparklineMode = () => displayMode() === 'sparklines';
  const [sortKey, setSortKey] = createSignal<HostSortKey | null>(null);
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const handleSort = (column: ProxmoxHostTableColumnId) => {
    const key = column as HostSortKey;
    const defaultDirection = TEXT_SORT_KEYS.has(key) ? 'asc' : 'desc';
    // Cycle: default direction → flipped → cleared.
    if (sortKey() === key) {
      if (sortDirection() === defaultDirection) {
        setSortDirection(defaultDirection === 'asc' ? 'desc' : 'asc');
      } else {
        setSortKey(null);
        setSortDirection('asc');
      }
      return;
    }
    setSortKey(key);
    setSortDirection(defaultDirection);
  };

  const sortedNodes = createMemo(() => {
    const key = sortKey();
    if (!key) return props.nodes;
    const dir = sortDirection() === 'asc' ? 1 : -1;
    const decorated = props.nodes.map(
      (node) => [getHostSortValue(node, props.guests, key), node] as const,
    );
    decorated.sort(([a], [b]) => {
      // Missing values stay last in either direction.
      if (isEmptyHostSortValue(a) || isEmptyHostSortValue(b)) return compareHostSortValues(a, b);
      return compareHostSortValues(a, b) * dir;
    });
    return decorated.map(([, node]) => node);
  });

  // Use the same canonical history reader the workloads table uses; cache
  // keys collide so the two readers dedupe their fetches.
  const metricHistory = useWorkloadTableMetricHistory({
    enabled: isSparklineMode,
    range: () => props.metricHistoryRange?.() ?? '1h',
    selectedNode: () => '',
  });

  return (
    <Show
      when={props.nodes.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <PlatformTableShell
        title="Nodes"
        tableClass={`${getProxmoxHostTableMinWidthClass(layoutMode())} table-fixed text-xs`}
        colgroup={
          <colgroup>
            <For each={visibleColumns()}>
              {(column) => (
                <col
                  style={getProxmoxHostColumnWidthStyle(
                    column.id,
                    layoutMode(),
                    visibleColumnIds(),
                  )}
                />
              )}
            </For>
          </colgroup>
        }
        header={
          <For each={visibleColumns()}>
            {(column) => (
              <TableHead
                class={`${getPlatformTableHeadClassForKind(column.kind)} cursor-pointer hover:bg-surface-hover`}
                aria-sort={
                  sortKey() === column.id
                    ? sortDirection() === 'asc'
                      ? 'ascending'
                      : 'descending'
                    : undefined
                }
                onClick={() => handleSort(column.id)}
              >
                {column.label}
                {sortKey() === column.id && (sortDirection() === 'asc' ? ' ▲' : ' ▼')}
              </TableHead>
            )}
          </For>
        }
        body={
          <For each={sortedNodes()}>
            {(node) => {
              const name = () => asTrimmedString(node.name) || node.id;
              const drawerNode = createMemo(() => nodeFromResource(node));
              const detailRowId = () => `proxmox-host-drawer-${node.id}`;
              const isSelected = () => selectedNodeId() === node.id;
              const toggleNodeDrawer = () =>
                setSelectedNodeId((current) => (current === node.id ? null : node.id));
              const handleActivationKey: JSX.EventHandler<HTMLTableRowElement, KeyboardEvent> = (
                event,
              ) => {
                if (event.key !== 'Enter' && event.key !== ' ') return;
                event.preventDefault();
                toggleNodeDrawer();
              };
              const version = () => asTrimmedString(getResourceVersion(node));
              const cluster = () => getResourceClusterLabel(node);
              const counts = () => countGuestsForNode(props.guests, getResourceNodeName(node));
              const indicator = () => getSimpleStatusIndicator(node.status);
              const isOnline = () => indicator().variant === 'success';
              const uptime = () => formatNodeUptime(node.uptime);
              const metricsKey = () => buildMetricKeyForUnifiedResource(node);
              const temperature = () => node.temperature;
              const cpuPercent = () => node.cpu?.current ?? 0;
              const memoryUsed = () => node.memory?.used ?? 0;
              const memoryTotal = () => node.memory?.total ?? 0;
              const memoryPercent = () =>
                memoryTotal() > 0
                  ? (memoryUsed() / memoryTotal()) * 100
                  : typeof node.memory?.current === 'number'
                    ? node.memory.current
                    : 0;
              const memoryPercentOnly = () =>
                !memoryTotal() && typeof node.memory?.current === 'number'
                  ? node.memory.current
                  : undefined;
              const diskPercent = () => node.disk?.current ?? 0;
              const aggregateDisk = (): Disk | undefined =>
                node.disk
                  ? ({
                      total: node.disk.total ?? 0,
                      used: node.disk.used ?? 0,
                      free: node.disk.free ?? 0,
                      usage: node.disk.current ?? 0,
                    } as Disk)
                  : undefined;
              const legacyNode = () => projectResourceToLegacyNode(node);
              const externalUrl = () => {
                const shimmed = drawerNode();
                return shimmed ? getNodeExternalUrl(shimmed) : '';
              };
              const pendingUpdates = () => drawerNode()?.pendingUpdates ?? 0;
              const alertStyles = createMemo(() =>
                getAlertStyles(node.id, activeAlerts, alertsEnabled()),
              );
              const alertAccentTone = createMemo<
                'critical' | 'warning' | 'acknowledged' | undefined
              >(() => {
                const styles = alertStyles();
                if (styles.hasUnacknowledgedAlert) {
                  return styles.severity === 'critical' ? 'critical' : 'warning';
                }
                return styles.hasAcknowledgedOnlyAlert ? 'acknowledged' : undefined;
              });
              const rowAlertBg = () => {
                const styles = alertStyles();
                if (!styles.hasUnacknowledgedAlert) return '';
                return styles.severity === 'critical'
                  ? 'bg-red-50 dark:bg-red-950'
                  : 'bg-yellow-50 dark:bg-yellow-950';
              };
              const cpuSeries = () => metricHistory.getNodeMetricSeries(legacyNode(), 'cpu');
              const memorySeries = () => metricHistory.getNodeMetricSeries(legacyNode(), 'memory');
              const diskSeries = () => metricHistory.getNodeMetricSeries(legacyNode(), 'disk');
              const renderColumnCell = (column: ProxmoxHostTableColumn): JSX.Element => {
                switch (column.id) {
                  case 'node':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <div class="flex min-w-0 items-center gap-2">
                          <PlatformResourceDetailToggleButton
                            expanded={isSelected()}
                            resourceLabel={name()}
                            controlsId={detailRowId()}
                            onToggle={toggleNodeDrawer}
                          />
                          <StatusDot
                            size="sm"
                            variant={indicator().variant}
                            title={node.status || 'unknown'}
                            ariaHidden
                          />
                          <WebInterfaceNameLink
                            name={name()}
                            url={externalUrl()}
                            class="truncate font-semibold text-base-content transition-colors hover:text-sky-600 dark:hover:text-sky-400"
                            fallbackClass="truncate font-semibold text-base-content"
                            title={`Open ${name()} web interface`}
                          />
                          <Show when={isOnline() && pendingUpdates() > 0}>
                            <span
                              class={`rounded px-1 py-0 text-[9px] font-medium whitespace-nowrap ${
                                pendingUpdates() >= 10
                                  ? 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400'
                                  : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-400'
                              }`}
                              title={`${pendingUpdates()} pending apt update${pendingUpdates() !== 1 ? 's' : ''}`}
                            >
                              {pendingUpdates()} updates
                            </span>
                          </Show>
                        </div>
                      </TableCell>
                    );
                  case 'version':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <Show when={version()} fallback={<span class="text-muted">—</span>}>
                          <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 font-mono text-[10px] text-base-content">
                            {version()}
                          </span>
                        </Show>
                      </TableCell>
                    );
                  case 'uptime':
                    return (
                      <TableCell
                        class={`${getPlatformTableCellClassForKind(column.kind)} tabular-nums ${
                          uptime().warn
                            ? 'text-orange-600 dark:text-orange-400'
                            : 'text-base-content'
                        }`}
                      >
                        {uptime().label}
                      </TableCell>
                    );
                  case 'cpu':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <Show
                          when={isSparklineMode()}
                          fallback={
                            <ResponsiveMetricCell
                              class="w-full"
                              value={cpuPercent()}
                              type="cpu"
                              resourceId={metricsKey()}
                              isRunning={isOnline()}
                              showMobile={false}
                            />
                          }
                        >
                          <MetricMiniSparkline
                            series={cpuSeries()}
                            valueLabel={formatPercentLabel(cpuPercent())}
                            title={`${name()} CPU history`}
                          />
                        </Show>
                      </TableCell>
                    );
                  case 'memory':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <Show
                          when={isSparklineMode()}
                          fallback={
                            <Show
                              when={
                                isOnline() && (memoryTotal() > 0 || memoryPercentOnly() != null)
                              }
                              fallback={
                                <div class="flex justify-center">
                                  <span class="text-xs text-muted" aria-hidden="true">
                                    —
                                  </span>
                                </div>
                              }
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                                cache={drawerNode()?.memory?.cache || 0}
                                swapUsed={drawerNode()?.memory?.swapUsed || 0}
                                swapTotal={drawerNode()?.memory?.swapTotal || 0}
                              />
                            </Show>
                          }
                        >
                          <MetricMiniSparkline
                            series={memorySeries()}
                            valueLabel={formatPercentLabel(memoryPercent())}
                            title={`${name()} memory history`}
                          />
                        </Show>
                      </TableCell>
                    );
                  case 'disk':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <Show
                          when={isSparklineMode()}
                          fallback={
                            <Show
                              when={isOnline() && (aggregateDisk() || node.agent?.disks?.length)}
                              fallback={
                                <div class="flex justify-center">
                                  <span class="text-xs text-muted" aria-hidden="true">
                                    —
                                  </span>
                                </div>
                              }
                            >
                              <StackedDiskBar
                                mode={
                                  (node.agent?.disks?.length ?? 0) > 1 ? 'vertical-bars' : undefined
                                }
                                disks={normalizeDiskArray(node.agent?.disks)}
                                aggregateDisk={aggregateDisk()}
                              />
                            </Show>
                          }
                        >
                          <MetricMiniSparkline
                            series={diskSeries()}
                            valueLabel={formatPercentLabel(diskPercent())}
                            title={`${name()} disk history`}
                          />
                        </Show>
                      </TableCell>
                    );
                  case 'temp':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <Show
                          when={typeof temperature() === 'number' && (temperature() as number) > 0}
                          fallback={<span class="text-xs text-muted">—</span>}
                        >
                          <TemperatureGauge value={temperature() as number} />
                        </Show>
                      </TableCell>
                    );
                  case 'vms':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <span class={counts().vms > 0 ? VMS_BADGE : ZERO_BADGE}>
                          {counts().vms}
                        </span>
                      </TableCell>
                    );
                  case 'cts':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <span class={counts().containers > 0 ? CTS_BADGE : ZERO_BADGE}>
                          {counts().containers}
                        </span>
                      </TableCell>
                    );
                  case 'cluster':
                    return (
                      <TableCell class={getPlatformTableCellClassForKind(column.kind)}>
                        <span class="inline-flex items-center rounded-md bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content">
                          {cluster()}
                        </span>
                      </TableCell>
                    );
                  default:
                    column.id satisfies never;
                    return <></>;
                }
              };

              return (
                <>
                  <TableRow
                    class={`host-row cursor-pointer text-[11px] outline-none sm:text-xs ${
                      isSelected() ? 'bg-surface-hover' : rowAlertBg()
                    } ${isOnline() ? '' : 'opacity-60'} focus-visible:ring-2 focus-visible:ring-blue-500/60 focus-visible:ring-offset-1 focus-visible:ring-offset-surface`}
                    aria-controls={isSelected() ? detailRowId() : undefined}
                    aria-expanded={isSelected() ? 'true' : 'false'}
                    data-proxmox-host-row={node.id}
                    data-workload-alert-accent={alertAccentTone()}
                    onClick={toggleNodeDrawer}
                    onKeyDown={handleActivationKey}
                    tabIndex={0}
                  >
                    <For each={visibleColumns()}>{(column) => renderColumnCell(column)}</For>
                  </TableRow>
                  <Show when={isSelected() && drawerNode()}>
                    {(selectedNode) => (
                      <InlineDetailTableRow
                        cellId={detailRowId()}
                        colspan={visibleColumns().length}
                        data-inline-node-detail-for={node.id}
                      >
                        <NodeDrawer
                          node={selectedNode()}
                          disks={normalizeDiskArray(node.agent?.disks)}
                          discoveryTarget={(() => {
                            const config = toDiscoveryConfig(node);
                            return config
                              ? { agentId: config.agentId, hostname: config.hostname }
                              : undefined;
                          })()}
                        />
                      </InlineDetailTableRow>
                    )}
                  </Show>
                </>
              );
            }}
          </For>
        }
      />
    </Show>
  );
};

export default ProxmoxNodesTable;
