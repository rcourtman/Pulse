import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
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
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';

// TrueNAS systems are storage appliances, not generic compute hosts.
// The generic infrastructure table's CPU / Memory / Disk columns are
// helpful (the agent payload carries them), but operators also want at-
// a-glance pool count, dataset count, share count, app count, uptime, version, and
// max-sensor temperature on the same row. This bespoke table reuses
// canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) and counts the per-system children
// client-side from the same TrueNAS resource scope already fetched by
// the page (no extra API calls).

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatBytes = (bytes: number | undefined): string => {
  if (!bytes || bytes <= 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let value = bytes;
  let unitIdx = 0;
  while (value >= 1024 && unitIdx < units.length - 1) {
    value /= 1024;
    unitIdx += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIdx]}`;
};

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent))
    return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{percent.toFixed(1)}%</span>;
};

const formatTemperature = (celsius: number | undefined): JSX.Element => {
  if (typeof celsius !== 'number' || celsius <= 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{celsius.toFixed(1)}°C</span>;
};

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

export const TrueNASSystemsTable: Component<{
  systems: Resource[];
  // Full TrueNAS resource scope so we can count pools / datasets / apps
  // per system without spawning additional fetches.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.systems,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-system-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

  // Single-system canonical mock is the common case today; counts span
  // the full TrueNAS resource scope per row. When multi-system support
  // arrives, the canonical adapter should attach a parent system id to
  // child resources and we can filter by it.
  const counts = createMemo(() => {
    const pools = props.scope.filter(
      (r) => r.type === 'pool' || (r.type === 'storage' && r.storage?.topology === 'pool'),
    ).length;
    const datasets = props.scope.filter(
      (r) => r.type === 'dataset' || (r.type === 'storage' && r.storage?.topology === 'dataset'),
    ).length;
    const shares = props.scope.filter((r) => r.type === 'network-share').length;
    const apps = props.scope.filter((r) => r.type === 'app-container').length;
    const disks = props.scope.filter((r) => r.type === 'physical_disk').length;
    return { pools, datasets, shares, apps, disks };
  });

  return (
    <Show
      when={props.systems.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search TrueNAS systems"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="systems"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No systems match current filters"
              description="Adjust the search or status filter to see more systems."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Systems'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1380px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  {/*
                    Desktop widths balance the bar-metric columns (CPU /
                    Memory / Storage) against the short integer-count columns
                    (Pools / Datasets / Shares / Disks / Apps) and give Version the
                    room it needs for full "TrueNAS-SCALE-24.10.2"-style
                    labels. Mobile widths are unchanged.
                  */}
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[13%]`}>
                    System
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[10%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[11%]`}>
                    Memory
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[12%]`}>
                    Storage
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Temp
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Uptime
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Pools
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                  >
                    Datasets
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Shares
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[5%]`}
                  >
                    Disks
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[5%]`}
                  >
                    Apps
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(system) => {
                    const name = () => asTrimmedString(system.name) || system.id;
                    const version = () => asTrimmedString(system.agent?.osVersion) || '—';
                    const indicator = () => getSimpleStatusIndicator(system.status);
                    const storagePercent = () => {
                      if (typeof system.disk?.current === 'number') return system.disk.current;
                      if (
                        typeof system.disk?.used === 'number' &&
                        typeof system.disk?.total === 'number' &&
                        system.disk.total > 0
                      ) {
                        return (system.disk.used / system.disk.total) * 100;
                      }
                      return undefined;
                    };
                    const storageFullLabel = () =>
                      typeof system.disk?.used === 'number' &&
                      typeof system.disk?.total === 'number'
                        ? `${formatBytes(system.disk.used)} / ${formatBytes(system.disk.total)}`
                        : formatPercent(storagePercent());
                    const c = counts();
                    const metricsKey = () => buildMetricKeyForUnifiedResource(system);
                    const cpuPercent = () => finiteMetric(system.cpu?.current);
                    const memoryTotal = () => finiteMetric(system.memory?.total) ?? 0;
                    const memoryUsed = () => finiteMetric(system.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0 ? undefined : finiteMetric(system.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const detailRowId = () => drawer.detailRowId(system);
                    const isExpanded = () => drawer.isExpanded(system);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-system-row={system.id}
                          onClick={() => drawer.toggle(system)}
                          onKeyDown={drawer.handleActivationKey(system)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={system.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {version()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <ResponsiveMetricCell
                              class="w-full"
                              value={cpuPercent() ?? 0}
                              type="cpu"
                              resourceId={metricsKey()}
                              isRunning={canRenderMetrics() && cpuPercent() !== undefined}
                              showMobile={false}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={metricFallback()}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                          >
                            <span class="md:hidden">{formatPercent(storagePercent())}</span>
                            <span class="hidden md:inline">{storageFullLabel()}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatTemperature(system.temperature)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatUptime(system.uptime)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {c.pools}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {c.datasets}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {c.shares}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {c.disks}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {c.apps}
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={system}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={12}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(system)}
                        />
                      </>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASSystemsTable;
