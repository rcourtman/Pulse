import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { DockerHostDrawer } from './DockerHostDrawer';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableMetricFallback,
  PlatformTableEmptyState,
  PlatformTableTemperatureValue,
  PlatformTableShell,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  formatPlatformTableUptimeValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  filterDockerResources,
  getDockerHostSystemBadge,
  hasDockerSwarmEvidence,
  type DockerResourceStatusFilter,
} from './dockerPageModel';

// Docker / Podman hosts are container hosts, not generic Pulse Agents.
// The operator columns that matter are runtime version, container count,
// and Swarm role, alongside the
// usual CPU / Memory / Disk / Uptime / Temperature from the agent
// telemetry. The generic infrastructure table renders the metrics fine
// but omits the runtime context that distinguishes a Docker host from
// any other agent. This bespoke table reuses canonical shared
// primitives and surfaces the Docker-native columns.

const percentFromMetric = (metric: Resource['cpu'] | undefined): number | undefined =>
  getPlatformTableFiniteMetric(metric?.current);

const memoryTotalFor = (host: Resource): number =>
  getPlatformTableFiniteMetric(host.memory?.total) ??
  getPlatformTableFiniteMetric(host.agent?.memory?.total) ??
  0;

const memoryUsedFor = (host: Resource): number =>
  getPlatformTableFiniteMetric(host.memory?.used) ??
  getPlatformTableFiniteMetric(host.agent?.memory?.used) ??
  0;

const memoryPercentOnlyFor = (host: Resource): number | undefined => {
  if (memoryTotalFor(host) > 0) return undefined;
  return (
    getPlatformTableFiniteMetric(host.memory?.current) ??
    getPlatformTableFiniteMetric(host.agent?.memory?.usage)
  );
};

const aggregateDiskFor = (host: Resource): Disk | undefined => {
  if (!host.disk) return undefined;
  const total = getPlatformTableFiniteMetric(host.disk.total) ?? 0;
  const used = getPlatformTableFiniteMetric(host.disk.used) ?? 0;
  const free =
    getPlatformTableFiniteMetric(host.disk.free) ?? (total > 0 ? Math.max(0, total - used) : 0);
  const usage =
    total > 0 && used > 0
      ? (used / total) * 100
      : (getPlatformTableFiniteMetric(host.disk.current) ?? 0);
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
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
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
          <PlatformTableShell
            title={props.title ?? 'Hosts'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1240px]"
            header={
              <>
                {/*
                    Desktop widths balance the three bar-metric columns (CPU /
                    Memory / Disk) against the short-content columns so the
                    bars aren't squeezed by table-fixed's equal split. Mobile
                    widths (w-[40%], w-[20%]) are unchanged.
                  */}
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} w-[40%] md:w-[13%]`}>
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[7%]`}
                >
                  System
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[7%]`}
                >
                  Version
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[9%]`}
                >
                  Containers
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[14%]`}
                >
                  CPU
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[14%]`}
                >
                  <span class="md:hidden">Mem</span>
                  <span class="hidden md:inline">Memory</span>
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[14%]`}
                >
                  Disk
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                >
                  Uptime
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                >
                  Temp
                </TableHead>
                <Show when={showSwarmColumn()}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Swarm role
                  </TableHead>
                </Show>
              </>
            }
            body={
              <>
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
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} w-[40%] md:w-auto`}
                          >
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isSelected()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={toggleDrawer}
                              />
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {version()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {containerCount()}
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
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <Show
                              when={canRenderMetrics() && hasDiskMetric()}
                              fallback={<PlatformTableMetricFallback />}
                            >
                              <StackedDiskBar
                                mode={(disks()?.length ?? 0) > 1 ? 'vertical-bars' : undefined}
                                disks={disks()}
                                aggregateDisk={aggregateDisk()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatPlatformTableUptimeValue(
                              host.uptime ?? docker()?.uptimeSeconds,
                            )}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableTemperatureValue
                              value={host.temperature ?? docker()?.temperature}
                            />
                          </TableCell>
                          <Show when={showSwarmColumn()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            >
                              {swarmRole()}
                            </TableCell>
                          </Show>
                        </TableRow>
                        <Show when={isSelected()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={drawerColspan()}
                            data-inline-docker-host-detail-for={host.id}
                          >
                            <DockerHostDrawer host={host} />
                          </InlineDetailTableRow>
                        </Show>
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

export default DockerHostsTable;
