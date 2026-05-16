import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
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
  PlatformTableEmptyState,
  filterPlatformResources,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// Docker / Podman hosts are container hosts, not generic Pulse Agents.
// The operator columns that matter are runtime (Docker vs Podman),
// runtime version, container count, and Swarm role, alongside the
// usual CPU / Memory / Disk / Uptime / Temperature from the agent
// telemetry. The generic infrastructure table renders the metrics fine
// but omits the runtime context that distinguishes a Docker host from
// any other agent. This bespoke table reuses canonical shared
// primitives and surfaces the Docker-native columns.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent))
    return <span class="text-muted">—</span>;
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

const runtimeLabel = (runtime: string | undefined): string => {
  const normalized = (runtime || '').trim().toLowerCase();
  if (normalized === 'docker') return 'Docker';
  if (normalized === 'podman') return 'Podman';
  return runtime || '—';
};

export const DockerHostsTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
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
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <div class="flex flex-wrap items-center gap-2">
            <div class="min-w-[200px] flex-1 sm:max-w-xs">
              <SearchInput value={search} onChange={setSearch} placeholder="Search Docker hosts" />
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
            <Table class="min-w-full table-fixed text-xs md:min-w-[1040px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Host</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Runtime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Containers
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>CPU</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Memory</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Disk</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Uptime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Temp
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Swarm role
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(host) => {
                    const docker = () =>
                      host.docker as
                        | (NonNullable<Resource['docker']> & {
                            runtime?: string;
                            runtimeVersion?: string;
                            containerCount?: number;
                            uptimeSeconds?: number;
                            temperature?: number;
                            swarm?: { nodeRole?: string };
                          })
                        | undefined;
                    const name = () => asTrimmedString(host.name) || host.id;
                    const runtime = () => runtimeLabel(docker()?.runtime);
                    const version = () => asTrimmedString(docker()?.runtimeVersion) || '—';
                    const containerCount = () => docker()?.containerCount ?? 0;
                    const swarmRole = () => {
                      const role = asTrimmedString(docker()?.swarm?.nodeRole);
                      return role ? role.charAt(0).toUpperCase() + role.slice(1) : '—';
                    };
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
                          {runtime()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          {version()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {containerCount()}
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
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(host.disk?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatUptime(host.uptime ?? docker()?.uptimeSeconds)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatTemperature(host.temperature ?? docker()?.temperature)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {swarmRole()}
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

export default DockerHostsTable;
