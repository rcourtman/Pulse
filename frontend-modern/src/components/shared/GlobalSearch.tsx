import { Component, For, Show, createEffect, createMemo, createResource, createSignal, onCleanup, onMount } from 'solid-js';
import type { JSX } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import SearchIcon from 'lucide-solid/icons/search';
import XIcon from 'lucide-solid/icons/x';
import ServerIcon from 'lucide-solid/icons/server';
import BoxesIcon from 'lucide-solid/icons/boxes';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import type { ResourceStatus, ResourceType } from '@/types/resource';
import { StatusDot } from '@/components/shared/StatusDot';
import { OFFLINE_HEALTH_STATUSES, DEGRADED_HEALTH_STATUSES } from '@/utils/status';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { apiFetchJSON } from '@/utils/apiClient';

type V2Resource = {
  id: string;
  type?: string;
  name?: string;
  status?: string;
  parentId?: string;
  sources?: string[];
};

type V2ListResponse = {
  data?: V2Resource[];
  resources?: V2Resource[];
  meta?: {
    total?: number;
  };
};

type SearchResource = {
  id: string;
  type: ResourceType;
  name: string;
  status: ResourceStatus;
  parentId?: string;
  sources?: string[];
};

type SearchResponse = {
  items: SearchResource[];
  total: number;
};

const resolveResourceType = (value?: string): ResourceType => {
  const normalized = (value || '').trim().toLowerCase();
  switch (normalized) {
    case 'node':
      return 'node';
    case 'host':
      return 'host';
    case 'docker-host':
    case 'docker_host':
      return 'docker-host';
    case 'k8s-cluster':
    case 'k8s_cluster':
    case 'kubernetes-cluster':
      return 'k8s-cluster';
    case 'k8s-node':
    case 'k8s_node':
      return 'k8s-node';
    case 'vm':
      return 'vm';
    case 'lxc':
      return 'container';
    case 'container':
      return 'container';
    case 'oci-container':
    case 'oci_container':
      return 'oci-container';
    case 'docker-container':
    case 'docker_container':
      return 'docker-container';
    case 'pod':
      return 'pod';
    case 'storage':
      return 'storage';
    case 'datastore':
      return 'datastore';
    case 'pool':
      return 'pool';
    case 'dataset':
      return 'dataset';
    case 'pbs':
      return 'pbs';
    case 'pmg':
      return 'pmg';
    default:
      return 'host';
  }
};

const resolveStatus = (value?: string): ResourceStatus => {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'online' || normalized === 'running') return 'online';
  if (normalized === 'offline' || normalized === 'stopped') return 'offline';
  if (normalized === 'warning' || normalized === 'degraded') return 'degraded';
  if (normalized === 'paused') return 'paused';
  return 'unknown';
};

const getResourceStatusIndicator = (status: ResourceStatus) => {
  const normalized = status.toLowerCase();
  if (OFFLINE_HEALTH_STATUSES.has(normalized)) {
    return { variant: 'danger', label: 'Offline' } as const;
  }
  if (DEGRADED_HEALTH_STATUSES.has(normalized)) {
    return { variant: 'warning', label: 'Degraded' } as const;
  }
  if (normalized === 'online' || normalized === 'running') {
    return { variant: 'success', label: 'Online' } as const;
  }
  return { variant: 'muted', label: 'Unknown' } as const;
};

const typeLabels: Record<ResourceType, string> = {
  node: 'Node',
  host: 'Host',
  'docker-host': 'Docker Host',
  'k8s-cluster': 'Kubernetes Cluster',
  'k8s-node': 'Kubernetes Node',
  truenas: 'TrueNAS',
  vm: 'VM',
  container: 'LXC',
  'oci-container': 'OCI Container',
  'docker-container': 'Docker Container',
  pod: 'Kubernetes Pod',
  jail: 'Jail',
  'docker-service': 'Docker Service',
  'k8s-deployment': 'Kubernetes Deployment',
  'k8s-service': 'Kubernetes Service',
  storage: 'Storage',
  datastore: 'Datastore',
  pool: 'Pool',
  dataset: 'Dataset',
  pbs: 'Backup Server',
  pmg: 'Mail Gateway',
};

const infrastructureTypes = new Set<ResourceType>([
  'node',
  'host',
  'docker-host',
  'k8s-cluster',
  'k8s-node',
  'truenas',
  'pbs',
  'pmg',
]);

const workloadTypes = new Set<ResourceType>([
  'vm',
  'container',
  'oci-container',
  'docker-container',
  'pod',
  'jail',
  'docker-service',
  'k8s-deployment',
  'k8s-service',
]);

const storageTypes = new Set<ResourceType>(['storage', 'datastore', 'pool', 'dataset']);

