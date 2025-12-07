import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, onMount, onCleanup, createResource } from 'solid-js';
import type { Resource, ResourceType, PlatformType, ResourceStatus, ResourcesResponse } from '@/types/resource';
import {
    RESOURCE_TYPE_LABELS,
    PLATFORM_LABELS,
    STATUS_LABELS,
    getStatusVariant,
    isInfrastructureType,
    isWorkloadType
} from '@/types/resource';
import { formatBytes, formatUptime, formatRelativeTime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
import { apiFetchJSON } from '@/utils/apiClient';

// Fetch resources from API
async function fetchResources(): Promise<ResourcesResponse> {
    try {
        const response = await apiFetchJSON<ResourcesResponse>('/api/resources');
        return response;
    } catch (error) {
        console.error('Failed to fetch resources:', error);
        return { resources: [], count: 0, stats: { totalResources: 0, byType: {}, byPlatform: {}, byStatus: {}, withAlerts: 0, lastUpdated: '' } };
    }
}

// Filter panel component
interface ResourceFilterProps {
    search: () => string;
    setSearch: (value: string) => void;
    typeFilter: () => ResourceType | 'all';
    setTypeFilter: (value: ResourceType | 'all') => void;
    platformFilter: () => PlatformType | 'all';
    setPlatformFilter: (value: PlatformType | 'all') => void;
    statusFilter: () => ResourceStatus | 'all';
    setStatusFilter: (value: ResourceStatus | 'all') => void;
    groupBy: () => 'none' | 'type' | 'platform' | 'parent';
    setGroupBy: (value: 'none' | 'type' | 'platform' | 'parent') => void;
    stats: () => ResourcesResponse['stats'] | undefined;
}

const ResourceFilter: Component<ResourceFilterProps> = (props) => {
    const selectClass = 'bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md px-2 py-1 text-xs focus:ring-2 focus:ring-blue-500 focus:border-blue-500';

    return (
        <Card padding="sm" tone="glass" class="mb-4">
            <div class="flex flex-wrap items-center gap-3">
                {/* Search */}
                <div class="flex-1 min-w-[200px]">
                    <input
                        type="text"
                        placeholder="Search resources..."
                        value={props.search()}
                        onInput={(e) => props.setSearch(e.currentTarget.value)}
                        class="w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md px-3 py-1.5 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                    />
                </div>

                {/* Type filter */}
                <select
                    value={props.typeFilter()}
                    onChange={(e) => props.setTypeFilter(e.currentTarget.value as ResourceType | 'all')}
                    class={selectClass}
                >
                    <option value="all">All Types</option>
                    <For each={Object.entries(RESOURCE_TYPE_LABELS)}>
                        {([key, label]) => <option value={key}>{label}</option>}
                    </For>
                </select>

                {/* Platform filter */}
                <select
                    value={props.platformFilter()}
                    onChange={(e) => props.setPlatformFilter(e.currentTarget.value as PlatformType | 'all')}
                    class={selectClass}
                >
                    <option value="all">All Platforms</option>
                    <For each={Object.entries(PLATFORM_LABELS)}>
                        {([key, label]) => <option value={key}>{label}</option>}
                    </For>
                </select>

                {/* Status filter */}
                <select
                    value={props.statusFilter()}
                    onChange={(e) => props.setStatusFilter(e.currentTarget.value as ResourceStatus | 'all')}
                    class={selectClass}
                >
                    <option value="all">All Statuses</option>
                    <For each={Object.entries(STATUS_LABELS)}>
                        {([key, label]) => <option value={key}>{label}</option>}
                    </For>
                </select>

                {/* Group by */}
                <select
                    value={props.groupBy()}
                    onChange={(e) => props.setGroupBy(e.currentTarget.value as 'none' | 'type' | 'platform' | 'parent')}
                    class={selectClass}
                >
                    <option value="none">No Grouping</option>
                    <option value="type">Group by Type</option>
                    <option value="platform">Group by Platform</option>
                    <option value="parent">Group by Parent</option>
                </select>
            </div>

            {/* Stats bar */}
            <Show when={props.stats()}>
                <div class="flex flex-wrap gap-4 mt-3 pt-3 border-t border-gray-200 dark:border-gray-700 text-xs text-gray-600 dark:text-gray-400">
                    <span class="font-medium">{props.stats()!.totalResources} resources</span>
                    <Show when={props.stats()!.withAlerts > 0}>
                        <span class="text-red-600 dark:text-red-400">
                            {props.stats()!.withAlerts} with alerts
                        </span>
                    </Show>
                    <span class="text-gray-400">•</span>
                    <For each={Object.entries(props.stats()!.byStatus)}>
                        {([status, count]) => (
                            <span class="flex items-center gap-1">
                                <StatusDot variant={getStatusVariant(status as ResourceStatus)} size="xs" />
                                {count} {status}
                            </span>
                        )}
                    </For>
                </div>
            </Show>
        </Card>
    );
};

// Resource row component
interface ResourceRowProps {
    resource: Resource;
    showParent?: boolean;
    onSelect?: (resource: Resource) => void;
}

const ResourceRow: Component<ResourceRowProps> = (props) => {
    const { resource } = props;

    const cpuPercent = resource.cpu?.current ?? 0;
    const memPercent = resource.memory?.current ?? 0;
    const diskPercent = resource.disk?.current ?? 0;

    const getProgressColor = (value: number) => {
        if (value >= 90) return 'bg-red-500';
        if (value >= 75) return 'bg-yellow-500';
        return 'bg-blue-500';
    };

    return (
        <tr
            class="hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors cursor-pointer"
            onClick={() => props.onSelect?.(resource)}
        >
            {/* Name & Status */}
            <td class="pl-4 pr-2 py-2">
                <div class="flex items-center gap-2">
                    <StatusDot
                        variant={getStatusVariant(resource.status)}
                        size="xs"
                        title={STATUS_LABELS[resource.status]}
                    />
                    <div>
                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                            {resource.displayName || resource.name}
                        </p>
                        <Show when={resource.displayName && resource.displayName !== resource.name}>
                            <p class="text-[10px] text-gray-500 dark:text-gray-400">{resource.name}</p>
                        </Show>
                    </div>
                    <Show when={resource.alerts && resource.alerts.length > 0}>
                        <span class="inline-flex items-center px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                            {resource.alerts.length} alert{resource.alerts.length > 1 ? 's' : ''}
                        </span>
                    </Show>
                </div>
            </td>

            {/* Type */}
            <td class="px-2 py-2">
                <span class="text-xs text-gray-600 dark:text-gray-400">
                    {RESOURCE_TYPE_LABELS[resource.type]}
                </span>
            </td>

            {/* Platform */}
            <td class="px-2 py-2">
                <span class="text-xs text-gray-600 dark:text-gray-400">
                    {PLATFORM_LABELS[resource.platform]}
                </span>
            </td>

            {/* Source */}
            <td class="px-2 py-2">
                <span class={`text-xs ${resource.sourceType === 'agent' ? 'text-green-600 dark:text-green-400' : 'text-gray-500'}`}>
                    {resource.sourceType}
                </span>
            </td>

            {/* CPU */}
            <td class="px-2 py-2 w-[100px]">
                <Show when={resource.cpu} fallback={<span class="text-gray-400">—</span>}>
                    <div class="flex items-center gap-2">
                        <div class="flex-1 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                            <div
                                class={`h-full ${getProgressColor(cpuPercent)} rounded-full`}
                                style={{ width: `${Math.min(cpuPercent, 100)}%` }}
                            />
                        </div>
                        <span class="text-xs text-gray-600 dark:text-gray-400 w-[36px] text-right">
                            {cpuPercent.toFixed(0)}%
                        </span>
                    </div>
                </Show>
            </td>

            {/* Memory */}
            <td class="px-2 py-2 w-[100px]">
                <Show when={resource.memory} fallback={<span class="text-gray-400">—</span>}>
                    <div class="flex items-center gap-2">
                        <div class="flex-1 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                            <div
                                class={`h-full ${getProgressColor(memPercent)} rounded-full`}
                                style={{ width: `${Math.min(memPercent, 100)}%` }}
                            />
                        </div>
                        <span class="text-xs text-gray-600 dark:text-gray-400 w-[36px] text-right">
                            {memPercent.toFixed(0)}%
                        </span>
                    </div>
                </Show>
            </td>

            {/* Disk */}
            <td class="px-2 py-2 w-[100px]">
                <Show when={resource.disk} fallback={<span class="text-gray-400">—</span>}>
                    <div class="flex items-center gap-2">
                        <div class="flex-1 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                            <div
                                class={`h-full ${getProgressColor(diskPercent)} rounded-full`}
                                style={{ width: `${Math.min(diskPercent, 100)}%` }}
                            />
                        </div>
                        <span class="text-xs text-gray-600 dark:text-gray-400 w-[36px] text-right">
                            {diskPercent.toFixed(0)}%
                        </span>
                    </div>
                </Show>
            </td>

            {/* Uptime */}
            <td class="px-2 py-2">
                <span class="text-xs text-gray-600 dark:text-gray-400">
                    {resource.uptime ? formatUptime(resource.uptime) : '—'}
                </span>
            </td>

            {/* Last Seen */}
            <td class="px-2 py-2 pr-4">
                <span class="text-xs text-gray-500 dark:text-gray-500">
                    {resource.lastSeen ? formatRelativeTime(resource.lastSeen) : '—'}
                </span>
            </td>
        </tr>
    );
};

// Main Resources Overview component
export const ResourcesOverview: Component = () => {
    const [search, setSearch] = createSignal('');
    const [typeFilter, setTypeFilter] = createSignal<ResourceType | 'all'>('all');
    const [platformFilter, setPlatformFilter] = createSignal<PlatformType | 'all'>('all');
    const [statusFilter, setStatusFilter] = createSignal<ResourceStatus | 'all'>('all');
    const [groupBy, setGroupBy] = createSignal<'none' | 'type' | 'platform' | 'parent'>('none');

    // Fetch resources with auto-refresh
    const [data, { refetch }] = createResource(fetchResources);

    // Auto-refresh every 10 seconds
    let refreshInterval: number;
    onMount(() => {
        refreshInterval = window.setInterval(() => refetch(), 10000);
    });
    onCleanup(() => {
        if (refreshInterval) clearInterval(refreshInterval);
    });

    // Filter resources
    const filteredResources = createMemo(() => {
        if (!data()) return [];
        let resources = data()!.resources;

        // Apply search
        if (search()) {
            const term = search().toLowerCase();
            resources = resources.filter(r =>
                r.name.toLowerCase().includes(term) ||
                r.displayName.toLowerCase().includes(term) ||
                r.identity?.hostname?.toLowerCase().includes(term) ||
                r.identity?.primaryIp?.includes(term)
            );
        }

        // Apply type filter
        if (typeFilter() !== 'all') {
            resources = resources.filter(r => r.type === typeFilter());
        }

        // Apply platform filter
        if (platformFilter() !== 'all') {
            resources = resources.filter(r => r.platform === platformFilter());
        }

        // Apply status filter
        if (statusFilter() !== 'all') {
            resources = resources.filter(r => r.status === statusFilter());
        }

        return resources;
    });

    // Group resources
    const groupedResources = createMemo(() => {
        const resources = filteredResources();
        const mode = groupBy();

        if (mode === 'none') {
            return [{ key: 'all', label: 'All Resources', resources }];
        }

        const groups: Record<string, Resource[]> = {};

        for (const r of resources) {
            let key: string;
            switch (mode) {
                case 'type':
                    key = r.type;
                    break;
                case 'platform':
                    key = r.platform;
                    break;
                case 'parent':
                    key = r.parentId || 'no-parent';
                    break;
                default:
                    key = 'all';
            }
            if (!groups[key]) groups[key] = [];
            groups[key].push(r);
        }

        return Object.entries(groups).map(([key, resources]) => ({
            key,
            label: mode === 'type' ? RESOURCE_TYPE_LABELS[key as ResourceType] || key :
                mode === 'platform' ? PLATFORM_LABELS[key as PlatformType] || key :
                    key === 'no-parent' ? 'Standalone Resources' : key,
            resources
        })).sort((a, b) => a.label.localeCompare(b.label));
    });

    const thClass = "px-2 py-2 text-left text-xs font-medium text-gray-600 dark:text-gray-400 uppercase tracking-wider";

    return (
        <div class="space-y-4 p-4">
            <div class="flex items-center justify-between mb-4">
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">All Resources</h1>
                <button
                    onClick={() => refetch()}
                    class="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    Refresh
                </button>
            </div>

            <ResourceFilter
                search={search}
                setSearch={setSearch}
                typeFilter={typeFilter}
                setTypeFilter={setTypeFilter}
                platformFilter={platformFilter}
                setPlatformFilter={setPlatformFilter}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                groupBy={groupBy}
                setGroupBy={setGroupBy}
                stats={() => data()?.stats}
            />

            <Show when={data.loading && !data()}>
                <Card padding="lg">
                    <EmptyState
                        icon={
                            <svg class="h-12 w-12 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                            </svg>
                        }
                        title="Loading resources..."
                        description="Fetching unified resource data."
                    />
                </Card>
            </Show>

            <Show when={data() && filteredResources().length === 0}>
                <Card padding="lg">
                    <EmptyState
                        title="No resources found"
                        description={search() ? "No resources match your search." : "No resources are being monitored."}
                    />
                </Card>
            </Show>

            <Show when={data() && filteredResources().length > 0}>
                <For each={groupedResources()}>
                    {(group) => (
                        <Card padding="none" tone="glass" class="overflow-hidden mb-4">
                            <Show when={groupBy() !== 'none'}>
                                <div class="px-4 py-2 bg-gray-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                                    <h3 class="font-medium text-sm text-gray-700 dark:text-gray-300">
                                        {group.label}
                                        <span class="ml-2 text-xs text-gray-500">({group.resources.length})</span>
                                    </h3>
                                </div>
                            </Show>
                            <div class="overflow-x-auto">
                                <table class="w-full border-collapse whitespace-nowrap">
                                    <thead>
                                        <tr class="bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700">
                                            <th class={`${thClass} pl-4`}>Name</th>
                                            <th class={thClass}>Type</th>
                                            <th class={thClass}>Platform</th>
                                            <th class={thClass}>Source</th>
                                            <th class={thClass}>CPU</th>
                                            <th class={thClass}>Memory</th>
                                            <th class={thClass}>Disk</th>
                                            <th class={thClass}>Uptime</th>
                                            <th class={`${thClass} pr-4`}>Last Seen</th>
                                        </tr>
                                    </thead>
                                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                                        <For each={group.resources}>
                                            {(resource) => <ResourceRow resource={resource} />}
                                        </For>
                                    </tbody>
                                </table>
                            </div>
                        </Card>
                    )}
                </For>
            </Show>
        </div>
    );
};

export default ResourcesOverview;
