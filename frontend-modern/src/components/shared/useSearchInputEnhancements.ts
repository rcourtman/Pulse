import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';
import { createSearchHistoryManager } from '@/utils/searchHistory';
import {
  getSearchInputSuggestionCompletions,
  rankSearchInputSuggestions,
  resolveSearchInputInlineCompletion,
  type SearchInputSuggestion,
  type SearchInputSuggestionsConfig,
} from '@/components/shared/searchInputModel';

export interface SearchTip {
  code: string;
  description: string;
}

export interface SearchHistoryConfig {
  storageKey: string;
  emptyMessage?: string;
}

export interface SearchTipsConfig {
  popoverId: string;
  intro: string;
  tips: SearchTip[];
  footerHighlight?: string;
  footerText?: string;
}

interface SearchInputEnhancementsOptions {
  history?: SearchHistoryConfig;
  tips?: SearchTipsConfig;
  suggestions?: SearchInputSuggestionsConfig;
  onFieldKeyDown?: (event: SearchInputKeyboardEvent) => void;
}

type SearchInputKeyboardEvent = KeyboardEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

type SearchInputFocusEvent = FocusEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

type SearchInputMouseEvent = MouseEvent & {
  currentTarget: HTMLButtonElement;
  target: Element;
};

export interface SearchInputEnhancementsState {
  hasHistory: Accessor<boolean>;
  hasTips: Accessor<boolean>;
  hasSuggestions: Accessor<boolean>;
  isSimple: Accessor<boolean>;
  searchHistory: Accessor<string[]>;
  isHistoryOpen: Accessor<boolean>;
  completionSuffix: Accessor<string>;
  emptyHistoryMessage: Accessor<string>;
  tipsPopoverId: Accessor<string>;
  onClearMouseDown?: (event: SearchInputMouseEvent) => void;
  setHistoryMenuRef: (el: HTMLDivElement | undefined) => void;
  setHistoryToggleRef: (el: HTMLButtonElement | undefined) => void;
  toggleHistory: () => void;
  closeHistory: () => void;
  clearHistory: () => void;
  deleteHistoryEntry: (term: string) => void;
  selectHistoryEntry: (term: string) => void;
  onValueChange: (value: string) => void;
  onFieldKeyDown: (event: SearchInputKeyboardEvent) => void;
  onFieldFocus: (event: SearchInputFocusEvent) => void;
  onFieldBlur: (event: SearchInputFocusEvent) => void;
}

