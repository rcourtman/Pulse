import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  platformChipStatusDot,
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
    leading: platformChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
  {
    value: 'inaccessible',
    label: 'Inaccessible',
    tone: 'danger',
    leading: platformChipStatusDot('bg-red-500'),
  },
  {
    value: 'maintenance',
    label: 'Maintenance',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
  { value: 'unknown', label: 'Unknown' },
];

const datastoreName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const datastoreType = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.datastoreType) ||
  asTrimmedString(resource.storage?.type)?.toUpperCase() ||
  '—';

const compactList = (values: Array<string | undefined>): string[] =>
  values.map((value) => asTrimmedString(value)).filter((value): value is string => Boolean(value));

const summarizeValues = (
  values: string[],
  empty = '—',
  visibleCount = 2,
): { label: string; title: string } => {
  if (values.length === 0) return { label: empty, title: '' };
  const visible = values.slice(0, visibleCount);
  const suffix = values.length > visible.length ? ` +${values.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: values.join(', ') };
};

const hostSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(compactList(resource.storage?.nodes ?? []), '—', 2);

const consumerSummary = (resource: Resource): { label: string; title: string } => {
  const consumers = resource.storage?.topConsumers?.map((consumer) => consumer.name) ?? [];
  return summarizeValues(compactList(consumers), '—', 2);
};

const consumerCount = (resource: Resource): number => resource.storage?.consumerCount ?? 0;

const capacityDisk = (resource: Resource) => {
  const total = typeof resource.disk?.total === 'number' ? resource.disk.total : 0;
  const used = typeof resource.disk?.used === 'number' ? resource.disk.used : 0;
  const free =
    typeof resource.disk?.free === 'number' ? resource.disk.free : Math.max(0, total - used);
  const usage =
    typeof resource.disk?.current === 'number' && Number.isFinite(resource.disk.current)
      ? resource.disk.current
      : total > 0
        ? (used / total) * 100
        : 0;
  return { total, used, free, usage };
};

const hasCapacityMetric = (resource: Resource): boolean => {
  if (typeof resource.disk?.current === 'number' && Number.isFinite(resource.disk.current)) {
    return true;
  }
  return (
    typeof resource.disk?.used === 'number' &&
    typeof resource.disk?.total === 'number' &&
    resource.disk.total > 0
  );
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
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
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
                            <Show
                              when={showCapacity()}
                              fallback={
                                <div class="flex justify-center">
                                  <span class="text-xs text-muted" aria-hidden="true">
                                    —
                                  </span>
                                </div>
                              }
                            >
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
