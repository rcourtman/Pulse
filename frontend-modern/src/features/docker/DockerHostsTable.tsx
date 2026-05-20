import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { DockerHostDrawer } from './DockerHostDrawer';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
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
import { normalizeDiskArray } from '@/utils/format';
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
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getDockerHostSystemBadge, hasDockerSwarmEvidence } from './dockerPageModel';

// Docker / Podman hosts are container hosts, not generic Pulse Agents.
// The operator columns that matter are runtime version, container count,
// and Swarm role, alongside the
// usual CPU / Memory / Disk / Uptime / Temperature from the agent
// telemetry. The generic infrastructure table renders the metrics fine
// but omits the runtime context that distinguishes a Docker host from
// any other agent. This bespoke table reuses canonical shared
// primitives and surfaces the Docker-native columns.

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatTemperature = (celsius: number | undefined): JSX.Element => {
  if (typeof celsius !== 'number' || celsius <= 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{celsius.toFixed(1)}°C</span>;
};

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const percentFromMetric = (metric: Resource['cpu'] | undefined): number | undefined =>
  finiteMetric(metric?.current);

const memoryTotalFor = (host: Resource): number =>
  finiteMetric(host.memory?.total) ?? finiteMetric(host.agent?.memory?.total) ?? 0;

const memoryUsedFor = (host: Resource): number =>
  finiteMetric(host.memory?.used) ?? finiteMetric(host.agent?.memory?.used) ?? 0;

const memoryPercentOnlyFor = (host: Resource): number | undefined => {
  if (memoryTotalFor(host) > 0) return undefined;
  return finiteMetric(host.memory?.current) ?? finiteMetric(host.agent?.memory?.usage);
};

const aggregateDiskFor = (host: Resource): Disk | undefined => {
  if (!host.disk) return undefined;
  const total = finiteMetric(host.disk.total) ?? 0;
  const used = finiteMetric(host.disk.used) ?? 0;
  const free = finiteMetric(host.disk.free) ?? (total > 0 ? Math.max(0, total - used) : 0);
  const usage =
    total > 0 && used > 0 ? (used / total) * 100 : (finiteMetric(host.disk.current) ?? 0);
  if (total <= 0 && usage <= 0) return undefined;
  return { total, used, free, usage };
};

export const DockerHostsTable: Component<{
  resources: Resource[];
  sourceCount?: number;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const showSwarmColumn = createMemo(() => props.resources.some(hasDockerSwarmEvidence));
  const [selectedHostId, setSelectedHostId] = createSignal<string | null>(null);
  const drawerColspan = createMemo(() => (showSwarmColumn() ? 10 : 9));

  const hasFilteredSourceRows = () => (props.sourceCount ?? props.resources.length) > 0;

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={hasFilteredSourceRows() ? 'No hosts match current filters' : props.emptyTitle}
          description={
            hasFilteredSourceRows()
              ? 'Adjust the shared Docker page filters to see more hosts.'
              : props.emptyDescription
          }
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search Docker hosts"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="hosts"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No hosts match current filters"
              description="Adjust the search or status filter to see more hosts."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Hosts'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1080px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  {/*
                    Desktop widths balance the three bar-metric columns (CPU /
                    Memory / Disk) against the short-content columns so the
                    bars aren't squeezed by table-fixed's equal split. Mobile
                    widths (w-[40%], w-[20%]) are unchanged.
                  */}
                  <TableHead class={`${getPlatformTableHeadClass()} w-[40%] md:w-[15%]`}>
                    Host
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell md:w-[7%]`}>
                    System
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell md:w-[7%]`}>
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell md:w-[9%]`}>
                    Containers
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} w-[20%] md:w-[14%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} w-[20%] md:w-[14%]`}>
                    <span class="md:hidden">Mem</span>
                    <span class="hidden md:inline">Memory</span>
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} w-[20%] md:w-[14%]`}>
                    Disk
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell md:w-[6%]`}>
                    Uptime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell md:w-[6%]`}>
                    Temp
                  </TableHead>
                  <Show when={showSwarmColumn()}>
                    <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell md:w-[9%]`}>
                      Swarm role
                    </TableHead>
                  </Show>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(host) => {
                    const docker = () =>
                      host.docker as
                        | (NonNullable<Resource['docker']> & {
                            runtime?: string;
                            runtimeVersion?: string;
                            containerCount?: number;
                            uptimeSeconds?: number;
                            temperature?: number;
                            swarm?: { nodeRole?: string };
                          })
                        | undefined;
                    const name = () => asTrimmedString(host.name) || host.id;
                    const systemBadge = () => getDockerHostSystemBadge(host);
                    const version = () => asTrimmedString(docker()?.runtimeVersion) || '—';
                    const containerCount = () => docker()?.containerCount ?? 0;
                    const swarmRole = () => {
                      if (!hasDockerSwarmEvidence(host)) return '—';
                      const role = asTrimmedString(docker()?.swarm?.nodeRole);
                      return role ? role.charAt(0).toUpperCase() + role.slice(1) : '—';
                    };
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    const canRenderMetrics = () => indicator().variant !== 'danger';
                    const metricsKey = () => buildMetricKeyForUnifiedResource(host);
                    const cpuPercent = () => percentFromMetric(host.cpu);
                    const memoryUsed = () => memoryUsedFor(host);
                    const memoryTotal = () => memoryTotalFor(host);
                    const memoryPercentOnly = () => memoryPercentOnlyFor(host);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const aggregateDisk = () => aggregateDiskFor(host);
                    const disks = () => normalizeDiskArray(host.agent?.disks);
                    const hasDiskMetric = () =>
                      aggregateDisk() !== undefined || (disks()?.length ?? 0) > 0;
                    const detailRowId = () => `docker-host-drawer-${host.id}`;
                    const isSelected = () => selectedHostId() === host.id;
                    const toggleDrawer = () =>
                      setSelectedHostId((current) => (current === host.id ? null : host.id));
                    const handleActivationKey: JSX.EventHandler<
                      HTMLTableRowElement,
                      KeyboardEvent
                    > = (event) => {
                      if (event.key !== 'Enter' && event.key !== ' ') return;
                      event.preventDefault();
                      toggleDrawer();
                    };
                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer text-[11px] outline-none sm:text-xs ${
                            isSelected() ? 'bg-surface-hover' : ''
                          } focus-visible:ring-2 focus-visible:ring-blue-500/60 focus-visible:ring-offset-1 focus-visible:ring-offset-surface`}
                          aria-controls={isSelected() ? detailRowId() : undefined}
                          aria-expanded={isSelected() ? 'true' : 'false'}
                          data-docker-host-row={host.id}
                          onClick={toggleDrawer}
                          onKeyDown={handleActivationKey}
                          tabIndex={0}
                        >
                          <TableCell class={`${getPlatformTableCellClass()} w-[40%] md:w-auto`}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={host.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                            <Show when={systemBadge()}>
                              {(badge) => (
                                <span
                                  class="mt-0.5 block truncate pl-5 text-[9px] text-muted sm:text-[10px] md:hidden"
                                  title={badge().title ?? badge().label}
                                >
                                  {badge().label}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                          >
                            <Show when={systemBadge()} fallback={<span class="text-muted">—</span>}>
                              {(badge) => (
                                <span
                                  class={badge().classes}
                                  title={badge().title ?? badge().label}
                                >
                                  {badge().label}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {version()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {containerCount()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass('right')} w-[20%] md:w-auto`}
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
                            class={`${getPlatformTableCellClass('right')} w-[20%] md:w-auto`}
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
                            class={`${getPlatformTableCellClass('right')} w-[20%] md:w-auto`}
                          >
                            <Show
                              when={canRenderMetrics() && hasDiskMetric()}
                              fallback={metricFallback()}
                            >
                              <StackedDiskBar
                                mode={(disks()?.length ?? 0) > 1 ? 'vertical-bars' : undefined}
                                disks={disks()}
                                aggregateDisk={aggregateDisk()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                          >
                            {formatUptime(host.uptime ?? docker()?.uptimeSeconds)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                          >
                            {formatTemperature(host.temperature ?? docker()?.temperature)}
                          </TableCell>
                          <Show when={showSwarmColumn()}>
                            <TableCell
                              class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                            >
                              {swarmRole()}
                            </TableCell>
                          </Show>
                        </TableRow>
                        <Show when={isSelected()}>
                          <TableRow data-inline-docker-host-detail-for={host.id}>
                            <TableCell
                              id={detailRowId()}
                              colspan={drawerColspan()}
                              class="p-0 border-b border-border bg-surface-alt"
                            >
                              <div
                                class="px-2 py-3 sm:px-4 sm:py-4"
                                onClick={(event) => event.stopPropagation()}
                              >
                                <DockerHostDrawer host={host} />
                              </div>
                            </TableCell>
                          </TableRow>
                        </Show>
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

export default DockerHostsTable;
