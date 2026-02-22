import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { buildWorkloadsPath } from '@/routing/resourceLinks';

type NamespaceCounts = {
  total: number;
  online: number;
  warning: number;
  offline: number;
  unknown: number;
};

type NamespaceRow = {
  namespace: string;
  pods: NamespaceCounts;
  deployments: NamespaceCounts;
};

type NamespacesResponse = {
  cluster: string;
  data: NamespaceRow[];
};

const PAGE_TITLE = 'Namespaces';

const normalize = (value?: string | null) => (value || '').trim();

const formatInteger = (value?: number | null): string => {
  const n = Number(value ?? 0);
  if (!Number.isFinite(n)) return '0';
  return Math.round(n).toLocaleString();
};

const statusTone = (counts: NamespaceCounts) => {
  if (counts.offline > 0) return 'bg-rose-500';
  if (counts.warning > 0) return 'bg-amber-500';
  if (counts.online > 0) return 'bg-emerald-500';
  return 'bg-slate-400';
};

export const K8sNamespacesDrawer: Component<{
  cluster: string;
  onOpenDeployments?: (namespace: string | null) => void;
}> = (props) => {
  const navigate = useNavigate();
  const [search, setSearch] = createSignal('');

  const clusterName = createMemo(() => normalize(props.cluster));

  const [namespaces] = createResource(
    clusterName,
    async (cluster) => {
      if (!cluster) return { cluster: '', data: [] } as NamespacesResponse;
      const params = new URLSearchParams();
      params.set('cluster', cluster);
      return apiFetchJSON<NamespacesResponse>(`/api/resources/k8s/namespaces?${params.toString()}`, { cache: 'no-store' });
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
        type: 'k8s',
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
 <div class="text-sm font-semibold text-base-content">{PAGE_TITLE}</div>
 <div class="text-xs text-muted">Scope Pods and Deployments by namespace</div>
 </div>

 <div class="flex flex-wrap items-center gap-2">
 <input
 value={search()}
 onInput={(e) => setSearch(e.currentTarget.value)}
 placeholder="Search namespaces..."
 class="w-[12rem] rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
 />
 <button
 type="button"
 onClick={() => openPods(null)}
 class="rounded-md border border-border bg-surface px-3 py-1 text-xs font-semibold shadow-sm hover:bg-slate-50 dark:hover:bg-slate-800"
 >
 Open All Pods
 </button>
 </div>
 </div>
 </Card>

 <Show
 when={!namespaces.loading}
 fallback={
 <Card padding="lg">
 <EmptyState title="Loading namespaces..." description="Aggregating Kubernetes namespaces." />
 </Card>
 }
 >
 <Show
 when={!loadError()}
 fallback={
 <Card padding="lg" tone="danger">
 <EmptyState title="Failed to load namespaces" description={loadError() ||'Unknown error'} tone="danger" />
            </Card>
          }
        >
          <Show
            when={filteredRows().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState
                  title={rows().length > 0 ? 'No namespaces match your filters' : 'No namespaces found'}
                  description={
                    rows().length > 0
                      ? 'Try clearing your search.'
                      : 'Enable Kubernetes collection and wait for the next report.'
                  }
                />
              </Card>
            }
          >
            <Card padding="none" tone="card" class="overflow-hidden">
              <div class="overflow-x-auto">
                <Table class="w-full min-w-[720px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">Namespace</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Pods</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Deployments</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-gray-100 dark:divide-gray-700">
                    <For each={filteredRows()}>
                      {(row) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell class="px-3 py-2">
                            <div class="flex items-center gap-2 min-w-0">
                              <span class={`h-2 w-2 rounded-full ${statusTone(row.pods)}`} />
                              <span class="font-semibold text-base-content truncate" title={row.namespace}>
                                {row.namespace}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            <span class="font-semibold">{formatInteger(row.pods.total)}</span>
                            <span class="ml-2 text-[11px] text-muted">
                              {row.pods.offline > 0 ? `${formatInteger(row.pods.offline)} off` : ''}
                              {row.pods.warning > 0 ? `${row.pods.offline > 0 ? ' · ' : ''}${formatInteger(row.pods.warning)} warn` : ''}
                            </span>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            <span class="font-semibold">{formatInteger(row.deployments.total)}</span>
                            <span class="ml-2 text-[11px] text-muted">
                              {row.deployments.warning > 0 ? `${formatInteger(row.deployments.warning)} warn` : ''}
                              {row.deployments.offline > 0 ? `${row.deployments.warning > 0 ? ' · ' : ''}${formatInteger(row.deployments.offline)} off` : ''}
                            </span>
                          </TableCell>
                          <TableCell class="px-3 py-2">
                            <div class="flex flex-wrap items-center gap-2">
                              <button
                                type="button"
                                onClick={() => openPods(row.namespace)}
                                class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-slate-50 dark:hover:bg-slate-800"
                              >
                                Open Pods
                              </button>
                              <Show when={props.onOpenDeployments}>
                                <button
                                  type="button"
                                  onClick={() => props.onOpenDeployments?.(row.namespace)}
                                  class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-slate-50 dark:hover:bg-slate-800"
                                >
                                  View Deployments
                                </button>
                              </Show>
                            </div>
                          </TableCell>
                        </TableRow>
                      )}
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
