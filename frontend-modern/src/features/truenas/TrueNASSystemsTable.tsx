import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  filterPlatformResources,
  formatPlatformTableUptimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  buildTrueNASSystemChildCounts,
  getTrueNASResourceDisplayStatus,
  type TrueNASSystemChildCounts,
} from '@/features/truenas/truenasPageModel';
import type { Resource } from '@/types/resource';

// TrueNAS systems are storage appliances, not generic compute hosts.
// The generic infrastructure table's CPU / Memory / Disk columns are
// helpful (the agent payload carries them), but operators also want at-
// a-glance native inventory on the same row. Keep that inventory grouped
// instead of giving every count a separate column; the Overview page should
// scan as an appliance summary, while the dedicated tabs own full detail.
// This bespoke table reuses
// canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) and counts the per-system children
// client-side from the same TrueNAS resource scope already fetched by
// the page (no extra API calls).

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

const EMPTY_COUNTS: TrueNASSystemChildCounts = {
  pools: 0,
  datasets: 0,
  shares: 0,
  vms: 0,
  apps: 0,
  disks: 0,
  services: 0,
};

const plural = (count: number, singular: string): string =>
  `${count} ${count === 1 ? singular : `${singular}s`}`;

const storageInventoryPrimary = (counts: TrueNASSystemChildCounts): string =>
  `${plural(counts.pools, 'pool')} / ${plural(counts.datasets, 'dataset')}`;

const storageInventorySecondary = (counts: TrueNASSystemChildCounts): string =>
  plural(counts.disks, 'disk');

const workloadInventoryPrimary = (counts: TrueNASSystemChildCounts): string =>
  plural(counts.vms, 'VM');

const workloadInventorySecondary = (counts: TrueNASSystemChildCounts): string =>
  plural(counts.apps, 'app');

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
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, getTrueNASResourceDisplayStatus),
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-system-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const countsBySystem = createMemo(() =>
    buildTrueNASSystemChildCounts(props.scope, props.systems),
  );

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
          <PlatformTableShell
            title={props.title ?? 'Systems'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[17%]`}>
                  System
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[10%]`}>
                  CPU
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[10%]`}>
                  Memory
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[13%]`}>
                  Capacity
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                >
                  Temp
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell lg:w-[15%]`}
                >
                  Inventory
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[8%]`}
                >
                  Shares
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell lg:w-[10%]`}
                >
                  VMs / Apps
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[10%]`}
                >
                  Services
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(system) => {
                    const name = () => asTrimmedString(system.name) || system.id;
                    const version = () => asTrimmedString(system.agent?.osVersion) || '—';
                    const displayStatus = () => getTrueNASResourceDisplayStatus(system);
                    const indicator = () => getSimpleStatusIndicator(displayStatus());
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
                    const c = () => countsBySystem().get(system.id) ?? EMPTY_COUNTS;
                    const uptimeLabel = () => formatPlatformTableUptimeValue(system.uptime);
                    const systemMeta = () =>
                      [
                        version() !== '—' ? version() : '',
                        uptimeLabel() !== '—' ? `up ${uptimeLabel()}` : '',
                      ]
                        .filter(Boolean)
                        .join(' · ');
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
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(system)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={displayStatus() || 'unknown'}
                                ariaHidden
                              />
                              <div class="min-w-0">
                                <div
                                  class="truncate font-semibold text-base-content"
                                  title={name()}
                                >
                                  {name()}
                                </div>
                                <Show when={systemMeta()}>
                                  <div class="truncate text-[11px] text-muted" title={systemMeta()}>
                                    {systemMeta()}
                                  </div>
                                </Show>
                              </div>
                            </div>
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden whitespace-normal text-base-content lg:table-cell`}
                            title={`${storageInventoryPrimary(c())} · ${storageInventorySecondary(c())}`}
                          >
                            <div class="leading-tight">
                              <div class="truncate">{storageInventoryPrimary(c())}</div>
                              <div class="truncate text-[11px] text-muted">
                                {storageInventorySecondary(c())}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums lg:table-cell`}
                          >
                            {c().shares}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden whitespace-normal text-base-content lg:table-cell`}
                            title={`${workloadInventoryPrimary(c())} · ${workloadInventorySecondary(c())}`}
                          >
                            <div class="leading-tight">
                              <div class="truncate">{workloadInventoryPrimary(c())}</div>
                              <div class="truncate text-[11px] text-muted">
                                {workloadInventorySecondary(c())}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums lg:table-cell`}
                          >
                            {c().services}
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={system}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={9}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(system)}
                        />
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASSystemsTable;
