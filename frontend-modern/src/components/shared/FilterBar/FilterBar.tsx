import { Component, For, JSX, Show, createMemo, createSignal } from 'solid-js';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import { Card } from '@/components/shared/Card';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { FilterActionButton, FilterMobileToggleButton } from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { AddFilterMenu } from './AddFilterMenu';
import { FilterChip } from './FilterChip';
import { SavedViewsMenu } from './SavedViewsMenu';
import { clearFilter, isFilterSet, type FilterBarProps, type FilterDef } from './filterCatalog';

const FilterBarClearAllButton: Component<{ onClick: () => void }> = (props) => (
  <FilterActionButton
    onClick={props.onClick}
    aria-label="Clear all"
    title="Reset filters"
    class="text-blue-600 dark:text-blue-400"
  >
    <RotateCcwIcon class="h-3 w-3" />
    Reset
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
      }))}
      value={props.filter.value()}
      onChange={props.filter.setValue}
    />
  </div>
);

const FilterBarRailDivider: Component = () => (
  <div aria-hidden="true" class="hidden h-6 w-px bg-border-subtle sm:block" />
);

export const FilterBar: Component<FilterBarProps> = (props) => {
  const [mobileExpanded, setMobileExpanded] = createSignal(false);

  const inlineFilters = createMemo<FilterDef[]>(() =>
    props.filters.filter((filter) => filter.inline),
  );
  const menuFilters = createMemo<FilterDef[]>(() =>
    props.filters.filter((filter) => !filter.inline),
  );
  const activeMenuFilters = createMemo<FilterDef[]>(() => menuFilters().filter(isFilterSet));
  const activeCount = createMemo(() => props.filters.filter(isFilterSet).length);
  const hasMenuFilters = createMemo(() => menuFilters().length > 0);

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
  const hasDesktopViewOptions = () => Boolean(props.viewOptionsTrailing);
  const hasSavedViews = () => Boolean(props.savedViewsKey);
  const showDesktopControlsRow = () =>
    !props.isMobile() &&
    (inlineFilters().length > 0 ||
      hasMenuFilters() ||
      hasSavedViews() ||
      hasDesktopViewOptions() ||
      hasClearableState());
  const showInlineRow = () => props.isMobile() && mobileExpanded() && inlineFilters().length > 0;
  const showDesktopChipRow = () => !props.isMobile() && activeMenuFilters().length > 0;
  const showMobileBody = () => props.isMobile() && mobileExpanded();
  const showChipRow = () =>
    showDesktopChipRow() ||
    (showMobileBody() && (activeMenuFilters().length > 0 || hasMenuFilters() || hasSavedViews()));

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
              tips={props.search.tips}
            />
          </div>
          {props.searchTrailing}
          <Show when={props.isMobile()}>
            <FilterMobileToggleButton
              onClick={() => setMobileExpanded((value) => !value)}
              count={activeCount()}
            />
          </Show>
        </div>

        <Show when={showDesktopControlsRow()}>
          <div class="flex flex-wrap items-center gap-2">
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
            <Show
              when={
                inlineFilters().length > 0 &&
                (hasMenuFilters() || hasSavedViews() || hasDesktopViewOptions())
              }
            >
              <FilterBarRailDivider />
            </Show>
            <Show when={hasMenuFilters()}>
              <AddFilterMenu filters={menuFilters()} />
            </Show>
            <Show when={props.savedViewsKey}>{(key) => <SavedViewsMenu storageKey={key()} />}</Show>
            <Show when={(hasMenuFilters() || hasSavedViews()) && hasDesktopViewOptions()}>
              <FilterBarRailDivider />
            </Show>
            <Show when={props.viewOptionsTrailing}>
              <div class="inline-flex flex-shrink-0 flex-wrap items-center gap-2">
                {props.viewOptionsTrailing}
              </div>
            </Show>
            <Show when={hasClearableState()}>
              <FilterBarRailDivider />
              <FilterBarClearAllButton onClick={clearAll} />
            </Show>
          </div>
        </Show>

        <Show when={showInlineRow()}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={inlineFilters()}>{(filter) => <InlineFilterControl filter={filter} />}</For>
            <Show when={hasClearableState() && !showChipRow()}>
              <FilterBarClearAllButton onClick={clearAll} />
            </Show>
          </div>
        </Show>

        <Show when={showChipRow()}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={activeMenuFilters()}>{(filter) => <FilterChip filter={filter} />}</For>
            <Show when={showMobileBody()}>
              <Show when={hasMenuFilters()}>
                <AddFilterMenu filters={menuFilters()} />
              </Show>
              <Show when={props.savedViewsKey}>
                {(key) => <SavedViewsMenu storageKey={key()} />}
              </Show>
            </Show>
            <Show when={hasClearableState() && showMobileBody()}>
              <FilterBarClearAllButton onClick={clearAll} />
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
