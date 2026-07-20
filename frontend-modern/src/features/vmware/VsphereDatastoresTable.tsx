import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableMetricFallback,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  summarizePlatformTableValues,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  filterVmwareDatastores,
  getVmwareResourceDisplayStatus,
  type VmwareDatastoreStatusFilter,
} from './vmwarePageModel';

const VSPHERE_DATASTORE_STATUS_OPTIONS: PlatformTableFilterOption<VmwareDatastoreStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'accessible',
    label: 'Accessible',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'inaccessible',
    label: 'Inaccessible',
    tone: 'danger',
    leading: filterChipStatusDot('bg-red-500'),
  },
  {
    value: 'maintenance',
    label: 'Maintenance',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  { value: 'unknown', label: 'Unknown' },
];

const datastoreName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const datastoreType = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.datastoreType) ||
  asTrimmedString(resource.storage?.type)?.toUpperCase() ||
  '—';

const hostSummary = (resource: Resource): { label: string; title: string } =>
  summarizePlatformTableValues(resource.storage?.nodes);

const consumerSummary = (resource: Resource): { label: string; title: string } => {
  const consumers = resource.storage?.topConsumers?.map((consumer) => consumer.name) ?? [];
  return summarizePlatformTableValues(consumers);
};

const consumerCount = (resource: Resource): number => resource.storage?.consumerCount ?? 0;

const capacityDisk = (resource: Resource) => {
  const total = getPlatformTableFiniteMetric(resource.disk?.total) ?? 0;
  const used = getPlatformTableFiniteMetric(resource.disk?.used) ?? 0;
  const free = getPlatformTableFiniteMetric(resource.disk?.free) ?? Math.max(0, total - used);
  const current = getPlatformTableFiniteMetric(resource.disk?.current);
  const usage = current !== undefined ? current : total > 0 ? (used / total) * 100 : 0;
  return { total, used, free, usage };
};

const hasCapacityMetric = (resource: Resource): boolean => {
  if (getPlatformTableFiniteMetric(resource.disk?.current) !== undefined) return true;
  return (
    typeof resource.disk?.used === 'number' &&
    typeof resource.disk?.total === 'number' &&
    resource.disk.total > 0
  );
};

// Columns a user can sort by. Hosts and Consumers summarize several names at
// once, so they carry no single scalar to order on. Capacity orders on the
// used percentage the bar shows.
const VSPHERE_DATASTORE_SORT_KEYS = ['datastore', 'type', 'capacity', 'vms', 'datacenter'] as const;

type VsphereDatastoreSortKey = (typeof VSPHERE_DATASTORE_SORT_KEYS)[number];

const getVsphereDatastoreSortValue = (
  resource: Resource,
  key: VsphereDatastoreSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'datastore':
      return datastoreName(resource);
    case 'type': {
      const type = datastoreType(resource);
      return type === '—' ? null : type;
    }
    case 'capacity':
      return hasCapacityMetric(resource) ? capacityDisk(resource).usage : null;
    case 'vms':
      return consumerCount(resource);
    case 'datacenter':
      return asTrimmedString(resource.vmware?.datacenterName) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const VsphereDatastoresTable: Component<{
  datastores: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.datastores,
    initialStatus: 'all' as VmwareDatastoreStatusFilter,
    filter: filterVmwareDatastores,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-datastore-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);
  const sort = createPlatformTableSortState({
    storageKey: 'vsphereDatastores',
    sortKeys: VSPHERE_DATASTORE_SORT_KEYS,
    descendingFirst: ['capacity', 'vms'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getVsphereDatastoreSortValue),
  );

  return (
    <Show
      when={props.datastores.length > 0}
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
            searchPlaceholder="Search vSphere datastores"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={VSPHERE_DATASTORE_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="datastores"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No datastores match current filters"
              description="Adjust the search or status filter to see more vSphere datastores."
            />
          }
        >
          <PlatformTableShell
            title="Datastores"
            tableClass="min-w-full table-fixed text-xs md:min-w-[1080px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="datastore"
                  class="md:w-[21%]"
                >
                  Datastore
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} sortKey="type" class="md:w-[9%]">
                  Type
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="capacity"
                  class="md:w-[18%]"
                >
                  Capacity
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[14%]"
                >
                  Hosts
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="vms"
                  class="hidden md:table-cell md:w-[7%]"
                >
                  VMs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden lg:table-cell md:w-[13%]"
                >
                  Consumers
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="datacenter"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Datacenter
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(datastore) => {
                    const hosts = createMemo(() => hostSummary(datastore));
                    const consumers = createMemo(() => consumerSummary(datastore));
                    const displayStatus = () => getVmwareResourceDisplayStatus(datastore);
                    const indicator = () => getSimpleStatusIndicator(displayStatus());
                    const name = () => datastoreName(datastore);
                    const datacenter = () =>
                      asTrimmedString(datastore.vmware?.datacenterName) || '—';
                    const datastoreSubtitle = () =>
                      asTrimmedString(datastore.vmware?.datastoreUrl) ||
                      asTrimmedString(datastore.vmware?.folderName) ||
                      asTrimmedString(datastore.vmware?.vcenterHost) ||
                      '';
                    const datastoreTitle = () =>
                      [name(), datastoreSubtitle()].filter(Boolean).join(' · ') || name();
                    const disk = createMemo(() => capacityDisk(datastore));
                    const showCapacity = () => hasCapacityMetric(datastore);
                    const detailRowId = () => drawer.detailRowId(datastore);
                    const isExpanded = () => drawer.isExpanded(datastore);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-vsphere-datastore-row={datastore.id}
                          onClick={() => drawer.toggle(datastore)}
                          onKeyDown={drawer.handleActivationKey(datastore)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(datastore)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <span
                                class="truncate font-medium text-base-content"
                                title={datastoreTitle()}
                              >
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="font-mono text-[11px] text-base-content">
                              {datastoreType(datastore)}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <Show when={showCapacity()} fallback={<PlatformTableMetricFallback />}>
                              <StackedDiskBar aggregateDisk={disk()} />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={hosts().title}
                          >
                            <span class="block truncate">{hosts().label}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {consumerCount(datastore)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                            title={consumers().title}
                          >
                            <span class="block truncate">{consumers().label}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={datacenter()}
                          >
                            <span class="block truncate">{datacenter()}</span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={datastore}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(datastore)}
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

export default VsphereDatastoresTable;
