import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// vSphere storage is just datastores — there are no "physical disks"
// on the Pulse side (vSphere doesn't expose them through the canonical
// SMART pipeline). The previous /vmware/storage tab mounted the
// generic <StorageSurface> which ships its own internal
// Pools/Physical-disks switcher, so users could click "Physical disks"
// and land on a confusing "No disks match these filters" empty state.
// This bespoke table renders one row per datastore with the
// VMware-specific facts an operator looks at: type (vSAN/NFS/VMFS/etc),
// datacenter, capacity utilisation, hosts mounting it, shared/multi-host
// access, maintenance mode, vCenter.

type DatastoreStatusFilter = 'all' | 'online' | 'degraded' | 'offline';

function indicatorFor(resource: Resource): {
  variant: StatusIndicatorVariant;
  label: string;
} {
  const accessible = resource.vmware?.datastoreAccessible;
  const overall = (resource.vmware?.overallStatus ?? '').toLowerCase();
  if (accessible === false) return { variant: 'danger', label: 'Offline' };
  if (overall === 'red') return { variant: 'danger', label: 'Critical' };
  if (overall === 'yellow') return { variant: 'warning', label: 'Warning' };
  if (resource.status === 'offline') return { variant: 'danger', label: 'Offline' };
  if (resource.status === 'unknown') return { variant: 'warning', label: 'Unknown' };
  return { variant: 'success', label: 'Online' };
}

function classify(resource: Resource): DatastoreStatusFilter {
  const ind = indicatorFor(resource);
  if (ind.variant === 'danger') return 'offline';
  if (ind.variant === 'warning') return 'degraded';
  return 'online';
}

function datastoreTypeLabel(resource: Resource): string {
  return (
    asTrimmedString(resource.vmware?.datastoreType) ||
    asTrimmedString(resource.storage?.type) ||
    asTrimmedString(resource.technology) ||
    '—'
  );
}

function capacityCell(resource: Resource): JSX.Element {
  const pct = resource.disk?.current;
  const total = resource.disk?.total;
  if (typeof pct !== 'number' && (typeof total !== 'number' || total <= 0)) {
    return <span class="text-muted">—</span>;
  }
  const pctLabel = typeof pct === 'number' ? `${pct.toFixed(1)}%` : '—';
  const sizeLabel = typeof total === 'number' && total > 0 ? formatBytes(total) : null;
  return (
    <span class="tabular-nums">
      {pctLabel}
      {sizeLabel ? <span class="text-muted text-[10px]"> of {sizeLabel}</span> : null}
    </span>
  );
}

function hostsCell(resource: Resource): JSX.Element {
  const nodes = resource.storage?.nodes ?? [];
  if (nodes.length === 0) return <span class="text-muted">—</span>;
  return (
    <span class="tabular-nums">
      {nodes.length}
      <span
        class="text-muted text-[10px] inline-block max-w-[14rem] truncate align-bottom"
        title={nodes.join(', ')}
      >
        {' · '}
        {nodes.join(', ')}
      </span>
    </span>
  );
}

function accessLabel(resource: Resource): JSX.Element {
  const multi = resource.vmware?.multipleHostAccess;
  const shared = resource.storage?.shared;
  if (multi === true || shared === true) {
    return (
      <span class="inline-flex items-center rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
        Shared
      </span>
    );
  }
  if (multi === false || shared === false) {
    return <span class="text-muted">Local</span>;
  }
  return <span class="text-muted">—</span>;
}

function maintenanceLabel(resource: Resource): JSX.Element {
  const mode = (resource.vmware?.maintenanceMode ?? '').toLowerCase();
  if (!mode || mode === 'normal') return <span class="text-muted">—</span>;
  return (
    <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
      {mode}
    </span>
  );
}

export const VsphereDatastoresTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<DatastoreStatusFilter>('all');

  const filtered = createMemo(() => {
    const term = search().trim().toLowerCase();
    const want = status();
    return props.resources.filter((row) => {
      if (want !== 'all' && classify(row) !== want) return false;
      if (!term) return true;
      const haystack = [
        row.name,
        row.displayName,
        row.vmware?.datacenterName,
        row.vmware?.datastoreType,
        row.vmware?.connectionName,
        row.vmware?.vcenterHost,
        ...(row.storage?.nodes ?? []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  const total = createMemo(() => props.resources.length);
  const visible = createMemo(() => filtered().length);

  return (
    <Show
      when={total() > 0}
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
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder="Search datastores, datacenters, hosts"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="datastores"
          />
        </Show>

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No datastores match current filters"
              description="Adjust the search or status filter to see more datastores."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Datastores'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1050px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Datastore</TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Type</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Datacenter
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Capacity</TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Hosts</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Access
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Maintenance
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    vCenter
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(row) => {
                    const ind = indicatorFor(row);
                    const name = asTrimmedString(row.name) || row.id;
                    const datacenter = asTrimmedString(row.vmware?.datacenterName) || '—';
                    const vcenter =
                      asTrimmedString(row.vmware?.connectionName) ||
                      asTrimmedString(row.vmware?.vcenterHost) ||
                      '—';
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={ind.variant}
                              title={ind.label}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name}>
                              {name}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} text-[10px] font-medium uppercase text-base-content`}
                        >
                          {datastoreTypeLabel(row)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {datacenter}
                        </TableCell>
                        <TableCell class={`${getPlatformTableCellClass()} text-base-content`}>
                          {capacityCell(row)}
                        </TableCell>
                        <TableCell class={`${getPlatformTableCellClass()} text-base-content`}>
                          {hostsCell(row)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {accessLabel(row)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {maintenanceLabel(row)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[12rem] truncate" title={vcenter}>
                            {vcenter}
                          </span>
                        </TableCell>
                      </TableRow>
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
