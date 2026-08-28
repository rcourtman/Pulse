import { Component, For, JSX, Show, createEffect, createMemo, createSignal } from 'solid-js';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import { Card } from '@/components/shared/Card';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { FilterActionButton, FilterMobileToggleButton } from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { AddFilterMenu } from './AddFilterMenu';
import { FilterChip } from './FilterChip';
import {
  ViewOptionsDisclosurePanel,
  ViewOptionsDisclosureTrigger,
  createViewOptionsDisclosureState,
} from './ViewOptionsDisclosure';
import {
  buildFilterSearchSuggestions,
  clearFilter,
  hasAddableFilterOptions,
  getSearchSuggestionValues,
  isFilterSet,
  serializeFilterSearch,
  toFilterSearchTerm,
  type FilterBarProps,
  type FilterDef,
  type FilterSearchTerm,
} from './filterCatalog';

const FilterBarClearAllButton: Component<{ onClick: () => void }> = (props) => (
  <FilterActionButton
    onClick={props.onClick}
    aria-label="Clear filters"
    title="Clear filters"
    class="text-blue-600 dark:text-blue-400"
  >
    <RotateCcwIcon class="h-3 w-3" />
    Clear filters
  </FilterActionButton>
);

const InlineFilterControl: Component<{ filter: FilterDef }> = (props) => (
  <div class="inline-flex min-w-0 items-center">
    <FilterButtonGroup
      ariaLabel={props.filter.label}
      variant="compact"
      options={props.filter.options().map((option) => ({
        value: option.value,
        label: option.label,
        ariaLabel: option.ariaLabel,
        compactLabel: option.compactLabel,
        leading: option.leading,
        visualLabel: option.visualLabel,
        icon: option.icon,
        tone: option.tone,
        title: option.title,
        count: option.count,
      }))}
      value={props.filter.value()}
      onChange={props.filter.setValue}
    />
  </div>
);

const FilterBarRailDivider: Component = () => (
  <div aria-hidden="true" class="hidden h-6 w-px bg-border-subtle sm:block" />
);

const FilterSearchTermChip: Component<{
  term: FilterSearchTerm;
  onRemove: () => void;
}> = (props) => (
  <div class="inline-flex items-center rounded-full border border-blue-200 bg-blue-50 text-xs dark:border-blue-900 dark:bg-blue-950/40">
    <span class="py-0.5 pl-2 pr-1 font-medium text-base-content">{props.term.label}</span>
    <button
      type="button"
      onClick={props.onRemove}
      aria-label={`Remove search term ${props.term.label}`}
      class="rounded-r-full py-0.5 pl-1 pr-1.5 text-muted hover:bg-blue-100 hover:text-base-content dark:hover:bg-blue-900/50"
    >
      <span aria-hidden="true">×</span>
    </button>
  </div>
);

