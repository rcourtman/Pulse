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
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { formatBytes } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type {
  Resource,
  ResourceVMwareNetworkAdapter,
  ResourceVMwareSnapshot,
} from '@/types/resource';
import {
  filterVmwareVirtualMachines,
  mapVmwareVirtualMachineStatus,
  type VmwareVirtualMachineStatusFilter,
} from './vmwarePageModel';

const VSPHERE_VM_STATUS_OPTIONS: PlatformTableFilterOption<VmwareVirtualMachineStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'powered-on',
    label: 'Powered on',
    tone: 'success',
    leading: statusDot('bg-emerald-500'),
  },
  { value: 'attention', label: 'Attention', tone: 'warning', leading: statusDot('bg-amber-500') },
  { value: 'powered-off', label: 'Powered off', tone: 'danger', leading: statusDot('bg-red-500') },
  { value: 'suspended', label: 'Suspended', tone: 'warning' },
  { value: 'unknown', label: 'Unknown' },
];

type VmwareVmGroup = {
  key: string;
  label: string;
  cluster: string;
  rows: Resource[];
};

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

function statusDot(className: string): JSX.Element {
  return <span class={`h-2 w-2 rounded-full ${className}`} />;
}

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      -
    </span>
  </div>
);

const normalizeToken = (value: string | undefined): string =>
  (value || '')
    .trim()
    .toLowerCase()
    .replace(/[\s_-]/g, '');

const vmName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const formatVmwarePowerState = (value: string | undefined): string => {
  const normalized = normalizeToken(value);
  if (normalized === 'poweredon' || normalized === 'on') return 'On';
  if (normalized === 'poweredoff' || normalized === 'off') return 'Off';
  if (normalized === 'suspended') return 'Suspended';
  return asTrimmedString(value) || 'Unknown';
};

const powerStateVariant = (
  state: string | undefined,
): 'success' | 'warning' | 'danger' | 'muted' => {
  const normalized = normalizeToken(state);
  if (normalized === 'poweredon' || normalized === 'on') return 'success';
  if (normalized === 'poweredoff' || normalized === 'off') return 'danger';
  if (normalized === 'suspended') return 'warning';
  return 'muted';
};

const summarizeValues = (
  values: Array<string | undefined>,
  empty = '-',
  visibleCount = 2,
): { label: string; title: string } => {
  const compact = values
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => Boolean(value));
  if (compact.length === 0) return { label: empty, title: '' };
  const visible = compact.slice(0, visibleCount);
  const suffix = compact.length > visible.length ? ` +${compact.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: compact.join(', ') };
};

const countSnapshotTree = (snapshots: ResourceVMwareSnapshot[] | undefined): number =>
  (snapshots ?? []).reduce(
    (total, snapshot) => total + 1 + countSnapshotTree(snapshot.children),
    0,
  );

const flattenSnapshotNames = (snapshots: ResourceVMwareSnapshot[] | undefined): string[] => {
  const names: string[] = [];
  for (const snapshot of snapshots ?? []) {
    const name =
      asTrimmedString(snapshot.name) ||
      asTrimmedString(snapshot.snapshot) ||
      (typeof snapshot.id === 'number' ? `Snapshot ${snapshot.id}` : '');
    if (name) names.push(snapshot.current ? `${name} (current)` : name);
    names.push(...flattenSnapshotNames(snapshot.children));
  }
  return names;
};

const networkAdapterName = (adapter: ResourceVMwareNetworkAdapter): string =>
  asTrimmedString(adapter.networkName) ||
  asTrimmedString(adapter.networkId) ||
  asTrimmedString(adapter.opaqueNetworkId) ||
  asTrimmedString(adapter.hostDevice) ||
  asTrimmedString(adapter.macAddress) ||
  asTrimmedString(adapter.label) ||
  '';

const networkAdapterTitle = (adapter: ResourceVMwareNetworkAdapter): string =>
  [
    asTrimmedString(adapter.label),
    asTrimmedString(adapter.type),
    networkAdapterName(adapter),
    asTrimmedString(adapter.macAddress),
    asTrimmedString(adapter.state),
  ]
    .filter(Boolean)
    .join(' · ');

const networkSummary = (
  adapters: ResourceVMwareNetworkAdapter[] | undefined,
): { label: string; title: string } => {
  const names = (adapters ?? []).map(networkAdapterName).filter(Boolean);
  const summary = summarizeValues(names, '-', 1);
  const title = (adapters ?? []).map(networkAdapterTitle).filter(Boolean).join(' | ');
  return { label: summary.label, title: title || summary.title };
};

const formatGuest = (resource: Resource): { label: string; detail: string; title: string } => {
  const family = asTrimmedString(resource.vmware?.guestOsFamily);
  const host = asTrimmedString(resource.vmware?.guestHostname);
  const ips = summarizeValues(resource.vmware?.guestIpAddresses ?? [], '', 1);
  return {
    label: family || host || ips.label || '-',
    detail: host || ips.label,
    title: [family, host, ips.title].filter(Boolean).join(' | '),
  };
};

const healthLabel = (resource: Resource): string => {
  const alarms = resource.vmware?.activeAlarmCount ?? 0;
  if (alarms > 0) return `${alarms} alarm${alarms === 1 ? '' : 's'}`;
  const overall = asTrimmedString(resource.vmware?.overallStatus);
  if (!overall) return 'Unknown';
  if (overall.toLowerCase() === 'green') return 'Healthy';
  return overall.charAt(0).toUpperCase() + overall.slice(1).toLowerCase();
};

const healthPillClass = (resource: Resource): string => {
  const status = mapVmwareVirtualMachineStatus(resource);
  if (status === 'attention') {
    const overall = asTrimmedString(resource.vmware?.overallStatus)?.toLowerCase();
    if (overall === 'red') return 'border-red-300/50 bg-red-500/10 text-red-700 dark:text-red-300';
    return 'border-amber-300/50 bg-amber-500/10 text-amber-700 dark:text-amber-300';
  }
  if (status === 'powered-on') {
    return 'border-emerald-300/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
  }
  if (status === 'powered-off') {
    return 'border-border bg-surface-alt text-muted';
  }
  return 'border-border bg-surface-alt text-muted';
};

const HealthPill: Component<{ resource: Resource }> = (props) => (
  <span
    class={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${healthPillClass(
      props.resource,
    )}`}
  >
    {healthLabel(props.resource)}
  </span>
);

