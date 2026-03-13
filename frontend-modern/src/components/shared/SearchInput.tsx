import { Component } from 'solid-js';

type SearchInputKeyboardEvent = KeyboardEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};
import { SearchField } from '@/components/shared/SearchField';
import {
  SearchInputHistoryDropdown,
  SearchInputTrailingControls,
} from '@/components/shared/SearchInputEnhancements';
import { useTypeToSearch } from '@/hooks/useTypeToSearch';
import {
  type SearchHistoryConfig,
  type SearchTipsConfig,
  useSearchInputEnhancements,
} from '@/components/shared/useSearchInputEnhancements';

export interface SearchInputProps {
  value: () => string;
  onChange: (value: string) => void;
  placeholder?: string;
  title?: string;
  history?: SearchHistoryConfig;
  tips?: SearchTipsConfig;
  inputRef?: (el: HTMLInputElement) => void;
  class?: string;
  inputClass?: string;
  disabled?: boolean;
  onKeyDown?: (event: SearchInputKeyboardEvent) => void;
  /** When false, disables the default type-to-search behavior for this search input. */
  typeToSearch?: boolean;
  /** When true, pressing Escape clears the search even if focus is elsewhere on the page. */
  clearOnEscape?: boolean;
  /** When false, pressing Escape while focused leaves the value unchanged. */
  clearOnFocusedEscape?: boolean;
  /** When true, Ctrl/Cmd+F focuses this search input via the shared search handler. */
  focusOnShortcut?: boolean;
  /** When true, Backspace outside editable fields focuses the input and deletes a character. */
  captureBackspace?: boolean;
  /** Optional trailing hint shown while the search is empty. */
  shortcutHint?: string;
  /** Called before auto-focus — return true to prevent focus (e.g. when AI chat should capture input instead). */
  onBeforeAutoFocus?: () => boolean;
}

export const SearchInput: Component<SearchInputProps> = (props) => {
  let searchInputEl: HTMLInputElement | undefined;

  useTypeToSearch({
    getInput: () => searchInputEl,
    enabled: () => props.typeToSearch ?? true,
    onBeforeFocus: props.onBeforeAutoFocus,
    clearOnEscape: () => props.clearOnEscape ?? false,
    getValue: props.value,
    onClear: () => props.onChange(''),
    focusOnShortcut: () => props.focusOnShortcut ?? false,
    captureBackspace: () => props.captureBackspace ?? false,
  });

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputEl?.focus());
  };

  const enhancements = useSearchInputEnhancements({
    history: props.history,
    tips: props.tips,
    value: props.value,
    onChange: props.onChange,
    onFieldKeyDown: props.onKeyDown,
    focusInput: focusSearchInput,
  });

  return (
    <div class={`relative w-full ${props.class ?? ''}`}>
      <SearchField
        value={props.value()}
        onChange={props.onChange}
        placeholder={props.placeholder}
        title={props.title}
        inputRef={(el) => {
          searchInputEl = el;
          props.inputRef?.(el);
        }}
        inputClass={props.inputClass}
        disabled={props.disabled}
        clearOnFocusedEscape={props.clearOnFocusedEscape}
        shortcutHint={enhancements.isSimple() ? props.shortcutHint : undefined}
        hasTrailingControls={!enhancements.isSimple()}
        onClearMouseDown={enhancements.onClearMouseDown}
        onKeyDown={enhancements.onFieldKeyDown}
        onBlur={enhancements.onFieldBlur}
        trailingControls={<SearchInputTrailingControls state={enhancements} tips={props.tips} />}
      />
      <SearchInputHistoryDropdown state={enhancements} />
    </div>
  );
};
