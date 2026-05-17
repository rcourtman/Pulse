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

// Kubernetes clusters are control-plane aggregates, not single processes —
// they have no per-cluster Disk I/O / Uptime / Temperature concepts that
// the generic infrastructure table would render. The cluster row already
// carries aggregated CPU / Memory through `metricsFromKubernetesCluster`,
// but the operator columns that matter for "where do my clusters stand at
// a glance" are name + context + version + counts of nodes, pods, and
// deployments. This bespoke table surfaces those alongside the canonical
// CPU/Memory utilisation. It reuses the same shared primitives every
// other platform-page table uses.

const formatPercent = (percent?: number): JSX.Element => {
  if (typeof percent !== 'number' || Number.isNaN(percent))
    return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{percent.toFixed(1)}%</span>;
};

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
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.clusters, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.clusters.length);

  const countsByCluster = createMemo(() => {
    const map = new Map<string, { nodes: number; pods: number; deployments: number }>();
    for (const cluster of props.clusters) {
      map.set(cluster.id, { nodes: 0, pods: 0, deployments: 0 });
    }
    for (const resource of props.scope) {
      const clusterId =
        asTrimmedString(resource.kubernetes?.clusterId) ||
        asTrimmedString(resource.kubernetes?.clusterName);
      if (!clusterId) continue;
      // Match against the cluster row by clusterId. The cluster row's
      // own canonical id differs from its `kubernetes.clusterId`, so map
      // by the kubernetes-side identifier first, then fall back to row id.
      let bucket = null as { nodes: number; pods: number; deployments: number } | null;
      for (const cluster of props.clusters) {
        const k = cluster.kubernetes;
        if (!k) continue;
        if (
          asTrimmedString(k.clusterId) === clusterId ||
          asTrimmedString(k.clusterName) === clusterId
        ) {
          bucket = map.get(cluster.id) ?? null;
          break;
        }
      }
      if (!bucket) continue;
      if (resource.type === 'k8s-node') bucket.nodes += 1;
      else if (resource.type === 'agent' && resource.sources?.includes('kubernetes'))
        bucket.nodes += 1;
      else if (resource.type === 'pod') bucket.pods += 1;
      else if (resource.type === 'k8s-deployment') bucket.deployments += 1;
    }
    // Fallback when scope-based counts come back empty (e.g. tests that
    // only supply the cluster rows): keep the rendered counts honest at 0.
    return map;
  });

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
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder="Search clusters"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="clusters"
          />
        </Show>

        <Show
          when={filtered().length > 0}
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
            <Table class="min-w-full table-fixed text-xs md:min-w-[860px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Cluster</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Context
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Version
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Nodes</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Pods
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Deployments
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>CPU</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Memory</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(cluster) => {
                    const name = () =>
                      asTrimmedString(cluster.kubernetes?.clusterName) ||
                      asTrimmedString(cluster.name) ||
                      cluster.id;
                    const context = () => asTrimmedString(cluster.kubernetes?.context) || '—';
                    const version = () => asTrimmedString(cluster.kubernetes?.version) || '—';
                    const counts = () =>
                      countsByCluster().get(cluster.id) ?? {
                        nodes: 0,
                        pods: 0,
                        deployments: 0,
                      };
                    const indicator = () => getSimpleStatusIndicator(cluster.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
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
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {context()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          {version()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content tabular-nums`}
                        >
                          {counts().nodes}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {counts().pods}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content tabular-nums md:table-cell`}
                        >
                          {counts().deployments}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(cluster.cpu?.current)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatPercent(cluster.memory?.current)}
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

export default KubernetesClustersTable;
