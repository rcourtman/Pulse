import { For, Show, createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import {
  FilterActionButton,
  FilterHeader,
  FilterMobileToggleButton,
  FilterSegmentedControl,
  LabeledFilterSelect,
} from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { InfrastructureSummary } from '@/components/Infrastructure/InfrastructureSummary';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import type { TimeRange } from '@/api/charts';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useKioskMode } from '@/hooks/useKioskMode';
import { AgentDeployModal } from '@/components/Infrastructure/AgentDeployModal';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import {
  tokenizeSearch,
  filterResources,
  collectAvailableSources,
  collectAvailableStatuses,
  buildStatusOptions,
} from '@/components/Infrastructure/infrastructureSelectors';
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

  const kioskMode = useKioskMode();

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
  const [infrastructureSummaryRange, setInfrastructureSummaryRange] =
    usePersistentSignal<TimeRange>(STORAGE_KEYS.INFRASTRUCTURE_SUMMARY_RANGE, '1h', {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    });
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
  const { isMobile } = useBreakpoint();
  const [deployCluster, setDeployCluster] = createSignal<{ id: string; name: string } | null>(null);
  const [filtersOpen, setFiltersOpen] = createSignal(false);
  const activeFilterCount = createMemo(
    () => (selectedSource() !== '' ? 1 : 0) + (selectedStatus() !== '' ? 1 : 0),
  );
  let highlightTimer: number | undefined;

  // URL sync can require multiple reactive updates (normalizing source values,
  // preserving deep-links). Navigating synchronously
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
    const normalized = normalizeSource(sourceParam) ?? '';
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
      !selectedResourceId && Boolean(currentLinkedResource) && !initialLoadComplete();
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
      case 'proxmox':
        return 'proxmox';
      case 'agent':
        return 'agent';
      case 'docker':
        return 'docker';
      case 'pbs':
        return 'pbs';
      case 'pmg':
        return 'pmg';
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
      <Show
        when={!loading() || initialLoadComplete()}
        fallback={
          <div class="space-y-3 animate-pulse pointer-events-none select-none">
            <div class="hidden lg:block h-[124px] w-full bg-surface-alt rounded-md border border-border"></div>
            <Card padding="sm" class="h-[52px] bg-surface-alt"></Card>
            <Card padding="none" tone="card" class="h-[600px] overflow-hidden">
              <div class="h-8 border-b"></div>
              <div class="space-y-4 p-4">
                <div class="h-4 w-1/4 rounded bg-surface-hover"></div>
                <div class="h-4 w-1/2 rounded bg-surface-hover"></div>
                <div class="h-4 w-1/3 rounded bg-surface-hover"></div>
              </div>
            </Card>
          </div>
        }
      >
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
                    class="inline-flex items-center gap-2 rounded-md border px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:"
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
                  description="Add Proxmox VE nodes or install the Pulse agent on your infrastructure to start monitoring."
                  actions={
                    <button
                      type="button"
                      onClick={() => navigate('/settings')}
                      class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                    >
                      <SettingsIcon class="h-3.5 w-3.5" />
                      Add Infrastructure
                    </button>
                  }
                />
              </Card>
            }
          >
            <div class="space-y-3">
              <Show when={!summaryCollapsed()}>
                <div class="hidden lg:block sticky-shield sticky top-0 z-20 bg-surface">
                  <InfrastructureSummary
                    resources={filteredResources()}
                    timeRange={infrastructureSummaryRange()}
                    onTimeRangeChange={setInfrastructureSummaryRange}
                    hoveredResourceId={hoveredResourceId()}
                    focusedResourceId={expandedResourceId()}
                  />
                </div>
              </Show>

              <Show when={!kioskMode()}>
                <Card padding="sm" class="mb-4">
                  <FilterHeader
                    search={
                      <SearchInput
                        value={searchQuery}
                        onChange={setSearchQuery}
                        placeholder="Search resources, IDs, IPs, or tags..."
                        class="w-full"
                        typeToSearch
                        history={{
                          storageKey: STORAGE_KEYS.RESOURCES_SEARCH_HISTORY,
                          emptyMessage: 'Recent infrastructure searches appear here.',
                        }}
                      />
                    }
                    searchAccessory={
                      <Show when={isMobile()}>
                        <FilterMobileToggleButton
                          onClick={() => setFiltersOpen((o) => !o)}
                          count={activeFilterCount()}
                        />
                      </Show>
                    }
                    showFilters={!isMobile() || filtersOpen()}
                    toolbarClass="lg:flex-nowrap"
                  >
                    <LabeledFilterSelect
                      id="infra-source-filter"
                      label="Source"
                      value={selectedSource()}
                      onChange={(e) => setSelectedSource(e.currentTarget.value)}
                      selectClass="min-w-[8rem]"
                    >
                      <option value="">All</option>
                      <For each={sourceOptions.filter((s) => availableSources().has(s.key))}>
                        {(source) => <option value={source.key}>{source.label}</option>}
                      </For>
                    </LabeledFilterSelect>

                    <LabeledFilterSelect
                      id="infra-status-filter"
                      label="Status"
                      value={selectedStatus()}
                      onChange={(e) => setSelectedStatus(e.currentTarget.value)}
                      selectClass="min-w-[7rem]"
                    >
                      <option value="">All</option>
                      <For each={statusOptions()}>
                        {(status) => <option value={status.key}>{status.label}</option>}
                      </For>
                    </LabeledFilterSelect>

                    <FilterSegmentedControl
                      value={groupingMode()}
                      onChange={(value) => setGroupingMode(value as GroupingMode)}
                      aria-label="Group By"
                      options={[
                        {
                          value: 'grouped',
                          title: 'Group by cluster',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
                              </svg>
                              Grouped
                            </>
                          ),
                        },
                        {
                          value: 'flat',
                          title: 'Flat list view',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <line x1="8" y1="6" x2="21" y2="6" />
                                <line x1="8" y1="12" x2="21" y2="12" />
                                <line x1="8" y1="18" x2="21" y2="18" />
                                <line x1="3" y1="6" x2="3.01" y2="6" />
                                <line x1="3" y1="12" x2="3.01" y2="12" />
                                <line x1="3" y1="18" x2="3.01" y2="18" />
                              </svg>
                              List
                            </>
                          ),
                        },
                      ]}
                    />

                    <FilterSegmentedControl
                      class="hidden lg:inline-flex"
                      value={summaryCollapsed() ? 'hidden' : 'shown'}
                      onChange={() => setSummaryCollapsed((c) => !c)}
                      aria-label="Charts"
                      options={[
                        {
                          value: 'shown',
                          title: summaryCollapsed() ? 'Show charts' : 'Hide charts',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                              </svg>
                              Charts
                            </>
                          ),
                        },
                      ]}
                    />

                    <Show when={hasActiveFilters()}>
                      <FilterActionButton onClick={clearFilters} class="ml-auto text-base-content">
                        Clear
                      </FilterActionButton>
                    </Show>
                  </FilterHeader>
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
                            class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
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
                  onDeployCluster={(id, name) => setDeployCluster({ id, name })}
                />
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
      <Show when={deployCluster()}>
        {(cluster) => (
          <AgentDeployModal
            isOpen={true}
            clusterId={cluster().id}
            clusterName={cluster().name}
            onClose={() => setDeployCluster(null)}
          />
        )}
      </Show>
      <ScrollToTopButton />
    </div>
  );
}

export default Infrastructure;
