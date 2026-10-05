import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableBytesValue,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
  withPlatformStatusCounts,
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
  mapTrueNASVMStatus,
  type TrueNASVMStatusFilter,
} from './truenasPageModel';

const TRUENAS_VM_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASVMStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'stopped', label: 'Stopped', tone: 'danger' },
];

const vmMeta = (resource: Resource): ResourceTrueNASVMMeta | undefined => resource.truenas?.vm;

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

const flagLabels = (vm: ResourceTrueNASVMMeta | undefined): string[] => {
  const labels: string[] = [];
  if (vm?.autostart) labels.push('Autostart');
  if (vm?.secureBoot) labels.push('Secure boot');
  if (vm?.trustedPlatformModule) labels.push('TPM');
  if (vm?.suspendOnSnapshot) labels.push('Suspend');
  return labels;
};

// A VM set to autostart that is not running is the one stopped VM the user
// did not choose, so the State cell says so beside the raw state, as it does
// for a state TrueNAS flags. Rows stay single-line (the shared platform-table
// rhythm), so the full sentence rides on the title. Bootloader and device
// counts are configuration, not condition, so they live in the drawer.
type VMStateNote = { short: string | null; full: string; tone: 'danger' | 'warning' };

const vmStateNote = (resource: Resource): VMStateNote | null => {
  const status = mapTrueNASVMStatus(resource);
  if (status === 'stopped') {
    return vmMeta(resource)?.autostart
      ? { short: 'should be running', full: 'Set to start at boot but stopped', tone: 'danger' }
      : null;
  }
  if (status !== 'attention') return null;
  if (hasImpairedResourceSource(resource, 'truenas')) {
    return { short: 'not updated', full: 'TrueNAS has not updated this recently', tone: 'warning' };
  }
  const state = asTrimmedString(vmMeta(resource)?.state || vmMeta(resource)?.domainState);
  return {
    short: state ? null : 'not reported',
    full: state
      ? `TrueNAS reports ${formatPlatformTableTitleCaseValue(state)}`
      : 'TrueNAS has not reported a state',
    tone: 'warning',
  };
};

const VM_STATE_NOTE_CLASS: Record<VMStateNote['tone'], string> = {
  danger: 'text-red-600 dark:text-red-300',
  warning: 'text-amber-700 dark:text-amber-300',
};

// Columns a user can sort by. Flags summarize several values at once, so they
// carry no single scalar to order on. CPU orders on the provisioned vCPU count
// (cores × threads when vCPUs are not reported) and Memory on the provisioned
// bytes.
const TRUENAS_VM_SORT_KEYS = ['vm', 'state', 'cpu', 'memory'] as const;

type TrueNASVMSortKey = (typeof TRUENAS_VM_SORT_KEYS)[number];

const getTrueNASVMSortValue = (
  resource: Resource,
  key: TrueNASVMSortKey,
): PlatformTableSortValue => {
  const vm = vmMeta(resource);
  switch (key) {
    case 'vm':
      return (
        asTrimmedString(vm?.name) ||
        asTrimmedString(resource.displayName) ||
        asTrimmedString(resource.name) ||
        resource.id
      );
    case 'state':
      return asTrimmedString(vm?.state || vm?.domainState) || null;
    case 'cpu': {
      const vcpus = vm?.vcpus;
      if (typeof vcpus === 'number' && Number.isFinite(vcpus) && vcpus > 0) return vcpus;
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
        return cores * threads;
      }
      return null;
    }
    case 'memory':
      return typeof vm?.memoryBytes === 'number' && Number.isFinite(vm.memoryBytes)
        ? vm.memoryBytes
        : null;
    default:
      key satisfies never;
      return null;
  }
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
  const sort = createPlatformTableSortState({
    storageKey: 'truenasVms',
    sortKeys: TRUENAS_VM_SORT_KEYS,
    descendingFirst: ['cpu', 'memory'],
  });
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getTrueNASVMSortValue));

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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              TRUENAS_VM_STATUS_OPTIONS,
              tableState.countForStatus,
            )}
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
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="vm"
                  class="platform-table-mobile-w-30 md:w-[34%]"
                >
                  VM
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="state"
                  class="platform-table-phone-hidden md:w-[14%]"
                >
                  State
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="cpu"
                  class="platform-table-mobile-w-15 md:w-[12%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="memory"
                  class="platform-table-mobile-w-15 md:w-[14%]"
                >
                  <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden sm:table-cell md:w-[26%]"
                >
                  Flags
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
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
                    const note = () => vmStateNote(resource);
                    const flags = createMemo(() => flagLabels(vm()));
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-truenas-vm-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
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
                                <div
                                  class="truncate font-medium text-base-content"
                                  title={[
                                    name(),
                                    vm()?.description ||
                                      vm()?.uuid ||
                                      resource.parentName ||
                                      'TrueNAS',
                                  ]
                                    .filter(Boolean)
                                    .join(' · ')}
                                >
                                  {name()}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                          >
                            <div
                              class="inline-flex max-w-full min-w-0 items-center gap-1.5"
                              title={note()?.full}
                              data-truenas-vm-state-note={note() ? note()!.tone : undefined}
                            >
                              <span class="shrink-0 text-[11px] font-medium text-base-content">
                                {stateLabel()}
                              </span>
                              <Show when={note()?.short}>
                                {(short) => (
                                  <span
                                    class={`min-w-0 truncate text-[11px] font-medium ${VM_STATE_NOTE_CLASS[note()!.tone]}`}
                                  >
                                    {short()}
                                  </span>
                                )}
                              </Show>
                              <Show when={note()}>
                                {(current) => <span class="sr-only">{current().full}</span>}
                              </Show>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatCPU(vm())}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatPlatformTableBytesValue(vm()?.memoryBytes, '-')}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
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
                          colSpan={5}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
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

export default TrueNASVirtualMachinesTable;
