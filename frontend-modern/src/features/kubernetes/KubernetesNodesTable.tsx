import { For, Show, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
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
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';

// Kubernetes nodes carry richer Kubelet/runtime metadata than a generic
// Pulse Agent — kubelet version, container runtime, roles
// (control-plane/worker), ready state, pod capacity. They're a hybrid
// row in the canonical model (the registry merges the K8s node onto
// the linked agent host), so the generic infrastructure table renders
// the agent metrics fine but omits the K8s context that matters to the
// cluster operator. This bespoke table reuses canonical shared
// primitives and surfaces the Kubelet-native columns alongside the
// canonical bar treatment (ResponsiveMetricCell / StackedMemoryBar) so
// the Overview stack reads as one consistent surface alongside the
// Docker / Proxmox / vSphere host tables.

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

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
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-node-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

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
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search nodes"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="nodes"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
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
            <Table class="min-w-full table-fixed text-xs md:min-w-[1280px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  {/*
                    Desktop widths: Node gets headroom for cluster-style
                    names, Runtime gets room for "containerd://1.7.20"
                    -style values, Capacity gets room for "6 cores /
                    51.0 GB" strings, CPU and Memory bars share an
                    equal slice, and the short-text columns (Cluster,
                    Roles, Kubelet, Uptime) trim accordingly. Mobile
                    widths are unchanged.
                  */}
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[15%]`}>
                    Node
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}>
                    Cluster
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}>
                    Roles
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[8%]`}>
                    Kubelet
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[15%]`}>
                    Runtime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[11%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[11%]`}>
                    Memory
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[6%]`}>
                    Uptime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[14%]`}>
                    Capacity
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
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
                    const metricsKey = () => buildMetricKeyForUnifiedResource(node);
                    const cpuPercent = () => finiteMetric(node.cpu?.current);
                    const memoryTotal = () => finiteMetric(node.memory?.total) ?? 0;
                    const memoryUsed = () => finiteMetric(node.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0 ? undefined : finiteMetric(node.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const detailRowId = () => drawer.detailRowId(node);
                    const isExpanded = () => drawer.isExpanded(node);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-node-row={node.id}
                          onClick={() => drawer.toggle(node)}
                          onKeyDown={drawer.handleActivationKey(node)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {cluster()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {formatRoles(meta()?.roles)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            {kubelet()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            <span class="truncate inline-block max-w-[10rem]" title={runtime()}>
                              {runtime()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <ResponsiveMetricCell
                              class="w-full"
                              value={cpuPercent() ?? 0}
                              type="cpu"
                              resourceId={metricsKey()}
                              isRunning={canRenderMetrics() && cpuPercent() !== undefined}
                              showMobile={false}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={metricFallback()}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            {formatUptime(node.uptime)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            <span class="md:hidden">{compactCapacityLabel()}</span>
                            <span class="hidden md:inline">{capacityLabel()}</span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={node}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={9}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(node)}
                        />
                      </>
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
