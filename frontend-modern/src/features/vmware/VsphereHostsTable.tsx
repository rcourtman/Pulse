import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
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
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  filterPlatformResources,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// vSphere ESXi hosts are virtualization hypervisors managed by vCenter,
// not generic Pulse Agents. The generic infrastructure table renders
// dashes for Uptime / Temperature (vCenter inventory does not expose
// host uptime today) and lacks the columns that matter for the cluster
// operator: datacenter, cluster, power state, connection state,
// datastore count, and VM count alongside CPU / Memory utilisation.
// This bespoke table reuses canonical shared primitives and surfaces
// those ESXi-native columns. Per-host VM count is computed from the
// page scope client-side (no extra API calls).

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent))
    return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{percent.toFixed(1)}%</span>;
};

const powerStateVariant = (
  state: string | undefined,
): 'success' | 'warning' | 'danger' | 'muted' => {
  const normalized = (state || '').trim().toUpperCase();
  if (normalized === 'POWERED_ON') return 'success';
  if (normalized === 'POWERED_OFF') return 'muted';
  if (normalized === 'SUSPENDED') return 'warning';
  return 'muted';
};

const formatPowerState = (state: string | undefined): string => {
  const normalized = (state || '').trim();
  if (!normalized) return '—';
  return normalized
    .split('_')
    .map((part) => part.charAt(0) + part.slice(1).toLowerCase())
    .join(' ');
};

export const VsphereHostsTable: Component<{
  hosts: Resource[];
  // Full vSphere scope so we can count VMs per host without spawning
  // additional fetches.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.hosts, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.hosts.length);

  const vmCountByHost = createMemo(() => {
    const map = new Map<string, number>();
    for (const resource of props.scope) {
      if (resource.type !== 'vm') continue;
      const runtimeHost = asTrimmedString(resource.vmware?.runtimeHostId);
      if (!runtimeHost) continue;
      map.set(runtimeHost, (map.get(runtimeHost) ?? 0) + 1);
    }
    return map;
  });

  return (
    <Show
      when={props.hosts.length > 0}
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
            searchPlaceholder="Search ESXi hosts"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="hosts"
          />
        </Show>

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No hosts match current filters"
              description="Adjust the search or status filter to see more hosts."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Hosts'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[960px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Host</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Datacenter
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Cluster
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Power
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>CPU</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Memory</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Datastores
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>VMs</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    vCenter
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(host) => {
                    const meta = () => host.vmware;
                    const name = () => asTrimmedString(host.name) || host.id;
                    const datacenter = () => asTrimmedString(meta()?.datacenterName) || '—';
                    const cluster = () => asTrimmedString(meta()?.clusterName) || '—';
                    const vcenter = () => asTrimmedString(meta()?.vcenterHost) || '—';
                    const datastoreCount = () =>
                      meta()?.datastoreIds?.length ?? meta()?.datastoreNames?.length ?? 0;
                    const vmCount = () =>
                      vmCountByHost().get(asTrimmedString(meta()?.managedObjectId) || '') ?? 0;
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={host.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {datacenter()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {cluster()}
                        </TableCell>
                        <TableCell class={`${getPlatformTableCellClass()} hidden md:table-cell`}>
                          <div class="flex items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={powerStateVariant(meta()?.powerState)}
                              title={meta()?.powerState || 'unknown'}
                              ariaHidden
                            />
                            <span class="text-base-content">
                              {formatPowerState(meta()?.powerState)}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(host.cpu?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(host.memory?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {datastoreCount()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content tabular-nums`}
                        >
                          {vmCount()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[12rem] truncate" title={vcenter()}>
                            {vcenter()}
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

export default VsphereHostsTable;
