import { createSignal, onMount, createEffect, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';
import { createSearchHistoryManager } from '@/utils/searchHistory';

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
  isSimple: Accessor<boolean>;
  searchHistory: Accessor<string[]>;
  isHistoryOpen: Accessor<boolean>;
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
  onFieldKeyDown: (event: SearchInputKeyboardEvent) => void;
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
  const isSimple = () => !hasHistory() && !hasTips();
  const tipsPopoverId = () => options.tips?.popoverId ?? '';
  const emptyHistoryMessage = () =>
    options.history?.emptyMessage ?? 'Searches you run will appear here.';

  const historyManager = options.history
    ? createSearchHistoryManager(options.history.storageKey)
    : null;

  const [searchHistory, setSearchHistory] = createSignal<string[]>([]);
  const [isHistoryOpen, setIsHistoryOpen] = createSignal(false);

  let historyMenuRef: HTMLDivElement | undefined;
  let historyToggleRef: HTMLButtonElement | undefined;
  let suppressBlurCommit = false;

  onMount(() => {
    if (historyManager) {
      setSearchHistory(historyManager.read());
    }
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
    if (!clickedMenu && !clickedToggle) {
      closeHistory();
    }
  };

  createEffect(() => {
    if (isHistoryOpen()) {
      document.addEventListener('mousedown', handleDocumentClick);
    } else {
      document.removeEventListener('mousedown', handleDocumentClick);
    }
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleDocumentClick);
  });

  return {
    hasHistory,
    hasTips,
    isSimple,
    searchHistory,
    isHistoryOpen,
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
      setIsHistoryOpen((prev) => {
        const next = !prev;
        if (!next) {
          queueMicrotask(() => historyToggleRef?.blur());
        }
        return next;
      });
    },
    closeHistory,
    clearHistory,
    deleteHistoryEntry,
    selectHistoryEntry: (term) => {
      options.onChange(term);
      commitSearchToHistory(term);
      setIsHistoryOpen(false);
      options.focusInput();
    },
    onFieldKeyDown: (e) => {
      if (hasHistory()) {
        if (e.key === 'Enter') {
          commitSearchToHistory(e.currentTarget.value);
          closeHistory();
        } else if (e.key === 'ArrowDown' && searchHistory().length > 0) {
          e.preventDefault();
          setIsHistoryOpen(true);
        }
      }
      options.onFieldKeyDown?.(e);
    },
    onFieldBlur: (e) => {
      if (!hasHistory()) return;
      if (suppressBlurCommit) return;
      const next = e.relatedTarget as HTMLElement | null;
      const interactingWithHistory = next
        ? historyMenuRef?.contains(next) || historyToggleRef?.contains(next)
        : false;
      const interactingWithTips = tipsPopoverId()
        ? next?.getAttribute('aria-controls') === tipsPopoverId()
        : false;
      if (!interactingWithHistory && !interactingWithTips) {
        commitSearchToHistory(e.currentTarget.value);
      }
    },
  };
};
