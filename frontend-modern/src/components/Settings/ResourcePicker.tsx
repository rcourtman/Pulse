import { createSignal, createMemo, For, Show, type Accessor } from 'solid-js';
import X from 'lucide-solid/icons/x';
import CheckSquare from 'lucide-solid/icons/check-square';
import XSquare from 'lucide-solid/icons/x-square';
import { formControl } from '@/components/shared/Form';
import { SearchField } from '@/components/shared/SearchField';
import { StatusDot } from '@/components/shared/StatusDot';
import { useResources, getDisplayName } from '@/hooks/useResources';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getResourcePickerEmptyState,
  getResourcePickerTypeFilterLabel,
  matchesReportableResourceTypeFilter,
  REPORTABLE_RESOURCE_TYPES,
  RESOURCE_PICKER_TYPE_FILTERS,
  reportableResourceTypeSortOrder,
  type ResourcePickerTypeFilter as TypeFilter,
} from '@/utils/reportableResourceTypes';
import { showWarning } from '@/utils/toast';
import { getResourceTypePresentation } from '@/utils/resourceTypePresentation';
import { getSimpleStatusIndicator } from '@/utils/status';

const MAX_SELECTION = 50;

export interface SelectedResource {
  id: string;
  type: ResourceType;
  name: string;
}

interface ResourcePickerProps {
  selected: Accessor<SelectedResource[]>;
  onSelectionChange: (items: SelectedResource[]) => void;
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
    result = result.filter((r) => matchesReportableResourceTypeFilter(r, typeFilter()));

    // Search filter (name or ID)
    const searchTerm = search().toLowerCase().trim();
    if (searchTerm) {
      result = result.filter((r) => {
        const name = getDisplayName(r).toLowerCase();
        const id = r.id.toLowerCase();
        return name.includes(searchTerm) || id.includes(searchTerm);
      });
    }

    // Tag filter
    const tag = tagFilter().toLowerCase().trim();
    if (tag) {
      result = result.filter((r) => r.tags && r.tags.some((t) => t.toLowerCase().includes(tag)));
    }

    // Sort by domain first, then by type, then alphabetical.
    result.sort((a, b) => {
      const aOrder = reportableResourceTypeSortOrder(a.type);
      const bOrder = reportableResourceTypeSortOrder(b.type);
      if (aOrder !== bOrder) return aOrder - bOrder;
      return getDisplayName(a).localeCompare(getDisplayName(b));
    });

