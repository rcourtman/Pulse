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

// Docker Swarm services are cluster-scoped declarations, not running
// processes — they have no CPU / Memory / Disk / Disk I/O / Uptime /
// Temperature of their own (those metrics live on the controlled tasks
// and the underlying nodes). The canonical infrastructure table renders
// dashes for those columns on docker-service rows. This service-native
// table reuses canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) but surfaces operator columns that the
// data actually backs: image, mode, replica counts, ports, host.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatPorts = (ports: Resource['docker'] extends infer T ? T : never): string => {
  const entries = (ports as { endpointPorts?: Array<{ publishedPort?: number; targetPort?: number; protocol?: string }> })?.endpointPorts ?? [];
  if (entries.length === 0) return '—';
  return entries
    .map((entry) => {
      const protocol = entry?.protocol ? `/${entry.protocol.toLowerCase()}` : '';
      if (entry?.publishedPort && entry?.targetPort) {
        return `${entry.publishedPort}:${entry.targetPort}${protocol}`;
      }
      const single = entry?.publishedPort ?? entry?.targetPort;
      return single ? `${single}${protocol}` : '';
    })
    .filter((part) => part.length > 0)
    .join(', ') || '—';
};

const replicaCount = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

export const DockerServicesTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.resources, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.resources.length);

  return (
    <Show
      when={props.resources.length > 0}
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
              placeholder="Search Swarm services"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} services</>}>
              {visible()} of {total()} services
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No services match current filters"
                description="Adjust the search or status filter to see more services."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[900px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Service</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Image</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Mode</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Desired</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Running</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Ports</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Host</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(service) => {
                    const name = () => asTrimmedString(service.name) || service.id;
                    const image = () => asTrimmedString(service.docker?.image) || '—';
                    const mode = () => asTrimmedString(service.docker?.mode) || '—';
                    const host = () => asTrimmedString(service.docker?.hostname) || '—';
                    const indicator = () => getSimpleStatusIndicator(service.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={service.status || 'unknown'}
                              ariaHidden
                            />
                            <span
                              class="font-semibold text-base-content truncate"
                              title={name()}
                            >
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">
                          <span class="truncate inline-block max-w-[18rem]" title={image()}>
                            {image()}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{mode()}</TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(service.docker?.desiredTasks)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(service.docker?.runningTasks)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">
                          <span class="font-mono text-[11px]" title={formatPorts(service.docker)}>
                            {formatPorts(service.docker)}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{host()}</TableCell>
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

export default DockerServicesTable;
