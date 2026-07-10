import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableRow } from '@/components/shared/Table';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableCountRatioValue,
  PlatformTableMetricFallback,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
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
  buildKubernetesClusterChildCounts,
  emptyKubernetesClusterChildCounts,
  filterKubernetesResources,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// Kubernetes clusters are control-plane aggregates, not single processes —
// they have no per-cluster Disk I/O / Uptime / Temperature concepts that
// the generic infrastructure table would render. The cluster row already
// carries aggregated CPU / Memory through `metricsFromKubernetesCluster`,
// but the operator columns that matter for "where do my clusters stand at
// a glance" are name + context + version + counts of nodes, pods, and
// deployments. This bespoke table surfaces those alongside the canonical
// CPU/Memory utilisation. It reuses the same shared primitives every
// other platform-page table uses.

const KUBERNETES_CLUSTER_SORT_KEYS = [
  'cluster',
  'context',
  'version',
  'nodes',
  'pods',
  'deployments',
  'cpu',
  'memory',
] as const;

type KubernetesClusterSortKey = (typeof KUBERNETES_CLUSTER_SORT_KEYS)[number];

export const KubernetesClustersTable: Component<{
  clusters: Resource[];
  // All Kubernetes-tagged resources from the same query, so the table can
  // count nodes/pods/deployments per cluster client-side without firing
  // additional requests.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.clusters,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-cluster-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

  const countsByCluster = createMemo(() =>
    buildKubernetesClusterChildCounts(props.scope, props.clusters),
  );

  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesClusters',
    sortKeys: KUBERNETES_CLUSTER_SORT_KEYS,
    descendingFirst: ['nodes', 'pods', 'deployments', 'cpu', 'memory'],
  });
  // Closure rather than a module-level accessor: the count columns derive
  // from the countsByCluster memo over props.scope.
  const getClusterSortValue = (
    cluster: Resource,
    key: KubernetesClusterSortKey,
  ): PlatformTableSortValue => {
    switch (key) {
      case 'cluster':
        return (
          asTrimmedString(cluster.kubernetes?.clusterName) ||
          asTrimmedString(cluster.name) ||
          cluster.id
        );
      case 'context':
        return asTrimmedString(cluster.kubernetes?.context) || null;
      case 'version':
        return asTrimmedString(cluster.kubernetes?.version) || null;
      case 'nodes':
        return countsByCluster().get(cluster.id)?.nodes.total ?? null;
      case 'pods':
        return countsByCluster().get(cluster.id)?.pods.total ?? null;
      case 'deployments':
        return countsByCluster().get(cluster.id)?.deployments.total ?? null;
      case 'cpu':
        return getPlatformTableFiniteMetric(cluster.cpu?.current) ?? null;
      case 'memory': {
        const total = getPlatformTableFiniteMetric(cluster.memory?.total) ?? 0;
        if (total > 0) {
          return ((getPlatformTableFiniteMetric(cluster.memory?.used) ?? 0) / total) * 100;
        }
        return getPlatformTableFiniteMetric(cluster.memory?.current) ?? null;
      }
      default:
        key satisfies never;
        return null;
    }
  };
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getClusterSortValue));

  return (
    <Show
      when={props.clusters.length > 0}
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
            searchPlaceholder="Search clusters"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="clusters"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No clusters match current filters"
              description="Adjust the search or status filter to see more clusters."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Clusters'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="cluster"
                  class="md:w-[17%]"
                >
                  Cluster
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="context"
                  class="hidden md:table-cell md:w-[15%]"
                >
                  Context
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="version"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Version
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="nodes"
                  class="md:w-[8%]"
                >
                  Nodes
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="pods"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Pods
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="deployments"
                  class="hidden md:table-cell md:w-[12%]"
                >
                  Deployments
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="cpu"
                  class="md:w-[15%]"
                >
                  CPU
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="metric-bar"
                  sort={sort}
                  sortKey="memory"
                  class="md:w-[15%]"
                >
                  Memory
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(cluster) => {
                    const name = () =>
                      asTrimmedString(cluster.kubernetes?.clusterName) ||
                      asTrimmedString(cluster.name) ||
                      cluster.id;
                    const context = () => formatPlatformTableTextValue(cluster.kubernetes?.context);
                    const version = () => formatPlatformTableTextValue(cluster.kubernetes?.version);
                    const metricsKey = () => buildMetricKeyForUnifiedResource(cluster);
                    const cpuPercent = () => getPlatformTableFiniteMetric(cluster.cpu?.current);
                    const memoryTotal = () =>
                      getPlatformTableFiniteMetric(cluster.memory?.total) ?? 0;
                    const memoryUsed = () =>
                      getPlatformTableFiniteMetric(cluster.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0
                        ? undefined
                        : getPlatformTableFiniteMetric(cluster.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const counts = () =>
                      countsByCluster().get(cluster.id) ?? emptyKubernetesClusterChildCounts();
                    const indicator = () => getSimpleStatusIndicator(cluster.status);
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const detailRowId = () => drawer.detailRowId(cluster);
                    const isExpanded = () => drawer.isExpanded(cluster);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-cluster-row={cluster.id}
                          onClick={() => drawer.toggle(cluster)}
                          onKeyDown={drawer.handleActivationKey(cluster)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(cluster)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={cluster.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                              <Show when={cluster.kubernetes?.pendingUninstall === true}>
                                <span class="inline-flex shrink-0 items-center rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                                  Pending uninstall
                                </span>
                              </Show>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {context()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 font-mono text-[10px] text-base-content">
                              {version()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableCountRatioValue
                              current={counts().nodes.total - counts().nodes.attention}
                              total={counts().nodes.total}
                              currentTone={counts().nodes.attention > 0 ? 'warning' : undefined}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableCountRatioValue
                              current={counts().pods.total - counts().pods.attention}
                              total={counts().pods.total}
                              currentTone={counts().pods.attention > 0 ? 'warning' : undefined}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableCountRatioValue
                              current={counts().deployments.total - counts().deployments.attention}
                              total={counts().deployments.total}
                              currentTone={
                                counts().deployments.attention > 0 ? 'warning' : undefined
                              }
                            />
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
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={cluster}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(cluster)}
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

export default KubernetesClustersTable;
