import { createSignal, createMemo, For, Show, type Accessor } from 'solid-js';
import Search from 'lucide-solid/icons/search';
import X from 'lucide-solid/icons/x';
import CheckSquare from 'lucide-solid/icons/check-square';
import XSquare from 'lucide-solid/icons/x-square';
import { formControl } from '@/components/shared/Form';
import { useResources, getDisplayName } from '@/hooks/useResources';
import type { Resource, ResourceType } from '@/types/resource';
import { showWarning } from '@/utils/toast';

const MAX_SELECTION = 50;

export interface SelectedResource {
    id: string;
    type: string;
    name: string;
}

interface ResourcePickerProps {
    selected: Accessor<SelectedResource[]>;
    onSelectionChange: (items: SelectedResource[]) => void;
}

type TypeFilter = 'all' | 'infrastructure' | 'workloads' | 'storage' | 'recovery';

const typeFilterLabels: Record<TypeFilter, string> = {
    all: 'All',
    infrastructure: 'Infrastructure',
    workloads: 'Workloads',
    storage: 'Storage',
    recovery: 'Recovery',
};

const REPORTABLE_RESOURCE_TYPES = new Set<ResourceType>([
    'node',
    'host',
    'docker-host',
    'k8s-cluster',
    'k8s-node',
    'vm',
    'container',
    'oci-container',
    'docker-container',
    'pod',
    'storage',
    'datastore',
    'pool',
    'dataset',
    'pbs',
    'pmg',
]);

const INFRASTRUCTURE_TYPES = new Set<ResourceType>([
    'node',
    'host',
    'docker-host',
    'k8s-cluster',
    'k8s-node',
    'pbs',
    'pmg',
]);

const WORKLOAD_TYPES = new Set<ResourceType>([
    'vm',
    'container',
    'oci-container',
    'docker-container',
    'pod',
]);

const STORAGE_TYPES = new Set<ResourceType>([
    'storage',
    'datastore',
    'pool',
    'dataset',
]);

const RECOVERY_TYPES = new Set<ResourceType>([
    'pbs',
    'datastore',
]);

function normalizeType(type: ResourceType): string {
    if (type === 'oci-container' || type === 'docker-container') return 'container';
    if (type === 'docker-host') return 'host';
    if (type === 'k8s-node') return 'node';
    if (type === 'k8s-cluster') return 'cluster';
    return type;
}

function toReportResourceType(type: ResourceType): string {
    if (type === 'oci-container' || type === 'docker-container') return 'container';
    if (type === 'docker-host') return 'host';
    return type;
}

function matchesTypeFilter(resource: Resource, filter: TypeFilter): boolean {
    if (filter === 'all') return true;
    if (filter === 'infrastructure') return INFRASTRUCTURE_TYPES.has(resource.type);
    if (filter === 'workloads') return WORKLOAD_TYPES.has(resource.type);
    if (filter === 'storage') return STORAGE_TYPES.has(resource.type);
    if (filter === 'recovery') return RECOVERY_TYPES.has(resource.type);
    return true;
}

function getStatusColor(status: string): string {
    switch (status) {
        case 'online':
        case 'running':
            return 'bg-emerald-400';
        case 'offline':
        case 'stopped':
            return 'bg-slate-500';
        case 'degraded':
            return 'bg-amber-400';
        case 'paused':
            return 'bg-blue-400';
        default:
            return 'bg-slate-500';
    }
}

function getTypeBadge(type: ResourceType): { label: string; classes: string } {
    switch (type) {
        case 'node':
        case 'k8s-node':
            return { label: 'Node', classes: 'bg-blue-500/20 text-blue-300' };
        case 'host':
        case 'docker-host':
            return { label: 'Host', classes: 'bg-gray-500/20 text-gray-300' };
        case 'k8s-cluster':
            return { label: 'K8s', classes: 'bg-gray-500/20 text-gray-300' };
        case 'vm':
            return { label: 'VM', classes: 'bg-gray-500/20 text-gray-300' };
        case 'container':
        case 'oci-container':
        case 'docker-container':
            return { label: 'Container', classes: 'bg-blue-500/20 text-blue-300' };
        case 'pod':
            return { label: 'Pod', classes: 'bg-gray-500/20 text-gray-300' };
        case 'pbs':
            return { label: 'PBS', classes: 'bg-gray-500/20 text-gray-300' };
        case 'pmg':
            return { label: 'PMG', classes: 'bg-gray-500/20 text-gray-300' };
        case 'datastore':
            return { label: 'Datastore', classes: 'bg-gray-500/20 text-gray-300' };
        case 'storage':
        case 'pool':
        case 'dataset':
            return { label: 'Storage', classes: 'bg-gray-500/20 text-gray-300' };
        default:
            return { label: type, classes: 'bg-slate-500/20 text-slate-400' };
    }
}

