import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount, untrack } from 'solid-js';
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
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { segmentedButtonClass } from '@/utils/segmentedButton';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';
import {
  isSummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import {
  tokenizeSearch,
  filterResources,
  collectAvailableSources,
  collectAvailableStatuses,
  buildStatusOptions,
} from '@/components/Infrastructure/infrastructureSelectors';
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
import { areSearchParamsEquivalent } from '@/utils/searchParams';

export function Infrastructure() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const location = useLocation();
  const navigate = useNavigate();

  const [kioskMode, setKioskMode] = createSignal(isKioskMode());
  onMount(() => {
    const unsubscribe = subscribeToKioskMode((enabled) => {
      setKioskMode(enabled);
    });
    return unsubscribe;
  });

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
  const [selectedSource, setSelectedSource] = createSignal('');
  const [selectedStatus, setSelectedStatus] = createSignal('');
  const [searchQuery, setSearchQuery] = createSignal('');
  const [infrastructureSummaryRange, setInfrastructureSummaryRange] = usePersistentSignal<TimeRange>(
    STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_RANGE,
    '1h',
    { deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h') },
  );
  const [summaryCollapsed, setSummaryCollapsed] = usePersistentSignal<boolean>(
    STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_COLLAPSED,
    false,
    { deserialize: (raw) => raw === 'true' },
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
  const [handledQueryParam, setHandledQueryParam] = createSignal<string>('');
  const [hideMigrationNotice, setHideMigrationNotice] = createSignal(true);
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);
  const activeFilterCount = createMemo(() => (selectedSource() !== '' ? 1 : 0) + (selectedStatus() !== '' ? 1 : 0));
  let highlightTimer: number | undefined;

  // URL sync can require multiple reactive updates (canonicalizing legacy params,
  // normalizing source aliases, preserving deep-links). Navigating synchronously
  // for each intermediate state can trigger Solid Router's redirect protection.
  // Coalesce URL sync into a single replace-navigation per tick.
  let pendingUrlSyncHandle: number | null = null;
  let pendingUrlSyncPath: string | null = null;
  const scheduleUrlSyncNavigate = (nextPath: string) => {
    pendingUrlSyncPath = nextPath;
    if (pendingUrlSyncHandle !== null) return;
    pendingUrlSyncHandle = window.setTimeout(() => {
      pendingUrlSyncHandle = null;
      const target = pendingUrlSyncPath;
      pendingUrlSyncPath = null;
      if (!target) return;
      const current = `${untrack(() => location.pathname)}${untrack(() => location.search)}`;
      if (current === target) return;
      navigate(target, { replace: true });
    }, 0);
  };
  const sourceOptions = [
    { key: 'proxmox', label: 'PVE' },
    { key: 'agent', label: 'Agent' },
    { key: 'docker', label: 'Containers' },
    { key: 'pbs', label: 'PBS' },
    { key: 'pmg', label: 'PMG' },
    { key: 'kubernetes', label: 'K8s' },
    { key: 'truenas', label: 'TrueNAS' },
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
    const { resource: resourceId } = parseInfrastructureLinkSearch(location.search);
    if (resourceId) return;

    // Only treat "missing resource param" as a close signal if we've previously
    // handled a resource deep-link or written one into the URL. Otherwise, this
    // can fight user-driven opens before the URL-sync effect runs.
    if (handledResourceId() === null) return;

    if (expandedResourceId() !== null) {
      setExpandedResourceId(null);
    }
    if (highlightedResourceId() !== null) {
      setHighlightedResourceId(null);
    }
    setHandledResourceId(null);
  });

  createEffect(() => {
    const { source: sourceParam } = parseInfrastructureLinkSearch(location.search);
    if (!sourceParam) {
      const previous = (handledSourceParam() ?? '').trim();
      if (previous) {
        if (selectedSource() !== '') setSelectedSource('');
        setHandledSourceParam('');
      } else if (handledSourceParam() === null) {
        setHandledSourceParam('');
      }
      return;
    }
    if (sourceParam === handledSourceParam()) return;
    // Support legacy comma-separated params by taking the first value
    const firstSource = sourceParam.split(',')[0].trim();
    const normalized = normalizeSource(firstSource) ?? '';
    setSelectedSource(normalized);
    setHandledSourceParam(sourceParam);
  });

  createEffect(() => {
    const { query: nextSearch } = parseInfrastructureLinkSearch(location.search);
    const normalized = nextSearch ?? '';
    if (normalized !== handledQueryParam()) {
      if (normalized !== untrack(searchQuery)) {
        setSearchQuery(normalized);
      }
      setHandledQueryParam(normalized);
    }
  });

  createEffect(() => {
    if (location.pathname !== INFRASTRUCTURE_PATH) return;

    // Avoid oscillation: only write managed params after we've processed the current URL.
    const parsed = parseInfrastructureLinkSearch(location.search);
    const urlSource = parsed.source ?? '';
    const urlQuery = parsed.query ?? '';
    const urlResource = parsed.resource ?? '';
    if ((handledSourceParam() ?? '') !== urlSource) return;
    if (handledQueryParam() !== urlQuery) return;
    if (urlResource && handledResourceId() !== urlResource && !initialLoadComplete()) return;

    const nextSource = selectedSource();
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
    const currentParams = new URLSearchParams(location.search);
    const nextParams = new URLSearchParams(location.search);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.source);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.query);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.legacyQuery);
    nextParams.delete(INFRASTRUCTURE_QUERY_PARAMS.resource);
    managedUrl.searchParams.forEach((value, key) => {
      nextParams.set(key, value);
    });

    if (!areSearchParamsEquivalent(currentParams, nextParams)) {
      const nextSearch = nextParams.toString();
      const nextPath = nextSearch ? `${INFRASTRUCTURE_PATH}?${nextSearch}` : INFRASTRUCTURE_PATH;
      scheduleUrlSyncNavigate(nextPath);
    }
  });

  onCleanup(() => {
    if (pendingUrlSyncHandle !== null) {
      window.clearTimeout(pendingUrlSyncHandle);
      pendingUrlSyncHandle = null;
      pendingUrlSyncPath = null;
    }
    if (highlightTimer) {
      window.clearTimeout(highlightTimer);
    }
  });

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
      case 'truenas':
        return 'truenas';
      default:
        return null;
    }
  }

  const availableSources = createMemo(() => collectAvailableSources(resources()));

  const availableStatuses = createMemo(() => collectAvailableStatuses(resources()));

  const statusOptions = createMemo(() => buildStatusOptions(availableStatuses()));

  const hasActiveFilters = createMemo(
    () => selectedSource() !== '' || selectedStatus() !== '' || searchQuery().trim().length > 0,
  );

  const clearFilters = () => {
    setSelectedSource('');
    setSelectedStatus('');
    setSearchQuery('');
  };

  const searchTerms = createMemo(() => tokenizeSearch(searchQuery()));

  const filteredResources = createMemo(() =>
    filterResources(
      resources(),
      selectedSource() !== '' ? new Set([selectedSource()]) : new Set(),
      selectedStatus() !== '' ? new Set([selectedStatus()]) : new Set(),
      searchTerms(),
    ),
  );

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
        <div class="space-y-3 animate-pulse pointer-events-none select-none">
          <div class="hidden lg:block h-[124px] w-full bg-surface-alt rounded-md border border-border"></div>
          <Card padding="sm" class="h-[52px] bg-surface-alt"></Card>
          <Card padding="none" tone="card" class="h-[600px] overflow-hidden">
            <div class="h-8 border-b border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800"></div>
            <div class="space-y-4 p-4">
              <div class="h-4 w-1/4 rounded bg-surface-hover"></div>
              <div class="h-4 w-1/2 rounded bg-surface-hover"></div>
              <div class="h-4 w-1/3 rounded bg-surface-hover"></div>
            </div>
          </Card>
        </div>
      }>
        <Show
          when={!error()}
          fallback={
            <Card class="p-6">
              <EmptyState
                icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
                title="Unable to load infrastructure"
                description="We couldn’t fetch unified resources. Check connectivity or retry."
                actions={
                  <button
                    type="button"
                    onClick={() => refetch()}
                    class="inline-flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 shadow-sm hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200"
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
                  icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
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

              <Show when={!summaryCollapsed()}>
                <div class="hidden lg:block sticky-shield sticky top-0 z-20 bg-surface">
                  <InfrastructureSummary
                    hosts={filteredResources()}
                    timeRange={infrastructureSummaryRange()}
                    onTimeRangeChange={setInfrastructureSummaryRange}
                    hoveredHostId={hoveredResourceId()}
                    focusedHostId={expandedResourceId()}
                  />
                </div>
              </Show>

              <Show when={!kioskMode()}>
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

                    <Show when={isMobile()}>
                      <button
                        type="button"
                        onClick={() => setFiltersOpen((o) => !o)}
                        class="flex items-center gap-1.5 rounded-md bg-surface-hover px-2.5 py-1.5 text-xs font-medium text-muted"
                      >
                        <ListFilterIcon class="w-3.5 h-3.5" />
                        Filters
                        <Show when={activeFilterCount() > 0}>
                          <span class="ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold text-white leading-none">
                            {activeFilterCount()}
                          </span>
                        </Show>
                      </button>
                    </Show>

                    <Show when={!isMobile() || filtersOpen()}>
                      <div class="flex flex-wrap items-center gap-2 text-xs text-muted lg:flex-nowrap">
                        <div class="inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5">
                          <label for="infra-source-filter" class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted">Source</label>
                          <select
                            id="infra-source-filter"
                            value={selectedSource()}
                            onChange={(e) => setSelectedSource(e.currentTarget.value)}
                            class="min-w-[8rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                          >
                            <option value="">All</option>
                            <For each={sourceOptions.filter((s) => availableSources().has(s.key))}>
                              {(source) => <option value={source.key}>{source.label}</option>}
                            </For>
                          </select>
                        </div>

                        <div class="inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5">
                          <label for="infra-status-filter" class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted">Status</label>
                          <select
                            id="infra-status-filter"
                            value={selectedStatus()}
                            onChange={(e) => setSelectedStatus(e.currentTarget.value)}
                            class="min-w-[7rem] rounded-md border border-slate-200 bg-white px-2 py-1 text-xs font-medium text-slate-900 shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                          >
                            <option value="">All</option>
                            <For each={statusOptions()}>
                              {(status) => <option value={status.key}>{status.label}</option>}
                            </For>
                          </select>
                        </div>

                        <div class="inline-flex rounded-md bg-surface-hover p-0.5">
                          <button
                            type="button"
                            onClick={() => setGroupingMode('grouped')}
                            class={segmentedButtonClass(groupingMode() === 'grouped')}
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
                            class={segmentedButtonClass(groupingMode() === 'flat')}
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

                        <div class="hidden lg:inline-flex rounded-md bg-surface-hover p-0.5">
                          <button
                            type="button"
                            onClick={() => setSummaryCollapsed((c) => !c)}
                            class={segmentedButtonClass(!summaryCollapsed())}
                            title={summaryCollapsed() ? 'Show charts' : 'Hide charts'}
                          >
                            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                            </svg>
                            Charts
                          </button>
                        </div>

                        <Show when={hasActiveFilters()}>
                          <button
                            type="button"
                            onClick={clearFilters}
                            class="ml-auto rounded-md bg-blue-100 px-2.5 py-1.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-900"
                          >
                            Clear
                          </button>
                        </Show>
                      </div>
                    </Show>
                  </div>
                </Card>
              </Show>

              <Show
                when={hasFilteredResources()}
                fallback={
                  <Card class="p-6">
                    <EmptyState
                      icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
                      title="No resources match filters"
                      description="Try adjusting the search, source, or status filters."
                      actions={
                        <Show when={hasActiveFilters()}>
                          <button
                            type="button"
                            onClick={clearFilters}
                            class="inline-flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 shadow-sm hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200"
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
