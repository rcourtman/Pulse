import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import { MetricBar } from '@/components/Workloads/MetricBar';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  PlatformHostScopedTableTitle,
  createPlatformHostScopeFocus,
} from '@/features/platformPage/PlatformHostScope';
import {
  PlatformResourceDetailTableRow,
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  getPlatformInlineStateClass,
  getPlatformStatusTextToneClass,
} from '@/features/platformPage/platformStatusTone';
import {
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableResetFiltersButton,
  PlatformTableShell,
  PlatformTableToolbar,
  PlatformWindowedRows,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  getPlatformTableFiniteMetric,
  withPlatformStatusCounts,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { formatBytes, formatSpeed } from '@/utils/format';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  STANDALONE_VM_SORT_KEYS,
  filterStandaloneVMs,
  getStandaloneVMDisplayStatus,
  getStandaloneVMHypervisorLabel,
  getStandaloneVMName,
  getStandaloneVMSortValue,
  getStandaloneVMStateLabel,
  getStandaloneVMTotalDiskIORate,
  getStandaloneVMTotalNetIORate,
  isStandaloneVMSourceImpaired,
  mapStandaloneVMStatus,
  matchesStandaloneVMHostScope,
  resolveStandaloneVMHostName,
  type StandaloneVMSortKey,
  type StandaloneVMStatusFilter,
} from './standaloneVmTableModel';

const STANDALONE_VM_STATUS_OPTIONS: PlatformTableFilterOption<StandaloneVMStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
];

type VMStateNote = { short: string | null; full: string; tone: 'danger' | 'warning' };

const vmStateNote = (resource: Resource): VMStateNote | null => {
  if (mapStandaloneVMStatus(resource) !== 'attention') return null;
  if (isStandaloneVMSourceImpaired(resource)) {
    return { short: 'stale', full: 'Host agent has not reported recently', tone: 'warning' };
  }
  const state = (asTrimmedString(resource.virtualMachine?.runtimeState) ?? '').toLowerCase();
  if (state === 'crashed') {
    return { short: 'crashed', full: 'Hypervisor reports this VM crashed', tone: 'danger' };
  }
  return {
    short: null,
    full: `Hypervisor reports ${getStandaloneVMStateLabel(resource)}`,
    tone: 'warning',
  };
};