const resolveGroup = (resource: SearchResource): 'infrastructure' | 'workloads' | 'storage' => {
  if (workloadTypes.has(resource.type)) return 'workloads';
  if (storageTypes.has(resource.type)) return 'storage';
  if (infrastructureTypes.has(resource.type)) return 'infrastructure';
  return 'infrastructure';
};

const iconForGroup = (group: 'infrastructure' | 'workloads' | 'storage') => {
  if (group === 'workloads') return BoxesIcon;
  if (group === 'storage') return HardDriveIcon;
  return ServerIcon;
};

const resolveWorkloadViewMode = (type: ResourceType): string | null => {
  switch (type) {
    case 'vm':
      return 'vm';
    case 'container':
    case 'oci-container':
    case 'jail':
      return 'lxc';
    case 'docker-container':
    case 'docker-service':
      return 'docker';
    case 'pod':
    case 'k8s-deployment':
    case 'k8s-service':
      return 'k8s';
    default:
      return null;
  }
};

const fetchSearchResults = async (query: string): Promise<SearchResponse> => {
  const params = new URLSearchParams();
  params.set('q', query);
  params.set('limit', '10');
  const response = await apiFetchJSON<V2ListResponse | V2Resource[]>(`/api/v2/resources?${params.toString()}`, { cache: 'no-store' });
  const payload = Array.isArray(response) ? response : response.data ?? response.resources ?? [];
  const total = Array.isArray(response)
    ? response.length
    : response.meta?.total ?? payload.length;

  const items = payload.map((resource) => ({
    id: resource.id,
    type: resolveResourceType(resource.type),
    name: resource.name || resource.id,
    status: resolveStatus(resource.status),
    parentId: resource.parentId,
    sources: resource.sources,
  }));

  return { items, total };
};

