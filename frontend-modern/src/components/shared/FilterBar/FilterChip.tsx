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
import XIcon from 'lucide-solid/icons/x';
import CheckIcon from 'lucide-solid/icons/check';
import SearchIcon from 'lucide-solid/icons/search';
import {
  clearFilter,
  formatFilterChipValue,
  type FilterDef,
} from './filterCatalog';

interface FilterChipProps {
  filter: FilterDef;
}

const matchesQuery = (label: string, query: string): boolean =>
  label.toLowerCase().includes(query);

export const FilterChip: Component<FilterChipProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  const [query, setQuery] = createSignal('');
  const [activeIndex, setActiveIndex] = createSignal(0);
  let containerRef: HTMLDivElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;

  const handleClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      setOpen(false);
    }
  };

  const handleEscape = (event: KeyboardEvent) => {
    if (event.key === 'Escape') setOpen(false);
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

  // Reset search + active index whenever the popover opens; auto-focus the
  // search input so typing narrows the value list immediately.
  createEffect(
    on(open, (isOpen) => {
      setQuery('');
      setActiveIndex(0);
      if (isOpen) {
        queueMicrotask(() => searchInputRef?.focus());
      }
    }),
  );

  const filteredOptions = createMemo(() => {
    const q = query().trim().toLowerCase();
    const all = props.filter.options();
    if (!q) return all;
    return all.filter((option) => matchesQuery(option.label, q));
  });

  // Clamp active index when the option list narrows.
  createEffect(() => {
    const length = filteredOptions().length;
    if (length === 0) {
      if (activeIndex() !== 0) setActiveIndex(0);
      return;
    }
    if (activeIndex() >= length) setActiveIndex(length - 1);
  });

  const commitActive = () => {
    const options = filteredOptions();
    if (options.length === 0) return;
    const index = Math.max(0, Math.min(activeIndex(), options.length - 1));
    const option = options[index];
    if (!option) return;
    props.filter.setValue(option.value);
    setOpen(false);
  };

  const handleSearchKeyDown = (event: KeyboardEvent) => {
    const length = filteredOptions().length;
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
    }
  };

  // Seed activeIndex on the currently-selected option when the popover opens
  // so Enter without further typing keeps the existing value (no-op) rather
  // than picking the first option in the list.
  createEffect(
    on(open, (isOpen) => {
      if (!isOpen) return;
      const options = filteredOptions();
      const selectedIndex = options.findIndex(
        (option) => option.value === props.filter.value(),
      );
      if (selectedIndex >= 0) setActiveIndex(selectedIndex);
    }),
  );

  return (
    <div ref={containerRef} class="relative inline-flex">
      <div class="inline-flex items-center rounded-full border border-blue-200 bg-blue-50 text-xs dark:border-blue-900 dark:bg-blue-950/40">
        <button
          type="button"
          onClick={() => setOpen((value) => !value)}
          aria-haspopup="listbox"
          aria-expanded={open()}
          class="inline-flex items-center gap-1 rounded-l-full py-0.5 pl-2 pr-1 text-base-content hover:bg-blue-100/70 dark:hover:bg-blue-900/40"
        >
          <span class="text-muted">{props.filter.label}:</span>
          <span class="font-medium">{formatFilterChipValue(props.filter)}</span>
        </button>
        <button
          type="button"
          onClick={() => clearFilter(props.filter)}
          aria-label={`Remove ${props.filter.label} filter`}
          class="rounded-r-full py-0.5 pr-1.5 pl-1 text-muted hover:bg-blue-100 hover:text-base-content dark:hover:bg-blue-900/50"
        >
          <XIcon class="h-3 w-3" />
        </button>
      </div>

      <Show when={open()}>
        <div
          role="listbox"
          aria-label={props.filter.label}
          class="absolute left-0 top-[calc(100%+0.25rem)] z-50 w-56 max-w-[calc(100vw-2rem)] rounded-md border border-border bg-surface shadow-lg"
        >
          <div class="border-b border-border-subtle px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
            {props.filter.label}
          </div>
          <div class="relative border-b border-border-subtle">
            <SearchIcon class="pointer-events-none absolute left-2.5 top-1/2 h-3 w-3 -translate-y-1/2 text-muted" />
            <input
              ref={searchInputRef}
              type="text"
              value={query()}
              onInput={(event) => setQuery(event.currentTarget.value)}
              onKeyDown={handleSearchKeyDown}
              placeholder="Filter values..."
              aria-label={`Filter ${props.filter.label} values`}
              class="w-full bg-transparent py-1.5 pl-7 pr-2 text-xs text-base-content placeholder-muted outline-none"
            />
          </div>
          <div class="max-h-64 overflow-y-auto py-1">
            <Show
              when={filteredOptions().length > 0}
              fallback={
                <div class="px-3 py-2 text-xs text-muted">No values match.</div>
              }
            >
              <For each={filteredOptions()}>
                {(option, index) => {
                  const isSelected = () => props.filter.value() === option.value;
                  const isActive = () => activeIndex() === index();
                  return (
                    <button
                      type="button"
                      role="option"
                      aria-selected={isSelected()}
                      onMouseEnter={() => setActiveIndex(index())}
                      onClick={() => {
                        props.filter.setValue(option.value);
                        setOpen(false);
                      }}
                      class={`flex w-full items-center justify-between px-3 py-1.5 text-left text-xs text-base-content hover:bg-surface-hover ${
                        isActive() ? 'bg-surface-hover' : ''
                      }`}
                    >
                      <span class={isSelected() ? 'font-medium' : ''}>{option.label}</span>
                      <Show when={isSelected()}>
                        <CheckIcon class="h-3 w-3 text-blue-600 dark:text-blue-400" />
                      </Show>
                    </button>
                  );
                }}
              </For>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};
