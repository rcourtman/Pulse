import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
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
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import {
  PlatformResourceDetailTableRow,
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
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import type { Disk } from '@/types/api';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { formatSpeed, normalizeDiskArray } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatTemperature as formatTemperatureValue } from '@/utils/temperature';
import {
  AGENT_MACHINE_COLUMNS,
  getAgentMachineCpuPercent,
  getAgentMachineDiskIOTotal,
  getAgentMachineIpValues,
  getAgentMachineNetworkTotal,
  getAgentMachinePrimaryIp,
  getAgentMachineRaidSummary,
  getAgentMachineTemperatureCelsius,
  getAgentMachineTemperatureDetailSections,
  getAgentMachineTemperatureTitle,
  getNextAgentMachineSortState,
  sortAgentMachines,
  timestampMillisFrom,
  type AgentMachineColumn,
  type AgentMachineColumnId,
  type AgentMachineSortKey,
  type AgentMachineTemperatureDetailSection,
} from './agentMachineTableModel';

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatLastSeen = (value: number | string | Date | undefined): string => {
  const timestampMillis = timestampMillisFrom(value);
  if (!timestampMillis) return '—';
  const ageSeconds = Math.max(0, Math.floor((Date.now() - timestampMillis) / 1000));
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

const hasPositiveTemperature = (celsius: number | undefined): celsius is number =>
  typeof celsius === 'number' && Number.isFinite(celsius) && celsius > 0;

const AgentMachineTemperatureCell: Component<{
  celsius: number | undefined;
  sections: AgentMachineTemperatureDetailSection[];
  title: string;
}> = (props) => {
  const [visible, setVisible] = createSignal(false);
  const [position, setPosition] = createSignal({ x: 0, y: 0 });
  const hasDetails = () => props.sections.length > 0;
  const positiveTemperature = () =>
    hasPositiveTemperature(props.celsius) ? props.celsius : undefined;
  const open = (event: MouseEvent | FocusEvent) => {
    if (!hasDetails()) return;
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    setPosition({ x: rect.left + rect.width / 2, y: rect.top });
    setVisible(true);
  };
  const close = () => setVisible(false);

  return (
    <>
      <span
        data-agent-machine-temperature-trigger="true"
        class="inline-flex min-w-[2.25rem] justify-end text-xs tabular-nums"
        aria-label={props.title || undefined}
        tabIndex={hasDetails() ? 0 : undefined}
        onMouseEnter={open}
        onMouseOver={open}
        onMouseLeave={close}
        onFocus={open}
        onBlur={close}
        onClick={(event) => {
          event.stopPropagation();
          open(event);
        }}
      >
        <Show when={positiveTemperature()} fallback={<span class="text-muted">—</span>}>
          {(value) => formatTemperatureValue(value())}
        </Show>
      </span>
      <TooltipPortal
        when={visible() && hasDetails()}
        x={position().x}
        y={position().y}
        maxWidth={320}
      >
        <div
          data-agent-machine-temperature-tooltip="true"
          class="min-w-[190px] max-w-[300px] space-y-2"
        >
          <For each={props.sections}>
            {(section) => (
              <section>
                <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">
                  {section.heading}
                </div>
                <div class="space-y-0.5">
                  <For each={section.rows}>
                    {(row) => (
                      <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
                        <span class="min-w-0 truncate text-muted" title={row.label}>
                          {row.label}
                        </span>
                        <span
                          classList={{
                            'text-base-content': !row.muted,
                            'text-muted': row.muted,
                          }}
                          class="font-mono tabular-nums"
                        >
                          {row.value}
                        </span>
                      </div>
                    )}
                  </For>
                </div>
              </section>
            )}
          </For>
        </div>
      </TooltipPortal>
    </>
  );
};

const availabilityFor = (machine: Resource): ResourceAvailabilityMeta | undefined =>
  machine.availability ??
  (machine.platformData?.availability as ResourceAvailabilityMeta | undefined);

const isAgentlessMachine = (machine: Resource): boolean =>
  String(availabilityFor(machine)?.targetKind ?? '')
    .trim()
    .toLowerCase() === 'machine';

const availabilityAddressFor = (machine: Resource): string => {
  const availability = availabilityFor(machine);
  const address = asTrimmedString(availability?.address);
  if (address) return address;
  const identityWithIPAddresses = machine.identity as
    | (Resource['identity'] & { ipAddresses?: string[] })
    | undefined;
  const firstIP = asTrimmedString(
    identityWithIPAddresses?.ipAddresses?.[0] ?? machine.identity?.ips?.[0],
  );
  if (firstIP) return firstIP;
  return asTrimmedString(machine.identity?.hostname) ?? '';
};

const memoryTotalFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.total) ?? finiteMetric(machine.agent?.memory?.total) ?? 0;

const memoryUsedFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.used) ?? finiteMetric(machine.agent?.memory?.used) ?? 0;

const memoryPercentOnlyFor = (machine: Resource): number | undefined => {
  if (memoryTotalFor(machine) > 0) return undefined;
  return finiteMetric(machine.memory?.current) ?? finiteMetric(machine.agent?.memory?.usage);
};

const memoryBalloonFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.balloon);

const memorySwapUsedFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.swapUsed);

const memorySwapTotalFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.swapTotal);

const cpuCoresFor = (machine: Resource): number | undefined => {
  const cores = finiteMetric(machine.agent?.cpuCount);
  return cores && cores > 0 ? cores : undefined;
};

const cpuLoadAverageFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.loadAverage?.[0]);

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
  if (isAgentlessMachine(machine)) {
    const protocol = (asTrimmedString(availabilityFor(machine)?.protocol) ?? '').toUpperCase();
    return protocol ? `${protocol} availability` : 'Agentless availability';
  }
  const osName = asTrimmedString(machine.agent?.osName);
  const osVersion = asTrimmedString(machine.agent?.osVersion);
  if (osName && osVersion) return `${osName} ${osVersion}`;
  if (osName) return osName;
  return titleCase(
    asTrimmedString(machine.agent?.platform) || asTrimmedString(machine.technology) || 'Agent',
  );
};

const agentVersionFor = (machine: Resource): string =>
  isAgentlessMachine(machine) ? 'Agentless' : asTrimmedString(machine.agent?.agentVersion) || '—';

const networkTitleFor = (machine: Resource): string => {
  if (!machine.network) return '';
  return `In ${formatSpeed(machine.network.rxBytes)}\nOut ${formatSpeed(machine.network.txBytes)}`;
};

const diskIOTitleFor = (machine: Resource): string => {
  if (!machine.diskIO) return '';
  return `Read ${formatSpeed(machine.diskIO.readRate)}\nWrite ${formatSpeed(machine.diskIO.writeRate)}`;
};

const machineColumnWidthClass = (columnId: AgentMachineColumnId): string => {
  switch (columnId) {
    case 'machine':
      return 'w-[34%] md:w-[16%]';
    case 'system':
      return 'hidden md:table-cell md:w-[12%]';
    case 'agent':
      return 'hidden md:table-cell md:w-[6%]';
    case 'cpu':
    case 'memory':
    case 'disk':
      return 'w-[22%] md:w-[8%]';
    case 'network':
    case 'diskio':
      return 'hidden lg:table-cell lg:w-[9%]';
    case 'uptime':
    case 'temp':
      return 'hidden md:table-cell md:w-[5%]';
    case 'lastSeen':
      return 'hidden lg:table-cell lg:w-[6%]';
    case 'ip':
      return 'hidden xl:table-cell xl:w-[8%]';
    case 'raid':
      return 'hidden xl:table-cell xl:w-[6%]';
    case 'arch':
      return 'hidden xl:table-cell xl:w-[5%]';
    case 'kernel':
      return 'hidden xl:table-cell xl:w-[10%]';
  }
};

const getSortIndicator = (
  activeKey: AgentMachineSortKey,
  direction: 'asc' | 'desc',
  key: AgentMachineSortKey | undefined,
): '▲' | '▼' | '' => {
  if (!key || activeKey !== key) return '';
  return direction === 'asc' ? '▲' : '▼';
};

const getCompactColumnLabel = (column: AgentMachineColumn): string => {
  switch (column.id) {
    case 'uptime':
      return 'Up';
    case 'lastSeen':
      return 'Seen';
    default:
      return column.label;
  }
};

