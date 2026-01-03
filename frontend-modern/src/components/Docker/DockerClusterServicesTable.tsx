import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { DockerHost } from '@/types/api';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
import { formatRelativeTime } from '@/utils/format';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import {
  groupHostsByCluster,
  aggregateClusterServices,
  formatNodeDistribution,
  getServiceHealthStatus,
  type SwarmCluster,
  type ClusterService,
} from './swarmClusterHelpers';

interface DockerClusterServicesTableProps {
  hosts: DockerHost[];
  searchTerm?: string;
}

type SortKey = 'name' | 'stack' | 'mode' | 'replicas' | 'nodes' | 'status';
type SortDirection = 'asc' | 'desc';

const SORT_KEYS: SortKey[] = ['name', 'stack', 'mode', 'replicas', 'nodes', 'status'];

const SORT_DEFAULT_DIRECTION: Record<SortKey, SortDirection> = {
  name: 'asc',
  stack: 'asc',
  mode: 'asc',
  replicas: 'desc',
  nodes: 'desc',
  status: 'desc',
};

const STATUS_PRIORITY: Record<string, number> = {
  critical: 3,
  degraded: 2,
  healthy: 1,
};

const parseSearchTerm = (term?: string): string[] => {
  if (!term) return [];
  return term
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
};

const serviceMatchesSearch = (service: ClusterService, tokens: string[]): boolean => {
  if (tokens.length === 0) return true;

  const searchableText = [
    service.service.name || '',
    service.service.stack || '',
    service.service.image || '',
    service.service.mode || '',
    ...service.nodes.map((n) => n.hostname),
  ]
    .join(' ')
    .toLowerCase();

  return tokens.every((token) => searchableText.includes(token));
};

const ClusterGroupHeader: Component<{
  cluster: SwarmCluster;
  serviceCount: number;
  isExpanded: boolean;
  onToggle: () => void;
}> = (props) => {
  const managerCount = createMemo(
    () => props.cluster.hosts.filter((h) => h.swarm?.nodeRole === 'manager').length
  );
  const workerCount = createMemo(
    () => props.cluster.hosts.filter((h) => h.swarm?.nodeRole === 'worker').length
  );

  return (
    <button
      type="button"
      onClick={props.onToggle}
      class="w-full flex items-center gap-3 px-4 py-3 bg-purple-50 dark:bg-purple-900/20 hover:bg-purple-100 dark:hover:bg-purple-900/30 border-b border-purple-200 dark:border-purple-800 transition-colors"
    >
      <svg
        class={`w-4 h-4 text-purple-600 dark:text-purple-400 transition-transform ${props.isExpanded ? 'rotate-90' : ''}`}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
      </svg>

      <div class="flex items-center gap-2">
        <svg
          class="w-5 h-5 text-purple-600 dark:text-purple-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
          />
        </svg>
        <span class="font-medium text-purple-900 dark:text-purple-100">
          {props.cluster.clusterName || 'Swarm Cluster'}
        </span>
      </div>

      <div class="flex items-center gap-4 text-sm text-purple-700 dark:text-purple-300">
        <span class="flex items-center gap-1">
          <span class="font-medium">{props.cluster.hosts.length}</span>
          <span class="text-purple-500 dark:text-purple-400">
            {props.cluster.hosts.length === 1 ? 'node' : 'nodes'}
          </span>
        </span>
        <Show when={managerCount() > 0}>
          <span class="text-purple-500 dark:text-purple-400">
            ({managerCount()} manager{managerCount() !== 1 ? 's' : ''}, {workerCount()} worker
            {workerCount() !== 1 ? 's' : ''})
          </span>
        </Show>
        <span class="text-purple-500 dark:text-purple-400">|</span>
        <span>
          <span class="font-medium">{props.serviceCount}</span>{' '}
          <span class="text-purple-500 dark:text-purple-400">
            {props.serviceCount === 1 ? 'service' : 'services'}
          </span>
        </span>
      </div>
    </button>
  );
};

