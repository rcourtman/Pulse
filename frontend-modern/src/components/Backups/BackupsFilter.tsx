import { Component, Show, For, createSignal, onMount, createEffect, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { createSearchHistoryManager } from '@/utils/searchHistory';

interface BackupsFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  viewMode: () => 'all' | 'snapshot' | 'pve' | 'pbs';
  setViewMode: (value: 'all' | 'snapshot' | 'pve' | 'pbs') => void;
  groupBy: () => 'date' | 'guest';
  setGroupBy: (value: 'date' | 'guest') => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  sortKey: () => string;
  setSortKey: (value: string) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortOptions?: { value: string; label: string }[];
  onReset?: () => void;
  // Column visibility (optional)
  columnVisibility?: {
    availableToggles: () => ColumnDef[];
    isHiddenByUser: (id: string) => boolean;
    toggle: (id: string) => void;
    resetToDefaults: () => void;
  };
}

export const BackupsFilter: Component<BackupsFilterProps> = (props) => {
  const sortOptions = props.sortOptions ?? [
    { value: 'backupTime', label: 'Time' },
    { value: 'name', label: 'Name' },
    { value: 'vmid', label: 'VMID' },
    { value: 'size', label: 'Size' },
  ];
  const historyManager = createSearchHistoryManager(STORAGE_KEYS.BACKUPS_SEARCH_HISTORY);
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

  const hasActiveFilters = () =>
    props.search().trim() !== '' ||
    props.viewMode() !== 'all' ||
    props.groupBy() !== 'date' ||
    props.sortKey() !== 'backupTime' ||
    props.sortDirection() !== 'desc';

  return (
    <Card class="backups-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Row 1: Search Bar - Full width */}
        <div class="relative w-full">
          <input
            ref={(el) => {
              searchInputEl = el;
              props.searchInputRef?.(el);
            }}
            type="text"
            placeholder="Search backups... (e.g., media, vm-104, node:pve1)"
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
                next?.getAttribute('aria-controls') === 'backups-search-help';
              if (!interactingWithHistory && !interactingWithTips) {
                commitSearchToHistory(e.currentTarget.value);
              }
            }}
            class="w-full pl-8 sm:pl-9 pr-14 sm:pr-20 py-1.5 sm:py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                   bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                   focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
            title="Search backups by name, VMID, or filter by node"
          />
          <svg
            class="absolute left-2.5 sm:left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400 dark:text-gray-500"
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
              class="absolute right-12 sm:right-14 top-1/2 -translate-y-1/2 transform p-1 rounded-full 
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
          <div class="absolute inset-y-0 right-2 flex items-center gap-1">
            <button
              ref={(el) => (historyToggleRef = el)}
              type="button"
              class={`flex h-7 w-7 items-center justify-center rounded-md transition-colors 
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
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
              <span class="sr-only">Show search history</span>
            </button>
            <SearchTipsPopover
              popoverId="backups-search-help"
              intro="Quick examples"
              tips={[
                { code: 'media', description: 'Backups with "media" in the name' },
                { code: 'node:pve1', description: 'Show backups from a specific node' },
                { code: 'vm-104', description: 'Locate backups for VM 104' },
              ]}
              footerHighlight="node:pve1 vm-104"
              footerText="Mix terms to focus on the exact backups you need."
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
                    Your recent backup searches will show here.
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
                          class="ml-1 flex h-6 w-6 items-center justify-center rounded text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none dark:text-gray-500 dark:hover:bg-gray-700/70 dark:hover:text-gray-200"
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
                  class="flex w-full items-center justify-center gap-2 border-t border-gray-200 px-3 py-2 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-700/80 dark:hover:text-gray-200"
                  onClick={clearHistory}
                  onMouseDown={markSuppressCommit}
                >
                  Clear history
                </button>
              </Show>
            </div>
          </Show>
        </div>

        {/* Row 2: Filters - Compact horizontal layout */}
        <div class="flex flex-wrap items-center gap-1.5 sm:gap-2">
          {/* Source Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setViewMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'all'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'snapshot' ? 'all' : 'snapshot')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'snapshot'
                ? 'bg-white dark:bg-gray-800 text-yellow-600 dark:text-yellow-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              Snapshots
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pve' ? 'all' : 'pve')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'pve'
                ? 'bg-white dark:bg-gray-800 text-orange-600 dark:text-orange-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              PVE
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'pbs' ? 'all' : 'pbs')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.viewMode() === 'pbs'
                ? 'bg-white dark:bg-gray-800 text-purple-600 dark:text-purple-400 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              PBS
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Group By Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupBy('date')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy() === 'date'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              By Date
            </button>
            <button
              type="button"
              onClick={() => props.setGroupBy(props.groupBy() === 'guest' ? 'date' : 'guest')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupBy() === 'guest'
                ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                }`}
            >
              By Guest
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Sort controls */}
          <div class="flex items-center gap-1.5">
            <select
              value={props.sortKey()}
              onChange={(e) => props.setSortKey(e.currentTarget.value)}
              class="px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
            >
              {sortOptions.map((option) => (
                <option value={option.value}>{option.label}</option>
              ))}
            </select>
            <button
              type="button"
              title={`Sort ${props.sortDirection() === 'asc' ? 'descending' : 'ascending'}`}
              onClick={() =>
                props.setSortDirection(props.sortDirection() === 'asc' ? 'desc' : 'asc')
              }
              class="inline-flex items-center justify-center h-7 w-7 rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            >
              <svg
                class={`h-4 w-4 transition-transform ${props.sortDirection() === 'asc' ? 'rotate-180' : ''}`}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M8 9l4-4 4 4m0 6l-4 4-4-4"
                />
              </svg>
            </button>
          </div>

          {/* Column Picker - Only if provided */}
          <Show when={props.columnVisibility}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <ColumnPicker
              columns={props.columnVisibility!.availableToggles()}
              isHidden={props.columnVisibility!.isHiddenByUser}
              onToggle={props.columnVisibility!.toggle}
              onReset={props.columnVisibility!.resetToDefaults}
            />
          </Show>

          {/* Reset Button - Only show when filters are active */}
          <Show when={hasActiveFilters()}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>
            <button
              onClick={() => {
                if (props.onReset) {
                  props.onReset();
                } else {
                  props.setSearch('');
                  props.setViewMode('all');
                  props.setGroupBy('date');
                }
              }}
              title="Reset all filters"
              class="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg transition-colors
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
