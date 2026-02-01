import { Component, Show, For, createSignal, onMount, createEffect, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import { MetricsViewToggle } from '@/components/shared/MetricsViewToggle';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { createSearchHistoryManager } from '@/utils/searchHistory';

interface DashboardFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  isSearchLocked: () => boolean;
  viewMode: () => 'all' | 'vm' | 'lxc';
  setViewMode: (value: 'all' | 'vm' | 'lxc') => void;
  statusMode: () => 'all' | 'running' | 'degraded' | 'stopped';
  setStatusMode: (value: 'all' | 'running' | 'degraded' | 'stopped') => void;
  groupingMode: () => 'grouped' | 'flat';
  setGroupingMode: (value: 'grouped' | 'flat') => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: string) => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  // Column visibility
  availableColumns?: ColumnDef[];
  isColumnHidden?: (id: string) => boolean;
  onColumnToggle?: (id: string) => void;
  onColumnReset?: () => void;
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  const historyManager = createSearchHistoryManager(STORAGE_KEYS.DASHBOARD_SEARCH_HISTORY);
  const [searchHistory, setSearchHistory] = createSignal<string[]>([]);
  const [isHistoryOpen, setIsHistoryOpen] = createSignal(false);

  let searchInputEl: HTMLInputElement | undefined;
  let historyMenuRef: HTMLDivElement | undefined;
  let historyToggleRef: HTMLButtonElement | undefined;

  onMount(() => {
    setSearchHistory(historyManager.read());
  });

  const commitSearchToHistory = (term: string) => {
    const updated = historyManager.add(term);
    setSearchHistory(updated);
  };

  const deleteHistoryEntry = (term: string) => {
    setSearchHistory(historyManager.remove(term));
  };

  const clearHistory = () => {
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

  return (
    <Card class="dashboard-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Row 1: Search Bar */}
        <div class="flex gap-2 items-center">
          <div class="relative flex-1 min-w-0">
            <input
              ref={(el) => {
                searchInputEl = el;
                props.searchInputRef?.(el);
              }}
              type="text"
              placeholder="Search or filter..."
              value={props.search()}
              onInput={(e) => props.setSearch(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  commitSearchToHistory(e.currentTarget.value);
                  closeHistory();
                } else if (e.key === 'ArrowDown' && searchHistory().length > 0) {
                  e.preventDefault();
                  setIsHistoryOpen(true);
                }
              }}
              onBlur={(e) => {
                if (suppressBlurCommit) {
                  return;
                }
                const next = e.relatedTarget as HTMLElement | null;
                const interactingWithHistory = next
                  ? historyMenuRef?.contains(next) || historyToggleRef?.contains(next)
                  : false;
                const interactingWithTips =
                  next?.getAttribute('aria-controls') === 'dashboard-search-help';
                if (!interactingWithHistory && !interactingWithTips) {
                  commitSearchToHistory(e.currentTarget.value);
                }
              }}
              class={`w-full pl-8 sm:pl-9 pr-14 sm:pr-16 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg
                     bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                     focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all`}
            />
            <svg
              class="absolute left-3 top-2 h-4 w-4 text-gray-400 dark:text-gray-500"
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
            <Show when={props.search()}>
              <button
                type="button"
                class="absolute right-[3.5rem] sm:right-[4.5rem] top-1/2 -translate-y-1/2 transform p-1 rounded-full 
                       bg-gray-200 dark:bg-gray-600 text-gray-500 dark:text-gray-300 
                       hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900/50 dark:hover:text-red-400 
                       transition-all duration-150 active:scale-90"
                onClick={() => props.setSearch('')}
                onMouseDown={markSuppressCommit}
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
            <div class="absolute inset-y-0 right-2 flex items-center gap-0.5">
              <button
                ref={(el) => (historyToggleRef = el)}
                type="button"
                class={`flex h-6 w-6 items-center justify-center rounded-md transition-colors 
                       ${isHistoryOpen()
                    ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400'
                    : 'text-gray-400 dark:text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-600 dark:hover:text-gray-300'
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
                {/* Dropdown chevron icon - clearer than clock */}
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
                <span class="sr-only">Show search history</span>
              </button>
              <SearchTipsPopover
                popoverId="dashboard-search-help"
                intro="Combine filters to narrow results"
                tips={[
                  { code: 'media', description: 'Guests with "media" in the name' },
                  { code: 'cpu>80', description: 'Highlight guests using more than 80% CPU' },
                  { code: 'memory<20', description: 'Find guests under 20% memory usage' },
                  { code: 'tags:prod', description: 'Filter by tag' },
                  { code: 'node:pve1', description: 'Show guests on a specific node' },
                ]}
                footerHighlight="node:pve1 cpu>60"
                footerText="Stack filters to get laser-focused results."
                triggerVariant="icon"
                buttonLabel="Search tips"
                openOnHover
              />
            </div>
            <Show when={isHistoryOpen()}>
              <div
                ref={(el) => (historyMenuRef = el)}
                class="absolute left-0 right-0 top-full z-50 mt-2 w-full overflow-hidden rounded-lg border border-gray-200 bg-white text-sm shadow-xl dark:border-gray-700 dark:bg-gray-800"
                role="listbox"
              >
                <Show
                  when={searchHistory().length > 0}
                  fallback={
                    <div class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">
                      Searches you run will appear here.
                    </div>
                  }
                >
                  <div class="max-h-52 overflow-y-auto py-1">
                    <For each={searchHistory()}>
                      {(entry) => (
                        <div class="flex items-center justify-between px-2 py-1.5 hover:bg-blue-50 dark:hover:bg-blue-900/20">
                          <button
                            type="button"
                            class="flex-1 truncate pr-2 text-left text-sm text-gray-700 transition-colors hover:text-blue-600 focus:outline-none dark:text-gray-200 dark:hover:text-blue-300"
                            onClick={() => {
                              props.setSearch(entry);
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
                            class="ml-1 flex h-6 w-6 items-center justify-center rounded text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500/40 focus:ring-offset-1 focus:ring-offset-white dark:text-gray-500 dark:hover:bg-gray-700/70 dark:hover:text-gray-200 dark:focus:ring-blue-400/40 dark:focus:ring-offset-gray-900"
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
                            <span class="sr-only">Remove from history</span>
                          </button>
                        </div>
                      )}
                    </For>
                  </div>
                  <button
                    type="button"
                    class="flex w-full items-center justify-center gap-2 border-t border-gray-200 px-3 py-2 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-700/80 dark:hover:text-gray-200"
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
        </div>

        {/* Row 2: All filters on one line */}
        <div class="flex flex-wrap items-center gap-x-1.5 sm:gap-x-2 gap-y-2">
          {/* Type Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setViewMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'vm' ? 'all' : 'vm')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'vm'
                ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm ring-1 ring-blue-200 dark:ring-blue-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="2" y="3" width="20" height="14" rx="2" />
                <path d="M8 21h8M12 17v4" />
              </svg>
              VMs
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'lxc' ? 'all' : 'lxc')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.viewMode() === 'lxc'
                ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
              </svg>
              LXCs
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Status Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setStatusMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'running' ? 'all' : 'running')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'running'
                ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'running' ? 'bg-green-500 shadow-sm shadow-green-500/50' : 'bg-green-400/60'}`} />
              Running
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'degraded' ? 'all' : 'degraded')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'degraded'
                ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm ring-1 ring-amber-200 dark:ring-amber-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'degraded' ? 'bg-amber-500 shadow-sm shadow-amber-500/50' : 'bg-amber-400/60'}`} />
              Degraded
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'stopped' ? 'all' : 'stopped')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.statusMode() === 'stopped'
                ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm ring-1 ring-red-200 dark:ring-red-800'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
            >
              <span class={`w-2 h-2 rounded-full ${props.statusMode() === 'stopped' ? 'bg-red-500 shadow-sm shadow-red-500/50' : 'bg-red-400/60'}`} />
              Stopped
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Grouping Mode Toggle */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupingMode('grouped')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.groupingMode() === 'grouped'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
              title="Group by node"
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
              </svg>
              Grouped
            </button>
            <button
              type="button"
              onClick={() => props.setGroupingMode(props.groupingMode() === 'flat' ? 'grouped' : 'flat')}
              class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${props.groupingMode() === 'flat'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
                }`}
              title="Flat list view"
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="8" y1="6" x2="21" y2="6" />
                <line x1="8" y1="12" x2="21" y2="12" />
                <line x1="8" y1="18" x2="21" y2="18" />
                <line x1="3" y1="6" x2="3.01" y2="6" />
                <line x1="3" y1="12" x2="3.01" y2="12" />
                <line x1="3" y1="18" x2="3.01" y2="18" />
              </svg>
              List
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Metrics View Toggle */}
          <MetricsViewToggle />

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Column Picker */}
          <Show when={props.availableColumns && props.isColumnHidden && props.onColumnToggle}>
            <ColumnPicker
              columns={props.availableColumns!}
              isHidden={props.isColumnHidden!}
              onToggle={props.onColumnToggle!}
              onReset={props.onColumnReset}
            />
          </Show>

          <Show when={props.availableColumns}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
          </Show>

          {/* Reset Button - Only show when filters are active */}
          <Show when={
            props.search().trim() !== '' ||
            props.viewMode() !== 'all' ||
            props.statusMode() !== 'all' ||
            props.groupingMode() !== 'grouped'
          }>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <button
              onClick={() => {
                props.setSearch('');
                props.setSortKey('vmid');
                props.setSortDirection('asc');
                props.setViewMode('all');
                props.setStatusMode('all');
                props.setGroupingMode('grouped');
              }}
              title="Reset all filters"
              class="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg transition-all duration-150 active:scale-95
                     text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70"
            >
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
                <path d="M21 3v5h-5" />
                <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
                <path d="M8 16H3v5" />
              </svg>
              Reset
            </button>
          </Show>
        </div>
      </div>
    </Card>
  );
};
