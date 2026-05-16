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

// Kubernetes clusters are control-plane aggregates, not single processes —
// they have no per-cluster Disk I/O / Uptime / Temperature concepts that
// the generic infrastructure table would render. The cluster row already
// carries aggregated CPU / Memory through `metricsFromKubernetesCluster`,
// but the operator columns that matter for "where do my clusters stand at
// a glance" are name + context + version + counts of nodes, pods, and
// deployments. This bespoke table surfaces those alongside the canonical
// CPU/Memory utilisation. It reuses the same shared primitives every
// other platform-page table uses.

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

export const KubernetesClustersTable: Component<{
  clusters: Resource[];
  // All Kubernetes-tagged resources from the same query, so the table can
  // count nodes/pods/deployments per cluster client-side without firing
  // additional requests.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
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
      else if (resource.type === 'agent' && resource.sources?.includes('kubernetes')) bucket.nodes += 1;
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
              placeholder="Search clusters"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} clusters</>}>
              {visible()} of {total()} clusters
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No clusters match current filters"
                description="Adjust the search or status filter to see more clusters."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[860px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Context</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Version</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Nodes</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Pods</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Deployments</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">CPU</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Memory</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(cluster) => {
                    const name = () =>
                      asTrimmedString(cluster.kubernetes?.clusterName) ||
                      asTrimmedString(cluster.name) ||
                      cluster.id;
                    const context = () => asTrimmedString(cluster.kubernetes?.context) || '—';
                    const version = () => asTrimmedString(cluster.kubernetes?.version) || '—';
                    const counts = () => countsByCluster().get(cluster.id) ?? { nodes: 0, pods: 0, deployments: 0 };
                    const indicator = () => getSimpleStatusIndicator(cluster.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={cluster.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="font-semibold text-base-content truncate" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{context()}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          {version()}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {counts().nodes}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {counts().pods}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                          {counts().deployments}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(cluster.cpu?.current)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatPercent(cluster.memory?.current)}
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

export default KubernetesClustersTable;
