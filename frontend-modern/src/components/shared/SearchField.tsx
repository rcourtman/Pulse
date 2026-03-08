import { Component, Show } from 'solid-js';
import type { JSX } from 'solid-js';

export interface SearchFieldProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  title?: string;
  inputRef?: (el: HTMLInputElement) => void;
  class?: string;
  inputClass?: string;
  disabled?: boolean;
  onKeyDown?: JSX.EventHandlerUnion<HTMLInputElement, KeyboardEvent>;
  onBlur?: JSX.EventHandlerUnion<HTMLInputElement, FocusEvent>;
  showClearButton?: boolean;
  clearOnFocusedEscape?: boolean;
  shortcutHint?: string;
  hasTrailingControls?: boolean;
  trailingControls?: JSX.Element;
  onClearMouseDown?: JSX.EventHandlerUnion<HTMLButtonElement, MouseEvent>;
}

export const SearchField: Component<SearchFieldProps> = (props) => {
  let inputEl: HTMLInputElement | undefined;

  const showShortcutHint = () => Boolean(props.shortcutHint && !props.value);
  const showClearButton = () =>
    (props.showClearButton ?? true) && Boolean(props.value) && !props.disabled;

  const inputPaddingRight = () => {
    if (props.hasTrailingControls) return 'pr-14 sm:pr-20';
    if (showShortcutHint()) return 'pr-20 sm:pr-24';
    if (showClearButton()) return 'pr-8';
    return 'pr-8';
  };

  return (
    <div class={`relative w-full ${props.class ?? ''}`}>
      <input
        ref={(el) => {
          inputEl = el;
          props.inputRef?.(el);
        }}
        type="text"
        placeholder={props.placeholder ?? 'Search...'}
        value={props.value}
        disabled={props.disabled}
        onInput={(e) => props.onChange(e.currentTarget.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape' && (props.clearOnFocusedEscape ?? true)) {
            if (props.value) {
              props.onChange('');
            }
            inputEl?.blur();
          }
          props.onKeyDown?.(e);
        }}
        onBlur={(e) => props.onBlur?.(e)}
        class={`w-full pl-8 sm:pl-9 ${inputPaddingRight()} py-1.5 sm:py-2 text-sm border border-border rounded-md
 bg-surface text-base-content placeholder-muted
 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all disabled:opacity-60 disabled:cursor-not-allowed ${props.inputClass ?? ''}`}
        title={props.title}
      />
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
        <Show when={showShortcutHint()}>
          <span class="pointer-events-none hidden items-center rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] font-semibold text-muted sm:inline-flex">
            {props.shortcutHint}
          </span>
        </Show>
        <Show when={showClearButton()}>
          <button
            type="button"
            class="p-1 rounded-full bg-surface-hover text-muted hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900 dark:hover:text-red-400 transition-all duration-150 active:scale-90"
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
