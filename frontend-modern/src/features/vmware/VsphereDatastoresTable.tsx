import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes, formatPercent } from '@/utils/format';
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
import type { Resource } from '@/types/resource';
import {
  filterVmwareDatastores,
  mapVmwareDatastoreStatus,
  type VmwareDatastoreStatusFilter,
} from './vmwarePageModel';

const VSPHERE_DATASTORE_STATUS_OPTIONS: PlatformTableFilterOption<VmwareDatastoreStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'accessible', label: 'Accessible', tone: 'success' },
  { value: 'attention', label: 'Attention', tone: 'warning' },
  { value: 'inaccessible', label: 'Inaccessible', tone: 'danger' },
  { value: 'maintenance', label: 'Maintenance', tone: 'warning' },
  { value: 'unknown', label: 'Unknown' },
];

const datastoreName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const datastoreType = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.datastoreType) ||
  asTrimmedString(resource.storage?.type)?.toUpperCase() ||
  '-';

const compactList = (values: Array<string | undefined>): string[] =>
  values.map((value) => asTrimmedString(value)).filter((value): value is string => Boolean(value));

const summarizeValues = (
  values: string[],
  empty = '-',
  visibleCount = 2,
): { label: string; title: string } => {
  if (values.length === 0) return { label: empty, title: '' };
  const visible = values.slice(0, visibleCount);
  const suffix = values.length > visible.length ? ` +${values.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: values.join(', ') };
};

const hostSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(compactList(resource.storage?.nodes ?? []), '-', 2);

const consumerSummary = (resource: Resource): { label: string; title: string } => {
  const consumers = resource.storage?.topConsumers?.map((consumer) => consumer.name) ?? [];
  return summarizeValues(compactList(consumers), '-', 2);
};

const consumerCount = (resource: Resource): number => resource.storage?.consumerCount ?? 0;

const capacityPercent = (resource: Resource): number | undefined => {
  if (typeof resource.disk?.current === 'number' && Number.isFinite(resource.disk.current)) {
    return resource.disk.current;
  }
  if (
    typeof resource.disk?.used === 'number' &&
    typeof resource.disk?.total === 'number' &&
    resource.disk.total > 0
  ) {
    return (resource.disk.used / resource.disk.total) * 100;
  }
  return undefined;
};

const capacityLabel = (resource: Resource): string => {
  if (typeof resource.disk?.used === 'number' && typeof resource.disk?.total === 'number') {
    return `${formatBytes(resource.disk.used)} / ${formatBytes(resource.disk.total)}`;
  }
  const percent = capacityPercent(resource);
  return percent === undefined ? '-' : formatPercent(percent);
};

const CapacityCell: Component<{ resource: Resource }> = (props) => {
  const percent = () => capacityPercent(props.resource);
  return (
    <div class="min-w-0">
      <div class="truncate text-base-content" title={capacityLabel(props.resource)}>
        {capacityLabel(props.resource)}
      </div>
      <Show when={percent() !== undefined}>
        <div class="mt-1 h-1.5 overflow-hidden rounded bg-surface-alt">
          <div
            class="h-full rounded bg-emerald-500"
            style={{ width: `${Math.max(0, Math.min(100, percent() ?? 0))}%` }}
          />
        </div>
      </Show>
    </div>
  );
};

const statusLabel = (resource: Resource): string => {
  switch (mapVmwareDatastoreStatus(resource)) {
    case 'accessible':
      return 'Accessible';
    case 'attention':
      return 'Attention';
    case 'inaccessible':
      return 'Inaccessible';
    case 'maintenance':
      return 'Maintenance';
    case 'unknown':
      return 'Unknown';
  }
};

const statusPillClass = (resource: Resource): string => {
  switch (mapVmwareDatastoreStatus(resource)) {
    case 'accessible':
      return 'border-emerald-300/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
    case 'attention':
    case 'maintenance':
      return 'border-amber-300/50 bg-amber-500/10 text-amber-700 dark:text-amber-300';
    case 'inaccessible':
      return 'border-red-300/50 bg-red-500/10 text-red-700 dark:text-red-300';
    case 'unknown':
      return 'border-border bg-surface-alt text-muted';
  }
};

const StatusPill: Component<{ resource: Resource }> = (props) => (
  <span
    class={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${statusPillClass(
      props.resource,
    )}`}
  >
    {statusLabel(props.resource)}
  </span>
);

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Datastores" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1080px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[21%]`}>
                    Datastore
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[9%]`}>
                    Type
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[18%]`}>
                    Capacity
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Hosts
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                  >
                    VMs
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[13%]`}
                  >
                    Consumers
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Datacenter
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[8%]`}>
                    State
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(datastore) => {
                    const hosts = createMemo(() => hostSummary(datastore));
                    const consumers = createMemo(() => consumerSummary(datastore));
                    const indicator = () => getSimpleStatusIndicator(datastore.status);
                    const name = () => datastoreName(datastore);
                    const datacenter = () =>
                      asTrimmedString(datastore.vmware?.datacenterName) || '-';
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
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <div class="min-w-0">
                                <div class="truncate font-medium text-base-content" title={name()}>
                                  {name()}
                                </div>
                                <div
                                  class="truncate text-[10px] text-muted"
                                  title={datastore.vmware?.datastoreUrl}
                                >
                                  {datastore.vmware?.datastoreUrl ||
                                    datastore.vmware?.folderName ||
                                    datastore.vmware?.vcenterHost ||
                                    'vSphere datastore'}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="font-mono text-[11px] text-base-content">
                              {datastoreType(datastore)}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('metric-bar')}>
                            <CapacityCell resource={datastore} />
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
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <StatusPill resource={datastore} />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={datastore}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(datastore)}
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

export default VsphereDatastoresTable;
