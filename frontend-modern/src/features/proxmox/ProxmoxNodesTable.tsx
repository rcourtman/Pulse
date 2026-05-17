import { For, Show, createMemo, type Accessor, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCard } from '@/components/shared/TableCard';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { MetricMiniSparkline } from '@/components/Workloads/MetricMiniSparkline';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatBytes, formatPercent, normalizeDiskArray } from '@/utils/format';
import { getMetricColorRgba } from '@/utils/metricThresholds';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { useWorkloadTableMetricHistory } from '@/components/Workloads/useWorkloadTableMetricHistory';
import { getWorkloadTableLayoutMode } from '@/components/Workloads/guestRowModel';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
} from '@/features/platformPage/sharedPlatformPage';
import { type WorkloadsMetricDisplayMode } from '@/components/Workloads/workloadsFilterModel';
import { type WorkloadTableMetricHistoryRange } from '@/components/Workloads/workloadMetricHistoryModel';
import type { Disk, Node as LegacyNode } from '@/types/api';
import type { Resource } from '@/types/resource';
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

const formatUptime = (seconds: number | undefined): { label: string; warn: boolean } => {
  if (!seconds || seconds <= 0) return { label: '—', warn: false };
  const warn = seconds < 3_600; // <1h matches v5 "recently restarted" highlight
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return { label: `${days}d`, warn };
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return { label: `${hours}h`, warn };
  const mins = Math.floor(seconds / 60);
  return { label: `${mins}m`, warn };
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

const getDiskUsagePercent = (disk: Disk): number => {
  if (disk.total > 0) return (disk.used / disk.total) * 100;
  if (Number.isFinite(disk.usage)) return disk.usage <= 1 ? disk.usage * 100 : disk.usage;
  return 0;
};

const getDiskShortLabel = (disk: Disk, index: number): string => {
  const raw = (disk.mountpoint || disk.device || `Disk ${index + 1}`).trim();
  if (raw.startsWith('/dev/')) return raw.slice('/dev/'.length);
  if (raw === '/') return '/';
  if (raw.startsWith('/')) {
    const parts = raw.split('/').filter(Boolean);
    if (parts.length > 0) return parts[parts.length - 1];
  }
  return raw;
};

const getWorstDiskPercent = (disks: Disk[]): number => {
  let worst = 0;
  for (const disk of disks) {
    const pct = getDiskUsagePercent(disk);
    if (pct > worst) worst = pct;
  }
  return worst;
};

const DISK_COUNT_BADGE_BASE =
  'inline-flex items-center gap-1.5 rounded-md bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-base-content';

const ProxmoxHostDiskSubRow: Component<{
  disk: Disk;
  index: number;
  visibleColumns: ProxmoxHostTableColumn[];
}> = (subProps) => {
  const percent = () => getDiskUsagePercent(subProps.disk);
  const fillPercent = () => Math.min(percent(), 100);
  const color = () => getMetricColorRgba(percent(), 'disk');
  const label = () => getDiskShortLabel(subProps.disk, subProps.index);
  const fullPath = () =>
    subProps.disk.mountpoint || subProps.disk.device || `Disk ${subProps.index + 1}`;
  const usedLabel = () => formatBytes(subProps.disk.used);
  const totalLabel = () =>
    subProps.disk.total > 0 ? formatBytes(subProps.disk.total) : '—';

  const diskIdx = () => subProps.visibleColumns.findIndex((c) => c.id === 'disk');
  const leadingColumns = () => subProps.visibleColumns.slice(0, diskIdx());
  const remainingAfterDisk = () => subProps.visibleColumns.length - diskIdx();
  // Span the Disk column plus up to two right neighbours so the row has room
  // for label/bar/percent/size without bleeding all the way to Cluster.
  const detailSpan = () => Math.min(remainingAfterDisk(), 3);
  const trailingColumns = () => subProps.visibleColumns.slice(diskIdx() + detailSpan());

  return (
    <TableRow class="text-[11px] bg-surface-alt/30">
      <For each={leadingColumns()}>
        {(column) => (
          <TableCell class={getPlatformTableCellClass(column.align)}>&nbsp;</TableCell>
        )}
      </For>
      <TableCell
        class={`${getPlatformTableCellClass('left')} text-muted`}
        colspan={detailSpan()}
      >
        <div
          class="flex items-center gap-2 min-w-0"
          title={`${fullPath()}: ${formatPercent(percent())} (${usedLabel()} / ${totalLabel()})`}
        >
          <span class="text-muted/70 select-none">└</span>
          <span class="font-mono text-base-content truncate max-w-[80px]">{label()}</span>
          <div class="relative h-2 w-20 shrink-0 rounded bg-surface-hover overflow-hidden">
            <div
              class="absolute inset-y-0 left-0 rounded"
              style={{ width: `${fillPercent()}%`, background: color() }}
            />
          </div>
          <span class="tabular-nums text-base-content w-[36px] text-right">
            {formatPercent(percent())}
          </span>
          <span class="tabular-nums text-muted whitespace-nowrap">
            {usedLabel()} / {totalLabel()}
          </span>
        </div>
      </TableCell>
      <For each={trailingColumns()}>
        {(column) => (
          <TableCell class={getPlatformTableCellClass(column.align)}>&nbsp;</TableCell>
        )}
      </For>
    </TableRow>
  );
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
  const layoutMode = createMemo(() => getWorkloadTableLayoutMode(breakpoint.width()));
  const visibleColumns = createMemo(() => getProxmoxHostVisibleColumnsForLayout(layoutMode()));
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const displayMode = () => props.metricDisplayMode?.() ?? 'bars';
  const isSparklineMode = () => displayMode() === 'sparklines';

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
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class={`${getProxmoxHostTableMinWidthClass(layoutMode())} table-fixed text-xs`}>
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
          <TableHeader>
            <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
              <For each={visibleColumns()}>
                {(column) => (
                  <TableHead class={getPlatformTableHeadClass(column.align)}>
                    {column.label}
                  </TableHead>
                )}
              </For>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <For each={props.nodes}>
              {(node) => {
                const name = () => asTrimmedString(node.name) || node.id;
                const version = () => asTrimmedString(getResourceVersion(node));
                const cluster = () => getResourceClusterLabel(node);
                const counts = () => countGuestsForNode(props.guests, getResourceNodeName(node));
                const indicator = () => getSimpleStatusIndicator(node.status);
                const isOnline = () => indicator().variant === 'success';
                const uptime = () => formatUptime(node.uptime);
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
                const diskList = (): Disk[] => normalizeDiskArray(node.agent?.disks) ?? [];
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
                const cpuSeries = () => metricHistory.getNodeMetricSeries(legacyNode(), 'cpu');
                const memorySeries = () =>
                  metricHistory.getNodeMetricSeries(legacyNode(), 'memory');
                const diskSeries = () => metricHistory.getNodeMetricSeries(legacyNode(), 'disk');
                const renderColumnCell = (column: ProxmoxHostTableColumn): JSX.Element => {
                  switch (column.id) {
                    case 'node':
                      return (
                        <TableCell class={getPlatformTableCellClass(column.align)}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={node.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                      );
                    case 'version':
                      return (
                        <TableCell class={getPlatformTableCellClass(column.align)}>
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
                          class={`${getPlatformTableCellClass(column.align)} tabular-nums ${
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
                        <TableCell class={getPlatformTableCellClass(column.align)}>
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
                        <TableCell class={getPlatformTableCellClass(column.align)}>
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
                        <TableCell class={getPlatformTableCellClass(column.align)}>
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
                                <Show
                                  when={diskList().length > 1}
                                  fallback={
                                    <StackedDiskBar
                                      disks={diskList()}
                                      aggregateDisk={aggregateDisk()}
                                    />
                                  }
                                >
                                  <span
                                    class={DISK_COUNT_BADGE_BASE}
                                    title={`${diskList().length} disks — worst ${formatPercent(getWorstDiskPercent(diskList()))}`}
                                  >
                                    <span
                                      class="inline-block h-2 w-2 rounded-full"
                                      style={{
                                        background: getMetricColorRgba(
                                          getWorstDiskPercent(diskList()),
                                          'disk',
                                        ),
                                      }}
                                      aria-hidden="true"
                                    />
                                    {diskList().length} disks
                                  </span>
                                </Show>
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
                        <TableCell class={getPlatformTableCellClass(column.align)}>
                          <Show
                            when={
                              typeof temperature() === 'number' && (temperature() as number) > 0
                            }
                            fallback={<span class="text-xs text-muted">—</span>}
                          >
                            <TemperatureGauge value={temperature() as number} />
                          </Show>
                        </TableCell>
                      );
                    case 'vms':
                      return (
                        <TableCell class={getPlatformTableCellClass(column.align)}>
                          <span class={counts().vms > 0 ? VMS_BADGE : ZERO_BADGE}>
                            {counts().vms}
                          </span>
                        </TableCell>
                      );
                    case 'cts':
                      return (
                        <TableCell class={getPlatformTableCellClass(column.align)}>
                          <span class={counts().containers > 0 ? CTS_BADGE : ZERO_BADGE}>
                            {counts().containers}
                          </span>
                        </TableCell>
                      );
                    case 'cluster':
                      return (
                        <TableCell class={getPlatformTableCellClass(column.align)}>
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
                    <TableRow class="text-[11px] sm:text-xs">
                      <For each={visibleColumns()}>{(column) => renderColumnCell(column)}</For>
                    </TableRow>
                    <Show when={!isSparklineMode() && isOnline() && diskList().length > 1}>
                      <For each={diskList()}>
                        {(disk, index) => (
                          <ProxmoxHostDiskSubRow
                            disk={disk}
                            index={index()}
                            visibleColumns={visibleColumns()}
                          />
                        )}
                      </For>
                    </Show>
                  </>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
};

export default ProxmoxNodesTable;
