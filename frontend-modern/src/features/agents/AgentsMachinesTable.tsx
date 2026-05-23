import { For, Show, type Component, type JSX } from 'solid-js';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';
import { normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';

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
  if (typeof celsius !== 'number' || !Number.isFinite(celsius) || celsius <= 0) {
    return <span class="text-muted">—</span>;
  }
  return <span class="tabular-nums">{Math.round(celsius)}°C</span>;
};

const formatLastSeen = (value: number | undefined): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return '—';
  const ageSeconds = Math.max(0, Math.floor((Date.now() - value) / 1000));
  if (ageSeconds < 60) return 'now';
  const minutes = Math.floor(ageSeconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
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

const memoryTotalFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.total) ?? finiteMetric(machine.agent?.memory?.total) ?? 0;

const memoryUsedFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.used) ?? finiteMetric(machine.agent?.memory?.used) ?? 0;

const memoryPercentOnlyFor = (machine: Resource): number | undefined => {
  if (memoryTotalFor(machine) > 0) return undefined;
  return finiteMetric(machine.memory?.current) ?? finiteMetric(machine.agent?.memory?.usage);
};

const aggregateDiskFor = (machine: Resource): Disk | undefined => {
  if (!machine.disk) return undefined;
  const total = finiteMetric(machine.disk.total) ?? 0;
  const used = finiteMetric(machine.disk.used) ?? 0;
  const free = finiteMetric(machine.disk.free) ?? (total > 0 ? Math.max(0, total - used) : 0);
  const usage =
    total > 0 && used > 0 ? (used / total) * 100 : (finiteMetric(machine.disk.current) ?? 0);
  if (total <= 0 && usage <= 0) return undefined;
  return { total, used, free, usage };
};

const titleCase = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const systemLabelFor = (machine: Resource): string => {
  const osName = asTrimmedString(machine.agent?.osName);
  const osVersion = asTrimmedString(machine.agent?.osVersion);
  if (osName && osVersion) return `${osName} ${osVersion}`;
  if (osName) return osName;
  return titleCase(asTrimmedString(machine.agent?.platform) || asTrimmedString(machine.technology) || 'Agent');
};

export const AgentsMachinesTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'agents-machine-drawer' });
  const detailColspan = 9;

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder="Search agent machines"
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="machines"
        />

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No machines match current filters"
              description="Adjust the search or status filter to see more agent-primary machines."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Machines" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1120px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('name')} w-[40%] md:w-[19%]`}
                  >
                    Machine
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    System
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[8%]`}
                  >
                    Agent
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[12%]`}
                  >
                    CPU
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[12%]`}
                  >
                    <span class="md:hidden">Mem</span>
                    <span class="hidden md:inline">Memory</span>
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('metric-bar')} w-[20%] md:w-[12%]`}
                  >
                    Disk
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                  >
                    Uptime
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                  >
                    Temp
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell lg:w-[9%]`}
                  >
                    Last seen
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(machine) => {
                    const name = () => asTrimmedString(machine.name) || machine.id;
                    const hostname = () =>
                      asTrimmedString(machine.agent?.hostname) ||
                      asTrimmedString(machine.identity?.hostname);
                    const systemLabel = () => systemLabelFor(machine);
                    const agentVersion = () => asTrimmedString(machine.agent?.agentVersion) || '—';
                    const indicator = () => getSimpleStatusIndicator(machine.status);
                    const canRenderMetrics = () => indicator().variant !== 'danger';
                    const metricsKey = () => buildMetricKeyForUnifiedResource(machine);
                    const cpuPercent = () => percentFromMetric(machine.cpu);
                    const memoryUsed = () => memoryUsedFor(machine);
                    const memoryTotal = () => memoryTotalFor(machine);
                    const memoryPercentOnly = () => memoryPercentOnlyFor(machine);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const aggregateDisk = () => aggregateDiskFor(machine);
                    const disks = () => normalizeDiskArray(machine.agent?.disks);
                    const hasDiskMetric = () =>
                      aggregateDisk() !== undefined || (disks()?.length ?? 0) > 0;
                    const isExpanded = () => drawer.isExpanded(machine);
                    const detailRowId = () => drawer.detailRowId(machine);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-agents-machine-row={machine.id}
                          onClick={() => drawer.toggle(machine)}
                          onKeyDown={drawer.handleActivationKey(machine)}
                          tabIndex={0}
                        >
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} w-[40%] md:w-auto`}
                          >
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={machine.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                            <span
                              class="mt-0.5 block truncate pl-5 text-[9px] text-muted sm:text-[10px] md:hidden"
                              title={hostname() || systemLabel()}
                            >
                              {hostname() || systemLabel()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="truncate" title={systemLabel()}>
                              {systemLabel()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {agentVersion()}
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
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
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
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatUptime(machine.uptime ?? machine.agent?.uptimeSeconds)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatTemperature(machine.temperature)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                          >
                            {formatLastSeen(machine.lastSeen)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <TableRow data-inline-agent-machine-detail-for={machine.id}>
                            <TableCell
                              id={detailRowId()}
                              colspan={detailColspan}
                              class="border-b border-border bg-surface-alt p-0"
                            >
                              <div
                                class="px-2 py-3 sm:px-4 sm:py-4"
                                onClick={(event) => event.stopPropagation()}
                              >
                                <ResourceDetailDrawer
                                  resource={machine}
                                  presentation="table-row"
                                  onClose={() => drawer.close(machine)}
                                />
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

export default AgentsMachinesTable;
