import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';

const PAGE_LIMIT = 100;
const MAX_PAGES = 20;

type SwarmInfo = {
  nodeId?: string;
  nodeRole?: string;
  localState?: string;
  controlAvailable?: boolean;
  clusterId?: string;
  clusterName?: string;
  scope?: string;
  error?: string;
};

type DockerServiceUpdate = {
  state?: string;
  message?: string;
  completedAt?: string;
};

type DockerServicePort = {
  name?: string;
  protocol?: string;
  targetPort?: number;
  publishedPort?: number;
  publishMode?: string;
};

type DockerServiceResource = {
  id: string;
  name?: string;
  status?: string;
  docker?: {
    serviceId?: string;
    stack?: string;
    image?: string;
    mode?: string;
    desiredTasks?: number;
    runningTasks?: number;
    completedTasks?: number;
    serviceUpdate?: DockerServiceUpdate;
    endpointPorts?: DockerServicePort[];
  };
};

type ResourcesListResponse = {
  data?: DockerServiceResource[];
  meta?: {
    totalPages?: number;
  };
};

const normalize = (value?: string | null) => (value || '').trim();

const buildServicesUrl = (cluster: string, page: number) => {
  const params = new URLSearchParams();
  params.set('type', 'docker_service');
  params.set('cluster', cluster);
  params.set('page', String(page));
  params.set('limit', String(PAGE_LIMIT));
  return `/api/resources?${params.toString()}`;
};

const fetchAllServices = async (cluster: string): Promise<DockerServiceResource[]> => {
  const first = await apiFetchJSON<ResourcesListResponse>(buildServicesUrl(cluster, 1), { cache: 'no-store' });
  const firstData = Array.isArray(first?.data) ? first.data : [];
  const totalPages = Number.isFinite(first?.meta?.totalPages)
    ? Math.max(1, Number(first.meta?.totalPages))
    : 1;

  const services: DockerServiceResource[] = [...firstData];

  const cappedPages = Math.min(totalPages, MAX_PAGES);
  if (cappedPages > 1) {
    const requests: Array<Promise<ResourcesListResponse>> = [];
    for (let page = 2; page <= cappedPages; page += 1) {
      requests.push(apiFetchJSON<ResourcesListResponse>(buildServicesUrl(cluster, page), { cache: 'no-store' }));
    }

    const settled = await Promise.allSettled(requests);
    for (const result of settled) {
      if (result.status !== 'fulfilled') continue;
      const data = Array.isArray(result.value?.data) ? result.value.data : [];
      services.push(...data);
    }
  }

  return Array.from(new Map(services.map((s) => [s.id, s])).values());
};

const statusTone = (status?: string) => {
  const normalized = (status || '').trim().toLowerCase();
  if (!normalized) return 'bg-gray-400';
  if (normalized === 'online' || normalized === 'running' || normalized === 'healthy') return 'bg-green-500';
  if (normalized === 'warning' || normalized === 'degraded') return 'bg-amber-500';
  if (normalized === 'offline' || normalized === 'stopped') return 'bg-red-500';
  return 'bg-gray-400';
};

const formatUpdate = (update?: DockerServiceUpdate) => {
  const state = normalize(update?.state);
  const message = normalize(update?.message);
  if (!state && !message) return '—';
  if (state && message) return `${state}: ${message}`;
  return state || message;
};

const formatPorts = (ports?: DockerServicePort[]) => {
  if (!ports || ports.length === 0) return '—';
  return ports
    .map((p) => {
      const target = typeof p.targetPort === 'number' ? String(p.targetPort) : '';
      const published = typeof p.publishedPort === 'number' ? String(p.publishedPort) : '';
      const proto = normalize(p.protocol) || 'tcp';
      if (published && target) return `${published}->${target}/${proto}`;
      if (target) return `${target}/${proto}`;
      return '';
    })
    .filter(Boolean)
    .join(', ');
};

