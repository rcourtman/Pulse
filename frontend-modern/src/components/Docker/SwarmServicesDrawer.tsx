import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { SearchField } from '@/components/shared/SearchField';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  formatSwarmClusterId,
  formatSwarmClusterSummary,
  formatSwarmControlLabel,
  formatSwarmRoleLabel,
  formatSwarmStateLabel,
  getSwarmDrawerPresentation,
  getSwarmServicesEmptyState,
  getSwarmServicesLoadingState,
} from '@/utils/swarmPresentation';

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
  params.set('type', 'docker-service');
  params.set('cluster', cluster);
  params.set('page', String(page));
  params.set('limit', String(PAGE_LIMIT));
  return `/api/resources?${params.toString()}`;
};

const fetchAllServices = async (cluster: string): Promise<DockerServiceResource[]> => {
  const first = await apiFetchJSON<ResourcesListResponse>(buildServicesUrl(cluster, 1), {
    cache: 'no-store',
  });
  const firstData = Array.isArray(first?.data) ? first.data : [];
  const totalPages = Number.isFinite(first?.meta?.totalPages)
    ? Math.max(1, Number(first.meta?.totalPages))
    : 1;

  const services: DockerServiceResource[] = [...firstData];

  const cappedPages = Math.min(totalPages, MAX_PAGES);
  if (cappedPages > 1) {
    const requests: Array<Promise<ResourcesListResponse>> = [];
    for (let page = 2; page <= cappedPages; page += 1) {
      requests.push(
        apiFetchJSON<ResourcesListResponse>(buildServicesUrl(cluster, page), { cache: 'no-store' }),
      );
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
  const drawerPresentation = getSwarmDrawerPresentation();

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
  const clusterName = createMemo(
    () => normalize(swarm()?.clusterName) || normalize(swarm()?.clusterId) || clusterKey(),
  );
  const clusterId = createMemo(() => normalize(swarm()?.clusterId));

  return (
    <div class="space-y-3">
      <Card padding="md">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-base-content">{drawerPresentation.title}</div>
            <div class="text-xs text-muted truncate" title={clusterName()}>
              {formatSwarmClusterSummary(clusterName())}
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <SearchField
              value={search()}
              onChange={setSearch}
              placeholder={drawerPresentation.searchPlaceholder}
              class="w-[12rem]"
              inputClass="py-1 text-xs font-medium shadow-sm"
            />
          </div>
        </div>

        <Show when={clusterId()}>
          <div class="mt-2 text-[10px] text-muted truncate" title={clusterId()}>
            {formatSwarmClusterId(clusterId())}
          </div>
        </Show>

        <Show
          when={
            normalize(swarm()?.nodeRole) ||
            normalize(swarm()?.localState) ||
            typeof swarm()?.controlAvailable === 'boolean'
          }
        >
          <div class="mt-2 flex flex-wrap gap-2 text-[11px]">
            <Show when={normalize(swarm()?.nodeRole)}>
              <span class="inline-flex items-center rounded bg-surface-alt px-2 py-0.5 text-base-content">
                {formatSwarmRoleLabel(normalize(swarm()?.nodeRole))}
              </span>
            </Show>
            <Show when={normalize(swarm()?.localState)}>
              <span class="inline-flex items-center rounded bg-surface-alt px-2 py-0.5 text-base-content">
                {formatSwarmStateLabel(normalize(swarm()?.localState))}
              </span>
            </Show>
            <Show when={typeof swarm()?.controlAvailable === 'boolean'}>
              <span class="inline-flex items-center rounded bg-surface-alt px-2 py-0.5 text-base-content">
                {formatSwarmControlLabel(swarm()?.controlAvailable)}
              </span>
            </Show>
          </div>
        </Show>

        <Show when={normalize(swarm()?.error)}>
          <div class="mt-2 rounded border border-amber-200 bg-amber-50 px-2 py-1.5 text-[10px] text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
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
                <EmptyState {...getSwarmServicesEmptyState(services().length > 0)} />
              </Card>
            }
          >
            <Card padding="none" tone="card" class="overflow-hidden">
              <div class="overflow-x-auto">
                <Table class="w-full min-w-[900px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.serviceColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.stackColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.imageColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.modeColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.desiredColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.runningColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.updateColumnLabel}
                      </TableHead>
                      <TableHead class="px-3 py-2 font-medium">
                        {drawerPresentation.portsColumnLabel}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
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
                        const status = () => getSimpleStatusIndicator(svc.status);

                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell class="px-3 py-2">
                              <div class="flex items-center gap-2 min-w-0">
                                <StatusDot
                                  size="sm"
                                  variant={status().variant}
                                  title={svc.status || 'unknown'}
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
                            <TableCell class="px-3 py-2 text-base-content">{stack()}</TableCell>
                            <TableCell class="px-3 py-2 text-base-content truncate" title={image()}>
                              {image()}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">{mode()}</TableCell>
                            <TableCell class="px-3 py-2 text-base-content">{desired()}</TableCell>
                            <TableCell class="px-3 py-2 text-base-content">{running()}</TableCell>
                            <TableCell
                              class="px-3 py-2 text-base-content truncate"
                              title={update()}
                            >
                              {update()}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content truncate" title={ports()}>
                              {ports()}
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
        }
      >
        <Card padding="lg">
          <div class="text-xs text-muted">{getSwarmServicesLoadingState().text}</div>
        </Card>
      </Show>
    </div>
  );
};
