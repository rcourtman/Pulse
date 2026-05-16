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

// Kubernetes nodes carry richer Kubelet/runtime metadata than a generic
// Pulse Agent — kubelet version, container runtime, roles
// (control-plane/worker), ready state, pod capacity. They're a hybrid
// row in the canonical model (the registry merges the K8s node onto
// the linked agent host), so the generic infrastructure table renders
// the agent metrics fine but omits the K8s context that matters to the
// cluster operator. This bespoke table reuses canonical shared
// primitives and surfaces the Kubelet-native columns alongside the
// usual CPU/Memory utilisation.

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
  return roles
    .map((role) => role.replace('node-role.kubernetes.io/', ''))
    .join(', ');
};

export const KubernetesNodesTable: Component<{
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
              placeholder="Search nodes"
            />
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
            <Table class="w-full min-w-[1000px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Roles</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Kubelet</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Runtime</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Memory</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Capacity</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Uptime</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
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
                        <TableCell class="px-3 py-2 text-base-content">{cluster()}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{formatRoles(meta()?.roles)}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          {kubelet()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          <span class="truncate inline-block max-w-[10rem]" title={runtime()}>
                            {runtime()}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(node.cpu?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(node.memory?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {capacityLabel()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatUptime(node.uptime)}
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

export default KubernetesNodesTable;