const AgentMachineSortableHead: Component<{
  column: AgentMachineColumn;
  activeSort: AgentMachineSortKey;
  direction: 'asc' | 'desc';
  onSort: (key: AgentMachineSortKey) => void;
}> = (props) => {
  const sortIndicator = () =>
    getSortIndicator(props.activeSort, props.direction, props.column.sortKey);
  const kind = (): NonNullable<AgentMachineColumn['kind']> => props.column.kind ?? 'text';

  return (
    <TableHead
      class={`${getPlatformTableHeadClassForKind(kind())} ${machineColumnWidthClass(props.column.id)}`}
      aria-sort={
        props.column.sortKey && props.activeSort === props.column.sortKey
          ? props.direction === 'asc'
            ? 'ascending'
            : 'descending'
          : undefined
      }
    >
      <Show when={props.column.sortKey} fallback={props.column.label}>
        {(sortKey) => (
          <button
            type="button"
            class="inline-flex max-w-full items-center gap-1 truncate hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
            onClick={() => props.onSort(sortKey())}
            aria-label={`Sort by ${props.column.label}`}
          >
            <span class="truncate">
              <Show
                when={props.column.id === 'memory'}
                fallback={getCompactColumnLabel(props.column)}
              >
                <span class="md:hidden">Mem</span>
                <span class="hidden md:inline">Memory</span>
              </Show>
            </span>
            <span class="w-2 shrink-0 text-[9px]" aria-hidden="true">
              {sortIndicator()}
            </span>
          </button>
        )}
      </Show>
    </TableHead>
  );
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
  const [sortKey, setSortKey] = createSignal<AgentMachineSortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const columnVisibility = useColumnVisibility(
    'pulse:standalone:machines:columns:v3',
    AGENT_MACHINE_COLUMNS,
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'agents-machine-drawer' });
  const visibleColumns = createMemo(
    () => columnVisibility.visibleColumns() as AgentMachineColumn[],
  );
  const detailColspan = createMemo(() => visibleColumns().length);
  const sortedMachines = createMemo(() =>
    sortAgentMachines(
      tableState.filtered(),
      sortKey(),
      sortDirection(),
      systemLabelFor,
      agentVersionFor,
    ),
  );
  const handleSort = (key: AgentMachineSortKey) => {
    const next = getNextAgentMachineSortState(sortKey(), sortDirection(), key);
    setSortKey(next.key);
    setSortDirection(next.direction);
  };

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
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[280px] flex-1">
            <PlatformTableToolbar
              search={tableState.search}
              onSearchChange={tableState.setSearch}
              searchPlaceholder="Search machines"
              status={tableState.status()}
              onStatusChange={tableState.setStatus}
              statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
              visible={tableState.visible()}
              total={tableState.total()}
              rowNoun="machines"
            />
          </div>
          <ColumnPicker
            columns={columnVisibility.availableToggles()}
            isHidden={columnVisibility.isHiddenByUser}
            onToggle={columnVisibility.toggle}
            onReset={columnVisibility.resetToDefaults}
          />
        </div>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No machines match current filters"
              description="Adjust the search or status filter to see more Pulse Agent machines."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Machines" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1160px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <For each={visibleColumns()}>
                    {(column) => (
                      <AgentMachineSortableHead
                        column={column}
                        activeSort={sortKey()}
                        direction={sortDirection()}
                        onSort={handleSort}
                      />
                    )}
                  </For>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedMachines()}>
                  {(machine) => {
                    const name = () => asTrimmedString(machine.name) || machine.id;
                    const hostname = () =>
                      isAgentlessMachine(machine)
                        ? availabilityAddressFor(machine)
                        : asTrimmedString(machine.agent?.hostname) ||
                          asTrimmedString(machine.identity?.hostname);
                    const systemLabel = () => systemLabelFor(machine);
                    const agentVersion = () => agentVersionFor(machine);
                    const indicator = () => getSimpleStatusIndicator(machine.status);
                    const canRenderMetrics = () => indicator().variant !== 'danger';
                    const metricsKey = () => buildMetricKeyForUnifiedResource(machine);
                    const cpuPercent = () => getAgentMachineCpuPercent(machine);
                    const cpuCores = () => cpuCoresFor(machine);
                    const cpuLoadAverage = () => cpuLoadAverageFor(machine);
                    const memoryUsed = () => memoryUsedFor(machine);
                    const memoryTotal = () => memoryTotalFor(machine);
                    const memoryBalloon = () => memoryBalloonFor(machine);
                    const memorySwapUsed = () => memorySwapUsedFor(machine);
                    const memorySwapTotal = () => memorySwapTotalFor(machine);
                    const memoryPercentOnly = () => memoryPercentOnlyFor(machine);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const aggregateDisk = () => aggregateDiskFor(machine);
                    const disks = () => normalizeDiskArray(machine.agent?.disks);
                    const hasDiskMetric = () =>
                      aggregateDisk() !== undefined || (disks()?.length ?? 0) > 0;
                    const networkTotal = () => getAgentMachineNetworkTotal(machine);
                    const diskIOTotal = () => getAgentMachineDiskIOTotal(machine);
                    const primaryIp = () => getAgentMachinePrimaryIp(machine);
                    const ipValues = () => getAgentMachineIpValues(machine);
                    const raidSummary = () => getAgentMachineRaidSummary(machine);
                    const temperature = () => getAgentMachineTemperatureCelsius(machine);
                    const temperatureSections = () =>
                      getAgentMachineTemperatureDetailSections(machine);
                    const temperatureTitle = () => getAgentMachineTemperatureTitle(machine);
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
                            class={`${getPlatformTableCellClassForKind('name')} ${machineColumnWidthClass('machine')}`}
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
                          <Show when={columnVisibility.isColumnVisible('system')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('system')} text-base-content`}
                            >
                              <span class="truncate" title={systemLabel()}>
                                {systemLabel()}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('agent')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('agent')} font-mono text-[11px] text-base-content`}
                            >
                              <span class="truncate" title={agentVersion()}>
                                {agentVersion()}
                              </span>
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('cpu')}`}
                          >
                            <Show
                              when={canRenderMetrics() && cpuPercent() !== undefined}
                              fallback={metricFallback()}
                            >
                              <EnhancedCPUBar
                                usage={cpuPercent() ?? 0}
                                loadAverage={cpuLoadAverage()}
                                cores={cpuCores()}
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('memory')}`}
                          >
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={metricFallback()}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                                balloon={memoryBalloon()}
                                swapUsed={memorySwapUsed()}
                                swapTotal={memorySwapTotal()}
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('disk')}`}
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
                          <Show when={columnVisibility.isColumnVisible('network')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('network')} text-base-content`}
                            >
                              <Show
                                when={canRenderMetrics() && networkTotal() !== undefined}
                                fallback={metricFallback()}
                              >
                                <div
                                  class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums"
                                  title={networkTitleFor(machine)}
                                >
                                  <span class="inline-flex w-3 justify-center text-emerald-500">
                                    ↓
                                  </span>
                                  <span class="min-w-0 truncate">
                                    {formatSpeed(machine.network?.rxBytes ?? 0)}
                                  </span>
                                  <span class="inline-flex w-3 justify-center text-orange-400">
                                    ↑
                                  </span>
                                  <span class="min-w-0 truncate">
                                    {formatSpeed(machine.network?.txBytes ?? 0)}
                                  </span>
                                </div>
                              </Show>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('diskio')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('diskio')} text-base-content`}
                            >
                              <Show
                                when={canRenderMetrics() && diskIOTotal() !== undefined}
                                fallback={metricFallback()}
                              >
                                <div
                                  class="grid w-full grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 text-[11px] tabular-nums"
                                  title={diskIOTitleFor(machine)}
                                >
                                  <span class="inline-flex w-3 justify-center font-mono text-blue-500">
                                    R
                                  </span>
                                  <span class="min-w-0 truncate">
                                    {formatSpeed(machine.diskIO?.readRate ?? 0)}
                                  </span>
                                  <span class="inline-flex w-3 justify-center font-mono text-amber-500">
                                    W
                                  </span>
                                  <span class="min-w-0 truncate">
                                    {formatSpeed(machine.diskIO?.writeRate ?? 0)}
                                  </span>
                                </div>
                              </Show>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('uptime')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('uptime')} text-base-content`}
                            >
                              {formatUptime(machine.uptime ?? machine.agent?.uptimeSeconds)}
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('temp')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('temp')} text-base-content`}
                            >
                              <AgentMachineTemperatureCell
                                celsius={temperature()}
                                sections={temperatureSections()}
                                title={temperatureTitle()}
                              />
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('lastSeen')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('lastSeen')} text-base-content`}
                            >
                              {formatLastSeen(
                                isAgentlessMachine(machine)
                                  ? availabilityFor(machine)?.lastChecked
                                  : machine.lastSeen,
                              )}
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('ip')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('ip')} text-base-content`}
                            >
                              <span class="truncate" title={ipValues().join('\n')}>
                                {primaryIp() || '—'}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('raid')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('raid')} text-base-content`}
                            >
                              <span class="truncate" title={raidSummary()}>
                                {raidSummary() || '—'}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('arch')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('arch')} text-base-content`}
                            >
                              <span class="truncate" title={machine.agent?.architecture}>
                                {machine.agent?.architecture || '—'}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('kernel')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('kernel')} text-base-content`}
                            >
                              <span class="truncate" title={machine.agent?.kernelVersion}>
                                {machine.agent?.kernelVersion || '—'}
                              </span>
                            </TableCell>
                          </Show>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={machine}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={detailColspan()}
                          onClose={() => drawer.close(machine)}
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

export default AgentsMachinesTable;
