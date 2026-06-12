import { createSignal, createMemo, For, Show, type Accessor } from 'solid-js';
import X from 'lucide-solid/icons/x';
import CheckSquare from 'lucide-solid/icons/check-square';
import XSquare from 'lucide-solid/icons/x-square';
import { formControl } from '@/components/shared/Form';
import { SearchField } from '@/components/shared/SearchField';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { Button } from '@/components/shared/Button';
import { StatusDot } from '@/components/shared/StatusDot';
import { useResources } from '@/hooks/useResources';
import type { Resource, ResourceType } from '@/types/resource';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
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

const DEFAULT_MAX_SELECTION = 50;

export interface SelectedResource {
  id: string;
  type: ResourceType;
  name: string;
}

interface ResourcePickerProps {
  maxSelection?: number;
  selected: Accessor<SelectedResource[]>;
  onSelectionChange: (items: SelectedResource[]) => void;
}

export function ResourcePicker(props: ResourcePickerProps) {
  const { resources } = useResources();
  const [search, setSearch] = createSignal('');
  const [typeFilter, setTypeFilter] = createSignal<TypeFilter>('all');
  const [tagFilter, setTagFilter] = createSignal('');
  const maxSelection = () => props.maxSelection ?? DEFAULT_MAX_SELECTION;
  const typeFilterOptions = createMemo<FilterOption<TypeFilter>[]>(() =>
    RESOURCE_PICKER_TYPE_FILTERS.map((type) => ({
      value: type,
      label: getResourcePickerTypeFilterLabel(type),
    })),
  );

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
        const name = getPreferredInfrastructureDisplayName(r).toLowerCase();
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
      return getPreferredInfrastructureDisplayName(a).localeCompare(
        getPreferredInfrastructureDisplayName(b),
      );
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
      if (current.length >= maxSelection()) {
        showWarning(`Maximum ${maxSelection()} resources can be selected`);
        return;
      }
      props.onSelectionChange([
        ...current,
        {
          id: resource.id,
          type: resource.type,
          name: getPreferredInfrastructureDisplayName(resource),
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
        name: getPreferredInfrastructureDisplayName(r),
      }));

    const newSelection = [...current, ...toAdd];
    if (newSelection.length > maxSelection()) {
      showWarning(
        `Maximum ${maxSelection()} resources can be selected. Only ${maxSelection() - current.length} more can be added.`,
      );
      props.onSelectionChange([...current, ...toAdd.slice(0, maxSelection() - current.length)]);
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
        <div class="flex flex-wrap gap-2">
          <SearchField
            class="min-w-[200px] flex-1"
            inputClass={formControl}
            value={search()}
            onChange={setSearch}
            placeholder="Search by name or ID..."
          />

          <SearchField
            class="w-full sm:w-48"
            inputClass={formControl}
            placeholder="Filter by tag..."
            title="Filter resources by tag"
            value={tagFilter()}
            onChange={setTagFilter}
          />
        </div>

        <FilterButtonGroup
          options={typeFilterOptions()}
          value={typeFilter()}
          onChange={setTypeFilter}
          ariaLabel="Resource type filter"
          variant="settings"
        />
      </div>

      {/* Resource list */}
      <div class="border border-border rounded-md overflow-hidden">
        <Show
          when={reportableResources().length > 0}
          fallback={
            <div class="p-8 text-center text-slate-400">
              <p class="text-sm">{getResourcePickerEmptyState(false).title}</p>
              <p class="text-xs mt-1 text-muted">
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
                        <div
                          class="text-sm text-white sm:truncate break-words"
                          title={getPreferredInfrastructureDisplayName(resource)}
                        >
                          {getPreferredInfrastructureDisplayName(resource)}
                        </div>
                        <div class="text-xs text-muted sm:truncate break-all" title={resource.id}>
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
                              <span class="text-xs text-muted">
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
                            <span class="text-xs text-muted">
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
          <Button
            variant="outline"
            size="settingsAction"
            class="w-full gap-1.5 text-muted sm:w-auto"
            onClick={selectAllVisible}
          >
            <CheckSquare size={14} />
            Select all visible ({filteredResources().length})
          </Button>
          <Show when={props.selected().length > 0}>
            <Button
              variant="dangerOutline"
              size="settingsAction"
              class="w-full gap-1.5 sm:w-auto"
              onClick={clearAll}
            >
              <XSquare size={14} />
              Clear all
            </Button>
          </Show>
        </div>
        <span class="text-xs sm:text-sm text-muted">
          {props.selected().length} selected
          <Show when={props.selected().length >= maxSelection()}>
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
                <Button
                  variant="ghost"
                  size="xs"
                  class="min-h-0 p-0.5 text-blue-300 hover:bg-blue-500"
                  onClick={() => removeSelected(item.id)}
                  aria-label={`Remove ${item.name}`}
                  title={`Remove ${item.name}`}
                >
                  <X size={12} />
                </Button>
              </span>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
