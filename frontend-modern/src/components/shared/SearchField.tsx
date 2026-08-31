import { Component, Show } from 'solid-js';
import { type SearchFieldProps } from './searchFieldModel';
import { useSearchFieldState } from './useSearchFieldState';

export type { SearchFieldProps } from './searchFieldModel';

export const SearchField: Component<SearchFieldProps> = (props) => {
  const search = useSearchFieldState(props);

  return (
    <div class={`relative w-full ${props.class ?? ''}`}>
      <input
        ref={search.setInputRef}
        type="text"
        placeholder={props.placeholder ?? 'Search...'}
        value={props.value}
        disabled={props.disabled}
        autofocus={props.autofocus}
        onInput={(e) => props.onChange(e.currentTarget.value)}
        onKeyDown={search.handleKeyDown}
        onFocus={search.handleFocus}
        onBlur={search.handleBlur}
        aria-label={props.title ?? props.placeholder ?? 'Search'}
        role={props.role}
        aria-autocomplete={props.ariaAutocomplete}
        class={`min-h-11 w-full pl-8 sm:min-h-10 sm:pl-9 ${search.inputPaddingRight()} py-1.5 sm:py-2 text-sm border border-border rounded-md
 bg-surface text-base-content placeholder-muted
 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all disabled:opacity-60 disabled:cursor-not-allowed ${props.inputClass ?? ''}`}
        title={props.title}
      />
      <Show when={props.completionSuffix}>
        <div
          aria-hidden="true"
          class="pointer-events-none absolute inset-0 flex min-h-11 items-center overflow-hidden whitespace-pre pl-8 pr-8 text-sm sm:min-h-10 sm:pl-9"
        >
          <span class="text-transparent">{props.value}</span>
          <span class="text-muted opacity-60" data-search-completion-suffix>
            {props.completionSuffix}
          </span>
        </div>
      </Show>
      <svg
        class="absolute left-2.5 sm:left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>
      <div class="absolute inset-y-0 right-2 flex items-center gap-1">
        <Show when={search.showShortcutHint()}>
          <span class="pointer-events-none hidden items-center rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] font-semibold text-muted sm:inline-flex">
            {props.shortcutHint}
          </span>
        </Show>
        <Show when={search.showClearButton()}>
          <button
            type="button"
            class="inline-flex h-11 w-11 items-center justify-center rounded-full bg-surface-hover text-muted transition-all duration-150 hover:bg-red-100 hover:text-red-600 active:scale-90 sm:h-6 sm:w-6 dark:hover:bg-red-900 dark:hover:text-red-400"
            onClick={() => props.onChange('')}
            onMouseDown={props.onClearMouseDown}
            aria-label="Clear search"
            title="Clear search"
          >
            <svg
              class="h-3 w-3"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="3"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </Show>
        <Show when={props.hasTrailingControls}>{props.trailingControls}</Show>
      </div>
    </div>
  );
};
