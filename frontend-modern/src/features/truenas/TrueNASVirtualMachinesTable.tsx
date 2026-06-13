import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource, ResourceTrueNASVMMeta } from '@/types/resource';
import {
  filterTrueNASVMs,
  getTrueNASResourceDisplayStatus,
  type TrueNASVMStatusFilter,
} from './truenasPageModel';

const TRUENAS_VM_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASVMStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
];

const vmMeta = (resource: Resource): ResourceTrueNASVMMeta | undefined => resource.truenas?.vm;

const formatBytes = (bytes: number | undefined): string => {
  if (!bytes || bytes <= 0) return '-';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unitIdx = 0;
  while (value >= 1024 && unitIdx < units.length - 1) {
    value /= 1024;
    unitIdx += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIdx]}`;
};

const formatCPU = (vm: ResourceTrueNASVMMeta | undefined): string => {
  const vcpus = vm?.vcpus;
  if (typeof vcpus === 'number' && Number.isFinite(vcpus) && vcpus > 0) return `${vcpus} vCPU`;
  const cores = vm?.cores;
  const threads = vm?.threads;
  if (
    typeof cores === 'number' &&
    Number.isFinite(cores) &&
    cores > 0 &&
    typeof threads === 'number' &&
    Number.isFinite(threads) &&
    threads > 0
  ) {
    return `${cores}c / ${threads}t`;
  }
  return '-';
};

const formatDevices = (vm: ResourceTrueNASVMMeta | undefined): { label: string; title: string } => {
  const counts = [
    ['disk', vm?.diskCount],
    ['nic', vm?.nicCount],
    ['display', vm?.displayCount],
    ['cdrom', vm?.cdromCount],
    ['usb', vm?.usbCount],
    ['pci', vm?.pciCount],
  ] as const;
  const parts = counts
    .filter(([, count]) => typeof count === 'number' && Number.isFinite(count) && count > 0)
    .map(([kind, count]) => `${count} ${kind}`);
  const total = vm?.deviceCount;
  if (parts.length === 0) {
    return typeof total === 'number' && total > 0
      ? { label: `${total} devices`, title: `${total} devices` }
      : { label: '-', title: '' };
  }
  const visible = parts.slice(0, 2);
  const suffix = parts.length > visible.length ? ` +${parts.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: parts.join(', ') };
};

const flagLabels = (vm: ResourceTrueNASVMMeta | undefined): string[] => {
  const labels: string[] = [];
  if (vm?.autostart) labels.push('Autostart');
  if (vm?.secureBoot) labels.push('Secure boot');
  if (vm?.trustedPlatformModule) labels.push('TPM');
  if (vm?.suspendOnSnapshot) labels.push('Suspend');
  return labels;
};

export const TrueNASVirtualMachinesTable: Component<{
  vms: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.vms,
    initialStatus: 'all' as TrueNASVMStatusFilter,
    filter: filterTrueNASVMs,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-vm-drawer' });
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
            searchPlaceholder="Search TrueNAS VMs"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_VM_STATUS_OPTIONS}
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
              description="Adjust the search or status filter to see more TrueNAS VMs."
            />
          }
        >
          <PlatformTableShell
            title="Virtual Machines"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                  VM
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                  State
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden sm:table-cell md:w-[10%]`}
                >
                  CPU
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden sm:table-cell md:w-[10%]`}
                >
                  Memory
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[11%]`}
                >
                  Boot
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
                >
                  Devices
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[19%]`}>
                  Flags
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const vm = () => vmMeta(resource);
                    const name = () =>
                      asTrimmedString(vm()?.name) ||
                      asTrimmedString(resource.displayName) ||
                      asTrimmedString(resource.name) ||
                      resource.id;
                    const displayStatus = () => getTrueNASResourceDisplayStatus(resource);
                    const indicator = () => getSimpleStatusIndicator(displayStatus());
                    const stateLabel = () =>
                      formatPlatformTableTitleCaseValue(vm()?.state || vm()?.domainState);
                    const devices = createMemo(() => formatDevices(vm()));
                    const flags = createMemo(() => flagLabels(vm()));
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-vm-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <div class="min-w-0">
                                <div class="truncate font-medium text-base-content" title={name()}>
                                  {name()}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {vm()?.description ||
                                    vm()?.uuid ||
                                    resource.parentName ||
                                    'TrueNAS'}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="text-[11px] font-medium text-base-content">
                              {stateLabel()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content sm:table-cell`}
                          >
                            {formatCPU(vm())}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content sm:table-cell`}
                          >
                            {formatBytes(vm()?.memoryBytes)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {vm()?.bootloader || '-'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={devices().title}
                          >
                            <span class="truncate">{devices().label}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            title={flags().join(', ')}
                          >
                            <Show
                              when={flags().length > 0}
                              fallback={<span class="text-muted">-</span>}
                            >
                              <span class="truncate">{flags().slice(0, 2).join(', ')}</span>
                            </Show>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
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

export default TrueNASVirtualMachinesTable;
