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

// Kubernetes Deployments are scheduling abstractions over their controlled
// pods, so the generic infrastructure table's CPU / Memory / Disk I/O /
// Uptime / Temperature columns are conceptually N/A on these rows and
// render as dashes. This deployment-native table reuses canonical shared
// primitives (Card, Table, SearchInput, FilterButtonGroup, StatusDot) but
// surfaces deployment-meaningful columns only: namespace, cluster,
// desired / updated / ready / available replicas.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const replicaCount = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

export const KubernetesDeploymentsTable: Component<{
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
              placeholder="Search deployments"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} deployments</>}>
              {visible()} of {total()} deployments
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No deployments match current filters"
                description="Adjust the search or status filter to see more deployments."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[820px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Deployment</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Namespace</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Cluster</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Desired</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Updated</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Ready</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Available</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(deployment) => {
                    const name = () => asTrimmedString(deployment.name) || deployment.id;
                    const ns = () => asTrimmedString(deployment.kubernetes?.namespace) || '—';
                    const cluster = () =>
                      asTrimmedString(deployment.kubernetes?.clusterName) ||
                      asTrimmedString(deployment.kubernetes?.clusterId) ||
                      '—';
                    const indicator = () => getSimpleStatusIndicator(deployment.status);
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <div class="flex items-center gap-2 min-w-0">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={deployment.status || 'unknown'}
                              ariaHidden
                            />
                            <span
                              class="font-semibold text-base-content truncate"
                              title={name()}
                            >
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{ns()}</TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{cluster()}</TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(deployment.kubernetes?.desiredReplicas)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(deployment.kubernetes?.updatedReplicas)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(deployment.kubernetes?.readyReplicas)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {replicaCount(deployment.kubernetes?.availableReplicas)}
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

export default KubernetesDeploymentsTable;
