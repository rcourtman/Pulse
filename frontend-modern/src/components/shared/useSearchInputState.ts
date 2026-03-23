import { useTypeToSearch } from '@/hooks/useTypeToSearch';
import {
  useSearchInputEnhancements,
  type SearchInputEnhancementsState,
} from '@/components/shared/useSearchInputEnhancements';
import {
  getSearchInputShortcutHint,
  shouldSearchInputShowTrailingControls,
  type SearchInputProps,
} from './searchInputModel';

type SearchInputStateOptions = Pick<
  SearchInputProps,
  | 'captureBackspace'
  | 'clearOnEscape'
  | 'history'
  | 'focusOnShortcut'
  | 'inputRef'
  | 'onBeforeAutoFocus'
  | 'onChange'
  | 'onKeyDown'
  | 'shortcutHint'
  | 'tips'
  | 'typeToSearch'
  | 'value'
>;

export function useSearchInputState(options: SearchInputStateOptions): {
  enhancements: SearchInputEnhancementsState;
  setInputRef: (el: HTMLInputElement) => void;
  shortcutHint: () => string | undefined;
  showTrailingControls: () => boolean;
} {
  let searchInputEl: HTMLInputElement | undefined;

  useTypeToSearch({
    getInput: () => searchInputEl,
    enabled: () => options.typeToSearch ?? true,
    onBeforeFocus: options.onBeforeAutoFocus,
    clearOnEscape: () => options.clearOnEscape ?? false,
    getValue: options.value,
    onClear: () => options.onChange(''),
    focusOnShortcut: () => options.focusOnShortcut ?? false,
    captureBackspace: () => options.captureBackspace ?? false,
  });

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputEl?.focus());
  };

  const enhancements = useSearchInputEnhancements({
    history: options.history,
    tips: options.tips,
    value: options.value,
    onChange: options.onChange,
    onFieldKeyDown: options.onKeyDown,
    focusInput: focusSearchInput,
  });

  const setInputRef = (el: HTMLInputElement) => {
    searchInputEl = el;
    options.inputRef?.(el);
  };

  const shortcutHint = () =>
    getSearchInputShortcutHint(enhancements.isSimple(), options.shortcutHint);
  const showTrailingControls = () =>
    shouldSearchInputShowTrailingControls(enhancements.isSimple());

  return {
    enhancements,
    setInputRef,
    shortcutHint,
    showTrailingControls,
  };
}