export const GlobalSearch: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [query, setQuery] = createSignal('');
  const [isOpen, setIsOpen] = createSignal(false);
  const [activeIndex, setActiveIndex] = createSignal<number>(-1);
  const debouncedQuery = useDebouncedValue(() => query().trim(), 300);

  const searchSource = createMemo(() => {
    const term = debouncedQuery();
    if (term.length < 2) return null;
    return term;
  });

  const [searchResults] = createResource(searchSource, fetchSearchResults, {
    initialValue: { items: [], total: 0 },
  });

  const results = createMemo(() => searchResults()?.items ?? []);
  const totalResults = createMemo(() => searchResults()?.total ?? results().length);
  const hasMoreResults = createMemo(() => totalResults() > results().length);
  const isLoading = createMemo(() => searchResults.loading && searchSource() !== null);
  const hasQuery = createMemo(() => query().trim().length > 0);
  const isTooShort = createMemo(() => {
    const term = query().trim();
    return term.length > 0 && term.length < 2;
  });
  const shouldShowDropdown = createMemo(() => isOpen() && (hasQuery() || isLoading()));

  const groupedResults = createMemo(() => {
    const groups = {
      infrastructure: [] as SearchResource[],
      workloads: [] as SearchResource[],
      storage: [] as SearchResource[],
    };

    results().forEach((resource) => {
      const group = resolveGroup(resource);
      groups[group].push(resource);
    });

    return groups;
  });

  const flattenedResults = createMemo(() => [
    ...groupedResults().infrastructure,
    ...groupedResults().workloads,
    ...groupedResults().storage,
  ]);

  createEffect(() => {
    const list = flattenedResults();
    if (list.length === 0) {
      setActiveIndex(-1);
      return;
    }
    setActiveIndex(0);
  });

  createEffect(() => {
    location.pathname;
    setIsOpen(false);
  });

  let containerRef: HTMLDivElement | undefined;

  const handleDocumentClick = (event: MouseEvent) => {
    const target = event.target as Node | null;
    if (!target || !containerRef) return;
    if (!containerRef.contains(target)) {
      setIsOpen(false);
    }
  };

  onMount(() => {
    document.addEventListener('mousedown', handleDocumentClick);
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleDocumentClick);
  });

  const clearSearch = () => {
    setQuery('');
    setIsOpen(false);
  };

  const navigateToResource = (resource: SearchResource) => {
    const group = resolveGroup(resource);
    if (group === 'workloads') {
      const viewMode = resolveWorkloadViewMode(resource.type);
      const params = new URLSearchParams();
      params.set('resource', resource.id);
      if (viewMode) {
        params.set('type', viewMode);
      }
      navigate(`/workloads?${params.toString()}`);
      return;
    }
    if (group === 'storage') {
      navigate(`/storage?resource=${encodeURIComponent(resource.id)}`);
      return;
    }
    navigate(`/infrastructure?resource=${encodeURIComponent(resource.id)}`);
  };

  const handleSelect = (resource: SearchResource) => {
    navigateToResource(resource);
    clearSearch();
  };

  const handleViewAll = () => {
    const term = query().trim();
    if (!term) return;
    navigate(`/infrastructure?q=${encodeURIComponent(term)}`);
    clearSearch();
  };

  const handleKeyDown: JSX.EventHandler<HTMLInputElement, KeyboardEvent> = (event) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      const list = flattenedResults();
      if (list.length === 0) return;
      setIsOpen(true);
      setActiveIndex((prev) => (prev + 1) % list.length);
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      const list = flattenedResults();
      if (list.length === 0) return;
      setIsOpen(true);
      setActiveIndex((prev) => (prev <= 0 ? list.length - 1 : prev - 1));
    }

    if (event.key === 'Enter') {
      const list = flattenedResults();
      const index = activeIndex();
      if (index >= 0 && index < list.length) {
        event.preventDefault();
        handleSelect(list[index]);
        return;
      }
      if (hasMoreResults()) {
        event.preventDefault();
        handleViewAll();
      }
    }

    if (event.key === 'Escape') {
      setIsOpen(false);
    }
  };

  return (
    <div class="relative w-full max-w-[360px]" ref={containerRef}>
      <div class="relative">
        <SearchIcon class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
        <input
          type="search"
          class="w-full rounded-md border border-gray-200 bg-white/90 px-9 py-2 text-sm text-gray-700 shadow-sm focus:border-blue-400 focus:outline-none focus:ring-1 focus:ring-blue-400 dark:border-gray-700 dark:bg-gray-900/70 dark:text-gray-200"
          placeholder="Search resources..."
          data-global-search
          value={query()}
          onInput={(event) => {
            setQuery(event.currentTarget.value);
            setIsOpen(true);
          }}
          onFocus={() => setIsOpen(true)}
          onKeyDown={handleKeyDown}
          aria-label="Search resources"
        />
        <Show when={isLoading()}>
          <span class="absolute right-8 top-1/2 h-3.5 w-3.5 -translate-y-1/2 rounded-full border-2 border-blue-500 border-t-transparent animate-spin" />
        </Show>
        <Show when={query().length > 0}>
          <button
            type="button"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
            onClick={clearSearch}
            aria-label="Clear search"
          >
            <XIcon class="h-4 w-4" />
          </button>
        </Show>
      </div>

      <Show when={shouldShowDropdown()}>
        <div class="absolute left-0 right-0 z-40 mt-2 rounded-md border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900">
          <Show when={isLoading()}>
            <div class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">Searching...</div>
          </Show>

          <Show when={isTooShort()}>
            <div class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">Type at least 2 characters to search.</div>
          </Show>

          <Show when={!isLoading() && searchSource() !== null && results().length === 0 && !isTooShort()}>
            <div class="px-3 py-3 text-xs text-gray-500 dark:text-gray-400">No results</div>
          </Show>

          <Show when={results().length > 0}>
            <div class="max-h-[320px] overflow-y-auto">
              <For each={([
                { key: 'infrastructure', label: 'Infrastructure' },
                { key: 'workloads', label: 'Workloads' },
                { key: 'storage', label: 'Storage' },
              ] as const)}>
                {(group) => {
                  const items = () => groupedResults()[group.key];
                  return (
                    <Show when={items().length > 0}>
                      <div class="px-3 py-2 text-[10px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
                        {group.label}
                      </div>
                      <For each={items()}>
                        {(resource) => {
                          const list = flattenedResults();
                          const index = list.findIndex((item) => item.id === resource.id);
                          const isActive = () => index === activeIndex();
                          const statusIndicator = () => getResourceStatusIndicator(resource.status);
                          const displayName = () => resource.name || resource.id;
                          const typeLabel = () => typeLabels[resource.type] ?? resource.type;
                          const Icon = iconForGroup(resolveGroup(resource));
                          return (
                            <button
                              type="button"
                              class={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors ${isActive()
                                ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
                                : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800/60'}`}
                              onMouseEnter={() => setActiveIndex(index)}
                              onMouseDown={(event) => event.preventDefault()}
                              onClick={() => handleSelect(resource)}
                              aria-selected={isActive()}
                              role="option"
                            >
                              <Icon class="h-4 w-4 text-gray-400" />
                              <StatusDot
                                variant={statusIndicator().variant}
                                ariaLabel={statusIndicator().label}
                                size="xs"
                              />
                              <div class="min-w-0 flex-1">
                                <div class="truncate font-medium">{displayName()}</div>
                                <div class="text-xs text-gray-500 dark:text-gray-400">{typeLabel()}</div>
                              </div>
                            </button>
                          );
                        }}
                      </For>
                    </Show>
                  );
                }}
              </For>
            </div>
          </Show>

          <Show when={hasMoreResults()}>
            <button
              type="button"
              class="w-full px-3 py-2 text-left text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-300 dark:hover:bg-blue-900/30"
              onMouseDown={(event) => event.preventDefault()}
              onClick={handleViewAll}
            >
              View all results ({totalResults()})
            </button>
          </Show>
        </div>
      </Show>
    </div>
  );
};

export default GlobalSearch;
