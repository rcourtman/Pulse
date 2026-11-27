import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformSortableTableHead,
  PlatformTableCountRatioValue,
  PlatformTableMetricFallback,
  PlatformResponsiveTableLabel,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableFiniteMetric,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  buildKubernetesClusterChildCounts,
  emptyKubernetesClusterChildCounts,
  filterKubernetesResources,
  hasStaleKubernetesReadings,
  mapKubernetesClusterStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';
import {
  KubernetesCpuMemoryCells,
  KubernetesDetailRow,
  KubernetesRowLead,
  KubernetesStaleDataCue,
  KubernetesTableFrame,
  createKubernetesDetailState,
  getKubernetesMemorySortValue,
} from './KubernetesRecordDetail';

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
  selectedClusterId?: string;
  onSelectCluster?: (cluster: Resource) => void;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.clusters,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
  });
  const drawer = createKubernetesDetailState('kubernetes-cluster-drawer');
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
      case 'memory':
        return getKubernetesMemorySortValue(cluster);
      default:
        key satisfies never;
        return null;
    }
  };
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getClusterSortValue));

  return (
    <KubernetesTableFrame
      hasResources={props.clusters.length > 0}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      rowNoun="clusters"
      tableState={tableState}
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
              class="platform-table-mobile-w-30 md:w-[17%]"
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
              class="platform-table-phone-hidden md:w-[10%]"
            >
              <PlatformResponsiveTableLabel compact="Ver" full="Version" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="metric-bar"
              sort={sort}
              sortKey="cpu"
              class="platform-table-mobile-w-20 md:w-[15%]"
            >
              CPU
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="metric-bar"
              sort={sort}
              sortKey="memory"
              class="platform-table-mobile-w-20 md:w-[15%]"
            >
              <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="nodes"
              class="platform-table-mobile-w-15 md:w-[8%]"
            >
              <PlatformResponsiveTableLabel compact="Nds" full="Nodes" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="pods"
              class="platform-table-mobile-w-15 md:w-[8%] platform-table-narrow-hidden"
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
          </>
        }
        body={
          <>
            <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
              {(cluster) => {
                const name = () =>
                  asTrimmedString(cluster.kubernetes?.clusterName) ||
                  asTrimmedString(cluster.name) ||
                  cluster.id;
                const clusterScopeId = () =>
                  asTrimmedString(cluster.kubernetes?.clusterId) ||
                  asTrimmedString(cluster.kubernetes?.clusterName) ||
                  '';
                const context = () => formatPlatformTableTextValue(cluster.kubernetes?.context);
                const version = () => formatPlatformTableTextValue(cluster.kubernetes?.version);
                const counts = () =>
                  countsByCluster().get(cluster.id) ?? emptyKubernetesClusterChildCounts();
                const indicator = () => mapKubernetesClusterStatus(cluster);
                const readingsStale = () => hasStaleKubernetesReadings(cluster);
                const canRenderMetrics = () => indicator().variant !== 'muted';
                const isExpanded = () => drawer.isExpanded(cluster);
                return (
                  <>
                    <TableRow
                      class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                      data-kubernetes-cluster-row={cluster.id}
                      onClick={() => drawer.toggle(cluster)}
                    >
                      <TableCell class={getPlatformTableCellClassForKind('name')}>
                        <KubernetesRowLead
                          resource={cluster}
                          drawer={drawer}
                          label={name()}
                          status={indicator()}
                        >
                          <div class="min-w-0">
                            <div class="flex min-w-0 items-center gap-1.5">
                              <ResourceNameWithWebInterfaceLink
                                name={name()}
                                url={cluster.customUrl}
                                class="min-w-0"
                                nameClass="truncate font-semibold text-base-content"
                              >
                                <Show when={props.onSelectCluster} fallback={name()}>
                                  <button
                                    type="button"
                                    class={`-my-3 inline-flex min-h-11 items-center truncate rounded-xs py-3 text-left font-semibold underline-offset-2 hover:text-blue-600 hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 sm:my-0 sm:min-h-0 sm:py-0 dark:hover:text-blue-400 ${
                                      props.selectedClusterId === clusterScopeId()
                                        ? 'text-blue-600 dark:text-blue-400'
                                        : 'text-base-content'
                                    }`}
                                    aria-label={`Show workloads for ${name()}`}
                                    aria-pressed={
                                      props.selectedClusterId === clusterScopeId()
                                        ? 'true'
                                        : 'false'
                                    }
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      props.onSelectCluster?.(cluster);
                                    }}
                                    onKeyDown={(event) => event.stopPropagation()}
                                  >
                                    {name()}
                                  </button>
                                </Show>
                              </ResourceNameWithWebInterfaceLink>
                              <Show when={cluster.kubernetes?.pendingUninstall === true}>
                                <span class="inline-flex shrink-0 items-center rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                                  Pending uninstall
                                </span>
                              </Show>
                            </div>
                            <KubernetesStaleDataCue when={readingsStale()} scope="cluster" />
                          </div>
                        </KubernetesRowLead>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        {context()}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden font-mono text-[11px] text-base-content`}
                      >
                        <span class="inline-flex items-center rounded-sm bg-surface-alt px-1.5 py-0.5 font-mono text-[10px] text-base-content">
                          {version()}
                        </span>
                      </TableCell>
                      <KubernetesCpuMemoryCells
                        resource={cluster}
                        canRenderMetrics={canRenderMetrics()}
                        memoryFallback={<PlatformTableMetricFallback />}
                      />
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
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content platform-table-narrow-hidden`}
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
                          currentTone={counts().deployments.attention > 0 ? 'warning' : undefined}
                        />
                      </TableCell>
                    </TableRow>
                    <KubernetesDetailRow
                      resource={cluster}
                      statusIndicator={indicator()}
                      drawer={drawer}
                      colSpan={8}
                      resolveResourceLabel={resolveResourceLabel}
                    />
                  </>
                );
              }}
            </PlatformWindowedRows>
          </>
        }
      />
    </KubernetesTableFrame>
  );
};

export default KubernetesClustersTable;
