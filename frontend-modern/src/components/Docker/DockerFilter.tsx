import { Component, Show, For, createSignal, createMemo, onMount, createEffect, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import { MetricsViewToggle } from '@/components/shared/MetricsViewToggle';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { createSearchHistoryManager } from '@/utils/searchHistory';

interface DockerFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  groupingMode?: () => 'grouped' | 'flat';
  setGroupingMode?: (mode: 'grouped' | 'flat') => void;
  statusFilter?: () => 'all' | 'online' | 'degraded' | 'offline';
  setStatusFilter?: (value: 'all' | 'online' | 'degraded' | 'offline') => void;
  searchInputRef?: (el: HTMLInputElement) => void;
  onReset?: () => void;
  activeHostName?: string;
  onClearHost?: () => void;
  updateAvailableCount?: number;
  onUpdateAll?: () => void;
}

const UpdateAllButton: Component<{ count: number; onUpdate: () => void }> = (props) => {
  const [confirming, setConfirming] = createSignal(false);

  return (
    <button
      type="button"
      onClick={() => {
        if (confirming()) {
          props.onUpdate();
          setConfirming(false);
        } else {
          setConfirming(true);
          // Auto-reset confirmation after 3s
          setTimeout(() => setConfirming(false), 3000);
        }
      }}
      class={`flex items-center gap-1.5 px-3 py-1 text-xs font-medium rounded-lg transition-colors ${confirming()
        ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/60 dark:text-amber-200 hover:bg-amber-200'
        : 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-200 hover:bg-blue-200'
        }`}
      title={confirming() ? "Click again to confirm" : `Update ${props.count} containers`}
    >
      <Show when={!confirming()} fallback={
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      }>
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
        </svg>
      </Show>
      <span>{confirming() ? 'Confirm Update All?' : `Update All (${props.count})`}</span>
    </button>
  );
};

