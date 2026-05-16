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

// TrueNAS systems are storage appliances, not generic compute hosts.
// The generic infrastructure table's CPU / Memory / Disk columns are
// helpful (the agent payload carries them), but operators also want at-
// a-glance pool count, dataset count, app count, uptime, version, and
// max-sensor temperature on the same row. This bespoke table reuses
// canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) and counts the per-system children
// client-side from the same TrueNAS resource scope already fetched by
// the page (no extra API calls).

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatBytes = (bytes: number | undefined): string => {
  if (!bytes || bytes <= 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let value = bytes;
  let unitIdx = 0;
  while (value >= 1024 && unitIdx < units.length - 1) {
    value /= 1024;
    unitIdx += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIdx]}`;
};

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent)) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{percent.toFixed(1)}%</span>;
};

const formatTemperature = (celsius: number | undefined): JSX.Element => {
  if (typeof celsius !== 'number' || celsius <= 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{celsius.toFixed(1)}°C</span>;
};

export const TrueNASSystemsTable: Component<{
  systems: Resource[];
  // Full TrueNAS resource scope so we can count pools / datasets / apps
  // per system without spawning additional fetches.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.systems, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.systems.length);

  // Single-system canonical mock is the common case today; counts span
  // the full TrueNAS resource scope per row. When multi-system support
  // arrives, the canonical adapter should attach a parent system id to
  // child resources and we can filter by it.
  const counts = createMemo(() => {
    const pools = props.scope.filter(
      (r) => r.type === 'pool' || (r.type === 'storage' && r.storage?.topology === 'pool'),
    ).length;
    const datasets = props.scope.filter(
      (r) => r.type === 'dataset' || (r.type === 'storage' && r.storage?.topology === 'dataset'),
    ).length;
    const apps = props.scope.filter((r) => r.type === 'app-container').length;
    const disks = props.scope.filter((r) => r.type === 'physical_disk').length;
    return { pools, datasets, apps, disks };
  });

  return (
    <Show
      when={props.systems.length > 0}
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
              placeholder="Search TrueNAS systems"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} systems</>}>
              {visible()} of {total()} systems
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No systems match current filters"
                description="Adjust the search or status filter to see more systems."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[960px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">System</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Version</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Uptime</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Memory</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Storage</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Temp</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Pools</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Datasets</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Disks</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Apps</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(system) => {
                    const name = () => asTrimmedString(system.name) || system.id;
                    const version = () => asTrimmedString(system.agent?.osVersion) || '—';
                    const indicator = () => getSimpleStatusIndicator(system.status);
                    const c = counts();
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={system.status || 'unknown'}
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
                          {formatUptime(system.uptime)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(system.cpu?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(system.memory?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {typeof system.disk?.used === 'number' && typeof system.disk?.total === 'number'
                            ? `${formatBytes(system.disk.used)} / ${formatBytes(system.disk.total)}`
                            : formatPercent(system.disk?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatTemperature(system.temperature)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {c.pools}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {c.datasets}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {c.disks}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {c.apps}
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

export default TrueNASSystemsTable;