const ServiceRow: Component<{ service: ClusterService }> = (props) => {
  const healthStatus = createMemo(() => getServiceHealthStatus(props.service));

  const statusVariant = createMemo(() => {
    switch (healthStatus()) {
      case 'critical':
        return 'danger';
      case 'degraded':
        return 'warning';
      default:
        return 'success';
    }
  });

  const statusLabel = createMemo(() => {
    const { totalDesired, totalRunning } = props.service;
    if (totalDesired === 0) return 'No replicas';
    return `${totalRunning}/${totalDesired} running`;
  });

  const updatedAt = createMemo(() => {
    const ts = props.service.service.updatedAt;
    if (!ts) return null;
    return typeof ts === 'number' ? ts : Date.parse(ts as unknown as string);
  });

  return (
    <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
      {/* Service Name */}
      <td class="px-4 py-3">
        <div class="flex flex-col">
          <span class="font-medium text-gray-900 dark:text-gray-100">
            {props.service.service.name}
          </span>
          <Show when={props.service.service.image}>
            <span class="text-xs text-gray-500 dark:text-gray-400 truncate max-w-[200px]">
              {props.service.service.image?.split('@')[0]}
            </span>
          </Show>
        </div>
      </td>

      {/* Stack */}
      <td class="px-4 py-3">
        <Show when={props.service.service.stack} fallback={<span class="text-gray-400">—</span>}>
          <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300">
            {props.service.service.stack}
          </span>
        </Show>
      </td>

      {/* Mode */}
      <td class="px-4 py-3">
        <span class="text-sm text-gray-600 dark:text-gray-400 capitalize">
          {props.service.service.mode || 'replicated'}
        </span>
      </td>

      {/* Replicas */}
      <td class="px-4 py-3">
        <div class="flex items-center gap-2">
          <StatusDot variant={statusVariant()} size="sm" />
          <span class="text-sm text-gray-700 dark:text-gray-300">{statusLabel()}</span>
        </div>
      </td>

      {/* Nodes */}
      <td class="px-4 py-3">
        <span class="text-sm text-gray-600 dark:text-gray-400">
          {formatNodeDistribution(props.service.nodes)}
        </span>
      </td>

      {/* Updated */}
      <td class="px-4 py-3 text-right">
        <Show when={updatedAt()} fallback={<span class="text-gray-400">—</span>}>
          <span class="text-sm text-gray-500 dark:text-gray-400">
            {formatRelativeTime(updatedAt()!)}
          </span>
        </Show>
      </td>
    </tr>
  );
};

const ClusterSection: Component<{
  cluster: SwarmCluster;
  services: ClusterService[];
  sortKey: SortKey;
  sortDirection: SortDirection;
}> = (props) => {
  const [isExpanded, setIsExpanded] = createSignal(true);

  const sortedServices = createMemo(() => {
    const services = [...props.services];
    const key = props.sortKey;
    const dir = props.sortDirection;

    services.sort((a, b) => {
      let cmp = 0;

      switch (key) {
        case 'name':
          cmp = (a.service.name || '').localeCompare(b.service.name || '');
          break;
        case 'stack':
          cmp = (a.service.stack || '').localeCompare(b.service.stack || '');
          break;
        case 'mode':
          cmp = (a.service.mode || '').localeCompare(b.service.mode || '');
          break;
        case 'replicas':
          cmp = a.totalRunning - b.totalRunning;
          break;
        case 'nodes':
          cmp = a.nodes.length - b.nodes.length;
          break;
        case 'status':
          cmp =
            (STATUS_PRIORITY[getServiceHealthStatus(a)] || 0) -
            (STATUS_PRIORITY[getServiceHealthStatus(b)] || 0);
          break;
      }

      return dir === 'asc' ? cmp : -cmp;
    });

    return services;
  });

  return (
    <div class="mb-4">
      <ClusterGroupHeader
        cluster={props.cluster}
        serviceCount={props.services.length}
        isExpanded={isExpanded()}
        onToggle={() => setIsExpanded(!isExpanded())}
      />

      <Show when={isExpanded()}>
        <div class="overflow-x-auto">
          <table class="w-full">
            <tbody>
              <For each={sortedServices()}>{(service) => <ServiceRow service={service} />}</For>
            </tbody>
          </table>
        </div>
      </Show>
    </div>
  );
};

