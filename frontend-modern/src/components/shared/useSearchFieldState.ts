import {
  getSearchFieldInputPaddingRightClass,
  shouldShowSearchFieldClearButton,
  shouldShowSearchFieldShortcutHint,
  type SearchFieldFocusEvent,
  type SearchFieldKeyboardEvent,
  type SearchFieldProps,
} from './searchFieldModel';

type SearchFieldStateOptions = Pick<
  SearchFieldProps,
  | 'clearOnFocusedEscape'
  | 'disabled'
  | 'hasTrailingControls'
  | 'inputRef'
  | 'onBlur'
  | 'onChange'
  | 'onKeyDown'
  | 'showClearButton'
  | 'shortcutHint'
  | 'value'
>;

export function useSearchFieldState(options: SearchFieldStateOptions) {
  let inputEl: HTMLInputElement | undefined;

  const normalizeEventTarget = <
    TEvent extends { currentTarget: EventTarget | null; target: EventTarget | null },
  >(
    event: TEvent,
  ) => {
    const currentTarget = event.currentTarget as HTMLInputElement;
    const normalizedTarget = event.target as Element;

    return new Proxy(event, {
      get(eventTarget, prop, receiver) {
        if (prop === 'currentTarget') return currentTarget;
        if (prop === 'target') return normalizedTarget;
        return Reflect.get(eventTarget, prop, receiver);
      },
    });
  };

  const showShortcutHint = () =>
    shouldShowSearchFieldShortcutHint(options.value, options.shortcutHint);
  const showClearButton = () =>
    shouldShowSearchFieldClearButton(options.value, options.disabled, options.showClearButton);
  const inputPaddingRight = () =>
    getSearchFieldInputPaddingRightClass({
      hasTrailingControls: options.hasTrailingControls,
      showShortcutHint: showShortcutHint(),
      showClearButton: showClearButton(),
    });

  const setInputRef = (el: HTMLInputElement) => {
    inputEl = el;
    options.inputRef?.(el);
  };

  const handleKeyDown = (event: KeyboardEvent & { currentTarget: HTMLInputElement }) => {
    if (event.key === 'Escape' && (options.clearOnFocusedEscape ?? true)) {
      if (options.value) {
        options.onChange('');
      }
      inputEl?.blur();
    }
    options.onKeyDown?.(normalizeEventTarget(event) as SearchFieldKeyboardEvent);
  };

  const handleBlur = (event: FocusEvent & { currentTarget: HTMLInputElement }) => {
    options.onBlur?.(normalizeEventTarget(event) as SearchFieldFocusEvent);
  };

  return {
    handleBlur,
    handleKeyDown,
    inputPaddingRight,
    setInputRef,
    showClearButton,
    showShortcutHint,
  };
}
