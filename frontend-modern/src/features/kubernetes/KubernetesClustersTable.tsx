import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
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
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  buildKubernetesClusterChildCounts,
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

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Clusters'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1040px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[17%]`}>
                    Cluster
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[15%]`}>
                    Context
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}>
                    Version
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}>
                    Nodes
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}>
                    Pods
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[12%]`}>
                    Deployments
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[15%]`}>
                    CPU
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[15%]`}>
                    Memory
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(cluster) => {
                    const name = () =>
                      asTrimmedString(cluster.kubernetes?.clusterName) ||
                      asTrimmedString(cluster.name) ||
                      cluster.id;
                    const context = () => asTrimmedString(cluster.kubernetes?.context) || '—';
                    const version = () => asTrimmedString(cluster.kubernetes?.version) || '—';
                    const metricsKey = () => buildMetricKeyForUnifiedResource(cluster);
                    const cpuPercent = () => finiteMetric(cluster.cpu?.current);
                    const memoryTotal = () => finiteMetric(cluster.memory?.total) ?? 0;
                    const memoryUsed = () => finiteMetric(cluster.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0 ? undefined : finiteMetric(cluster.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const counts = () =>
                      countsByCluster().get(cluster.id) ?? {
                        nodes: 0,
                        pods: 0,
                        deployments: 0,
                      };
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
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={cluster.status || 'unknown'}
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
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {counts().nodes}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {counts().pods}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {counts().deployments}
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
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesClustersTable;
