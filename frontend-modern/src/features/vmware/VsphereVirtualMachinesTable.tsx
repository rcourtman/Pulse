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
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  platformChipStatusDot,
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
  ResourceVMwareVirtualDisk,
} from '@/types/resource';
import {
  filterVmwareVirtualMachines,
  formatVmwarePowerState,
  getVmwarePowerStateVariant,
  type VmwareVirtualMachineStatusFilter,
} from './vmwarePageModel';

const VSPHERE_VM_STATUS_OPTIONS: PlatformTableFilterOption<VmwareVirtualMachineStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'powered-on',
    label: 'Powered on',
    tone: 'success',
    leading: platformChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
  {
    value: 'powered-off',
    label: 'Powered off',
    tone: 'danger',
    leading: platformChipStatusDot('bg-red-500'),
  },
  {
    value: 'suspended',
    label: 'Suspended',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
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

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

const vmName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const summarizeValues = (
  values: Array<string | undefined>,
  empty = '—',
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
  const summary = summarizeValues(names, '—', 1);
  const title = (adapters ?? []).map(networkAdapterTitle).filter(Boolean).join(' | ');
  return { label: summary.label, title: title || summary.title };
};

const virtualDiskAddress = (disk: ResourceVMwareVirtualDisk): string => {
  const type = (asTrimmedString(disk.type) || '').toUpperCase();
  if (type === 'SCSI' && disk.scsiBus !== undefined && disk.scsiUnit !== undefined) {
    return `SCSI ${disk.scsiBus}:${disk.scsiUnit}`;
  }
  if (type === 'SATA' && disk.sataBus !== undefined && disk.sataUnit !== undefined) {
    return `SATA ${disk.sataBus}:${disk.sataUnit}`;
  }
  if (type === 'NVME' && disk.nvmeBus !== undefined && disk.nvmeUnit !== undefined) {
    return `NVMe ${disk.nvmeBus}:${disk.nvmeUnit}`;
  }
  return type;
};

const virtualDiskTitle = (disk: ResourceVMwareVirtualDisk): string =>
  [
    asTrimmedString(disk.label) || asTrimmedString(disk.disk),
    virtualDiskAddress(disk),
    typeof disk.capacityBytes === 'number' && Number.isFinite(disk.capacityBytes)
      ? formatBytes(disk.capacityBytes)
      : '',
    asTrimmedString(disk.datastoreName),
    asTrimmedString(disk.vmdkFile),
  ]
    .filter(Boolean)
    .join(' · ');

const virtualDiskCount = (disks: ResourceVMwareVirtualDisk[] | undefined): number =>
  disks?.length ?? 0;

const virtualDiskSummaryTitle = (disks: ResourceVMwareVirtualDisk[] | undefined): string =>
  (disks ?? []).map(virtualDiskTitle).filter(Boolean).join(' | ');

const formatGuest = (resource: Resource): { label: string; detail: string; title: string } => {
  const family = asTrimmedString(resource.vmware?.guestOsFamily);
  const host = asTrimmedString(resource.vmware?.guestHostname);
  const ips = summarizeValues(resource.vmware?.guestIpAddresses ?? [], '', 1);
  return {
    label: family || host || ips.label || '—',
    detail: host || ips.label,
    title: [family, host, ips.title].filter(Boolean).join(' | '),
  };
};

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
            <Table class="min-w-full table-fixed text-xs md:min-w-[1420px]">
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
                    Disks
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}
                  >
                    Snapshots
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
                            '—';
                          const cluster = () =>
                            asTrimmedString(meta()?.clusterName) ||
                            asTrimmedString(meta()?.computeResourceName) ||
                            '—';
                          const clusterServices = () => formatVmwareClusterServices(meta());
                          const pool = () => asTrimmedString(meta()?.resourcePoolName) || '—';
                          const guest = createMemo(() => formatGuest(vm));
                          const network = createMemo(() => networkSummary(meta()?.networkAdapters));
                          const datastores = createMemo(() =>
                            summarizeValues(meta()?.datastoreNames ?? [], '—', 1),
                          );
                          const disks = () => virtualDiskCount(meta()?.virtualDisks);
                          const diskTitle = () => virtualDiskSummaryTitle(meta()?.virtualDisks);
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
                                    <span
                                      class="truncate font-medium text-base-content"
                                      title={
                                        [
                                          name(),
                                          meta()?.managedObjectId,
                                          meta()?.instanceUuid,
                                        ]
                                          .filter(Boolean)
                                          .join(' · ') || name()
                                      }
                                    >
                                      {name()}
                                    </span>
                                  </div>
                                </TableCell>
                                <TableCell class={getPlatformTableCellClassForKind('text')}>
                                  <div class="flex items-center gap-2">
                                    <StatusDot
                                      size="sm"
                                      variant={getVmwarePowerStateVariant(meta()?.powerState)}
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
                                  title={[cluster(), clusterServices()].filter(Boolean).join(' · ')}
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
                                  title={diskTitle()}
                                >
                                  {disks()}
                                </TableCell>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                                  title={snapshotTitle()}
                                >
                                  {snapshots()}
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
