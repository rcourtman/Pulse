import { Component, Show, For, createSignal, onMount, createEffect, onCleanup } from 'solid-js';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import { createSearchHistoryManager } from '@/utils/searchHistory';

interface SearchTip {
  code: string;
  description: string;
}

export interface SearchInputProps {
  value: () => string;
  onChange: (value: string) => void;
  placeholder?: string;
  title?: string;
  history?: {
    storageKey: string;
    emptyMessage?: string;
  };
  tips?: {
    popoverId: string;
    intro: string;
    tips: SearchTip[];
    footerHighlight?: string;
    footerText?: string;
  };
  inputRef?: (el: HTMLInputElement) => void;
  class?: string;
  /** When true, typing any printable character while not in an input field will auto-focus this search input. */
  autoFocus?: boolean;
  /** Called before auto-focus — return true to prevent focus (e.g. when AI chat should capture input instead). */
  onBeforeAutoFocus?: () => boolean;
}

export const SearchInput: Component<SearchInputProps> = (props) => {
  const hasHistory = () => !!props.history;
  const hasTips = () => !!props.tips;

  const historyManager = props.history
    ? createSearchHistoryManager(props.history.storageKey)
    : null;

  const [searchHistory, setSearchHistory] = createSignal<string[]>([]);
  const [isHistoryOpen, setIsHistoryOpen] = createSignal(false);

  let searchInputEl: HTMLInputElement | undefined;
  let historyMenuRef: HTMLDivElement | undefined;
  let historyToggleRef: HTMLButtonElement | undefined;

  onMount(() => {
    if (historyManager) {
      setSearchHistory(historyManager.read());
    }
  });

  const commitSearchToHistory = (term: string) => {
    if (!historyManager) return;
    const updated = historyManager.add(term);
    setSearchHistory(updated);
  };

  const deleteHistoryEntry = (term: string) => {
    if (!historyManager) return;
    setSearchHistory(historyManager.remove(term));
  };

  const clearHistory = () => {
    if (!historyManager) return;
    setSearchHistory(historyManager.clear());
    setIsHistoryOpen(false);
    queueMicrotask(() => historyToggleRef?.blur());
  };

  const closeHistory = () => {
    setIsHistoryOpen(false);
    queueMicrotask(() => historyToggleRef?.blur());
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

  // Auto-focus on printable character keypress (opt-in)
  if (props.autoFocus) {
    const handleAutoFocusKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isEditable =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.isContentEditable;
      if (isEditable) return;
      if (e.key.length !== 1 || e.ctrlKey || e.metaKey || e.altKey) return;

      if (props.onBeforeAutoFocus?.()) return;

      // Prevent default so the browser doesn't try to insert the character
      // into the previously-focused element. We manually append it below.
      e.preventDefault();
      searchInputEl?.focus();
      props.onChange(props.value() + e.key);
    };
    document.addEventListener('keydown', handleAutoFocusKey);
    onCleanup(() => document.removeEventListener('keydown', handleAutoFocusKey));
  }

  const focusSearchInput = () => {
    queueMicrotask(() => searchInputEl?.focus());
  };

  let suppressBlurCommit = false;

  const markSuppressCommit = () => {
    suppressBlurCommit = true;
    queueMicrotask(() => {
      suppressBlurCommit = false;
    });
  };

  const tipsPopoverId = () => props.tips?.popoverId ?? '';

  // Simple mode: no history, no tips — just icon + clear button
  const isSimple = () => !hasHistory() && !hasTips();

  // Compute right padding based on what controls are visible
  const inputPaddingRight = () => {
    if (isSimple()) return 'pr-8';
    return 'pr-14 sm:pr-20';
  };

  return (
    <div class={`relative w-full ${props.class ?? ''}`}>
      <input
        ref={(el) => {
          searchInputEl = el;
          props.inputRef?.(el);
        }}
        type="text"
        placeholder={props.placeholder ?? 'Search...'}
        value={props.value()}
        onInput={(e) => props.onChange(e.currentTarget.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            if (props.value()) {
              props.onChange('');
            }
            searchInputEl?.blur();
            return;
          }
          if (hasHistory()) {
            if (e.key === 'Enter') {
              commitSearchToHistory(e.currentTarget.value);
              closeHistory();
            } else if (e.key === 'ArrowDown' && searchHistory().length > 0) {
              e.preventDefault();
              setIsHistoryOpen(true);
            }
          }
        }}
        onBlur={(e) => {
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
        }}
        class={`w-full pl-8 sm:pl-9 ${inputPaddingRight()} py-1.5 sm:py-2 text-sm border border-slate-300 dark:border-slate-600 rounded-md
               bg-white dark:bg-slate-900 text-base-content placeholder-muted
               focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all`}
        title={props.title}
        data-global-search
      />
      {/* Magnifying card icon */}
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
      {/* Clear button */}
      <Show when={props.value()}>
        <button
          type="button"
          class={`absolute top-1/2 -translate-y-1/2 transform p-1 rounded-full
                 bg-slate-200 dark:bg-slate-600 text-slate-500 dark:text-slate-300
                 hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900 dark:hover:text-red-400
                 transition-all duration-150 active:scale-90 ${isSimple() ? 'right-2' : 'right-12 sm:right-14'}`}
          onClick={() => props.onChange('')}
          onMouseDown={hasHistory() ? markSuppressCommit : undefined}
          aria-label="Clear search"
          title="Clear search"
        >
          <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </Show>
      {/* History toggle + tips icon (full mode only) */}
      <Show when={!isSimple()}>
        <div class="absolute inset-y-0 right-2 flex items-center gap-1">
          <Show when={hasHistory()}>
            <button
              ref={(el) => (historyToggleRef = el)}
              type="button"
              class={`flex h-7 w-7 items-center justify-center rounded-md transition-colors
                     ${isHistoryOpen()
                  ? 'bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400'
                  : 'text-muted hover:bg-surface-hover hover:text-slate-600 dark:hover:text-slate-300'
                }`}
              onClick={() =>
                setIsHistoryOpen((prev) => {
                  const next = !prev;
                  if (!next) {
                    queueMicrotask(() => historyToggleRef?.blur());
                  }
                  return next;
                })
              }
              onMouseDown={markSuppressCommit}
              aria-haspopup="listbox"
              aria-expanded={isHistoryOpen()}
              title={
                searchHistory().length > 0
                  ? 'Show recent searches'
                  : 'No recent searches yet'
              }
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
              <span class="sr-only">Show search history</span>
            </button>
          </Show>
          <Show when={hasTips()}>
            <SearchTipsPopover
              popoverId={props.tips!.popoverId}
              intro={props.tips!.intro}
              tips={props.tips!.tips}
              footerHighlight={props.tips!.footerHighlight}
              footerText={props.tips!.footerText}
              triggerVariant="icon"
              buttonLabel="Search tips"
              openOnHover
            />
          </Show>
        </div>
      </Show>
      {/* History dropdown */}
      <Show when={hasHistory() && isHistoryOpen()}>
        <div
          ref={(el) => (historyMenuRef = el)}
          class="absolute left-0 right-0 top-full z-50 mt-2 w-full overflow-hidden rounded-md border border-slate-200 bg-white text-sm shadow-sm dark:border-slate-700 dark:bg-slate-800"
          role="listbox"
        >
          <Show
            when={searchHistory().length > 0}
            fallback={
              <div class="px-3 py-2 text-xs text-muted">
                {props.history?.emptyMessage ?? 'Searches you run will appear here.'}
              </div>
            }
          >
            <div class="max-h-52 overflow-y-auto py-1">
              <For each={searchHistory()}>
                {(entry) => (
                  <div class="flex items-center justify-between px-2 py-1.5 hover:bg-blue-50 dark:hover:bg-blue-900">
                    <button
                      type="button"
                      class="flex-1 truncate pr-2 text-left text-sm text-slate-700 transition-colors hover:text-blue-600 focus:outline-none dark:text-slate-200 dark:hover:text-blue-300"
                      onClick={() => {
                        props.onChange(entry);
                        commitSearchToHistory(entry);
                        setIsHistoryOpen(false);
                        focusSearchInput();
                      }}
                      onMouseDown={markSuppressCommit}
                    >
                      {entry}
                    </button>
                    <button
                      type="button"
                      class="ml-1 flex h-6 w-6 items-center justify-center rounded text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 focus:outline-none dark:text-slate-500 dark:hover:bg-slate-700 dark:hover:text-slate-200"
                      title="Remove from history"
                      onClick={() => deleteHistoryEntry(entry)}
                      onMouseDown={markSuppressCommit}
                    >
                      <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                )}
              </For>
            </div>
            <button
              type="button"
              class="flex w-full items-center justify-center gap-2 border-t border-slate-200 px-3 py-2 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-700 dark:hover:text-slate-200"
              onClick={clearHistory}
              onMouseDown={markSuppressCommit}
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4a1 1 0 011-1h4a1 1 0 011 1v3m-9 0h12"
                />
              </svg>
              Clear history
            </button>
          </Show>
        </div>
      </Show>
    </div>
  );
};