export const DockerClusterServicesTable: Component<DockerClusterServicesTableProps> = (props) => {
  const [sortKey, setSortKey] = usePersistentSignal<SortKey>(
    'dockerClusterSortKey',
    'name',
    { deserialize: (v) => (SORT_KEYS.includes(v as SortKey) ? (v as SortKey) : 'name') }
  );
  const [sortDirection, setSortDirection] = usePersistentSignal<SortDirection>(
    'dockerClusterSortDirection',
    'asc',
    { deserialize: (v) => (v === 'asc' || v === 'desc' ? v : 'asc') }
  );

  const handleSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      setSortDirection(SORT_DEFAULT_DIRECTION[key]);
    }
  };

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? ' ▲' : ' ▼';
  };

  const searchTokens = createMemo(() => parseSearchTerm(props.searchTerm));

  const clusters = createMemo(() => groupHostsByCluster(props.hosts));

  const clustersWithServices = createMemo(() => {
    const tokens = searchTokens();

    return clusters()
      .map((cluster) => {
        const allServices = aggregateClusterServices(cluster);
        const filteredServices = allServices.filter((s) => serviceMatchesSearch(s, tokens));
        return { cluster, services: filteredServices };
      })
      .filter((c) => c.services.length > 0);
  });

  const totalServices = createMemo(() =>
    clustersWithServices().reduce((sum, c) => sum + c.services.length, 0)
  );

  return (
    <Card class="docker-cluster-services-table" padding="none">
      <Show
        when={clustersWithServices().length > 0}
        fallback={
          <EmptyState
            title="No Swarm clusters found"
            description="Deploy Docker agents to multiple nodes in the same Swarm cluster to see the cluster view."
            icon={
              <svg
                class="w-12 h-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                />
              </svg>
            }
          />
        }
      >
        {/* Header row */}
        <div class="sticky top-0 z-10 bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
          <table class="w-full">
            <thead>
              <tr class="text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                <th
                  class="px-4 py-3 cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  onClick={() => handleSort('name')}
                >
                  Service{renderSortIndicator('name')}
                </th>
                <th
                  class="px-4 py-3 cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  onClick={() => handleSort('stack')}
                >
                  Stack{renderSortIndicator('stack')}
                </th>
                <th
                  class="px-4 py-3 cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  onClick={() => handleSort('mode')}
                >
                  Mode{renderSortIndicator('mode')}
                </th>
                <th
                  class="px-4 py-3 cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  onClick={() => handleSort('replicas')}
                >
                  Replicas{renderSortIndicator('replicas')}
                </th>
                <th
                  class="px-4 py-3 cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  onClick={() => handleSort('nodes')}
                >
                  Nodes{renderSortIndicator('nodes')}
                </th>
                <th class="px-4 py-3 text-right">Updated</th>
              </tr>
            </thead>
          </table>
        </div>

        {/* Cluster sections */}
        <div>
          <For each={clustersWithServices()}>
            {({ cluster, services }) => (
              <ClusterSection
                cluster={cluster}
                services={services}
                sortKey={sortKey()}
                sortDirection={sortDirection()}
              />
            )}
          </For>
        </div>

        {/* Footer summary */}
        <div class="px-4 py-2 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700 text-sm text-gray-500 dark:text-gray-400">
          {clustersWithServices().length} cluster{clustersWithServices().length !== 1 ? 's' : ''},{' '}
          {totalServices()} service{totalServices() !== 1 ? 's' : ''}
        </div>
      </Show>
    </Card>
  );
};
