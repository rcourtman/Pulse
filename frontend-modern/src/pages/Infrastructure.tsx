import { For, Show, createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { InfrastructureSummary } from '@/components/Infrastructure/InfrastructureSummary';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { isRangeLocked, licenseLoaded, loadLicenseStatus } from '@/stores/license';
import type { TimeRange } from '@/api/charts';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import type { Resource } from '@/types/resource';

const INFRASTRUCTURE_TREND_RANGES: { value: TimeRange; label: string }[] = [
  { value: '1h', label: '1h' },
  { value: '4h', label: '4h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
];

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
  const [searchQuery, setSearchQuery] = createSignal('');
  type GroupingMode = 'grouped' | 'flat';
  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'infrastructureGroupingMode',
    'grouped',
    { deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped') },
  );
  const [trendRange, setTrendRange] = usePersistentSignal<TimeRange>(
    'infrastructureTrendRange',
    '1h',
    {
      deserialize: (raw) =>
        INFRASTRUCTURE_TREND_RANGES.some((option) => option.value === raw)
          ? (raw as TimeRange)
          : '1h',
    },
  );
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [highlightedResourceId, setHighlightedResourceId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledSourceParam, setHandledSourceParam] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  createEffect(() => {
    void loadLicenseStatus();
  });

  createEffect(() => {
    if (!licenseLoaded()) return;
    if (isRangeLocked(trendRange())) {
      setTrendRange('7d');
    }
  });

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

  createEffect(() => {
    const params = new URLSearchParams(location.search);
    const sourceParam = params.get('source');
    if (!sourceParam || sourceParam === handledSourceParam()) return;
    const sources = sourceParam
      .split(',')
      .map((value) => normalizeSource(value.trim()))
      .filter((value): value is string => Boolean(value));
    if (sources.length === 0) return;
    setSelectedSources(new Set(sources));
    setHandledSourceParam(sourceParam);
  });

  createEffect(() => {
    const params = new URLSearchParams(location.search);
    const nextSearch = params.get('q') ?? params.get('search') ?? '';
    if (nextSearch !== untrack(searchQuery)) {
      setSearchQuery(nextSearch);
    }
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

  function normalizeSource(value: string): string | null {
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
  }

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
    () => selectedSources().size > 0 || selectedStatuses().size > 0 || searchQuery().trim().length > 0,
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
    setSearchQuery('');
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

  const isTrendRangeLocked = (range: TimeRange) => licenseLoaded() && isRangeLocked(range);

  const matchesSearch = (resource: Resource, term: string) => {
    if (!term) return true;
    const normalizedTerm = term.toLowerCase();
    const candidates: string[] = [
      resource.name,
      resource.displayName,
      resource.id,
      resource.identity?.hostname ?? '',
      ...(resource.identity?.ips ?? []),
      ...(resource.tags ?? []),
    ];
    const haystack = candidates
      .filter((value): value is string => typeof value === 'string' && value.length > 0)
      .join(' ')
      .toLowerCase();
    return haystack.includes(normalizedTerm);
  };

  const searchTerms = createMemo(() =>
    searchQuery()
      .trim()
      .toLowerCase()
      .split(/\s+/)
      .filter((term) => term.length > 0),
  );

  const filteredResources = createMemo(() => {
    let filtered = resources();
    const sources = selectedSources();
    const statuses = selectedStatuses();
    const terms = searchTerms();

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

    if (terms.length > 0) {
      filtered = filtered.filter((resource) =>
        terms.every((term) => matchesSearch(resource, term)),
      );
    }

    return filtered;
  });

  const hasFilteredResources = createMemo(() => filteredResources().length > 0);

  const stats = createMemo(() => {
    const all = filteredResources();
    const online = all.filter((r) => r.status === 'online').length;
    const offline = all.length - online;
    return { total: all.length, online, offline };
  });

  return (
    <div class="space-y-4">
      <Show when={!loading() || initialLoadComplete()} fallback={
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
                <div class="space-y-3">
                  <SearchInput
                    value={searchQuery}
                    onChange={setSearchQuery}
                    placeholder="Search hosts, IDs, IPs, or tags..."
                    autoFocus
                  />

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

                    <div class="h-5 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block" />

                    <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <button
                        type="button"
                        onClick={() => setGroupingMode('grouped')}
                        class={`inline-flex items-center gap-1.5 ${segmentedButtonClass(groupingMode() === 'grouped', false)}`}
                        title="Group by cluster"
                      >
                        <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
                        </svg>
                        Grouped
                      </button>
                      <button
                        type="button"
                        onClick={() => setGroupingMode(groupingMode() === 'flat' ? 'grouped' : 'flat')}
                        class={`inline-flex items-center gap-1.5 ${segmentedButtonClass(groupingMode() === 'flat', false)}`}
                        title="Flat list view"
                      >
                        <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <line x1="8" y1="6" x2="21" y2="6" />
                          <line x1="8" y1="12" x2="21" y2="12" />
                          <line x1="8" y1="18" x2="21" y2="18" />
                          <line x1="3" y1="6" x2="3.01" y2="6" />
                          <line x1="3" y1="12" x2="3.01" y2="12" />
                          <line x1="3" y1="18" x2="3.01" y2="18" />
                        </svg>
                        List
                      </button>
                    </div>

                    <div class="h-5 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block" />

                    <div class="flex items-center gap-2">
                      <span class="uppercase tracking-wide text-[9px] text-gray-400 dark:text-gray-500">Trend</span>
                      <div class="inline-flex flex-wrap rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                        <For each={INFRASTRUCTURE_TREND_RANGES}>
                          {(rangeOption) => {
                            const isSelected = () => trendRange() === rangeOption.value;
                            const isLocked = () => isTrendRangeLocked(rangeOption.value);
                            const isDisabled = () => isLocked() && !isSelected();
                            return (
                              <button
                                type="button"
                                disabled={isDisabled()}
                                aria-pressed={isSelected()}
                                onClick={() => setTrendRange(rangeOption.value)}
                                class={segmentedButtonClass(isSelected(), isDisabled())}
                                title={isLocked() ? `${rangeOption.label} history requires Pulse Pro` : `Show ${rangeOption.label} trend history`}
                              >
                                {rangeOption.label}
                                <Show when={isLocked()}>
                                  <span class="ml-0.5 text-[8px] font-semibold uppercase text-blue-600 dark:text-blue-300">
                                    Pro
                                  </span>
                                </Show>
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
                </div>
              </Card>

              <InfrastructureSummary
                hosts={filteredResources()}
                timeRange={trendRange()}
              />

              <div class="flex items-center gap-3 px-1 text-[11px] text-gray-500 dark:text-gray-400">
                <span class="font-medium text-gray-700 dark:text-gray-200">{stats().total} {stats().total === 1 ? 'host' : 'hosts'}</span>
                <Show when={stats().online > 0}>
                  <span class="text-emerald-600 dark:text-emerald-400">{stats().online} online</span>
                </Show>
                <Show when={stats().offline > 0}>
                  <span class="text-gray-400 dark:text-gray-500">{stats().offline} offline</span>
                </Show>
              </div>

              <Show
                when={hasFilteredResources()}
                fallback={
                  <Card class="p-6">
                    <EmptyState
                      icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                      title="No resources match filters"
                      description="Try adjusting the search, source, or status filters."
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
                  groupingMode={groupingMode()}
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
