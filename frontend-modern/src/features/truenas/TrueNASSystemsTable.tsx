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

// TrueNAS systems are storage appliances, not generic compute hosts.
// The generic infrastructure table's CPU / Memory / Disk columns are
// helpful (the agent payload carries them), but operators also want at-
// a-glance pool count, dataset count, app count, uptime, version, and
// max-sensor temperature on the same row. This bespoke table reuses
// canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) and counts the per-system children
// client-side from the same TrueNAS resource scope already fetched by
// the page (no extra API calls).

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
  if (typeof percent !== 'number' || Number.isNaN(percent))
    return <span class="text-muted">—</span>;
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
  title?: string;
  showToolbar?: boolean;
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
            searchPlaceholder="Search TrueNAS systems"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="systems"
          />
        </Show>

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No systems match current filters"
              description="Adjust the search or status filter to see more systems."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Systems'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1040px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>System</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Uptime
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>CPU</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Memory</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Storage</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Temp
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Pools
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Datasets
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Disks
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Apps
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(system) => {
                    const name = () => asTrimmedString(system.name) || system.id;
                    const version = () => asTrimmedString(system.agent?.osVersion) || '—';
                    const indicator = () => getSimpleStatusIndicator(system.status);
                    const storagePercent = () => {
                      if (typeof system.disk?.current === 'number') return system.disk.current;
                      if (
                        typeof system.disk?.used === 'number' &&
                        typeof system.disk?.total === 'number' &&
                        system.disk.total > 0
                      ) {
                        return (system.disk.used / system.disk.total) * 100;
                      }
                      return undefined;
                    };
                    const storageFullLabel = () =>
                      typeof system.disk?.used === 'number' &&
                      typeof system.disk?.total === 'number'
                        ? `${formatBytes(system.disk.used)} / ${formatBytes(system.disk.total)}`
                        : formatPercent(storagePercent());
                    const c = counts();
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={system.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          {version()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatUptime(system.uptime)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(system.cpu?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(system.memory?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          <span class="md:hidden">{formatPercent(storagePercent())}</span>
                          <span class="hidden md:inline">{storageFullLabel()}</span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatTemperature(system.temperature)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {c.pools}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {c.datasets}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {c.disks}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {c.apps}
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

export default TrueNASSystemsTable;