export const useSearchInputEnhancements = (
  options: SearchInputEnhancementsOptions & {
    value: () => string;
    onChange: (value: string) => void;
    focusInput: () => void;
  },
): SearchInputEnhancementsState => {
  const hasHistory = () => !!options.history;
  const hasTips = () => !!options.tips;
  const hasSuggestions = () => !!options.suggestions;
  const isSimple = () => !hasHistory() && !hasTips();
  const tipsPopoverId = () => options.tips?.popoverId ?? '';
  const emptyHistoryMessage = () =>
    options.history?.emptyMessage ?? 'Searches you run will appear here.';

  const historyManager = options.history
    ? createSearchHistoryManager(options.history.storageKey)
    : null;

  const [searchHistory, setSearchHistory] = createSignal<string[]>([]);
  const [isHistoryOpen, setIsHistoryOpen] = createSignal(false);
  const [isFieldFocused, setIsFieldFocused] = createSignal(false);
  const [showInlineCompletion, setShowInlineCompletion] = createSignal(true);
  const [acceptedSuggestionId, setAcceptedSuggestionId] = createSignal<string>();

  const rankedSuggestions = createMemo<SearchInputSuggestion[]>(() => {
    const config = options.suggestions;
    const query = options.value().trim();
    if (!config || query.length < (config.minQueryLength ?? 1)) return [];
    const items = config.items();
    return rankSearchInputSuggestions(items, query, config.maxResults ?? items.length);
  });

  const inlineCompletion = createMemo(() =>
    resolveSearchInputInlineCompletion(rankedSuggestions(), options.value()),
  );

  const exactSuggestion = (): SearchInputSuggestion | undefined => {
    const normalizedQuery = options.value().trim().toLowerCase();
    if (!normalizedQuery) return undefined;
    const acceptedId = acceptedSuggestionId();
    return (
      rankedSuggestions().find(
        (suggestion) =>
          suggestion.id === acceptedId &&
          getSearchInputSuggestionCompletions(suggestion).some(
            (completion) => completion.toLowerCase() === normalizedQuery,
          ),
      ) ??
      rankedSuggestions().find((suggestion) =>
        getSearchInputSuggestionCompletions(suggestion).some(
          (completion) => completion.toLowerCase() === normalizedQuery,
        ),
      )
    );
  };

  const completionSuffix = () => {
    if (!isFieldFocused() || !showInlineCompletion()) return '';
    const completion = inlineCompletion();
    return completion ? completion.text.slice(options.value().length) : '';
  };

  let historyMenuRef: HTMLDivElement | undefined;
  let historyToggleRef: HTMLButtonElement | undefined;
  let suppressBlurCommit = false;

  onMount(() => {
    if (historyManager) setSearchHistory(historyManager.read());
  });

  const commitSearchToHistory = (term: string) => {
    if (!historyManager) return;
    setSearchHistory(historyManager.add(term));
  };

  const deleteHistoryEntry = (term: string) => {
    if (!historyManager) return;
    setSearchHistory(historyManager.remove(term));
  };

  const closeHistory = () => {
    setIsHistoryOpen(false);
    queueMicrotask(() => historyToggleRef?.blur());
  };

  const clearHistory = () => {
    if (!historyManager) return;
    setSearchHistory(historyManager.clear());
    closeHistory();
  };

  const markSuppressCommit = () => {
    suppressBlurCommit = true;
    queueMicrotask(() => {
      suppressBlurCommit = false;
    });
  };

  const handleDocumentClick = (event: MouseEvent) => {
    const target = event.target as Node;
    const clickedMenu = historyMenuRef?.contains(target) ?? false;
    const clickedToggle = historyToggleRef?.contains(target) ?? false;
    if (!clickedMenu && !clickedToggle) closeHistory();
  };

  createEffect(() => {
    if (isHistoryOpen()) document.addEventListener('mousedown', handleDocumentClick);
    else document.removeEventListener('mousedown', handleDocumentClick);
  });

  onCleanup(() => document.removeEventListener('mousedown', handleDocumentClick));

  const acceptInlineCompletion = () => {
    const completion = inlineCompletion();
    if (!completion) return;
    setAcceptedSuggestionId(completion.suggestion.id);
    options.onChange(completion.text);
    setShowInlineCompletion(false);
  };

  const applyExactSuggestion = (suggestion: SearchInputSuggestion, input: HTMLInputElement) => {
    closeHistory();
    if (suggestion.onSelect) {
      suggestion.onSelect();
      options.onChange('');
      setAcceptedSuggestionId(undefined);
      options.focusInput();
      return;
    }

    const value = suggestion.value ?? options.value().trim();
    options.onChange(value);
    commitSearchToHistory(value);
    input.blur();
  };

  return {
    hasHistory,
    hasTips,
    hasSuggestions,
    isSimple,
    searchHistory,
    isHistoryOpen,
    completionSuffix,
    emptyHistoryMessage,
    tipsPopoverId,
    onClearMouseDown: hasHistory() ? markSuppressCommit : undefined,
    setHistoryMenuRef: (el) => {
      historyMenuRef = el;
    },
    setHistoryToggleRef: (el) => {
      historyToggleRef = el;
    },
    toggleHistory: () => {
      setIsHistoryOpen((previous) => !previous);
    },
    closeHistory,
    clearHistory,
    deleteHistoryEntry,
    selectHistoryEntry: (term) => {
      options.onChange(term);
      commitSearchToHistory(term);
      setIsHistoryOpen(false);
      setAcceptedSuggestionId(undefined);
      options.focusInput();
    },
    onValueChange: (value) => {
      options.onChange(value);
      setAcceptedSuggestionId(undefined);
      setShowInlineCompletion(true);
      setIsHistoryOpen(false);
    },
    onFieldKeyDown: (event) => {
      const cursorAtEnd =
        event.currentTarget.selectionStart === options.value().length &&
        event.currentTarget.selectionEnd === options.value().length;

      if (
        completionSuffix() &&
        (event.key === 'Tab' || (event.key === 'ArrowRight' && cursorAtEnd))
      ) {
        event.preventDefault();
        acceptInlineCompletion();
        return;
      }

      if (event.key === 'Enter') {
        const exact = exactSuggestion();
        if (exact) {
          event.preventDefault();
          applyExactSuggestion(exact, event.currentTarget);
          return;
        }
        const query = options.value().trim();
        const matches = rankedSuggestions();
        if (query && matches.length > 0 && options.suggestions?.onCommitQuery) {
          event.preventDefault();
          closeHistory();
          options.suggestions.onCommitQuery(query, matches);
          options.onChange('');
          setAcceptedSuggestionId(undefined);
          options.focusInput();
          return;
        }
        commitSearchToHistory(event.currentTarget.value);
        closeHistory();
        event.currentTarget.blur();
      } else if (hasHistory() && event.key === 'ArrowDown' && searchHistory().length > 0) {
        event.preventDefault();
        setIsHistoryOpen(true);
      } else if (event.key === 'ArrowLeft' || event.key === 'Home') {
        setShowInlineCompletion(false);
      }
      options.onFieldKeyDown?.(event);
    },
    onFieldFocus: (event) => {
      setIsFieldFocused(true);
      setShowInlineCompletion(
        event.currentTarget.selectionStart === event.currentTarget.value.length,
      );
    },
    onFieldBlur: (event) => {
      setIsFieldFocused(false);
      if (suppressBlurCommit) return;
      const next = event.relatedTarget as HTMLElement | null;
      const interactingWithHistory = next
        ? historyMenuRef?.contains(next) || historyToggleRef?.contains(next)
        : false;
      const interactingWithTips = tipsPopoverId()
        ? next?.getAttribute('aria-controls') === tipsPopoverId()
        : false;
      if (!interactingWithHistory && !interactingWithTips) {
        commitSearchToHistory(event.currentTarget.value);
      }
    },
  };
};
