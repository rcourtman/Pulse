import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { buildWorkloadsPath } from '@/routing/resourceLinks';

const PAGE_LIMIT = 100;
const MAX_PAGES = 20;

type K8sDeploymentResource = {
  id: string;
  name?: string;
  status?: string;
  kubernetes?: {
    namespace?: string;
    desiredReplicas?: number;
    updatedReplicas?: number;
    readyReplicas?: number;
    availableReplicas?: number;
  };
};

type ResourcesListResponse = {
  data?: K8sDeploymentResource[];
  meta?: {
    totalPages?: number;
  };
};

const normalize = (value?: string | null) => (value || '').trim();

const buildDeploymentsUrl = (cluster: string, page: number) => {
  const params = new URLSearchParams();
  params.set('type', 'k8s-deployment');
  params.set('cluster', cluster);
  params.set('page', String(page));
  params.set('limit', String(PAGE_LIMIT));
  return `/api/resources?${params.toString()}`;
};

const fetchAllDeployments = async (cluster: string): Promise<K8sDeploymentResource[]> => {
  const first = await apiFetchJSON<ResourcesListResponse>(buildDeploymentsUrl(cluster, 1), { cache: 'no-store' });
  const firstData = Array.isArray(first?.data) ? first.data : [];
  const totalPages = Number.isFinite(first?.meta?.totalPages)
    ? Math.max(1, Number(first.meta?.totalPages))
    : 1;

  const deployments: K8sDeploymentResource[] = [...firstData];

  const cappedPages = Math.min(totalPages, MAX_PAGES);
  if (cappedPages > 1) {
    const requests: Array<Promise<ResourcesListResponse>> = [];
    for (let page = 2; page <= cappedPages; page += 1) {
      requests.push(apiFetchJSON<ResourcesListResponse>(buildDeploymentsUrl(cluster, page), { cache: 'no-store' }));
    }

    const settled = await Promise.allSettled(requests);
    for (const result of settled) {
      if (result.status !== 'fulfilled') continue;
      const data = Array.isArray(result.value?.data) ? result.value.data : [];
      deployments.push(...data);
    }
  }

  return Array.from(new Map(deployments.map((d) => [d.id, d])).values());
};

const statusTone = (status?: string) => {
  const normalized = (status || '').trim().toLowerCase();
  if (!normalized) return 'bg-gray-400';
  if (normalized === 'online' || normalized === 'running' || normalized === 'healthy') return 'bg-green-500';
  if (normalized === 'warning' || normalized === 'degraded') return 'bg-amber-500';
  if (normalized === 'offline' || normalized === 'stopped') return 'bg-red-500';
  return 'bg-gray-400';
};

