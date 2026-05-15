import { A } from '@solidjs/router';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { TableCard } from '@/components/shared/TableCard';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import type { Resource, ResourceStatus } from '@/types/resource';

export type PlatformTabSpec<TabId extends string> = {
  id: TabId;
  label: string;
  path: string;
};

export function PlatformSectionTabs<TabId extends string>(props: {
  tabs: readonly PlatformTabSpec<TabId>[];
  active: TabId;
  ariaLabel: string;
}) {
  return (
    <nav class="flex flex-wrap items-center gap-1 border-b border-border" aria-label={props.ariaLabel}>
      <For each={props.tabs}>
        {(tab) => (
          <A
            href={tab.path}
            class={`inline-flex min-h-10 items-center border-b-2 px-3 text-sm font-medium transition-colors ${
              props.active === tab.id
                ? 'border-blue-500 text-blue-600 dark:text-blue-300'
                : 'border-transparent text-muted hover:border-border hover:text-base-content'
            }`}
            aria-current={props.active === tab.id ? 'page' : undefined}
          >
            {tab.label}
          </A>
        )}
      </For>
    </nav>
  );
}

export function PlatformTableEmptyState(props: {
  icon: JSX.Element;
  title: string;
  description: string;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState icon={props.icon} title={props.title} description={props.description} />
      </div>
    </TableCard>
  );
}

export function PlatformErrorState(props: {
  title: string;
  description: string;
  onRefresh: () => void;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState
          icon={<TriangleAlertIcon class="h-6 w-6 text-slate-400" />}
          title={props.title}
          description={props.description}
          actions={
            <button
              type="button"
              onClick={props.onRefresh}
              class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
            >
              Refresh
            </button>
          }
        />
      </div>
    </TableCard>
  );
}

// Status filter applied client-side by the platform-page toolbar. Mirrors
// the v5 dashboard/storage status segmented control: All / Online (running)
// / Degraded / Offline (stopped). Resource statuses are normalized through
// `mapResourceStatusToTriad` so per-platform vocabulary differences (e.g.
// 'running' vs 'online', 'stopped' vs 'offline') collapse to one chip set.
export type PlatformResourceStatusFilter = 'all' | 'online' | 'degraded' | 'offline';

const PLATFORM_STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Online' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

const ONLINE_STATUSES = new Set<ResourceStatus>(['online', 'running']);
const OFFLINE_STATUSES = new Set<ResourceStatus>(['offline', 'stopped']);
const DEGRADED_STATUSES = new Set<ResourceStatus>(['degraded', 'paused']);

const mapResourceStatusToTriad = (
  status: ResourceStatus | undefined,
): Exclude<PlatformResourceStatusFilter, 'all'> | 'unknown' => {
  if (!status) return 'unknown';
  if (ONLINE_STATUSES.has(status)) return 'online';
  if (DEGRADED_STATUSES.has(status)) return 'degraded';
  if (OFFLINE_STATUSES.has(status)) return 'offline';
  return 'unknown';
};

const matchesPlatformSearch = (resource: Resource, search: string): boolean => {
  if (!search) return true;
  const needle = search.trim().toLowerCase();
  if (!needle) return true;
  const haystack = [
    resource.name,
    resource.displayName,
    resource.id,
    resource.parentName,
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string')
    .join(' ')
    .toLowerCase();
  return haystack.includes(needle);
};

export const filterPlatformResources = (
  resources: Resource[],
  search: string,
  status: PlatformResourceStatusFilter,
): Resource[] => {
  const result: Resource[] = [];
  for (const resource of resources) {
    if (!matchesPlatformSearch(resource, search)) continue;
    if (status !== 'all') {
      const mapped = mapResourceStatusToTriad(resource.status);
      if (mapped !== status) continue;
    }
    result.push(resource);
  }
  return result;
};

// Compact operator-facing counter shown at the right of the toolbar so
// users can read total / matching at a glance, mirroring v5's dense
// dashboard counters without spawning a card grid.
const PlatformResourceCounter: Component<{ visible: number; total: number }> = (props) => (
  <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
    <Show when={props.visible !== props.total} fallback={<>{props.total} rows</>}>
      {props.visible} of {props.total} rows
    </Show>
  </span>
);

export const PlatformResourceTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  groupingMode?: 'grouped' | 'flat';
  searchPlaceholder?: string;
}> = (props) => {
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filteredResources = createMemo(() =>
    filterPlatformResources(props.resources, search(), status()),
  );

  const visibleCount = createMemo(() => filteredResources().length);
  const totalCount = createMemo(() => props.resources.length);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[180px] flex-1 sm:max-w-xs">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder={props.searchPlaceholder ?? 'Search rows'}
            />
          </div>
          <FilterButtonGroup
            options={PLATFORM_STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <PlatformResourceCounter visible={visibleCount()} total={totalCount()} />
        </div>
        <Show
          when={filteredResources().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No rows match current filters"
              description="Adjust the search or status filter to see more rows."
            />
          }
        >
          <UnifiedResourceTable
            resources={filteredResources()}
            expandedResourceId={expandedResourceId()}
            onExpandedResourceChange={setExpandedResourceId}
            groupingMode={props.groupingMode ?? 'grouped'}
          />
        </Show>
      </div>
    </Show>
  );
};