    return result;
  });

  const selectedIds = createMemo(() => new Set(props.selected().map((s) => s.id)));

  const isSelected = (id: string) => selectedIds().has(id);

  const toggleResource = (resource: Resource) => {
    const current = props.selected();
    if (isSelected(resource.id)) {
      props.onSelectionChange(current.filter((s) => s.id !== resource.id));
    } else {
      if (current.length >= MAX_SELECTION) {
        showWarning(`Maximum ${MAX_SELECTION} resources can be selected`);
        return;
      }
      props.onSelectionChange([
        ...current,
        {
          id: resource.id,
          type: resource.type,
          name: getDisplayName(resource),
        },
      ]);
    }
  };

  const selectAllVisible = () => {
    const current = props.selected();
    const currentIds = new Set(current.map((s) => s.id));
    const toAdd = filteredResources()
      .filter((r) => !currentIds.has(r.id))
      .map((r) => ({
        id: r.id,
        type: r.type,
        name: getDisplayName(r),
      }));

    const newSelection = [...current, ...toAdd];
    if (newSelection.length > MAX_SELECTION) {
      showWarning(
        `Maximum ${MAX_SELECTION} resources can be selected. Only ${MAX_SELECTION - current.length} more can be added.`,
      );
      props.onSelectionChange([...current, ...toAdd.slice(0, MAX_SELECTION - current.length)]);
      return;
    }
    props.onSelectionChange(newSelection);
  };

  const clearAll = () => {
    props.onSelectionChange([]);
  };

  const removeSelected = (id: string) => {
    props.onSelectionChange(props.selected().filter((s) => s.id !== id));
  };

  return (
    <div class="space-y-3">
      {/* Filter bar */}
      <div class="flex flex-col gap-3">
        <div class="flex gap-2 flex-wrap">
          {/* Search input */}
          <SearchField
            class="flex-1 min-w-[200px]"
            inputClass={formControl}
            value={search()}
            onChange={setSearch}
            placeholder="Search by name or ID..."
          />

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
          <For each={RESOURCE_PICKER_TYPE_FILTERS}>
            {(type) => (
              <button
                class={`min-h-10 sm:min-h-9 min-w-10 px-3 py-2 rounded-md text-sm font-medium transition-all ${
                  typeFilter() === type
                    ? 'bg-blue-600 border border-blue-500 text-blue-400'
                    : 'bg-surface border border-border text-muted hover:border-border'
                }`}
                onClick={() => setTypeFilter(type)}
              >
                {getResourcePickerTypeFilterLabel(type)}
              </button>
            )}
          </For>
        </div>
      </div>

      {/* Resource list */}
      <div class="border border-border rounded-md overflow-hidden">
        <Show
          when={reportableResources().length > 0}
          fallback={
            <div class="p-8 text-center text-slate-400">
              <p class="text-sm">{getResourcePickerEmptyState(false).title}</p>
              <p class="text-xs mt-1 text-slate-500">
                {getResourcePickerEmptyState(false).description}
              </p>
            </div>
          }
        >
          <Show
            when={filteredResources().length > 0}
            fallback={
              <div class="p-6 text-center text-slate-400 text-sm">
                {getResourcePickerEmptyState(true).title}
              </div>
            }
          >
            <div class="max-h-[300px] overflow-y-auto">
              <For each={filteredResources()}>
                {(resource) => {
                  const badge = getResourceTypePresentation(resource.type) || {
                    label: resource.type,
                    badgeClasses: 'bg-slate-500 text-slate-400',
                  };
                  return (
                    <button
                      class={`w-full flex items-start sm:items-center gap-3 px-3 py-2 text-left transition-colors border-b border-border last:border-b-0 ${
                        isSelected(resource.id) ? 'bg-blue-600' : 'hover:bg-slate-800'
                      }`}
                      onClick={() => toggleResource(resource)}
                    >
                      {/* Checkbox */}
                      <div
                        class={`w-4 h-4 rounded border flex-shrink-0 flex items-center justify-center ${
                          isSelected(resource.id)
                            ? 'bg-blue-600 border-blue-600'
                            : 'border-slate-600'
                        }`}
                      >
                        <Show when={isSelected(resource.id)}>
                          <svg
                            class="w-3 h-3 text-white"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            stroke-width="3"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                        </Show>
                      </div>

                      {/* Status dot */}
                      <StatusDot
                        variant={getSimpleStatusIndicator(resource.status).variant}
                        size="sm"
                        ariaHidden
                      />

                      {/* Name and ID */}
                      <div class="flex-1 min-w-0">
                        <div class="text-sm text-white sm:truncate break-words">
                          {getDisplayName(resource)}
                        </div>
                        <div class="text-xs text-slate-500 sm:truncate break-all">
                          {resource.id}
                        </div>
                        <div class="mt-1 flex flex-wrap items-center gap-1 sm:hidden">
                          <span class={`text-xs px-2 py-0.5 rounded-full ${badge.badgeClasses}`}>
                            {badge.label}
                          </span>
                          <Show when={resource.tags && resource.tags.length > 0}>
                            <For each={resource.tags?.slice(0, 2)}>
                              {(tag) => (
                                <span class="text-xs px-1.5 py-0.5 rounded bg-surface-hover text-slate-300">
                                  {tag}
                                </span>
                              )}
                            </For>
                            <Show when={(resource.tags?.length ?? 0) > 2}>
                              <span class="text-xs text-slate-500">
                                +{(resource.tags?.length ?? 0) - 2}
                              </span>
                            </Show>
                          </Show>
                        </div>
                      </div>

                      {/* Type badge */}
                      <span
                        class={`hidden sm:inline-flex text-xs px-2 py-0.5 rounded-full flex-shrink-0 ${badge.badgeClasses}`}
                      >
                        {badge.label}
                      </span>

                      {/* Tags */}
                      <Show when={resource.tags && resource.tags.length > 0}>
                        <div class="hidden sm:flex gap-1 flex-shrink-0">
                          <For each={resource.tags?.slice(0, 2)}>
                            {(tag) => (
                              <span class="text-xs px-1.5 py-0.5 rounded bg-surface-hover text-slate-300">
                                {tag}
                              </span>
                            )}
                          </For>
                          <Show when={(resource.tags?.length ?? 0) > 2}>
                            <span class="text-xs text-slate-500">
                              +{(resource.tags?.length ?? 0) - 2}
                            </span>
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
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
        <div class="flex flex-col sm:flex-row gap-2">
          <button
            class="w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-1.5 px-3 py-2.5 text-sm rounded-md border border-border text-slate-400 hover:border-slate-500 hover:text-slate-300 transition-colors"
            onClick={selectAllVisible}
          >
            <CheckSquare size={14} />
            Select all visible ({filteredResources().length})
          </button>
          <Show when={props.selected().length > 0}>
            <button
              class="w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-1.5 px-3 py-2.5 text-sm rounded-md border border-border text-slate-400 hover:border-red-500 hover:text-red-400 transition-colors"
              onClick={clearAll}
            >
              <XSquare size={14} />
              Clear all
            </button>
          </Show>
        </div>
        <span class="text-xs sm:text-sm text-slate-500">
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
              <span class="inline-flex items-center gap-1 pl-2 pr-1 py-1 rounded-md bg-blue-600 border border-blue-500 text-sm text-blue-300">
                {item.name}
                <button
                  class="p-0.5 rounded hover:bg-blue-500 transition-colors"
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
