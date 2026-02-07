import { For, Show, createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { MigrationNoticeBanner } from '@/components/shared/MigrationNoticeBanner';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { InfrastructureSummary } from '@/components/Infrastructure/InfrastructureSummary';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import type { TimeRange } from '@/api/charts';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  isSummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import {
  dismissMigrationNotice,
  isMigrationNoticeDismissed,
  resolveMigrationNotice,
} from '@/routing/migrationNotices';
import {
  buildInfrastructurePath,
  INFRASTRUCTURE_PATH,
  INFRASTRUCTURE_QUERY_PARAMS,
  parseInfrastructureLinkSearch,
} from '@/routing/resourceLinks';

export function Infrastructure() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const location = useLocation();
  const navigate = useNavigate();

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
  const [infrastructureSummaryRange, setInfrastructureSummaryRange] = usePersistentSignal<TimeRange>(
    STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_RANGE,
    '1h',
    { deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h') },
  );
  type GroupingMode = 'grouped' | 'flat';
  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'infrastructureGroupingMode',
    'grouped',
    { deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped') },
  );
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [hoveredResourceId, setHoveredResourceId] = createSignal<string | null>(null);
  const [highlightedResourceId, setHighlightedResourceId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledSourceParam, setHandledSourceParam] = createSignal<string | null>(null);
  const [hideMigrationNotice, setHideMigrationNotice] = createSignal(true);
  let highlightTimer: number | undefined;
  const sourceOptions = [
    { key: 'proxmox', label: 'PVE' },
    { key: 'agent', label: 'Agent' },
    { key: 'docker', label: 'Docker' },
    { key: 'pbs', label: 'PBS' },
    { key: 'pmg', label: 'PMG' },
    { key: 'kubernetes', label: 'K8s' },
  ];

  const migrationNotice = createMemo(() => {
    const notice = resolveMigrationNotice(location.search);
    if (!notice || notice.target !== 'infrastructure') return null;
    return notice;
  });

  createEffect(() => {
    const notice = migrationNotice();
    if (!notice) {
      setHideMigrationNotice(true);
      return;
    }
    setHideMigrationNotice(isMigrationNoticeDismissed(notice.id));
  });

  const handleDismissMigrationNotice = () => {
    const notice = migrationNotice();
    if (!notice) return;
    dismissMigrationNotice(notice.id);
    setHideMigrationNotice(true);
  };

  createEffect(() => {
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
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
    const { source: sourceParam } = parseInfrastructureLinkSearch(location.search);
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
    const { query: nextSearch } = parseInfrastructureLinkSearch(location.search);
    if (nextSearch !== untrack(searchQuery)) {
      setSearchQuery(nextSearch);
    }
  });

  createEffect(() => {
    if (location.pathname !== INFRASTRUCTURE_PATH) return;

    const selectedSourceValues = sourceOptions
      .map((source) => source.key)
      .filter((source) => selectedSources().has(source));
    const nextSource = selectedSourceValues.join(',');
    const nextQuery = searchQuery().trim();
    const currentLinkedResource = parseInfrastructureLinkSearch(location.search).resource;
    const selectedResourceId = expandedResourceId();
    const shouldPreserveIncomingResource =
      !selectedResourceId &&
      Boolean(currentLinkedResource) &&
      !initialLoadComplete();
    const nextResource = shouldPreserveIncomingResource
      ? currentLinkedResource
      : (selectedResourceId ?? '');

    const managedPath = buildInfrastructurePath({
      source: nextSource || null,
      query: nextQuery || null,
      resource: nextResource || null,
    });
    const managedUrl = new URL(managedPath, 'http://pulse.local');
    const params = new URLSearchParams(location.search);
    params.delete(INFRASTRUCTURE_QUERY_PARAMS.source);
    params.delete(INFRASTRUCTURE_QUERY_PARAMS.query);
    params.delete(INFRASTRUCTURE_QUERY_PARAMS.legacyQuery);
    params.delete(INFRASTRUCTURE_QUERY_PARAMS.resource);
    managedUrl.searchParams.forEach((value, key) => {
      params.set(key, value);
    });

    const nextSearch = params.toString();
    const nextPath = nextSearch ? `${INFRASTRUCTURE_PATH}?${nextSearch}` : INFRASTRUCTURE_PATH;
    const currentPath = `${location.pathname}${location.search || ''}`;
    if (nextPath !== currentPath) {
      navigate(nextPath, { replace: true });
    }
  });

  onCleanup(() => {
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
  });

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
    const base = 'px-2 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95';
    if (disabled) {
      return `${base} text-gray-400 dark:text-gray-600 cursor-not-allowed`;
    }
    if (selected) {
      return `${base} bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600`;
    }
    return `${base} text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50`;
  };

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

  createEffect(() => {
    const hoveredId = hoveredResourceId();
    if (!hoveredId) return;
    const exists = filteredResources().some((resource) => resource.id === hoveredId);
    if (!exists) {
      setHoveredResourceId(null);
    }
  });

  return (
    <div data-testid="infrastructure-page" class="space-y-4">
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
                  description="Once resources are reporting, they will appear here."
                />
              </Card>
            }
          >
            <div class="space-y-3">
              <Show when={migrationNotice() && !hideMigrationNotice()}>
                <MigrationNoticeBanner
                  title={migrationNotice()!.title}
                  message={migrationNotice()!.message}
                  learnMoreHref={migrationNotice()!.learnMoreHref}
                  onDismiss={handleDismissMigrationNotice}
                />
              </Show>

              <div class="sticky-shield sticky top-0 z-20 bg-white dark:bg-gray-800">
                <InfrastructureSummary
                  hosts={filteredResources()}
                  timeRange={infrastructureSummaryRange()}
                  onTimeRangeChange={setInfrastructureSummaryRange}
                  hoveredHostId={hoveredResourceId()}
                  focusedHostId={expandedResourceId()}
                />
              </div>

              <Card padding="sm" class="mb-4">
                <div class="flex flex-col gap-2">
                  <SearchInput
                    value={searchQuery}
                    onChange={setSearchQuery}
                    placeholder="Search resources, IDs, IPs, or tags..."
                    class="w-full"
                    autoFocus
                    history={{
                      storageKey: STORAGE_KEYS.RESOURCES_SEARCH_HISTORY,
                      emptyMessage: 'Recent infrastructure searches appear here.',
                    }}
                  />

                  <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400 lg:flex-nowrap">
                    <div class="flex items-center gap-2">
                      <span class="uppercase tracking-wide text-[9px] text-gray-400 dark:text-gray-500">Source</span>
                      <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
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

                    <div class="flex items-center gap-2">
                      <span class="uppercase tracking-wide text-[9px] text-gray-400 dark:text-gray-500">Status</span>
                      <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
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
                        onClick={() => setGroupingMode('flat')}
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

                    <Show when={hasActiveFilters()}>
                      <button
                        type="button"
                        onClick={clearFilters}
                        class="ml-auto rounded-lg bg-blue-100 px-2.5 py-1.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900/40 dark:text-blue-300 dark:hover:bg-blue-900/60"
                      >
                        Clear
                      </button>
                    </Show>
                  </div>
                </div>
              </Card>

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
                  hoveredResourceId={hoveredResourceId()}
                  highlightedResourceId={highlightedResourceId()}
                  onExpandedResourceChange={setExpandedResourceId}
                  onHoverChange={setHoveredResourceId}
                  groupingMode={groupingMode()}
                />
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
      <ScrollToTopButton />
    </div>
  );
}

export default Infrastructure;
