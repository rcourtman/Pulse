import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  on,
  onCleanup,
} from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import CheckIcon from 'lucide-solid/icons/check';
import SearchIcon from 'lucide-solid/icons/search';
import { clearFilter, formatFilterChipValue, type FilterDef } from './filterCatalog';

interface FilterChipProps {
  filter: FilterDef;
}

const matchesQuery = (label: string, query: string): boolean => label.toLowerCase().includes(query);

export const FilterChip: Component<FilterChipProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  const [query, setQuery] = createSignal('');
  const [activeIndex, setActiveIndex] = createSignal(0);
  const listboxId = createUniqueId();
  const popoverId = createUniqueId();
  let containerRef: HTMLDivElement | undefined;
  let searchInputRef: HTMLInputElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;

  const close = (restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) queueMicrotask(() => triggerRef?.focus());
  };

  const handleClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      close();
    }
  };

  const handleEscape = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      close(true);
    }
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
    close(true);
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

  const activeOptionId = createMemo(() =>
    filteredOptions().length > 0 ? `${listboxId}-option-${activeIndex()}` : undefined,
  );

  // Seed activeIndex on the currently-selected option when the popover opens
  // so Enter without further typing keeps the existing value (no-op) rather
  // than picking the first option in the list.
  createEffect(
    on(open, (isOpen) => {
      if (!isOpen) return;
      const options = filteredOptions();
      const selectedIndex = options.findIndex((option) => option.value === props.filter.value());
      if (selectedIndex >= 0) setActiveIndex(selectedIndex);
    }),
  );

  return (
    <div ref={containerRef} class="relative inline-flex">
      <div class="inline-flex items-center rounded-full border border-blue-200 bg-blue-50 text-xs dark:border-blue-900 dark:bg-blue-950/40">
        <button
          ref={triggerRef}
          type="button"
          onClick={() => setOpen((value) => !value)}
          onKeyDown={(event) => {
            if (open() || (event.key !== 'ArrowDown' && event.key !== 'ArrowUp')) return;
            event.preventDefault();
            setOpen(true);
          }}
          aria-haspopup="listbox"
          aria-expanded={open()}
          aria-controls={open() ? listboxId : undefined}
          class="inline-flex min-h-6 items-center gap-1 rounded-l-full py-0.5 pl-2 pr-1 text-base-content hover:bg-blue-100/70 focus-visible:z-10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500 dark:hover:bg-blue-900/40"
        >
          <span class="text-muted">{props.filter.label}:</span>
          <span class="font-medium">{formatFilterChipValue(props.filter)}</span>
        </button>
        <button
          type="button"
          onClick={() => clearFilter(props.filter)}
          aria-label={`Remove ${props.filter.label} filter`}
          class="inline-flex min-h-6 min-w-6 items-center justify-center rounded-r-full py-0.5 pr-1.5 pl-1 text-muted hover:bg-blue-100 hover:text-base-content focus-visible:z-10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500 dark:hover:bg-blue-900/50"
        >
          <XIcon class="h-3 w-3" aria-hidden="true" />
        </button>
      </div>

      <Show when={open()}>
        <div
          id={popoverId}
          class="absolute bottom-[calc(100%+0.25rem)] left-0 z-50 w-56 max-w-[calc(100vw-2rem)] rounded-md border border-border bg-surface shadow-lg sm:bottom-auto sm:top-[calc(100%+0.25rem)]"
        >
          <div class="border-b border-border-subtle px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wide text-muted">
            {props.filter.label}
          </div>
          <div class="relative border-b border-border-subtle">
            <SearchIcon
              class="pointer-events-none absolute left-2.5 top-1/2 h-3 w-3 -translate-y-1/2 text-muted"
              aria-hidden="true"
            />
            <input
              ref={searchInputRef}
              type="text"
              role="combobox"
              value={query()}
              onInput={(event) => {
                setActiveIndex(0);
                setQuery(event.currentTarget.value);
              }}
              onKeyDown={handleSearchKeyDown}
              placeholder="Filter values..."
              aria-label={`Filter ${props.filter.label} values`}
              aria-autocomplete="list"
              aria-controls={listboxId}
              aria-expanded="true"
              aria-activedescendant={activeOptionId()}
              class="w-full bg-transparent py-1.5 pl-7 pr-2 text-xs text-base-content placeholder-muted outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:rounded"
            />
          </div>
          <div
            id={listboxId}
            role="listbox"
            aria-label={props.filter.label}
            class="max-h-64 overflow-y-auto py-1"
          >
            <For each={filteredOptions()}>
              {(option, index) => {
                const isSelected = () => props.filter.value() === option.value;
                const isActive = () => activeIndex() === index();
                return (
                  <button
                    id={`${listboxId}-option-${index()}`}
                    type="button"
                    role="option"
                    tabIndex={-1}
                    aria-selected={isSelected()}
                    aria-label={option.ariaLabel}
                    onMouseEnter={() => setActiveIndex(index())}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      props.filter.setValue(option.value);
                      close(true);
                    }}
                    class={`flex min-h-6 w-full items-center justify-between px-3 py-1.5 text-left text-xs text-base-content hover:bg-surface-hover ${
                      isActive() ? 'bg-surface-hover' : ''
                    }`}
                  >
                    <span class={isSelected() ? 'font-medium' : ''}>{option.label}</span>
                    <Show when={isSelected()}>
                      <CheckIcon
                        class="h-3 w-3 text-blue-600 dark:text-blue-400"
                        aria-hidden="true"
                      />
                    </Show>
                  </button>
                );
              }}
            </For>
          </div>
          <Show when={filteredOptions().length === 0}>
            <div role="status" class="px-3 py-2 text-xs text-muted">
              No values match.
            </div>
          </Show>
        </div>
      </Show>
    </div>
  );
};