export const StandaloneVirtualMachinesTable: Component<{
  vms: Resource[];
  machines: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  selectedHostId?: () => string | undefined;
  onClearHostScope?: () => void;
  externalSearch?: () => string;
  onExternalSearchChange?: (value: string) => void;
  externalStatus?: () => StandaloneVMStatusFilter;
  onExternalStatusChange?: (status: StandaloneVMStatusFilter) => void;
  onResetFilters?: () => void;
}> = (props) => {
  const machinesById = createMemo(() => {
    const map = new Map<string, Resource>();
    for (const machine of props.machines) map.set(machine.id, machine);
    return map;
  });
  const machinesByName = createMemo(() => {
    const map = new Map<string, Resource>();
    for (const machine of props.machines) {
      for (const candidate of [machine.name, machine.displayName, machine.agent?.hostname]) {
        const normalized = candidate?.trim().toLowerCase();
        if (normalized && !map.has(normalized)) map.set(normalized, machine);
      }
    }
    return map;
  });
  const selectedHostId = () => asTrimmedString(props.selectedHostId?.()) || undefined;
  const hostScopedVms = createMemo(() => {
    const scope = selectedHostId();
    if (!scope) return props.vms;
    return props.vms.filter((vm) =>
      matchesStandaloneVMHostScope(vm, scope, machinesById(), machinesByName()),
    );
  });
  const selectedHostLabel = createMemo(() => {
    const scope = selectedHostId();
    if (!scope) return null;
    const machine = machinesById().get(scope) ?? machinesByName().get(scope.toLowerCase());
    if (machine)
      return asTrimmedString(machine.name) || asTrimmedString(machine.displayName) || scope;
    const matchedVm = hostScopedVms()[0];
    if (matchedVm) {
      const hostName = resolveStandaloneVMHostName(matchedVm, machinesById(), machinesByName());
      if (hostName !== '—') return hostName;
    }
    return scope;
  });
  const tableState = createPlatformTableFilterState({
    resources: hostScopedVms,
    initialStatus: 'all' as StandaloneVMStatusFilter,
    filter: (resources, search, status) =>
      filterStandaloneVMs(resources, search, status, machinesById(), machinesByName()),
    externalSearch: props.externalSearch,
    onExternalSearchChange: props.onExternalSearchChange,
    externalStatus: props.externalStatus,
    onExternalStatusChange: props.onExternalStatusChange,
  });
  const sort = createPlatformTableSortState<StandaloneVMSortKey>({
    storageKey: 'standaloneVms',
    sortKeys: STANDALONE_VM_SORT_KEYS,
    descendingFirst: ['cpu', 'memory', 'diskio', 'netio'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), (resource, key) =>
      getStandaloneVMSortValue(resource, key, machinesById(), machinesByName()),
    ),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'standalone-vm-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => [
    ...props.machines,
    ...props.vms,
  ]);
  const vmsFocus = createPlatformHostScopeFocus();

  const clearHostScope = () => {
    props.onClearHostScope?.();
    vmsFocus.focus();
  };
  const resetFilters = () => {
    if (props.onResetFilters) {
      props.onResetFilters();
      return;
    }
    tableState.setSearch('');
    tableState.setStatus('all');
    props.onClearHostScope?.();
  };

  return (
    <Show
      when={props.vms.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3" data-testid="standalone-vms-table">
        <PlatformTableToolbar
          search={tableState.search}
          onSearchChange={tableState.setSearch}
          searchPlaceholder="Search VMs by name, host, hypervisor, or state"
          searchSuggestions={tableState.searchSuggestions}
          status={tableState.status()}
          onStatusChange={tableState.setStatus}
          statusOptions={withPlatformStatusCounts(
            STANDALONE_VM_STATUS_OPTIONS,
            tableState.countForStatus,
          )}
          visible={tableState.visible()}
          total={tableState.total()}
          rowNoun="VMs"
        />

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No VMs match current filters"
              description={
                selectedHostLabel()
                  ? `No virtual machines on ${selectedHostLabel()} match the current search or status filter.`
                  : 'Adjust the search or status filter to see more virtual machines.'
              }
              actions={<PlatformTableResetFiltersButton onReset={resetFilters} />}
            />
          }
        >
          <PlatformTableShell
            title={
              <PlatformHostScopedTableTitle
                noun="Virtual Machines"
                hostLabel={selectedHostLabel()}
                count={selectedHostLabel() ? sortedRows().length : null}
                clearLabel="Show all hosts"
                onClear={clearHostScope}
                headingId="standalone-vms-heading"
                headingRef={vmsFocus.setHeading}
              />
            }
            tableClass="min-w-full table-fixed text-xs md:min-w-[840px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="vm"
                  class="platform-table-mobile-w-30 md:w-[24%]"
                >
                  VM
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="host"
                  class="hidden sm:table-cell md:w-[15%]"
                >
                  Host
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="state"
                  class="platform-table-phone-hidden md:w-[11%]"
                >
                  State
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="cpu"
                  class="platform-table-mobile-w-15 md:w-[14%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="memory"
                  class="platform-table-mobile-w-15 md:w-[16%]"
                >
                  <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="diskio"
                  class="hidden lg:table-cell md:w-[10%]"
                >
                  Disk I/O
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="netio"
                  class="hidden lg:table-cell md:w-[10%]"
                >
                  Net I/O
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(vm) => {
                    const name = () => getStandaloneVMName(vm);
                    const hostName = () =>
                      resolveStandaloneVMHostName(vm, machinesById(), machinesByName());
                    const hypervisor = () => getStandaloneVMHypervisorLabel(vm);
                    const statusBucket = () => mapStandaloneVMStatus(vm);
                    const indicator = () =>
                      getSimpleStatusIndicator(getStandaloneVMDisplayStatus(vm));
                    const stateLabel = () => getStandaloneVMStateLabel(vm);
                    const note = () => vmStateNote(vm);
                    const isStopped = () => statusBucket() === 'stopped';
                    const cpuPercent = () => getPlatformTableFiniteMetric(vm.cpu?.current);
                    const vcpus = () => {
                      const count = vm.virtualMachine?.vcpus;
                      return typeof count === 'number' && Number.isFinite(count) && count > 0
                        ? count
                        : undefined;
                    };
                    const memPercent = () => getPlatformTableFiniteMetric(vm.memory?.current);
                    const memUsed = () => getPlatformTableFiniteMetric(vm.memory?.used);
                    const memTotal = () => getPlatformTableFiniteMetric(vm.memory?.total);
                    const hasMemory = () =>
                      !isStopped() &&
                      (memPercent() !== undefined ||
                        (memTotal() !== undefined && (memTotal() ?? 0) > 0));
                    const memorySublabel = () => {
                      const used = memUsed();
                      const total = memTotal();
                      if (used !== undefined && total !== undefined && total > 0) {
                        return `${formatBytes(used, 0)}/${formatBytes(total, 0)}`;
                      }
                      if (total !== undefined && total > 0) {
                        return formatBytes(total, 0);
                      }
                      return undefined;
                    };
                    const diskIORate = () => getStandaloneVMTotalDiskIORate(vm);
                    const diskIOTitle = () => {
                      const read = getPlatformTableFiniteMetric(vm.diskIO?.readRate);
                      const write = getPlatformTableFiniteMetric(vm.diskIO?.writeRate);
                      if (read === undefined && write === undefined) return undefined;
                      return `Read ${formatSpeed(read ?? 0, 1)} · Write ${formatSpeed(write ?? 0, 1)}`;
                    };
                    const netIORate = () => getStandaloneVMTotalNetIORate(vm);
                    const netIOTitle = () => {
                      const rx = getPlatformTableFiniteMetric(vm.network?.rxBytes);
                      const tx = getPlatformTableFiniteMetric(vm.network?.txBytes);
                      if (rx === undefined && tx === undefined) return undefined;
                      return `In ${formatSpeed(rx ?? 0, 1)} · Out ${formatSpeed(tx ?? 0, 1)}`;
                    };
                    const metricsKey = () => buildMetricKeyForUnifiedResource(vm);
                    const detailRowId = () => drawer.detailRowId(vm);
                    const isExpanded = () => drawer.isExpanded(vm);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-standalone-vm-row={vm.id}
                          onClick={() => drawer.toggle(vm)}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(vm)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={stateLabel()}
                                ariaLabel={stateLabel()}
                              />
                              <div class="flex min-w-0 flex-1 items-center gap-1.5">
                                <div
                                  class="min-w-0 truncate font-medium text-base-content"
                                  title={[
                                    name(),
                                    hostName() !== '—' ? `Host ${hostName()}` : '',
                                    hypervisor(),
                                    vcpus() ? `${vcpus()} vCPU` : '',
                                  ]
                                    .filter(Boolean)
                                    .join(' · ')}
                                >
                                  {name()}
                                </div>
                                <span
                                  class={`platform-table-phone-only-inline text-[11px] ${getPlatformInlineStateClass(Boolean(note()))} ${note() ? getPlatformStatusTextToneClass(note()!.tone) : ''}`}
                                  data-standalone-vm-state-phone={note() ? note()!.tone : 'none'}
                                >
                                  <span class="min-w-0 truncate">
                                    {[stateLabel(), note()?.short].filter(Boolean).join(' · ')}
                                  </span>
                                </span>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden sm:table-cell text-base-content`}
                          >
                            <span
                              class="block truncate"
                              title={[hostName(), hypervisor()].filter(Boolean).join(' · ')}
                            >
                              {hostName()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                          >
                            <div
                              class="inline-flex max-w-full min-w-0 items-center gap-1.5"
                              title={note()?.full ?? stateLabel()}
                            >
                              <span class="shrink-0 text-[11px] font-medium text-base-content">
                                {stateLabel()}
                              </span>
                              <Show when={note()?.short}>
                                {(short) => (
                                  <span
                                    class={`min-w-0 truncate text-[11px] font-medium ${getPlatformStatusTextToneClass(note()!.tone)}`}
                                  >
                                    {short()}
                                  </span>
                                )}
                              </Show>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={!isStopped() && cpuPercent() !== undefined}
                              fallback={
                                <span
                                  class="text-muted tabular-nums"
                                  title={vcpus() ? `${vcpus()} vCPU` : undefined}
                                >
                                  {vcpus() ? `${vcpus()} vCPU` : '—'}
                                </span>
                              }
                            >
                              <EnhancedCPUBar
                                usage={cpuPercent() ?? 0}
                                cores={vcpus()}
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show
                              when={hasMemory()}
                              fallback={
                                <span
                                  class="text-muted tabular-nums"
                                  title={
                                    memTotal()
                                      ? `${formatBytes(memTotal()!, 0)} configured`
                                      : undefined
                                  }
                                >
                                  {memTotal() ? formatBytes(memTotal()!, 0) : '—'}
                                </span>
                              }
                            >
                              <MetricBar
                                value={memPercent() ?? 0}
                                label={`${Math.round(memPercent() ?? 0)}%`}
                                sublabel={memorySublabel()}
                                type="memory"
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden lg:table-cell text-base-content tabular-nums`}
                            title={diskIOTitle()}
                          >
                            <Show
                              when={!isStopped() && diskIORate() !== null}
                              fallback={<span class="text-muted">—</span>}
                            >
                              {formatSpeed(diskIORate() ?? 0, 1)}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden lg:table-cell text-base-content tabular-nums`}
                            title={netIOTitle()}
                          >
                            <Show
                              when={!isStopped() && netIORate() !== null}
                              fallback={<span class="text-muted">—</span>}
                            >
                              {formatSpeed(netIORate() ?? 0, 1)}
                            </Show>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={vm}
                          statusIndicator={indicator()}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(vm)}
                        />
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default StandaloneVirtualMachinesTable;
