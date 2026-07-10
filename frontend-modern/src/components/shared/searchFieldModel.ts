import type { JSX } from 'solid-js';

export type SearchFieldKeyboardEvent = KeyboardEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

export type SearchFieldFocusEvent = FocusEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

export type SearchFieldMouseEvent = MouseEvent & {
  currentTarget: HTMLButtonElement;
  target: Element;
};

export interface SearchFieldProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  title?: string;
  inputRef?: (el: HTMLInputElement) => void;
  class?: string;
  inputClass?: string;
  disabled?: boolean;
  onKeyDown?: (event: SearchFieldKeyboardEvent) => void;
  onBlur?: (event: SearchFieldFocusEvent) => void;
  showClearButton?: boolean;
  clearOnFocusedEscape?: boolean;
  shortcutHint?: string;
  hasTrailingControls?: boolean;
  trailingControlCount?: number;
  trailingControls?: JSX.Element;
  onClearMouseDown?: (event: SearchFieldMouseEvent) => void;
}

export const shouldShowSearchFieldShortcutHint = (value: string, shortcutHint?: string) =>
  Boolean(shortcutHint && !value);

export const shouldShowSearchFieldClearButton = (
  value: string,
  disabled?: boolean,
  showClearButton?: boolean,
) => (showClearButton ?? true) && Boolean(value) && !disabled;

export const getSearchFieldInputPaddingRightClass = (options: {
  hasTrailingControls?: boolean;
  trailingControlCount?: number;
  showShortcutHint: boolean;
  showClearButton: boolean;
}) => {
  const trailingControlCount = Math.max(
    options.trailingControlCount ?? (options.hasTrailingControls ? 1 : 0),
    0,
  );
  const visibleControlCount = trailingControlCount + (options.showClearButton ? 1 : 0);

  if (visibleControlCount >= 3) return 'pr-36 sm:pr-24';
  if (visibleControlCount === 2) return 'pr-24 sm:pr-20';
  if (trailingControlCount === 1) return 'pr-14 sm:pr-20';
  if (options.showShortcutHint) return 'pr-20 sm:pr-24';
  if (options.showClearButton) return 'pr-12 sm:pr-8';
  return 'pr-8';
};
