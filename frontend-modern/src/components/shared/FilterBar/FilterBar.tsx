import { Component, For, JSX, Show, createMemo, createSignal } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { FilterMobileToggleButton } from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { AddFilterMenu } from './AddFilterMenu';
import { FilterChip } from './FilterChip';
import { SavedViewsMenu } from './SavedViewsMenu';
import {
  clearFilter,
  isFilterSet,
  type FilterBarProps,
  type FilterDef,
} from './filterCatalog';

export const FilterBar: Component<FilterBarProps> = (props) => {
  const [mobileExpanded, setMobileExpanded] = createSignal(false);

  const activeFilters = createMemo<FilterDef[]>(() => props.filters.filter(isFilterSet));
  const activeCount = createMemo(() => activeFilters().length);

  const clearAll = () => {
    if (props.onClearAll) {
      props.onClearAll();
      return;
    }
    for (const filter of props.filters) clearFilter(filter);
    props.search.setValue('');
  };

  const hasClearableState = () => {
    if (props.showClearAll) return props.showClearAll() || activeCount() > 0;
    return activeCount() > 0;
  };
  const showDesktopChipRow = () => !props.isMobile() && hasClearableState();
  const showMobileBody = () => props.isMobile() && mobileExpanded();
  const showChipRow = () => showDesktopChipRow() || showMobileBody();

  const searchHistory = () => {
    const key = props.search.historyKey;
    if (!key) return undefined;
    return { storageKey: key, emptyMessage: props.search.emptyMessage };
  };

  return (
    <Card
      padding="sm"
      class="filter-bar mb-4"
      role={props.role as JSX.AriaAttributes['role']}
      aria-label={props.ariaLabel}
    >
      <div class="flex flex-col gap-2">
        <div class="flex w-full items-center gap-2">
          <div class="min-w-0 flex-1">
            <SearchInput
              value={props.search.value}
              onChange={props.search.setValue}
              placeholder={props.search.placeholder}
              class="w-full"
              typeToSearch
              clearOnEscape={props.search.clearOnEscape}
              onBeforeAutoFocus={props.search.onBeforeAutoFocus}
              history={searchHistory()}
            />
          </div>
          {props.searchTrailing}
          <Show when={!props.isMobile()}>
            <AddFilterMenu filters={props.filters} />
            <Show when={props.savedViewsKey}>
              {(key) => <SavedViewsMenu storageKey={key()} />}
            </Show>
            <Show when={props.viewOptionsTrailing}>
              <div
                aria-hidden="true"
                class="hidden h-5 w-px bg-border-subtle sm:block"
              />
              <div class="inline-flex flex-shrink-0 items-center gap-1">
                {props.viewOptionsTrailing}
              </div>
            </Show>
          </Show>
          <Show when={props.isMobile()}>
            <FilterMobileToggleButton
              onClick={() => setMobileExpanded((value) => !value)}
              count={activeCount()}
            />
          </Show>
        </div>

        <Show when={showChipRow()}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={activeFilters()}>{(filter) => <FilterChip filter={filter} />}</For>
            <Show when={showMobileBody()}>
              <AddFilterMenu filters={props.filters} />
            </Show>
            <Show when={hasClearableState()}>
              <button
                type="button"
                onClick={clearAll}
                class="ml-auto text-xs text-muted hover:text-base-content hover:underline underline-offset-2"
              >
                Clear all
              </button>
            </Show>
          </div>
        </Show>

        <Show when={showMobileBody() && props.viewOptionsTrailing}>
          <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle pt-2">
            {props.viewOptionsTrailing}
          </div>
        </Show>
      </div>
    </Card>
  );
};