export const K8sDeploymentsDrawer: Component<{ cluster: string }> = (props) => {
  const navigate = useNavigate();
  const [namespace, setNamespace] = createSignal('');
  const [search, setSearch] = createSignal('');

  const clusterName = createMemo(() => normalize(props.cluster));

  const [deployments] = createResource(
    clusterName,
    async (cluster) => {
      if (!cluster) return [];
      return fetchAllDeployments(cluster);
    },
    { initialValue: [] },
  );

  const namespaceOptions = createMemo(() => {
    const set = new Set<string>();
    for (const dep of deployments()) {
      const ns = normalize(dep.kubernetes?.namespace);
      if (ns) set.add(ns);
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b));
  });

  createEffect(() => {
    const current = normalize(namespace());
    if (!current) return;
    const exists = namespaceOptions().some((ns) => ns.toLowerCase() === current.toLowerCase());
    if (!exists) {
      setNamespace('');
    }
  });

  const filteredDeployments = createMemo(() => {
    const ns = normalize(namespace());
    const term = normalize(search()).toLowerCase();

    return deployments()
      .filter((dep) => {
        if (ns && normalize(dep.kubernetes?.namespace) !== ns) return false;
        if (!term) return true;
        const name = normalize(dep.name) || dep.id;
        return name.toLowerCase().includes(term);
      })
      .sort((a, b) => {
        const aName = (normalize(a.name) || a.id).toLowerCase();
        const bName = (normalize(b.name) || b.id).toLowerCase();
        if (aName !== bName) return aName < bName ? -1 : 1;
        return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
      });
  });

  const openPods = (ns?: string) => {
    const cluster = clusterName();
    if (!cluster) return;
    navigate(
      buildWorkloadsPath({
        type: 'k8s',
        context: cluster,
        namespace: normalize(ns) || null,
      }),
    );
  };

  return (
    <div class="space-y-3">
      <Card padding="md">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-gray-800 dark:text-gray-100">Deployments</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">Desired state controllers (not Pods)</div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <input
              value={search()}
              onInput={(e) => setSearch(e.currentTarget.value)}
              placeholder="Search deployments..."
              class="w-[12rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-900 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-100"
            />

            <Show when={namespaceOptions().length > 0}>
              <div class="inline-flex items-center rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                <label
                  for="k8s-deployments-namespace"
                  class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                >
                  Namespace
                </label>
                <select
                  id="k8s-deployments-namespace"
                  value={namespace()}
                  onChange={(e) => setNamespace(e.currentTarget.value)}
                  class="min-w-[10rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                >
                  <option value="">All namespaces</option>
                  <For each={namespaceOptions()}>
                    {(ns) => <option value={ns}>{ns}</option>}
                  </For>
                </select>
              </div>
            </Show>

            <button
              type="button"
              onClick={() => openPods(namespace() || undefined)}
              class="rounded-md border border-gray-200 bg-white px-3 py-1 text-xs font-semibold text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-200 dark:hover:bg-gray-800"
            >
              Open Pods
            </button>
          </div>
        </div>
      </Card>

      <Show
        when={deployments.loading}
        fallback={
          <Show
            when={filteredDeployments().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState
                  title={deployments().length > 0 ? 'No deployments match your filters' : 'No deployments found'}
                  description={
                    deployments().length > 0
                      ? 'Try clearing the search or namespace filter.'
                      : 'Enable the Kubernetes agent deployment collection, then wait for the next report.'
                  }
                />
              </Card>
            }
          >
            <Card padding="none" tone="glass" class="overflow-hidden">
              <div class="overflow-x-auto">
                <table class="w-full min-w-[760px] border-collapse text-xs">
                  <thead class="bg-gray-50 dark:bg-gray-800/60 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                    <tr class="text-left text-[10px] uppercase tracking-wide">
                      <th class="px-3 py-2 font-medium">Deployment</th>
                      <th class="px-3 py-2 font-medium">Namespace</th>
                      <th class="px-3 py-2 font-medium">Desired</th>
                      <th class="px-3 py-2 font-medium">Updated</th>
                      <th class="px-3 py-2 font-medium">Ready</th>
                      <th class="px-3 py-2 font-medium">Available</th>
                      <th class="px-3 py-2 font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                    <For each={filteredDeployments()}>
                      {(dep) => {
                        const name = () => normalize(dep.name) || dep.id;
                        const ns = () => normalize(dep.kubernetes?.namespace) || '—';
                        const desired = () => dep.kubernetes?.desiredReplicas ?? 0;
                        const updated = () => dep.kubernetes?.updatedReplicas ?? 0;
                        const ready = () => dep.kubernetes?.readyReplicas ?? 0;
                        const available = () => dep.kubernetes?.availableReplicas ?? 0;

                        return (
                          <tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
                            <td class="px-3 py-2">
                              <div class="flex items-center gap-2 min-w-0">
                                <span class={`h-2 w-2 rounded-full ${statusTone(dep.status)}`} title={dep.status || 'unknown'} />
                                <span class="font-semibold text-gray-900 dark:text-gray-100 truncate" title={name()}>
                                  {name()}
                                </span>
                              </div>
                            </td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{ns()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{desired()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{updated()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{ready()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{available()}</td>
                            <td class="px-3 py-2">
                              <button
                                type="button"
                                onClick={() => openPods(dep.kubernetes?.namespace)}
                                class="rounded-md border border-gray-200 bg-white px-2 py-1 text-[11px] font-semibold text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-200 dark:hover:bg-gray-800"
                              >
                                View Pods
                              </button>
                            </td>
                          </tr>
                        );
                      }}
                    </For>
                  </tbody>
                </table>
              </div>
            </Card>
          </Show>
        }
      >
        <Card padding="lg">
          <EmptyState title="Loading deployments..." description="Fetching unified resources." />
        </Card>
      </Show>
    </div>
  );
};

export default K8sDeploymentsDrawer;
