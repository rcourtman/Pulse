import { For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import type { Resource } from '@/types/resource';

export function Infrastructure() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const location = useLocation();

  // Track if we've completed initial load to prevent flash of empty state
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  const hasResources = createMemo(() => resources().length > 0);
  // Only show "no resources" after initial load completes with zero results
  const showNoResources = createMemo(() => initialLoadComplete() && !hasResources() && !error());
  const [selectedSources, setSelectedSources] = createSignal<Set<string>>(new Set());
  const [selectedStatuses, setSelectedStatuses] = createSignal<Set<string>>(new Set());
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [highlightedResourceId, setHighlightedResourceId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  createEffect(() => {
    const params = new URLSearchParams(location.search);
    const resourceId = params.get('resource');
    if (!resourceId || resourceId === handledResourceId()) return;
    const matching = resources().some((resource) => resource.id === resourceId);
    if (!matching) return;
    setExpandedResourceId(resourceId);
    setHighlightedResourceId(resourceId);
    setHandledResourceId(resourceId);
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
    highlightTimer = window.setTimeout(() => {
      setHighlightedResourceId(null);
    }, 2000);
  });

  onCleanup(() => {
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
  });

  const sourceOptions = [
    { key: 'proxmox', label: 'PVE' },
    { key: 'agent', label: 'Agent' },
    { key: 'docker', label: 'Docker' },
    { key: 'pbs', label: 'PBS' },
    { key: 'pmg', label: 'PMG' },
    { key: 'kubernetes', label: 'K8s' },
  ];

  const statusLabels: Record<string, string> = {
    online: 'Online',
    offline: 'Offline',
    degraded: 'Degraded',
    paused: 'Paused',
    unknown: 'Unknown',
    running: 'Running',
    stopped: 'Stopped',
  };

  const statusOrder = ['online', 'degraded', 'paused', 'offline', 'stopped', 'unknown', 'running'];

  const normalizeSource = (value: string): string | null => {
    const normalized = value.toLowerCase();
    switch (normalized) {
      case 'pve':
      case 'proxmox':
      case 'proxmox-pve':
        return 'proxmox';
      case 'agent':
      case 'host-agent':
        return 'agent';
      case 'docker':
        return 'docker';
      case 'pbs':
      case 'proxmox-pbs':
        return 'pbs';
      case 'pmg':
      case 'proxmox-pmg':
        return 'pmg';
      case 'k8s':
      case 'kubernetes':
        return 'kubernetes';
      default:
        return null;
    }
  };

  const getResourceSources = (resource: Resource): string[] => {
    const platformData = resource.platformData as { sources?: string[] } | undefined;
    const normalized = (platformData?.sources ?? [])
      .map((source) => normalizeSource(source))
      .filter((source): source is string => Boolean(source));
    return Array.from(new Set(normalized));
  };

  const availableSources = createMemo(() => {
    const set = new Set<string>();
    resources().forEach((resource) => {
      getResourceSources(resource).forEach((source) => set.add(source));
    });
    return set;
  });

  const availableStatuses = createMemo(() => {
    const set = new Set<string>();
    resources().forEach((resource) => {
      const status = (resource.status || 'unknown').toLowerCase();
      if (status) set.add(status);
    });
    return set;
  });

  const statusOptions = createMemo(() => {
    const statuses = Array.from(availableStatuses());
    statuses.sort((a, b) => {
      const indexA = statusOrder.indexOf(a);
      const indexB = statusOrder.indexOf(b);
      if (indexA === -1 && indexB === -1) return a.localeCompare(b);
      if (indexA === -1) return 1;
      if (indexB === -1) return -1;
      return indexA - indexB;
    });
    return statuses.map((status) => ({
      key: status,
      label: statusLabels[status] ?? status,
    }));
  });

  const hasActiveFilters = createMemo(
    () => selectedSources().size > 0 || selectedStatuses().size > 0,
  );

  const toggleSource = (source: string) => {
    const next = new Set(selectedSources());
    if (next.has(source)) {
      next.delete(source);
    } else {
      next.add(source);
    }
    setSelectedSources(next);
  };

  const toggleStatus = (status: string) => {
    const next = new Set(selectedStatuses());
    if (next.has(status)) {
      next.delete(status);
    } else {
      next.add(status);
    }
    setSelectedStatuses(next);
  };

  const clearFilters = () => {
    setSelectedSources(new Set<string>());
    setSelectedStatuses(new Set<string>());
  };

  const segmentedButtonClass = (selected: boolean, disabled: boolean) => {
    const base = 'px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95';
    if (disabled) {
      return `${base} text-gray-400 dark:text-gray-600 cursor-not-allowed`;
    }
    if (selected) {
      return `${base} bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600`;
    }
    return `${base} text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50`;
  };

  const filteredResources = createMemo(() => {
    let filtered = resources();
    const sources = selectedSources();
    const statuses = selectedStatuses();

    if (sources.size > 0) {
      filtered = filtered.filter((resource) => {
        const resourceSources = getResourceSources(resource);
        if (resourceSources.length === 0) return false;
        return resourceSources.some((source) => sources.has(source));
      });
    }

    if (statuses.size > 0) {
      filtered = filtered.filter((resource) => {
        const status = (resource.status || 'unknown').toLowerCase();
        return statuses.has(status);
      });
    }

    return filtered;
  });

  const hasFilteredResources = createMemo(() => filteredResources().length > 0);

  return (
    <div class="space-y-4">
      <SectionHeader
        title="Infrastructure"
        description="Unified host inventory across monitored platforms."
        size="lg"
      />

      <Show when={!loading()} fallback={
        <Card class="p-6">
          <div class="text-sm text-gray-600 dark:text-gray-300">Loading infrastructure resources...</div>
        </Card>
      }>
        <Show
          when={!error()}
          fallback={
            <Card class="p-6">
              <EmptyState
                icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                title="Unable to load infrastructure"
                description="We couldn’t fetch unified resources. Check connectivity or retry."
                actions={
                  <button
                    type="button"
                    onClick={() => refetch()}
                    class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                  >
                    <RefreshCwIcon class="h-3.5 w-3.5" />
                    Retry
                  </button>
                }
              />
            </Card>
          }
        >
          <Show
            when={!showNoResources()}
            fallback={
              <Card class="p-6">
                <EmptyState
                  icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                  title="No infrastructure resources yet"
                  description="Once hosts are reporting, they will appear here."
                />
              </Card>
            }
          >
            <div class="space-y-3">
              <Card padding="sm" class="mb-4">
                <div class="flex flex-wrap items-center gap-x-2 gap-y-2 text-xs text-gray-500 dark:text-gray-400">
                  <div class="flex items-center gap-2">
                    <span class="uppercase tracking-wide text-[9px] text-gray-400 dark:text-gray-500">Source</span>
                    <div class="inline-flex flex-wrap rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <For each={sourceOptions}>
                        {(source) => {
                          const isSelected = () => selectedSources().has(source.key);
                          const isDisabled = () =>
                            !availableSources().has(source.key) && !selectedSources().has(source.key);
                          return (
                            <button
                              type="button"
                              disabled={isDisabled()}
                              aria-pressed={isSelected()}
                              onClick={() => toggleSource(source.key)}
                              class={segmentedButtonClass(isSelected(), isDisabled())}
                            >
                              {source.label}
                            </button>
                          );
                        }}
                      </For>
                    </div>
                  </div>

                  <div class="h-5 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block" />

                  <div class="flex items-center gap-2">
                    <span class="uppercase tracking-wide text-[9px] text-gray-400 dark:text-gray-500">Status</span>
                    <div class="inline-flex flex-wrap rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <For each={statusOptions()}>
                        {(status) => {
                          const isSelected = () => selectedStatuses().has(status.key);
                          const isDisabled = () =>
                            !availableStatuses().has(status.key) && !selectedStatuses().has(status.key);
                          return (
                            <button
                              type="button"
                              disabled={isDisabled()}
                              aria-pressed={isSelected()}
                              onClick={() => toggleStatus(status.key)}
                              class={segmentedButtonClass(isSelected(), isDisabled())}
                            >
                              {status.label}
                            </button>
                          );
                        }}
                      </For>
                    </div>
                  </div>

                  <Show when={hasActiveFilters()}>
                    <button
                      type="button"
                      onClick={clearFilters}
                      class="ml-auto text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                    >
                      Clear
                    </button>
                  </Show>
                </div>
              </Card>

              <Show
                when={hasFilteredResources()}
                fallback={
                  <Card class="p-6">
                    <EmptyState
                      icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                      title="No resources match filters"
                      description="Try adjusting the source or status filters."
                      actions={
                        <Show when={hasActiveFilters()}>
                          <button
                            type="button"
                            onClick={clearFilters}
                            class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                          >
                            Clear filters
                          </button>
                        </Show>
                      }
                    />
                  </Card>
                }
              >
                <UnifiedResourceTable
                  resources={filteredResources()}
                  expandedResourceId={expandedResourceId()}
                  highlightedResourceId={highlightedResourceId()}
                  onExpandedResourceChange={setExpandedResourceId}
                />
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default Infrastructure;
