import { Component, For, createMemo, createUniqueId } from 'solid-js';
import {
  filterGroupClass,
  filterLabelClass,
  filterSelectClass,
} from '@/components/shared/FilterToolbar';
import {
  isFilterSet,
  type FilterDef,
  type FilterGroupKey,
  type FilterSelectOption,
} from './filterCatalog';

interface AddFilterMenuProps {
  filters: FilterDef[];
}

interface SelectableFilterOption {
  token: string;
  filter: FilterDef;
  option: FilterSelectOption;
}

interface SelectableFilter {
  filter: FilterDef;
  options: SelectableFilterOption[];
}

interface SelectableFilterGroup {
  key: FilterGroupKey;
  filters: SelectableFilter[];
}

const GROUP_LABELS: Record<FilterGroupKey, string> = {
  scope: 'Scope',
  status: 'Status',
  properties: 'Properties',
};

const GROUP_ORDER: FilterGroupKey[] = ['scope', 'status', 'properties'];

const FILTER_VALUE_SEPARATOR = '\u001f';

const optionToken = (filter: FilterDef, option: FilterSelectOption): string =>
  `${filter.id}${FILTER_VALUE_SEPARATOR}${option.value}`;

export const AddFilterMenu: Component<AddFilterMenuProps> = (props) => {
  const selectId = createUniqueId();
  const availableFilters = createMemo(() => props.filters.filter((filter) => !isFilterSet(filter)));

  const selectableGroups = createMemo<SelectableFilterGroup[]>(() => {
    const groups = new Map<FilterGroupKey, SelectableFilter[]>();

    for (const filter of availableFilters()) {
      const options = filter
        .options()
        .filter((option) => option.value !== filter.defaultValue)
        .map((option) => ({
          token: optionToken(filter, option),
          filter,
          option,
        }));

      if (options.length === 0) continue;

      const key: FilterGroupKey = filter.group ?? 'properties';
      const bucket = groups.get(key);
      const selectable = { filter, options };
      if (bucket) {
        bucket.push(selectable);
      } else {
        groups.set(key, [selectable]);
      }
    }

    return GROUP_ORDER.filter((key) => groups.has(key)).map((key) => ({
      key,
      filters: groups.get(key)!,
    }));
  });

  const selectableByToken = createMemo(() => {
    const entries = new Map<string, SelectableFilterOption>();
    for (const group of selectableGroups()) {
      for (const filter of group.filters) {
        for (const option of filter.options) {
          entries.set(option.token, option);
        }
      }
    }
    return entries;
  });

  const isDisabled = () => selectableByToken().size === 0;

  const handleChange = (event: Event) => {
    const select = event.currentTarget as HTMLSelectElement;
    const selected = selectableByToken().get(select.value);
    if (!selected) return;
    selected.filter.setValue(selected.option.value);
    select.value = '';
  };

  return (
    <div class={`${filterGroupClass} flex-shrink-0`}>
      <label for={selectId} class={filterLabelClass}>
        Filter
      </label>
      <select
        id={selectId}
        value=""
        onChange={handleChange}
        disabled={isDisabled()}
        aria-label="Filter"
        class={`${filterSelectClass} min-w-[9rem] disabled:cursor-not-allowed disabled:opacity-50`}
      >
        <option value="">
          {isDisabled() ? 'No filters' : 'Add filter'}
        </option>
        <For each={selectableGroups()}>
          {(group) => (
            <optgroup label={GROUP_LABELS[group.key]}>
              <For each={group.filters}>
                {(filter) => (
                  <For each={filter.options}>
                    {(option) => (
                      <option value={option.token}>
                        {filter.filter.label}: {option.option.label}
                      </option>
                    )}
                  </For>
                )}
              </For>
            </optgroup>
          )}
        </For>
      </select>
    </div>
  );
};
