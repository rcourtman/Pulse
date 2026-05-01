import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
} from 'solid-js';
import PlusIcon from 'lucide-solid/icons/plus';
import ArrowLeftIcon from 'lucide-solid/icons/arrow-left';
import SearchIcon from 'lucide-solid/icons/search';
import {
  isFilterSet,
  type FilterDef,
  type FilterGroupKey,
  type FilterSelectOption,
} from './filterCatalog';

interface AddFilterMenuProps {
  filters: FilterDef[];
}

const GROUP_LABELS: Record<FilterGroupKey, string> = {
  scope: 'Scope',
  status: 'Status',
  properties: 'Properties',
};

const GROUP_ORDER: FilterGroupKey[] = ['scope', 'status', 'properties'];

const matchesQuery = (label: string, query: string): boolean =>
  label.toLowerCase().includes(query);

export const AddFilterMenu: Component<AddFilterMenuProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  const [activeFilterId, setActiveFilterId] = createSignal<string | null>(null);
  const [query, setQuery] = createSignal('');
  const [activeIndex, setActiveIndex] = createSignal(0);
  let containerRef: HTMLDivElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;

  const close = () => {
    setOpen(false);
    setActiveFilterId(null);
    setQuery('');
    setActiveIndex(0);
  };

  const handleClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      close();
    }
  };

  const handleEscape = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return;
    if (activeFilterId()) {
      setActiveFilterId(null);
      setQuery('');
      setActiveIndex(0);
      return;
    }
    close();
  };

  createEffect(() => {
    if (!open()) return;
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    });
  });

  // Reset query + active index whenever the menu opens or its phase changes
  // (filter list ↔ value sub-menu). Auto-focus the search input after the
  // phase change so typing is captured immediately.
  createEffect(
    on([open, activeFilterId], () => {
      setQuery('');
      setActiveIndex(0);
      if (open()) {
        queueMicrotask(() => searchInputRef?.focus());
      }
    }),
  );

  const availableFilters = createMemo(() => props.filters.filter((f) => !isFilterSet(f)));

  const groupedAvailable = createMemo(() => {
    const groups = new Map<FilterGroupKey, FilterDef[]>();
    for (const filter of availableFilters()) {
      const key: FilterGroupKey = filter.group ?? 'properties';
      const bucket = groups.get(key);
      if (bucket) {
        bucket.push(filter);
      } else {
        groups.set(key, [filter]);
      }
    }
    return GROUP_ORDER.filter((key) => groups.has(key)).map((key) => ({
      key,
      filters: groups.get(key)!,
    }));
  });

  const activeFilter = createMemo(() => {
    const id = activeFilterId();
    if (!id) return null;
    return props.filters.find((filter) => filter.id === id) ?? null;
  });

  const activeFilterPickableOptions = createMemo(() => {
    const filter = activeFilter();
    if (!filter) return [];
    return filter.options().filter((option) => option.value !== filter.defaultValue);
  });

  const filteredGroupedAvailable = createMemo(() => {
    const q = query().trim().toLowerCase();
    if (!q) return groupedAvailable();
    return groupedAvailable()
      .map((group) => ({
        key: group.key,
        filters: group.filters.filter((filter) => matchesQuery(filter.label, q)),
      }))
      .filter((group) => group.filters.length > 0);
  });

  const flatVisibleFilters = createMemo<FilterDef[]>(() =>
    filteredGroupedAvailable().flatMap((group) => group.filters),
  );

  const filteredOptions = createMemo<FilterSelectOption[]>(() => {
    const q = query().trim().toLowerCase();
    const options = activeFilterPickableOptions();
    if (!q) return options;
    return options.filter((option) => matchesQuery(option.label, q));
  });

  const isDisabled = () => availableFilters().length === 0;

  const navigableLength = () =>
    activeFilter() ? filteredOptions().length : flatVisibleFilters().length;

  // Clamp active index whenever the navigable list shrinks (e.g. typing
  // narrows the result set).
  createEffect(() => {
    const length = navigableLength();
    if (length === 0) {
      if (activeIndex() !== 0) setActiveIndex(0);
      return;
    }
    if (activeIndex() >= length) setActiveIndex(length - 1);
  });

  const commitActive = () => {
    const length = navigableLength();
    if (length === 0) return;
    const index = Math.max(0, Math.min(activeIndex(), length - 1));
    const filter = activeFilter();
    if (filter) {
      const option = filteredOptions()[index];
      if (option) {
        filter.setValue(option.value);
        close();
      }
      return;
    }
    const next = flatVisibleFilters()[index];
    if (next) {
      setActiveFilterId(next.id);
    }
  };

  const handleSearchKeyDown = (event: KeyboardEvent) => {
    const length = navigableLength();
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      if (length === 0) return;
      setActiveIndex((index) => (index + 1) % length);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      if (length === 0) return;
      setActiveIndex((index) => (index - 1 + length) % length);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      commitActive();
    } else if (event.key === 'Backspace' && query() === '' && activeFilterId()) {
      event.preventDefault();
      setActiveFilterId(null);
    }
  };

  const isActiveFilterIndex = (index: number) =>
    !activeFilter() && activeIndex() === index;
  const isActiveOptionIndex = (index: number) =>
    Boolean(activeFilter()) && activeIndex() === index;

  return (
    <div ref={containerRef} class="relative inline-flex">
      <button
        type="button"
        onClick={() => {
          if (isDisabled()) return;
          setOpen((value) => !value);
        }}
        disabled={isDisabled()}
        aria-haspopup="menu"
        aria-expanded={open()}
        class="inline-flex items-center gap-1 rounded-md border border-dashed border-border px-2 py-0.5 text-xs text-muted transition-colors hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-muted"
      >
        <PlusIcon class="h-3 w-3" />
        Filter
      </button>

      <Show when={open()}>
        <div
          role="menu"
          class="absolute left-0 top-[calc(100%+0.25rem)] z-50 w-64 max-w-[calc(100vw-2rem)] rounded-md border border-border bg-surface shadow-lg"
        >
          <Show when={activeFilter()}>
            {(filter) => (
              <button
                type="button"
                onClick={() => {
                  setActiveFilterId(null);
                  setQuery('');
                }}
                class="flex w-full items-center gap-1.5 border-b border-border-subtle px-3 py-2 text-left text-[10px] font-semibold uppercase tracking-wide text-muted hover:bg-surface-hover"
              >
                <ArrowLeftIcon class="h-3 w-3" />
                {filter().label}
              </button>
            )}
          </Show>

          <div class="relative border-b border-border-subtle">
            <SearchIcon class="pointer-events-none absolute left-2.5 top-1/2 h-3 w-3 -translate-y-1/2 text-muted" />
            <input
              ref={searchInputRef}
              type="text"
              value={query()}
              onInput={(event) => setQuery(event.currentTarget.value)}
              onKeyDown={handleSearchKeyDown}
              placeholder={activeFilter() ? 'Filter values...' : 'Filter filters...'}
              aria-label={activeFilter() ? 'Filter values' : 'Filter filters'}
              class="w-full bg-transparent py-1.5 pl-7 pr-2 text-xs text-base-content placeholder-muted outline-none"
            />
          </div>

          <Show
            when={activeFilter()}
            fallback={
              <div class="max-h-64 overflow-y-auto py-1">
                <Show
                  when={flatVisibleFilters().length > 0}
                  fallback={
                    <div class="px-3 py-2 text-xs text-muted">No filters match.</div>
                  }
                >
                  {(() => {
                    let cursor = 0;
                    return (
                      <For each={filteredGroupedAvailable()}>
                        {(group) => {
                          const groupItems = group.filters.map((filter) => ({
                            filter,
                            index: cursor++,
                          }));
                          return (
                            <div>
                              <div class="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
                                {GROUP_LABELS[group.key]}
                              </div>
                              <For each={groupItems}>
                                {(item) => (
                                  <button
                                    type="button"
                                    role="menuitem"
                                    onMouseEnter={() => setActiveIndex(item.index)}
                                    onClick={() => setActiveFilterId(item.filter.id)}
                                    class={`flex w-full items-center justify-between px-3 py-1.5 text-left text-xs text-base-content hover:bg-surface-hover ${
                                      isActiveFilterIndex(item.index)
                                        ? 'bg-surface-hover'
                                        : ''
                                    }`}
                                  >
                                    <span>{item.filter.label}</span>
                                    <span class="text-muted" aria-hidden="true">
                                      {item.filter.options().length}
                                    </span>
                                  </button>
                                )}
                              </For>
                            </div>
                          );
                        }}
                      </For>
                    );
                  })()}
                </Show>
              </div>
            }
          >
            {(filter) => (
              <div class="max-h-64 overflow-y-auto py-1">
                <Show
                  when={filteredOptions().length > 0}
                  fallback={
                    <div class="px-3 py-2 text-xs text-muted">
                      {activeFilterPickableOptions().length === 0
                        ? 'No options available.'
                        : 'No options match.'}
                    </div>
                  }
                >
                  <For each={filteredOptions()}>
                    {(option, index) => (
                      <button
                        type="button"
                        onMouseEnter={() => setActiveIndex(index())}
                        onClick={() => {
                          filter().setValue(option.value);
                          close();
                        }}
                        class={`flex w-full items-center px-3 py-1.5 text-left text-xs text-base-content hover:bg-surface-hover ${
                          isActiveOptionIndex(index()) ? 'bg-surface-hover' : ''
                        }`}
                      >
                        {option.label}
                      </button>
                    )}
                  </For>
                </Show>
              </div>
            )}
          </Show>
        </div>
      </Show>
    </div>
  );
};
