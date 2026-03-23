import { createEffect, createSignal } from 'solid-js';
import { useTypeToSearch } from '@/hooks/useTypeToSearch';
import {
  getCollapsibleSearchRootClass,
  getCollapsibleSearchTriggerLabel,
  shouldShowCollapsibleSearchExpanded,
  type CollapsibleSearchInputProps,
} from './collapsibleSearchInputModel';

type CollapsibleSearchInputStateOptions = Pick<
  CollapsibleSearchInputProps,
  'class' | 'fullWidthWhenExpanded' | 'onBeforeAutoFocus' | 'triggerLabel' | 'value'
>;

export function useCollapsibleSearchInputState(options: CollapsibleSearchInputStateOptions) {
  const [isExpanded, setIsExpanded] = createSignal(options.value().trim().length > 0);
  let rootRef: HTMLDivElement | undefined;
  let inputRef: HTMLInputElement | undefined;
  let suppressCollapse = false;

  const focusInput = (selectText = false) => {
    queueMicrotask(() => {
      if (!inputRef) return;
      inputRef.focus();
      if (selectText) {
        inputRef.select?.();
      }
    });
  };

  const expandSearch = (selectText = false) => {
    suppressCollapse = true;
    queueMicrotask(() => {
      suppressCollapse = false;
    });
    if (!isExpanded()) {
      setIsExpanded(true);
    }
    focusInput(selectText);
  };

  const collapseIfEmpty = () => {
    if (options.value().trim().length > 0) return;
    setIsExpanded(false);
  };

  createEffect(() => {
    if (options.value().trim().length > 0 && !isExpanded()) {
      setIsExpanded(true);
    }
  });

  useTypeToSearch({
    getInput: () => inputRef,
    prepareInput: () => {
      if (!isExpanded()) {
        setIsExpanded(true);
      }
    },
    onBeforeFocus: options.onBeforeAutoFocus,
  });

  const showExpanded = () => shouldShowCollapsibleSearchExpanded(isExpanded(), options.value());
  const triggerLabel = () => getCollapsibleSearchTriggerLabel(options.triggerLabel);
  const rootClass = () =>
    getCollapsibleSearchRootClass({
      className: options.class,
      fullWidthWhenExpanded: options.fullWidthWhenExpanded,
      showExpanded: showExpanded(),
    });

  const setRootRef = (element: HTMLDivElement) => {
    rootRef = element;
  };

  const setInputRef = (element: HTMLInputElement) => {
    inputRef = element;
  };

  const handleFocusOut = (event: FocusEvent & { relatedTarget: EventTarget | null }) => {
    if (suppressCollapse) return;
    const next = event.relatedTarget as Node | null;
    if (next && rootRef?.contains(next)) return;
    collapseIfEmpty();
  };

  return {
    expandSearch,
    handleFocusOut,
    rootClass,
    setInputRef,
    setRootRef,
    showExpanded,
    triggerLabel,
  };
}
