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
import {
  getResourceClusterLabel,
  getResourceNodeName,
  getResourceVersion,
} from './proxmoxPageModel';

// Proxmox Overview now mirrors the v5 layout: a dedicated hosts table sits
// above the canonical Workloads filter + guest table on /proxmox/overview, so
// operators can see node-level uptime / load / temperature without scrolling
// past every guest row. The Workloads filter still drives the guest table
// below; this table has its own narrow search + status filter the same way
// the sibling platform-host tables (Docker / K8s / TrueNAS / vSphere) do, so
// the page composes one canonical shape across the platform-first nav.

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

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatTemperature = (celsius: number | undefined): JSX.Element => {
  if (typeof celsius !== 'number' || celsius <= 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{celsius.toFixed(1)}°C</span>;
};

type GuestCounts = { vms: number; containers: number };

const countGuestsForNode = (guests: Resource[], nodeName: string): GuestCounts => {
  const counts: GuestCounts = { vms: 0, containers: 0 };
  for (const guest of guests) {
    if (getResourceNodeName(guest) !== nodeName) continue;
    if (guest.type === 'vm') counts.vms += 1;
    else if (guest.type === 'system-container' || guest.type === 'oci-container') {
      counts.containers += 1;
    }
  }
  return counts;
};

export const ProxmoxNodesTable: Component<{
  nodes: Resource[];
  guests: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.nodes, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.nodes.length);

  return (
    <Show
      when={props.nodes.length > 0}
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
            <SearchInput value={search} onChange={setSearch} placeholder="Search Proxmox nodes" />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} nodes</>}>
              {visible()} of {total()} nodes
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No nodes match current filters"
                description="Adjust the search or status filter to see more nodes."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[920px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Version</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Uptime</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Memory</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Disk</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Temp</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">VMs</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CTs</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(node) => {
                    const name = () => asTrimmedString(node.name) || node.id;
                    const version = () => asTrimmedString(getResourceVersion(node)) || '—';
                    const cluster = () => getResourceClusterLabel(node);
                    const counts = () => countGuestsForNode(props.guests, getResourceNodeName(node));
                    const indicator = () => getSimpleStatusIndicator(node.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={node.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="font-semibold text-base-content truncate" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          {version()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatUptime(node.uptime)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(node.cpu?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(node.memory?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(node.disk?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatTemperature(node.temperature)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {counts().vms}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {counts().containers}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{cluster()}</TableCell>
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

export default ProxmoxNodesTable;
