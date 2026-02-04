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

type TypeFilter = 'all' | 'node' | 'vm' | 'container';

const typeFilterLabels: Record<TypeFilter, string> = {
    all: 'All',
    node: 'Nodes',
    vm: 'VMs',
    container: 'Containers',
};

function normalizeType(type: ResourceType): string {
    if (type === 'oci-container') return 'container';
    return type;
}

function matchesTypeFilter(resource: Resource, filter: TypeFilter): boolean {
    if (filter === 'all') return true;
    const normalized = normalizeType(resource.type);
    return normalized === filter;
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
    const normalized = normalizeType(type);
    switch (normalized) {
        case 'node':
            return { label: 'Node', classes: 'bg-blue-500/20 text-blue-400' };
        case 'vm':
            return { label: 'VM', classes: 'bg-purple-500/20 text-purple-400' };
        case 'container':
            return { label: 'CT', classes: 'bg-emerald-500/20 text-emerald-400' };
        default:
            return { label: type, classes: 'bg-slate-500/20 text-slate-400' };
    }
}

export function ResourcePicker(props: ResourcePickerProps) {
    const { resources } = useResources();
    const [search, setSearch] = createSignal('');
    const [typeFilter, setTypeFilter] = createSignal<TypeFilter>('all');
    const [tagFilter, setTagFilter] = createSignal('');

    // Filter to only reportable resource types (nodes, VMs, containers)
    const reportableResources = createMemo(() => {
        return resources().filter(r =>
            r.type === 'node' || r.type === 'vm' || r.type === 'container' || r.type === 'oci-container'
        );
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

        // Sort: nodes first, then VMs, then containers, then alphabetical
        result.sort((a, b) => {
            const typeOrder: Record<string, number> = { node: 0, vm: 1, container: 2 };
            const aOrder = typeOrder[normalizeType(a.type)] ?? 3;
            const bOrder = typeOrder[normalizeType(b.type)] ?? 3;
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
                type: normalizeType(resource.type),
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
                type: normalizeType(r.type),
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
                    <For each={(['all', 'node', 'vm', 'container'] as TypeFilter[])}>
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
                            <p class="text-xs mt-1 text-slate-500">Resources will appear when connected to a Proxmox instance</p>
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