export const DockerFilter: Component<DockerFilterProps> = (props) => {
  const historyManager = createSearchHistoryManager(STORAGE_KEYS.DOCKER_SEARCH_HISTORY);
  const [searchHistory, setSearchHistory] = createSignal<string[]>([]);
  const [isHistoryOpen, setIsHistoryOpen] = createSignal(false);

  let searchInputEl: HTMLInputElement | undefined;
  let historyMenuRef: HTMLDivElement | undefined;
  let historyToggleRef: HTMLButtonElement | undefined;

  onMount(() => {
    setSearchHistory(historyManager.read());
  });

  const commitSearchToHistory = (term: string) => {
    const trimmed = term.trim();
    if (!trimmed) return;
    const updated = historyManager.add(trimmed);
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

  const hasActiveFilters = createMemo(
    () =>
      props.search().trim() !== '' ||
      (!!props.groupingMode && props.groupingMode() === 'flat') ||
      (!!props.statusFilter && props.statusFilter() !== 'all') ||
      Boolean(props.activeHostName),
  );

  const handleReset = () => {
    props.setSearch('');
    props.setGroupingMode?.('grouped');
    props.setStatusFilter?.('all');
    props.onClearHost?.();
    props.onReset?.();
    closeHistory();
    focusSearchInput();
  };

  return (
    <Card class="docker-filter mb-3" padding="sm">
      <div class="flex flex-col gap-3">
        {/* Search - full width on its own row */}
        <div class="relative">
          <input
            ref={(el) => {
              searchInputEl = el;
              props.searchInputRef?.(el);
            }}
            type="text"
            placeholder="Search containers or host:hostname"
            value={props.search()}
            onInput={(e) => props.setSearch(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                commitSearchToHistory(e.currentTarget.value);
                closeHistory();
              } else if (e.key === 'Escape') {
                props.setSearch('');
                closeHistory();
                e.currentTarget.blur();
              } else if (e.key === 'ArrowDown' && searchHistory().length > 0) {
                e.preventDefault();
                setIsHistoryOpen(true);
              }
            }}
            onBlur={(e) => {
              if (suppressBlurCommit) return;
              const next = e.relatedTarget as HTMLElement | null;
              const interactingWithHistory = next
                ? historyMenuRef?.contains(next) || historyToggleRef?.contains(next)
                : false;
              const interactingWithTips =
                next?.getAttribute('aria-controls') === 'container-search-help';
              if (!interactingWithHistory && !interactingWithTips) {
                commitSearchToHistory(e.currentTarget.value);
              }
            }}
            class="w-full pl-9 pr-16 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
            title="Search containers by name, image, ID, or host"
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
              class="absolute right-9 top-1/2 -translate-y-1/2 transform text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
              onClick={() => props.setSearch('')}
              onMouseDown={markSuppressCommit}
              aria-label="Clear search"
              title="Clear search"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </Show>
          <div class="absolute inset-y-0 right-2 flex items-center gap-1">
            <button
              ref={(el) => (historyToggleRef = el)}
              type="button"
              class="flex h-6 w-6 items-center justify-center rounded-lg border border-transparent text-gray-400 transition-colors hover:border-gray-200 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500/40 focus:ring-offset-1 focus:ring-offset-white dark:text-gray-500 dark:hover:border-gray-700 dark:hover:text-gray-200 dark:focus:ring-blue-400/40 dark:focus:ring-offset-gray-900"
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
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4l2.5 1.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <span class="sr-only">Show search history</span>
            </button>
            <SearchTipsPopover
              popoverId="container-search-help"
              intro="Filter containers quickly"
              tips={[
                { code: 'name:api', description: 'Match containers with "api" in the name' },
                { code: 'image:postgres', description: 'Find containers running a specific image' },
                { code: 'host:prod', description: 'Show containers on a host' },
                { code: 'state:running', description: 'Running containers only' },
              ]}
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

        {/* Filters - second row */}
        <div class="flex flex-wrap items-center gap-2">
          <Show when={props.statusFilter && props.setStatusFilter}>
            <div class="inline-flex rounded-lg bg-gray-100 p-0.5 dark:bg-gray-700">
              <button
                type="button"
                aria-pressed={props.statusFilter?.() === 'all'}
                onClick={() => props.setStatusFilter?.('all')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter?.() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                title="Show all hosts"
              >
                All
              </button>
              <button
                type="button"
                aria-pressed={props.statusFilter?.() === 'online'}
                onClick={() =>
                  props.setStatusFilter?.(props.statusFilter?.() === 'online' ? 'all' : 'online')
                }
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter?.() === 'online'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                title="Show online hosts only"
              >
                Online
              </button>
              <button
                type="button"
                aria-pressed={props.statusFilter?.() === 'degraded'}
                onClick={() =>
                  props.setStatusFilter?.(
                    props.statusFilter?.() === 'degraded' ? 'all' : 'degraded',
                  )
                }
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter?.() === 'degraded'
                  ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                title="Show degraded hosts only"
              >
                Degraded
              </button>
              <button
                type="button"
                aria-pressed={props.statusFilter?.() === 'offline'}
                onClick={() =>
                  props.setStatusFilter?.(
                    props.statusFilter?.() === 'offline' ? 'all' : 'offline',
                  )
                }
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.statusFilter?.() === 'offline'
                  ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                title="Show offline hosts only"
              >
                Offline
              </button>
            </div>
          </Show>

          <Show when={props.groupingMode && props.setGroupingMode}>
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => props.setGroupingMode?.('grouped')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupingMode?.() === 'grouped'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Grouped
              </button>
              <button
                type="button"
                onClick={() => props.setGroupingMode?.('flat')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${props.groupingMode?.() === 'flat'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                List
              </button>
            </div>
          </Show>

          <Show when={props.activeHostName}>
            <div class="flex items-center gap-1 rounded-full bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
              <span>Host: {props.activeHostName}</span>
              <button
                type="button"
                class="text-blue-500 hover:text-blue-700 dark:text-blue-300 dark:hover:text-blue-100"
                onClick={() => props.onClearHost?.()}
                title="Clear host filter"
              >
                ×
              </button>
            </div>
          </Show>

          {/* Metrics View Toggle */}
          <MetricsViewToggle />

          <Show when={props.updateAvailableCount && props.updateAvailableCount > 1}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" aria-hidden="true"></div>
            <UpdateAllButton
              count={props.updateAvailableCount!}
              onUpdate={props.onUpdateAll!}
            />
          </Show>

          <Show when={hasActiveFilters()}>
            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" aria-hidden="true"></div>
            <button
              type="button"
              onClick={handleReset}
              class="flex items-center justify-center gap-1 px-2.5 py-1 text-xs font-medium rounded-lg text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70 transition-colors"
              title="Reset filters"
            >
              <svg
                width="14"
                height="14"
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
              <span class="hidden sm:inline">Reset</span>
            </button>
          </Show>
        </div>
      </div>
    </Card >
  );
};
