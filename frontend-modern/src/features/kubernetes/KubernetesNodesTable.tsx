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

// Kubernetes nodes carry richer Kubelet/runtime metadata than a generic
// Pulse Agent — kubelet version, container runtime, roles
// (control-plane/worker), ready state, pod capacity. They're a hybrid
// row in the canonical model (the registry merges the K8s node onto
// the linked agent host), so the generic infrastructure table renders
// the agent metrics fine but omits the K8s context that matters to the
// cluster operator. This bespoke table reuses canonical shared
// primitives and surfaces the Kubelet-native columns alongside the
// usual CPU/Memory utilisation.

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

const formatRoles = (roles: string[] | undefined): string => {
  if (!roles || roles.length === 0) return '—';
  return roles.map((role) => role.replace('node-role.kubernetes.io/', '')).join(', ');
};

export const KubernetesNodesTable: Component<{
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
          <PlatformTableToolbar
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder="Search nodes"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="nodes"
          />
        </Show>

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No nodes match current filters"
              description="Adjust the search or status filter to see more nodes."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Nodes'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1000px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Node</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Cluster
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Roles
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Kubelet
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Runtime
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>CPU</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Memory</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Capacity</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Uptime
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(node) => {
                    const meta = () => node.kubernetes;
                    const name = () => asTrimmedString(node.name) || node.id;
                    const cluster = () =>
                      asTrimmedString(meta()?.clusterName) ||
                      asTrimmedString(meta()?.clusterId) ||
                      '—';
                    const kubelet = () => asTrimmedString(meta()?.kubeletVersion) || '—';
                    const runtime = () => asTrimmedString(meta()?.containerRuntimeVersion) || '—';
                    const capacityLabel = () => {
                      const cores = meta()?.capacityCpuCores;
                      const mem = meta()?.capacityMemoryBytes;
                      const parts: string[] = [];
                      if (typeof cores === 'number' && cores > 0) parts.push(`${cores} cores`);
                      if (typeof mem === 'number' && mem > 0) parts.push(formatBytes(mem));
                      return parts.join(' / ') || '—';
                    };
                    const compactCapacityLabel = () => {
                      const cores = meta()?.capacityCpuCores;
                      if (typeof cores === 'number' && cores > 0) return `${cores} cores`;
                      return capacityLabel();
                    };
                    const indicator = () => getSimpleStatusIndicator(node.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={node.status || 'unknown'}
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
                          {cluster()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {formatRoles(meta()?.roles)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          {kubelet()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          <span class="truncate inline-block max-w-[10rem]" title={runtime()}>
                            {runtime()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(node.cpu?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(node.memory?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content tabular-nums`}
                        >
                          <span class="md:hidden">{compactCapacityLabel()}</span>
                          <span class="hidden md:inline">{capacityLabel()}</span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatUptime(node.uptime)}
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

export default KubernetesNodesTable;
