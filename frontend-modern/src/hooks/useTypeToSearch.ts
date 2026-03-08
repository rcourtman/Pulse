import type { Accessor } from 'solid-js';
import { onCleanup } from 'solid-js';

const isEditableElement = (target: EventTarget | null): boolean => {
  if (!(target instanceof HTMLElement)) return false;

  const tag = target.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (target.isContentEditable) return true;

  return target.getAttribute('role') === 'textbox';
};

const isPrintableSearchKey = (event: KeyboardEvent): boolean =>
  event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey;

const readEnabled = (value: boolean | Accessor<boolean> | undefined): boolean =>
  typeof value === 'function' ? value() : value ?? true;

const insertCharacter = (input: HTMLInputElement, key: string) => {
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? input.value.length;
  const nextValue = `${input.value.slice(0, start)}${key}${input.value.slice(end)}`;
  const nextCaret = start + key.length;

  input.value = nextValue;
  input.setSelectionRange(nextCaret, nextCaret);
  input.dispatchEvent(new Event('input', { bubbles: true }));
};

const deleteBackward = (input: HTMLInputElement) => {
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? input.value.length;

  if (start !== end) {
    const nextValue = `${input.value.slice(0, start)}${input.value.slice(end)}`;
    input.value = nextValue;
    input.setSelectionRange(start, start);
    input.dispatchEvent(new Event('input', { bubbles: true }));
    return;
  }

  if (start <= 0) return;

  const nextValue = `${input.value.slice(0, start - 1)}${input.value.slice(end)}`;
  const nextCaret = start - 1;
  input.value = nextValue;
  input.setSelectionRange(nextCaret, nextCaret);
  input.dispatchEvent(new Event('input', { bubbles: true }));
};

const isVisibleInput = (input: HTMLInputElement | undefined): input is HTMLInputElement => {
  if (!input || !input.isConnected || input.disabled) return false;
  if (input.hidden) return false;
  if (typeof window !== 'undefined') {
    const style = window.getComputedStyle(input);
    if (style.display === 'none' || style.visibility === 'hidden') {
      return false;
    }
  }
  return true;
};

export interface TypeToSearchOptions {
  getInput: () => HTMLInputElement | undefined;
  enabled?: boolean | Accessor<boolean>;
  prepareInput?: () => void;
  onBeforeFocus?: () => boolean;
  clearOnEscape?: boolean | Accessor<boolean>;
  getValue?: () => string;
  onClear?: () => void;
  blurOnClear?: boolean;
  focusOnShortcut?: boolean | Accessor<boolean>;
  captureBackspace?: boolean | Accessor<boolean>;
}

type RegistryEntry = TypeToSearchOptions & { id: number };

let nextEntryId = 1;
const registry: RegistryEntry[] = [];
let listenerAttached = false;

const focusInput = (
  entry: RegistryEntry,
  action?: (input: HTMLInputElement) => void,
  attempt = 0,
) => {
  if (attempt === 0) {
    entry.prepareInput?.();
  }

  if (attempt > 4) return;

  queueMicrotask(() => {
    const input = entry.getInput();
    if (!isVisibleInput(input)) {
      focusInput(entry, action, attempt + 1);
      return;
    }
    input.focus();
    action?.(input);
  });
};

const getActiveEntry = (
  predicate: (entry: RegistryEntry, input?: HTMLInputElement) => boolean,
  options?: { allowPrepared?: boolean },
): [RegistryEntry, HTMLInputElement | undefined] | null => {
  for (let index = registry.length - 1; index >= 0; index -= 1) {
    const entry = registry[index];
    if (!readEnabled(entry.enabled)) continue;
    const input = entry.getInput();
    const visibleInput = isVisibleInput(input) ? input : undefined;
    if (!visibleInput && !(options?.allowPrepared && entry.prepareInput)) continue;
    if (!predicate(entry, visibleInput)) continue;
    return [entry, visibleInput];
  }
  return null;
};

export const focusActiveTypeToSearch = (options?: { selectText?: boolean }): boolean => {
  const match = getActiveEntry(() => true, { allowPrepared: true });
  if (!match) return false;

  const [entry] = match;
  if (entry.onBeforeFocus?.()) return false;
  focusInput(entry, (input) => {
    if (options?.selectText) {
      input.select();
    }
  });
  return true;
};

export const blurFocusedTypeToSearch = (): boolean => {
  const match = getActiveEntry(
    (_entry, input) => Boolean(input && document.activeElement === input),
    { allowPrepared: false },
  );
  if (!match) return false;

  const [, input] = match;
  input?.blur();
  return true;
};

const handleTypeToSearchKeyDown = (event: KeyboardEvent) => {
  if (event.defaultPrevented) return;

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
    const match = getActiveEntry((entry) => readEnabled(entry.focusOnShortcut), {
      allowPrepared: true,
    });
    if (!match) return;
    const [entry] = match;
    if (entry.onBeforeFocus?.()) return;
    event.preventDefault();
    focusInput(entry, (input) => input.select());
    return;
  }

  if (event.key === 'Escape') {
    const match = getActiveEntry(
      (entry) => readEnabled(entry.clearOnEscape) && Boolean(entry.getValue?.().trim()),
    );
    if (!match) return;

    const [entry, input] = match;
    const activeElement = document.activeElement;
    const target = event.target;
    const searchFocused = activeElement === input;
    const editableElsewhere =
      isEditableElement(target) || (Boolean(activeElement) && isEditableElement(activeElement));

    if (searchFocused || !editableElsewhere) {
      event.preventDefault();
      entry.onClear?.();
      if (searchFocused && input && entry.blurOnClear !== false) {
        queueMicrotask(() => input.blur());
      }
    }
    return;
  }

  if (event.key === 'Backspace') {
    const match = getActiveEntry((entry) => readEnabled(entry.captureBackspace), {
      allowPrepared: true,
    });
    if (!match) return;
    const [entry] = match;
    if (isEditableElement(event.target) || isEditableElement(document.activeElement)) return;
    if (entry.onBeforeFocus?.()) return;
    event.preventDefault();
    focusInput(entry, (input) => deleteBackward(input));
    return;
  }

  if (!isPrintableSearchKey(event)) return;

  const match = getActiveEntry(() => true, { allowPrepared: true });
  if (!match) return;
  const [entry] = match;
  if (isEditableElement(event.target) || isEditableElement(document.activeElement)) return;
  if (entry.onBeforeFocus?.()) return;

  event.preventDefault();
  focusInput(entry, (input) => insertCharacter(input, event.key));
};

const attachListenerIfNeeded = () => {
  if (listenerAttached || typeof document === 'undefined') return;
  document.addEventListener('keydown', handleTypeToSearchKeyDown);
  listenerAttached = true;
};

const detachListenerIfNeeded = () => {
  if (!listenerAttached || registry.length > 0 || typeof document === 'undefined') return;
  document.removeEventListener('keydown', handleTypeToSearchKeyDown);
  listenerAttached = false;
};

export const useTypeToSearch = (options: TypeToSearchOptions) => {
  if (typeof document === 'undefined') return;

  const entry: RegistryEntry = { ...options, id: nextEntryId++ };
  registry.push(entry);
  attachListenerIfNeeded();

  onCleanup(() => {
    const index = registry.findIndex((candidate) => candidate.id === entry.id);
    if (index !== -1) {
      registry.splice(index, 1);
    }
    detachListenerIfNeeded();
  });
};

export { deleteBackward, insertCharacter, isEditableElement, isPrintableSearchKey };