export const SwarmServicesDrawer: Component<{ cluster: string; swarm?: SwarmInfo }> = (props) => {
  const [search, setSearch] = createSignal('');

  const clusterKey = createMemo(() => normalize(props.cluster));

  const [services] = createResource(
    clusterKey,
    async (cluster) => {
      if (!cluster) return [];
      return fetchAllServices(cluster);
    },
    { initialValue: [] },
  );

  const filteredServices = createMemo(() => {
    const term = normalize(search()).toLowerCase();
    return services()
      .filter((svc) => {
        if (!term) return true;
        const name = normalize(svc.name) || svc.id;
        const stack = normalize(svc.docker?.stack);
        const image = normalize(svc.docker?.image);
        return [name, stack, image].some((value) => value.toLowerCase().includes(term));
      })
      .sort((a, b) => {
        const aName = (normalize(a.name) || a.id).toLowerCase();
        const bName = (normalize(b.name) || b.id).toLowerCase();
        if (aName !== bName) return aName < bName ? -1 : 1;
        return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
      });
  });

  const swarm = createMemo(() => props.swarm);
  const clusterName = createMemo(() => normalize(swarm()?.clusterName) || normalize(swarm()?.clusterId) || clusterKey());
  const clusterId = createMemo(() => normalize(swarm()?.clusterId));

  return (
    <div class="space-y-3">
      <Card padding="md">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-gray-800 dark:text-gray-100">Swarm</div>
            <div class="text-xs text-gray-500 dark:text-gray-400 truncate" title={clusterName()}>
              {clusterName() ? `Cluster: ${clusterName()}` : 'No Swarm cluster detected'}
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <input
              value={search()}
              onInput={(e) => setSearch(e.currentTarget.value)}
              placeholder="Search services..."
              class="w-[12rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-900 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-100"
            />
          </div>
        </div>

        <Show when={clusterId()}>
          <div class="mt-2 text-[10px] text-gray-500 dark:text-gray-400 truncate" title={clusterId()}>
            Cluster ID: {clusterId()}
          </div>
        </Show>

        <Show when={normalize(swarm()?.nodeRole) || normalize(swarm()?.localState) || typeof swarm()?.controlAvailable === 'boolean'}>
          <div class="mt-2 flex flex-wrap gap-2 text-[11px]">
            <Show when={normalize(swarm()?.nodeRole)}>
              <span class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-gray-800 dark:text-gray-200">
                Role: {normalize(swarm()?.nodeRole)}
              </span>
            </Show>
            <Show when={normalize(swarm()?.localState)}>
              <span class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-gray-800 dark:text-gray-200">
                State: {normalize(swarm()?.localState)}
              </span>
            </Show>
            <Show when={typeof swarm()?.controlAvailable === 'boolean'}>
              <span class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-gray-800 dark:text-gray-200">
                Control: {swarm()?.controlAvailable ? 'available' : 'unavailable'}
              </span>
            </Show>
          </div>
        </Show>

        <Show when={normalize(swarm()?.error)}>
          <div class="mt-2 rounded border border-amber-200 bg-amber-50/70 px-2 py-1.5 text-[10px] text-amber-800 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200">
            {normalize(swarm()?.error)}
          </div>
        </Show>
      </Card>

      <Show
        when={services.loading}
        fallback={
          <Show
            when={filteredServices().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState
                  title={services().length > 0 ? 'No services match your filters' : 'No Swarm services found'}
                  description={
                    services().length > 0
                      ? 'Try clearing the search.'
                      : 'Enable Swarm service collection in the Docker agent (includeServices) and wait for the next report.'
                  }
                />
              </Card>
            }
          >
            <Card padding="none" tone="glass" class="overflow-hidden">
              <div class="overflow-x-auto">
                <table class="w-full min-w-[900px] border-collapse text-xs">
                  <thead class="bg-gray-50 dark:bg-gray-800/60 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                    <tr class="text-left text-[10px] uppercase tracking-wide">
                      <th class="px-3 py-2 font-medium">Service</th>
                      <th class="px-3 py-2 font-medium">Stack</th>
                      <th class="px-3 py-2 font-medium">Image</th>
                      <th class="px-3 py-2 font-medium">Mode</th>
                      <th class="px-3 py-2 font-medium">Desired</th>
                      <th class="px-3 py-2 font-medium">Running</th>
                      <th class="px-3 py-2 font-medium">Update</th>
                      <th class="px-3 py-2 font-medium">Ports</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                    <For each={filteredServices()}>
                      {(svc) => {
                        const name = () => normalize(svc.name) || svc.id;
                        const stack = () => normalize(svc.docker?.stack) || '—';
                        const image = () => normalize(svc.docker?.image) || '—';
                        const mode = () => normalize(svc.docker?.mode) || '—';
                        const desired = () => svc.docker?.desiredTasks ?? 0;
                        const running = () => svc.docker?.runningTasks ?? 0;
                        const update = () => formatUpdate(svc.docker?.serviceUpdate);
                        const ports = () => formatPorts(svc.docker?.endpointPorts);

                        return (
                          <tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
                            <td class="px-3 py-2">
                              <div class="flex items-center gap-2 min-w-0">
                                <span class={`h-2 w-2 rounded-full ${statusTone(svc.status)}`} title={svc.status || 'unknown'} />
                                <span class="font-semibold text-gray-900 dark:text-gray-100 truncate" title={name()}>
                                  {name()}
                                </span>
                              </div>
                            </td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{stack()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200 truncate" title={image()}>
                              {image()}
                            </td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{mode()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{desired()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200">{running()}</td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200 truncate" title={update()}>
                              {update()}
                            </td>
                            <td class="px-3 py-2 text-gray-700 dark:text-gray-200 truncate" title={ports()}>
                              {ports()}
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
          <div class="text-xs text-gray-500 dark:text-gray-400">Loading Swarm services...</div>
        </Card>
      </Show>
    </div>
  );
};