export function ResourcePicker(props: ResourcePickerProps) {
    const { resources } = useResources();
    const [search, setSearch] = createSignal('');
    const [typeFilter, setTypeFilter] = createSignal<TypeFilter>('all');
    const [tagFilter, setTagFilter] = createSignal('');

    // Filter to reportable resource types across infrastructure, workloads, storage, and recovery.
    const reportableResources = createMemo(() => {
        return resources().filter((r) => REPORTABLE_RESOURCE_TYPES.has(r.type));
    });

    // Apply filters
    const filteredResources = createMemo(() => {
        let result = reportableResources();

        // Type filter
        result = result.filter(r => matchesTypeFilter(r, typeFilter()));

        // Search filter (name or ID)
        const searchTerm = search().toLowerCase().trim();
        if (searchTerm) {
            result = result.filter(r => {
                const name = getDisplayName(r).toLowerCase();
                const id = r.id.toLowerCase();
                return name.includes(searchTerm) || id.includes(searchTerm);
            });
        }

        // Tag filter
        const tag = tagFilter().toLowerCase().trim();
        if (tag) {
            result = result.filter(r =>
                r.tags && r.tags.some(t => t.toLowerCase().includes(tag))
            );
        }

        // Sort by domain first, then by type, then alphabetical.
        result.sort((a, b) => {
            const typeOrder: Record<string, number> = {
                node: 0,
                host: 1,
                cluster: 2,
                pbs: 3,
                pmg: 4,
                vm: 5,
                container: 6,
                pod: 7,
                storage: 8,
                datastore: 9,
                pool: 10,
                dataset: 11,
            };
            const aOrder = typeOrder[normalizeType(a.type)] ?? 12;
            const bOrder = typeOrder[normalizeType(b.type)] ?? 12;
            if (aOrder !== bOrder) return aOrder - bOrder;
            return getDisplayName(a).localeCompare(getDisplayName(b));
        });

        return result;
    });

    const selectedIds = createMemo(() => new Set(props.selected().map(s => s.id)));

    const isSelected = (id: string) => selectedIds().has(id);

    const toggleResource = (resource: Resource) => {
        const current = props.selected();
        if (isSelected(resource.id)) {
            props.onSelectionChange(current.filter(s => s.id !== resource.id));
        } else {
            if (current.length >= MAX_SELECTION) {
                showWarning(`Maximum ${MAX_SELECTION} resources can be selected`);
                return;
            }
            props.onSelectionChange([...current, {
                id: resource.id,
                type: toReportResourceType(resource.type),
                name: getDisplayName(resource),
            }]);
        }
    };

    const selectAllVisible = () => {
        const current = props.selected();
        const currentIds = new Set(current.map(s => s.id));
        const toAdd = filteredResources()
            .filter(r => !currentIds.has(r.id))
            .map(r => ({
                id: r.id,
                type: toReportResourceType(r.type),
                name: getDisplayName(r),
            }));

        const newSelection = [...current, ...toAdd];
        if (newSelection.length > MAX_SELECTION) {
            showWarning(`Maximum ${MAX_SELECTION} resources can be selected. Only ${MAX_SELECTION - current.length} more can be added.`);
            props.onSelectionChange([...current, ...toAdd.slice(0, MAX_SELECTION - current.length)]);
            return;
        }
        props.onSelectionChange(newSelection);
    };

    const clearAll = () => {
        props.onSelectionChange([]);
    };

    const removeSelected = (id: string) => {
        props.onSelectionChange(props.selected().filter(s => s.id !== id));
    };

    return (
        <div class="space-y-3">
            {/* Filter bar */}
            <div class="flex flex-col gap-3">
                <div class="flex gap-2 flex-wrap">
                    {/* Search input */}
                    <div class="relative flex-1 min-w-[200px]">
                        <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                        <input
                            type="text"
                            class={`${formControl} pl-9`}
                            placeholder="Search by name or ID..."
                            value={search()}
                            onInput={(e) => setSearch(e.currentTarget.value)}
                        />
                    </div>

                    {/* Tag filter */}
                    <input
                        type="text"
                        class={`${formControl} w-40`}
                        placeholder="Filter by tag..."
                        value={tagFilter()}
                        onInput={(e) => setTagFilter(e.currentTarget.value)}
                    />
                </div>

                {/* Type toggle buttons */}
                <div class="flex gap-1">
                    <For each={(['all', 'infrastructure', 'workloads', 'storage', 'recovery'] as TypeFilter[])}>
                        {(type) => (
                            <button
                                class={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${
                                    typeFilter() === type
                                        ? 'bg-blue-600/20 border border-blue-500 text-blue-400'
                                        : 'bg-slate-800/50 border border-slate-700 text-slate-400 hover:border-slate-500'
                                }`}
                                onClick={() => setTypeFilter(type)}
                            >
                                {typeFilterLabels[type]}
                            </button>
                        )}
                    </For>
                </div>
            </div>

            {/* Resource list */}
            <div class="border border-slate-700 rounded-lg overflow-hidden">
                <Show
                    when={reportableResources().length > 0}
                    fallback={
                        <div class="p-8 text-center text-slate-400">
                            <p class="text-sm">No resources available</p>
                            <p class="text-xs mt-1 text-slate-500">Resources appear as Pulse collects infrastructure and workload metrics</p>
                        </div>
                    }
                >
                    <Show
                        when={filteredResources().length > 0}
                        fallback={
                            <div class="p-6 text-center text-slate-400 text-sm">
                                No resources match your filters
                            </div>
                        }
                    >
                        <div class="max-h-[300px] overflow-y-auto">
                            <For each={filteredResources()}>
                                {(resource) => {
                                    const badge = getTypeBadge(resource.type);
                                    return (
                                        <button
                                            class={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors border-b border-slate-800 last:border-b-0 ${
                                                isSelected(resource.id)
                                                    ? 'bg-blue-600/10'
                                                    : 'hover:bg-slate-800/50'
                                            }`}
                                            onClick={() => toggleResource(resource)}
                                        >
                                            {/* Checkbox */}
                                            <div class={`w-4 h-4 rounded border flex-shrink-0 flex items-center justify-center ${
                                                isSelected(resource.id)
                                                    ? 'bg-blue-600 border-blue-600'
                                                    : 'border-slate-600'
                                            }`}>
                                                <Show when={isSelected(resource.id)}>
                                                    <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                                                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                                                    </svg>
                                                </Show>
                                            </div>

                                            {/* Status dot */}
                                            <div class={`w-2 h-2 rounded-full flex-shrink-0 ${getStatusColor(resource.status)}`} />

                                            {/* Name and ID */}
                                            <div class="flex-1 min-w-0">
                                                <div class="text-sm text-white truncate">{getDisplayName(resource)}</div>
                                                <div class="text-xs text-slate-500 truncate">{resource.id}</div>
                                            </div>

                                            {/* Type badge */}
                                            <span class={`text-xs px-2 py-0.5 rounded-full flex-shrink-0 ${badge.classes}`}>
                                                {badge.label}
                                            </span>

                                            {/* Tags */}
                                            <Show when={resource.tags && resource.tags.length > 0}>
                                                <div class="flex gap-1 flex-shrink-0">
                                                    <For each={resource.tags?.slice(0, 2)}>
                                                        {(tag) => (
                                                            <span class="text-xs px-1.5 py-0.5 rounded bg-slate-700 text-slate-300">
                                                                {tag}
                                                            </span>
                                                        )}
                                                    </For>
                                                    <Show when={(resource.tags?.length ?? 0) > 2}>
                                                        <span class="text-xs text-slate-500">+{(resource.tags?.length ?? 0) - 2}</span>
                                                    </Show>
                                                </div>
                                            </Show>
                                        </button>
                                    );
                                }}
                            </For>
                        </div>
                    </Show>
                </Show>
            </div>

            {/* Action bar */}
            <div class="flex items-center justify-between">
                <div class="flex gap-2">
                    <button
                        class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-slate-700 text-slate-400 hover:border-slate-500 hover:text-slate-300 transition-colors"
                        onClick={selectAllVisible}
                    >
                        <CheckSquare size={14} />
                        Select all visible ({filteredResources().length})
                    </button>
                    <Show when={props.selected().length > 0}>
                        <button
                            class="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-slate-700 text-slate-400 hover:border-red-500/50 hover:text-red-400 transition-colors"
                            onClick={clearAll}
                        >
                            <XSquare size={14} />
                            Clear all
                        </button>
                    </Show>
                </div>
                <span class="text-xs text-slate-500">
                    {props.selected().length} selected
                    <Show when={props.selected().length >= MAX_SELECTION}>
                        <span class="text-amber-400 ml-1">(max)</span>
                    </Show>
                </span>
            </div>

            {/* Selected summary chips */}
            <Show when={props.selected().length > 0}>
                <div class="flex flex-wrap gap-1.5">
                    <For each={props.selected()}>
                        {(item) => (
                            <span class="inline-flex items-center gap-1 pl-2 pr-1 py-1 rounded-lg bg-blue-600/10 border border-blue-500/30 text-sm text-blue-300">
                                {item.name}
                                <button
                                    class="p-0.5 rounded hover:bg-blue-500/20 transition-colors"
                                    onClick={() => removeSelected(item.id)}
                                >
                                    <X size={12} />
                                </button>
                            </span>
                        )}
                    </For>
                </div>
            </Show>
        </div>
    );
}