export const FilterBar: Component<FilterBarProps> = (props) => {
  const [mobileExpanded, setMobileExpanded] = createSignal(false);
  const [searchDraft, setSearchDraft] = createSignal('');
  const [searchTerms, setSearchTerms] = createSignal<FilterSearchTerm[]>([]);
  const viewOptionsDisclosure = createViewOptionsDisclosureState();
  let lastAppliedSearch: string | undefined;

  const inlineFilters = createMemo<FilterDef[]>(() =>
    props.filters.filter((filter) => filter.inline),
  );
  const menuFilters = createMemo<FilterDef[]>(() =>
    props.filters.filter((filter) => !filter.inline),
  );
  const activeMenuFilters = createMemo<FilterDef[]>(() => menuFilters().filter(isFilterSet));
  const activeCount = createMemo(
    () =>
      props.filters.filter(isFilterSet).length +
      searchTerms().length +
      (searchDraft().trim() ? 1 : 0),
  );
  const hasMenuFilters = createMemo(() => menuFilters().length > 0);
  const hasAddableMenuFilters = createMemo(() => hasAddableFilterOptions(menuFilters()));
  const applySearch = (terms: readonly FilterSearchTerm[], draft: string) => {
    const serialized = serializeFilterSearch(terms, draft);
    lastAppliedSearch = serialized;
    props.search.setValue(serialized);
  };
  const addSearchTerm = (term: FilterSearchTerm) => {
    const next = searchTerms().some(
      (current) => current.value.toLowerCase() === term.value.toLowerCase(),
    )
      ? searchTerms()
      : [...searchTerms(), term];
    setSearchTerms(next);
    setSearchDraft('');
    applySearch(next, '');
  };
  const commitRecognizedSearchQuery = (value: string) =>
    addSearchTerm({
      id: `search-query:${value.toLowerCase()}`,
      label: value,
      value,
    });
  const removeSearchTerm = (id: string) => {
    const next = searchTerms().filter((term) => term.id !== id);
    setSearchTerms(next);
    applySearch(next, searchDraft());
  };
  const infrastructureSuggestions = createMemo(() =>
    (props.search.suggestions?.() ?? []).map((suggestion) => ({
      ...suggestion,
      onSelect: () => addSearchTerm(toFilterSearchTerm(suggestion)),
    })),
  );
  const searchSuggestions = createMemo(() => [
    ...infrastructureSuggestions(),
    ...buildFilterSearchSuggestions(props.filters),
  ]);

  createEffect(() => {
    const externalSearch = props.search.value();
    const suggestions = props.search.suggestions?.() ?? [];
    if (externalSearch === lastAppliedSearch) return;

    const parts = externalSearch
      .split(',')
      .map((part) => part.trim())
      .filter(Boolean);
    const matched = parts.map((part) =>
      suggestions.find((suggestion) =>
        getSearchSuggestionValues(suggestion).some(
          (value) => value.toLowerCase() === part.toLowerCase(),
        ),
      ),
    );
    if (parts.length > 0 && matched.every(Boolean)) {
      setSearchTerms(matched.map((suggestion) => toFilterSearchTerm(suggestion!)));
      setSearchDraft('');
    } else {
      setSearchTerms([]);
      setSearchDraft(externalSearch);
    }
    if (externalSearch || suggestions.length > 0) lastAppliedSearch = externalSearch;
  });

  const clearAll = () => {
    setSearchTerms([]);
    setSearchDraft('');
    lastAppliedSearch = '';
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
  const hasAuxiliaryControls = () =>
    Boolean(props.leadingControls || props.viewOptions || props.trailingControls);
  const showDesktopControlsRow = () =>
    !props.isMobile() &&
    (inlineFilters().length > 0 ||
      hasMenuFilters() ||
      hasAuxiliaryControls() ||
      hasClearableState());
  const showInlineRow = () => props.isMobile() && mobileExpanded() && inlineFilters().length > 0;
  const showDesktopChipRow = () =>
    !props.isMobile() && (activeMenuFilters().length > 0 || searchTerms().length > 0);
  const showMobileBody = () => props.isMobile() && mobileExpanded();
  const showMobileActionRow = () => showMobileBody() && hasAuxiliaryControls();
  const showAddFilterInMobileActionRow = () =>
    showMobileActionRow() &&
    activeMenuFilters().length === 0 &&
    hasAddableMenuFilters() &&
    !hasClearableState() &&
    !props.leadingControls;
  const showChipRow = () =>
    showDesktopChipRow() ||
    (showMobileBody() &&
      (activeMenuFilters().length > 0 ||
        searchTerms().length > 0 ||
        (hasAddableMenuFilters() && !showAddFilterInMobileActionRow())));
  const showClearAllInMobileActionRow = () =>
    showMobileActionRow() && hasClearableState() && !showChipRow();

  const searchHistory = () => {
    const key = props.search.historyKey;
    if (!key) return undefined;
    return { storageKey: key, emptyMessage: props.search.emptyMessage };
  };
  const toggleMobileExpanded = () => {
    const next = !mobileExpanded();
    if (!next) viewOptionsDisclosure.close();
    setMobileExpanded(next);
  };

  return (
    <Card
      padding="sm"
      class="filter-bar mb-2 sm:mb-4"
      role={props.role as JSX.AriaAttributes['role']}
      aria-label={props.ariaLabel}
    >
      <div class="flex flex-col gap-2">
        <div class="flex w-full items-center gap-2">
          <div class="min-w-0 flex-1">
            <SearchInput
              value={searchDraft}
              onChange={(value) => {
                setSearchDraft(value);
                applySearch(searchTerms(), value);
              }}
              placeholder={props.search.placeholder}
              class="w-full"
              typeToSearch
              clearOnEscape={props.search.clearOnEscape}
              onBeforeAutoFocus={props.search.onBeforeAutoFocus}
              history={searchHistory()}
              tips={props.search.tips}
              suggestions={{
                items: searchSuggestions,
                onCommitQuery: commitRecognizedSearchQuery,
              }}
            />
          </div>
          {props.searchTrailing}
          <Show when={props.isMobile()}>
            <FilterMobileToggleButton onClick={toggleMobileExpanded} count={activeCount()} />
          </Show>
        </div>

        <Show when={props.isMobile() && !mobileExpanded() && props.leadingControls}>
          <div class="flex min-w-0 flex-wrap items-center gap-2 pt-1">{props.leadingControls}</div>
        </Show>

        <Show when={showDesktopControlsRow()}>
          <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
            <Show when={inlineFilters().length > 0}>
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <For each={inlineFilters()}>
                  {(filter, index) => (
                    <>
                      <InlineFilterControl filter={filter} />
                      <Show when={index() < inlineFilters().length - 1}>
                        <FilterBarRailDivider />
                      </Show>
                    </>
                  )}
                </For>
              </div>
            </Show>
            <Show when={hasAddableMenuFilters() || hasClearableState() || hasAuxiliaryControls()}>
              <div
                class="inline-flex max-w-full flex-shrink-0 flex-wrap items-center justify-start gap-2"
                data-filter-action-cluster
              >
                <Show when={hasAddableMenuFilters()}>
                  <AddFilterMenu
                    filters={menuFilters()}
                    showLabel={props.showAddFilterLabel === true}
                  />
                </Show>
                <Show when={hasClearableState()}>
                  <FilterBarClearAllButton onClick={clearAll} />
                </Show>
                {props.leadingControls}
                <Show when={props.viewOptions}>
                  <ViewOptionsDisclosureTrigger state={viewOptionsDisclosure} />
                </Show>
                {props.trailingControls}
              </div>
            </Show>
          </div>
        </Show>

        <Show when={showInlineRow()}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={inlineFilters()}>{(filter) => <InlineFilterControl filter={filter} />}</For>
            <Show when={hasClearableState() && !showChipRow() && !showClearAllInMobileActionRow()}>
              <FilterBarClearAllButton onClick={clearAll} />
            </Show>
          </div>
        </Show>

        <Show when={showChipRow()}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={searchTerms()}>
              {(term) => (
                <FilterSearchTermChip term={term} onRemove={() => removeSearchTerm(term.id)} />
              )}
            </For>
            <For each={activeMenuFilters()}>{(filter) => <FilterChip filter={filter} />}</For>
            <Show when={showMobileBody()}>
              <Show when={hasAddableMenuFilters()}>
                <AddFilterMenu
                  filters={menuFilters()}
                  showLabel={props.showAddFilterLabel === true}
                />
              </Show>
            </Show>
            <Show when={hasClearableState() && showMobileBody()}>
              <FilterBarClearAllButton onClick={clearAll} />
            </Show>
          </div>
        </Show>

        <Show when={showMobileActionRow()}>
          <div
            class="relative flex flex-wrap items-center gap-2 border-t border-border-subtle pt-2"
            role="group"
            aria-label="Filter actions"
          >
            <div class="flex shrink-0 flex-nowrap items-center gap-2" data-filter-action-cluster>
              <Show when={showAddFilterInMobileActionRow()}>
                <AddFilterMenu
                  filters={menuFilters()}
                  showLabel={props.showAddFilterLabel === true}
                />
              </Show>
              <Show when={showClearAllInMobileActionRow()}>
                <FilterBarClearAllButton onClick={clearAll} />
              </Show>
              {props.leadingControls}
              <Show when={props.viewOptions}>
                <ViewOptionsDisclosureTrigger state={viewOptionsDisclosure} />
              </Show>
            </div>
            <Show when={props.trailingControls}>
              <div class="ml-auto flex min-w-0 flex-wrap items-center justify-end gap-2">
                {props.trailingControls}
              </div>
            </Show>
          </div>
        </Show>
        <Show when={props.viewOptions}>
          <ViewOptionsDisclosurePanel state={viewOptionsDisclosure}>
            {props.viewOptions}
          </ViewOptionsDisclosurePanel>
        </Show>
      </div>
    </Card>
  );
};
