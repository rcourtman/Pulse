import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { SearchField } from '@/components/shared/SearchField';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
import { buildWorkloadsPath } from '@/routing/resourceLinks';
import {
  getK8sNamespacesDrawerPresentation,
  getK8sNamespacesEmptyState,
  getK8sNamespacesFailureState,
  getK8sNamespacesLoadingState,
} from '@/utils/k8sNamespacePresentation';
import { getNamespaceCountsIndicator, type NamespaceCounts } from '@/utils/k8sStatusPresentation';

type NamespaceRow = {
  namespace: string;
  pods: NamespaceCounts;
  deployments: NamespaceCounts;
};

type NamespacesResponse = {
  cluster: string;
  data: NamespaceRow[];
};

const normalize = (value?: string | null) => (value || '').trim();

const formatInteger = (value?: number | null): string => {
  const n = Number(value ?? 0);
  if (!Number.isFinite(n)) return '0';
  return Math.round(n).toLocaleString();
};

export const K8sNamespacesDrawer: Component<{
  cluster: string;
  onOpenDeployments?: (namespace: string | null) => void;
}> = (props) => {
  const navigate = useNavigate();
  const [search, setSearch] = createSignal('');
  const drawerPresentation = getK8sNamespacesDrawerPresentation();

  const clusterName = createMemo(() => normalize(props.cluster));

  const [namespaces] = createResource(
    clusterName,
    async (cluster) => {
      if (!cluster) return { cluster: '', data: [] } as NamespacesResponse;
      const params = new URLSearchParams();
      params.set('cluster', cluster);
      return apiFetchJSON<NamespacesResponse>(
        `/api/resources/k8s/namespaces?${params.toString()}`,
        { cache: 'no-store' },
      );
    },
    { initialValue: { cluster: '', data: [] } as NamespacesResponse },
  );

  const loadError = createMemo(() => {
    const err = namespaces.error;
    if (!err) return '';
    return (err as Error)?.message || 'Failed to fetch namespaces';
  });

  const rows = createMemo(() => (Array.isArray(namespaces()?.data) ? namespaces()!.data : []));

  const filteredRows = createMemo(() => {
    const term = normalize(search()).toLowerCase();
    if (!term) return rows();
    return rows().filter((row) => row.namespace.toLowerCase().includes(term));
  });

  const openPods = (namespace: string | null) => {
    const cluster = clusterName();
    if (!cluster) return;
    navigate(
      buildWorkloadsPath({
        type: 'pod',
        context: cluster,
        namespace: namespace ? normalize(namespace) : null,
      }),
    );
  };

  return (
    <div class="space-y-3">
      <Card padding="md">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-base-content">{drawerPresentation.title}</div>
            <div class="text-xs text-muted">{drawerPresentation.description}</div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <SearchField
              value={search()}
              onChange={setSearch}
              placeholder={drawerPresentation.searchPlaceholder}
              class="w-[12rem]"
              inputClass="py-1 text-xs font-medium shadow-sm"
            />
            <button
              type="button"
              onClick={() => openPods(null)}
              class="rounded-md border border-border bg-surface px-3 py-1 text-xs font-semibold shadow-sm hover:bg-surface-hover"
            >
              {drawerPresentation.openAllPodsLabel}
            </button>
          </div>
        </div>
      </Card>

      <Show
        when={!namespaces.loading}
        fallback={
          <Card padding="lg">
            <EmptyState {...getK8sNamespacesLoadingState()} />
          </Card>
        }
      >
        <Show
          when={!loadError()}
          fallback={
            <Card padding="lg" tone="danger">
              <EmptyState {...getK8sNamespacesFailureState(loadError())} tone="danger" />
            </Card>
          }
        >
          <Show
            when={filteredRows().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState {...getK8sNamespacesEmptyState(rows().length > 0)} />
              </Card>
            }
          >
            <Card padding="none" tone="card" class="overflow-hidden">
              <div class="overflow-x-auto">
                <Table class="w-full min-w-[720px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.namespaceColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.podsColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.deploymentsColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.actionsColumnLabel}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={filteredRows()}>
                      {(row) => {
                        const podIndicator = () => getNamespaceCountsIndicator(row.pods);
                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell class="px-3 py-2">
                              <div class="flex items-center gap-2 min-w-0">
                                <StatusDot
                                  size="sm"
                                  variant={podIndicator().variant}
                                  title={podIndicator().label}
                                  ariaHidden
                                />
                                <span
                                  class="font-semibold text-base-content truncate"
                                  title={row.namespace}
                                >
                                  {row.namespace}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              <span class="font-semibold">{formatInteger(row.pods.total)}</span>
                              <span class="ml-2 text-[11px] text-muted">
                                {row.pods.offline > 0
                                  ? `${formatInteger(row.pods.offline)} off`
                                  : ''}
                                {row.pods.warning > 0
                                  ? `${row.pods.offline > 0 ? ' · ' : ''}${formatInteger(row.pods.warning)} warn`
                                  : ''}
                              </span>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              <span class="font-semibold">
                                {formatInteger(row.deployments.total)}
                              </span>
                              <span class="ml-2 text-[11px] text-muted">
                                {row.deployments.warning > 0
                                  ? `${formatInteger(row.deployments.warning)} warn`
                                  : ''}
                                {row.deployments.offline > 0
                                  ? `${row.deployments.warning > 0 ? ' · ' : ''}${formatInteger(row.deployments.offline)} off`
                                  : ''}
                              </span>
                            </TableCell>
                            <TableCell class="px-3 py-2">
                              <div class="flex flex-wrap items-center gap-2">
                                <button
                                  type="button"
                                  onClick={() => openPods(row.namespace)}
                                  class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-surface-hover"
                                >
                                  {drawerPresentation.openPodsLabel}
                                </button>
                                <Show when={props.onOpenDeployments}>
                                  <button
                                    type="button"
                                    onClick={() => props.onOpenDeployments?.(row.namespace)}
                                    class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-surface-hover"
                                  >
                                    {drawerPresentation.viewDeploymentsLabel}
                                  </button>
                                </Show>
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </div>
            </Card>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default K8sNamespacesDrawer;