const vmGroupKey = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.runtimeHostName) ||
  asTrimmedString(resource.parentName) ||
  'Unassigned host';

const vmGroupCluster = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.clusterName) ||
  asTrimmedString(resource.vmware?.computeResourceName) ||
  '';

const groupVirtualMachines = (vms: Resource[]): VmwareVmGroup[] => {
  const groups: VmwareVmGroup[] = [];
  const byKey = new Map<string, VmwareVmGroup>();
  for (const vm of vms) {
    const label = vmGroupKey(vm);
    const key = label.toLowerCase();
    let group = byKey.get(key);
    if (!group) {
      group = { key, label, cluster: vmGroupCluster(vm), rows: [] };
      byKey.set(key, group);
      groups.push(group);
    }
    if (!group.cluster) group.cluster = vmGroupCluster(vm);
    group.rows.push(vm);
  }
  return groups;
};

export const VsphereVirtualMachinesTable: Component<{
  vms: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.vms,
    initialStatus: 'all' as VmwareVirtualMachineStatusFilter,
    filter: filterVmwareVirtualMachines,
  });
  const groups = createMemo(() => groupVirtualMachines(tableState.filtered()));
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-vm-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

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
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search vSphere VMs"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={VSPHERE_VM_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="VMs"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No VMs match current filters"
              description="Adjust the search or power-state filter to see more vSphere VMs."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Virtual Machines" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1520px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[18%]`}>
                    VM
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[8%]`}>
                    Power
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
                  >
                    Host
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Cluster
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[8%]`}
                  >
                    Pool
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[10%]`}
                  >
                    Guest
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[9%]`}
                  >
                    Network
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[9%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[11%]`}>
                    Memory
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[8%]`}
                  >
                    Datastores
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Snapshots
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[8%]`}>
                    Health
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
	                <For each={groups()}>
	                  {(group) => (
	                    <>
	                      <TableRow class="bg-surface-alt/70 text-[11px] font-semibold text-base-content">
	                        <TableCell colSpan={12} class="px-2 py-1">
                          <span>{group.label}</span>
                          <Show when={group.cluster}>
                            <span class="ml-2 rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-medium text-muted">
                              {group.cluster}
                            </span>
                          </Show>
                        </TableCell>
                      </TableRow>
                      <For each={group.rows}>
                        {(vm) => {
                          const meta = () => vm.vmware;
                          const name = () => vmName(vm);
                          const indicator = () => getSimpleStatusIndicator(vm.status);
                          const power = () => formatVmwarePowerState(meta()?.powerState);
                          const host = () =>
                            asTrimmedString(meta()?.runtimeHostName) ||
                            asTrimmedString(vm.parentName) ||
                            '-';
                          const cluster = () =>
                            asTrimmedString(meta()?.clusterName) ||
                            asTrimmedString(meta()?.computeResourceName) ||
                            '-';
                          const pool = () => asTrimmedString(meta()?.resourcePoolName) || '-';
                          const guest = createMemo(() => formatGuest(vm));
                          const network = createMemo(() => networkSummary(meta()?.networkAdapters));
                          const datastores = createMemo(() =>
                            summarizeValues(meta()?.datastoreNames ?? [], '-', 1),
                          );
                          const snapshots = () =>
                            Math.max(
                              0,
                              meta()?.snapshotCount ?? countSnapshotTree(meta()?.snapshotTree),
                            );
                          const snapshotTitle = () =>
                            flattenSnapshotNames(meta()?.snapshotTree).join(', ');
                          const metricsKey = () => buildMetricKeyForUnifiedResource(vm);
                          const cpuPercent = () => finiteMetric(vm.cpu?.current);
                          const memoryTotal = () => finiteMetric(vm.memory?.total) ?? 0;
                          const memoryUsed = () => finiteMetric(vm.memory?.used) ?? 0;
                          const memoryPercentOnly = () =>
                            memoryTotal() > 0 ? undefined : finiteMetric(vm.memory?.current);
                          const hasMemoryMetric = () =>
                            memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                          const canRenderMetrics = () => indicator().variant !== 'muted';
                          const memoryTitle = () =>
                            memoryTotal() > 0
                              ? `${formatBytes(memoryUsed())} / ${formatBytes(memoryTotal())}`
                              : '';
                          const detailRowId = () => drawer.detailRowId(vm);
                          const isExpanded = () => drawer.isExpanded(vm);

                          return (
                            <>
                              <TableRow
                                class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                                aria-controls={isExpanded() ? detailRowId() : undefined}
                                aria-expanded={isExpanded() ? 'true' : 'false'}
                                data-vsphere-vm-row={vm.id}
                                onClick={() => drawer.toggle(vm)}
                                onKeyDown={drawer.handleActivationKey(vm)}
                                tabIndex={0}
                              >
                                <TableCell class={getPlatformTableCellClassForKind('name')}>
                                  <div class="flex min-w-0 items-center gap-2">
                                    <StatusDot
                                      size="sm"
                                      variant={indicator().variant}
                                      title={indicator().label}
                                    />
                                    <div class="min-w-0">
                                      <div
                                        class="truncate font-medium text-base-content"
                                        title={name()}
                                      >
                                        {name()}
                                      </div>
                                      <div
                                        class="truncate text-[10px] text-muted"
                                        title={meta()?.managedObjectId}
                                      >
                                        {meta()?.managedObjectId ||
                                          meta()?.instanceUuid ||
                                          'vSphere VM'}
                                      </div>
                                    </div>
                                  </div>
                                </TableCell>
                                <TableCell class={getPlatformTableCellClassForKind('text')}>
                                  <div class="flex items-center gap-2">
                                    <StatusDot
                                      size="sm"
                                      variant={powerStateVariant(meta()?.powerState)}
                                      title={meta()?.powerState || 'unknown'}
                                      ariaHidden
                                    />
                                    <span class="truncate text-base-content" title={power()}>
                                      {power()}
                                    </span>
                                  </div>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                                  title={host()}
                                >
                                  <span class="block truncate">{host()}</span>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                                  title={cluster()}
                                >
                                  <span class="block truncate">{cluster()}</span>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                                  title={pool()}
                                >
                                  <span class="block truncate">{pool()}</span>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                                  title={guest().title}
                                >
                                  <span class="block truncate">{guest().label}</span>
                                  <Show when={guest().detail && guest().detail !== guest().label}>
                                    <span class="block truncate text-[10px] text-muted">
                                      {guest().detail}
                                    </span>
                                  </Show>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                                  title={network().title}
                                >
                                  <span class="block truncate">{network().label}</span>
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
                                  title={memoryTitle()}
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
                                  class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                                  title={datastores().title}
                                >
                                  <span class="block truncate">{datastores().label}</span>
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                                  title={snapshotTitle()}
                                >
                                  {snapshots()}
                                </TableCell>
                                <TableCell class={getPlatformTableCellClassForKind('badge')}>
                                  <HealthPill resource={vm} />
                                </TableCell>
                              </TableRow>
                              <PlatformResourceDetailTableRow
                                resource={vm}
                                open={isExpanded()}
                                detailRowId={detailRowId()}
                                colSpan={12}
                                resolveResourceLabel={resolveResourceLabel}
                                onClose={() => drawer.close(vm)}
                              />
                            </>
                          );
                        }}
                      </For>
                    </>
                  )}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default VsphereVirtualMachinesTable;
