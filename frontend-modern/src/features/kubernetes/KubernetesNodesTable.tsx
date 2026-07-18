import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import { getAlertStyles } from '@/utils/alerts';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableMetricFallback,
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableBytesValue,
  formatPlatformTableTextValue,
  formatPlatformTableUptimeValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesNodes,
  filterKubernetesResources,
  kubernetesClusterLabel,
  mapKubernetesNodeStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

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

const formatRoles = (roles: string[] | undefined): string => {
  if (!roles || roles.length === 0) return '—';
  return roles.map((role) => role.replace('node-role.kubernetes.io/', '')).join(', ');
};

const KUBERNETES_NODE_SORT_KEYS = [
  'node',
  'cluster',
  'roles',
  'kubelet',
  'runtime',
  'cpu',
  'memory',
  'uptime',
  'capacity',
] as const;

type KubernetesNodeSortKey = (typeof KUBERNETES_NODE_SORT_KEYS)[number];

// Scalar per column that user-controlled sorting orders on. Capacity is a
// composite label (cores / bytes / pods); CPU core count is its dominant
// scalar, so that is what the column sorts by.
const getKubernetesNodeSortValue = (
  node: Resource,
  key: KubernetesNodeSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'node':
      return asTrimmedString(node.name) || node.id;
    case 'cluster':
      return kubernetesClusterLabel(node) || null;
    case 'roles': {
      const roles = formatRoles(node.kubernetes?.roles);
      return roles === '—' ? null : roles;
    }
    case 'kubelet':
      return asTrimmedString(node.kubernetes?.kubeletVersion) || null;
    case 'runtime':
      return asTrimmedString(node.kubernetes?.containerRuntimeVersion) || null;
    case 'cpu':
      return getPlatformTableFiniteMetric(node.cpu?.current) ?? null;
    case 'memory': {
      const total = getPlatformTableFiniteMetric(node.memory?.total) ?? 0;
      if (total > 0) {
        return ((getPlatformTableFiniteMetric(node.memory?.used) ?? 0) / total) * 100;
      }
      return getPlatformTableFiniteMetric(node.memory?.current) ?? null;
    }
    case 'uptime':
      return getPlatformTableFiniteMetric(node.uptime) ?? null;
    case 'capacity': {
      const cores = node.kubernetes?.capacityCpuCores;
      return typeof cores === 'number' && cores > 0 ? cores : null;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesNodesTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = alertsActivation.detectionEnabled;
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-node-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  // User-controlled sorting layered over the attention-first default: rows
  // are pre-sorted by the status compare, so a user sort keeps that order
  // for ties and the table falls straight back to it when the sort clears.
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesNodes',
    sortKeys: KUBERNETES_NODE_SORT_KEYS,
    descendingFirst: ['cpu', 'memory', 'uptime', 'capacity'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(
      [...tableState.filtered()].sort(compareKubernetesNodes),
      getKubernetesNodeSortValue,
    ),
  );

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
          <PlatformTableShell
            title={props.title ?? 'Nodes'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1100px] 2xl:min-w-[1280px]"
            header={
              <>
                {/*
                    Desktop widths: Node gets headroom for cluster-style
                    names, Runtime gets room for "containerd://1.7.20"
                    -style values, Capacity gets room for "6 cores /
                    51.0 GB / 110 pods" strings, CPU and Memory bars share
                    an equal slice, and the short-text columns (Cluster,
                    Roles, Kubelet, Uptime) trim accordingly. Wide
                    desktop gets extra room without forcing normal desktop
                    viewports to hide Capacity behind horizontal scroll.
                  */}
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="node"
                  class="md:w-[15%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="cluster"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Cluster
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="roles"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Roles
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="kubelet"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Kubelet
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="runtime"
                  class="hidden md:table-cell md:w-[15%]"
                >
                  Runtime
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="cpu"
                  class="md:w-[11%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="memory"
                  class="md:w-[11%]"
                >
                  Memory
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="uptime"
                  class="hidden md:table-cell md:w-[6%]"
                >
                  Uptime
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="capacity"
                  class="md:w-[14%]"
                >
                  Capacity
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(node) => {
                    const meta = () => node.kubernetes;
                    const name = () => asTrimmedString(node.name) || node.id;
                    const cluster = () => kubernetesClusterLabel(node) || '—';
                    const kubelet = () => formatPlatformTableTextValue(meta()?.kubeletVersion);
                    const runtime = () =>
                      formatPlatformTableTextValue(meta()?.containerRuntimeVersion);
                    const capacityLabel = () => {
                      const cores = meta()?.capacityCpuCores;
                      const mem = meta()?.capacityMemoryBytes;
                      const pods = meta()?.capacityPods;
                      const parts: string[] = [];
                      if (typeof cores === 'number' && cores > 0) parts.push(`${cores} cores`);
                      if (typeof mem === 'number' && mem > 0) {
                        parts.push(formatPlatformTableBytesValue(mem));
                      }
                      if (typeof pods === 'number' && pods > 0) parts.push(`${pods} pods`);
                      return parts.join(' / ') || '—';
                    };
                    const compactCapacityLabel = () => {
                      const cores = meta()?.capacityCpuCores;
                      if (typeof cores === 'number' && cores > 0) return `${cores} cores`;
                      return capacityLabel();
                    };
                    const indicator = () => mapKubernetesNodeStatus(node);
                    const metricsKey = () => buildMetricKeyForUnifiedResource(node);
                    const cpuPercent = () => getPlatformTableFiniteMetric(node.cpu?.current);
                    const memoryTotal = () => getPlatformTableFiniteMetric(node.memory?.total) ?? 0;
                    const memoryUsed = () => getPlatformTableFiniteMetric(node.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0
                        ? undefined
                        : getPlatformTableFiniteMetric(node.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const detailRowId = () => drawer.detailRowId(node);
                    const isExpanded = () => drawer.isExpanded(node);
                    const nodeAlertStyles = createMemo(() =>
                      getAlertStyles(node.id, activeAlerts, alertsEnabled(), name()),
                    );
                    const nodeAlertBg = () => {
                      const s = nodeAlertStyles();
                      if (!s.hasUnacknowledgedAlert) return '';
                      return s.severity === 'critical'
                        ? 'bg-red-50 dark:bg-red-950'
                        : 'bg-yellow-50 dark:bg-yellow-950';
                    };
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs ${nodeAlertBg()}`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-node-row={node.id}
                          onClick={() => drawer.toggle(node)}
                          onKeyDown={drawer.handleActivationKey(node)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(node)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
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
                              fallback={<PlatformTableMetricFallback />}
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
                            {formatPlatformTableUptimeValue(node.uptime)}
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
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesNodesTable;
