import type {
  SearchHistoryConfig,
  SearchTipsConfig,
} from '@/components/shared/useSearchInputEnhancements';

export type SearchInputKeyboardEvent = KeyboardEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

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
  typeToSearch?: boolean;
  clearOnEscape?: boolean;
  clearOnFocusedEscape?: boolean;
  focusOnShortcut?: boolean;
  captureBackspace?: boolean;
  shortcutHint?: string;
  onBeforeAutoFocus?: () => boolean;
}

export const getSearchInputShortcutHint = (isSimple: boolean, shortcutHint?: string) =>
  isSimple ? shortcutHint : undefined;

export const shouldSearchInputShowTrailingControls = (isSimple: boolean) => !isSimple;
