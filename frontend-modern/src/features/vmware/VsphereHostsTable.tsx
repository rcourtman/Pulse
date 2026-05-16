import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
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
  filterPlatformResources,
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

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent)) return <span class="text-muted">—</span>;
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
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={props.emptyTitle}
            description={props.emptyDescription}
          />
        </Card>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[200px] flex-1 sm:max-w-xs">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Search ESXi hosts"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} hosts</>}>
              {visible()} of {total()} hosts
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No hosts match current filters"
                description="Adjust the search or status filter to see more hosts."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[960px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Host</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Datacenter</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Power</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Memory</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Datastores</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">VMs</TableHead>
                  <TableHead class="px-3 py-2 font-medium">vCenter</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(host) => {
                    const meta = () => host.vmware;
                    const name = () => asTrimmedString(host.name) || host.id;
                    const datacenter = () => asTrimmedString(meta()?.datacenterName) || '—';
                    const cluster = () => asTrimmedString(meta()?.clusterName) || '—';
                    const vcenter = () => asTrimmedString(meta()?.vcenterHost) || '—';
                    const datastoreCount = () => meta()?.datastoreIds?.length ?? meta()?.datastoreNames?.length ?? 0;
                    const vmCount = () =>
                      vmCountByHost().get(asTrimmedString(meta()?.managedObjectId) || '') ?? 0;
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={host.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="font-semibold text-base-content truncate" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{datacenter()}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{cluster()}</TableCell>
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={powerStateVariant(meta()?.powerState)}
                              title={meta()?.powerState || 'unknown'}
                              ariaHidden
                            />
                            <span class="text-base-content">{formatPowerState(meta()?.powerState)}</span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(host.cpu?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(host.memory?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {datastoreCount()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {vmCount()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          <span class="truncate inline-block max-w-[12rem]" title={vcenter()}>
                            {vcenter()}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </Card>
        </Show>
      </div>
    </Show>
  );
};

export default VsphereHostsTable;
