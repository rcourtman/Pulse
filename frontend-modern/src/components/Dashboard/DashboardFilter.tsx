import { Component, Show, createEffect, createSignal, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';

interface DashboardFilterProps {
  search: () => string;
  setSearch: (value: string) => void;
  isSearchLocked: () => boolean;
  viewMode: () => 'all' | 'vm' | 'lxc';
  setViewMode: (value: 'all' | 'vm' | 'lxc') => void;
  statusMode: () => 'all' | 'running' | 'stopped';
  setStatusMode: (value: 'all' | 'running' | 'stopped') => void;
  groupingMode: () => 'grouped' | 'flat';
  setGroupingMode: (value: 'grouped' | 'flat') => void;
  setSortKey: (value: string) => void;
  setSortDirection: (value: string) => void;
  searchInputRef?: (el: HTMLInputElement) => void;
}

export const DashboardFilter: Component<DashboardFilterProps> = (props) => {
  const [showSearchHelp, setShowSearchHelp] = createSignal(false);
  let helpPopoverRef: HTMLDivElement | undefined;
  let helpButtonRef: HTMLButtonElement | undefined;

  const closeSearchHelp = () => setShowSearchHelp(false);

  createEffect(() => {
    if (!showSearchHelp()) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      const clickedInsidePopover = helpPopoverRef?.contains(target) ?? false;
      const clickedHelpButton = helpButtonRef?.contains(target) ?? false;

      if (!clickedInsidePopover && !clickedHelpButton) {
        closeSearchHelp();
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        closeSearchHelp();
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);

    onCleanup(() => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    });
  });

  return (
    <Card class="dashboard-filter mb-3" padding="sm">
      <div class="flex flex-col lg:flex-row gap-3">
        {/* Search Bar */}
        <div class="flex gap-2 flex-1 items-center">
          <div class="relative flex-1">
            <input
              ref={props.searchInputRef}
              type="text"
              placeholder="Search or filter..."
              value={props.search()}
              onInput={(e) => props.setSearch(e.currentTarget.value)}
              class={`w-full pl-9 pr-9 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg
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
                class="absolute right-3 top-2 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                onClick={() => props.setSearch('')}
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
          </div>

          {/* Search Help Popover */}
          <div class="relative flex-shrink-0">
            <button
              type="button"
              ref={(el) => (helpButtonRef = el)}
              class="flex items-center gap-1 rounded-md border border-gray-200 px-2.5 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500/40 focus:ring-offset-1 focus:ring-offset-white dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700 dark:focus:ring-blue-400/40 dark:focus:ring-offset-gray-900"
              onClick={() => setShowSearchHelp((value) => !value)}
              aria-expanded={showSearchHelp()}
              aria-controls="dashboard-search-help"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 2a7 7 0 00-7 7c0 2.8 1.5 4.6 3 5.8.6.5 1 1.1 1 1.8V17a1 1 0 001 1h4a1 1 0 001-1v-.4c0-.7.3-1.3.9-1.8 1.5-1.2 3.1-3 3.1-5.8a7 7 0 00-7-7z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M10 21h4"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 17v4"
                />
              </svg>
              <span>Search tips</span>
            </button>

            <Show when={showSearchHelp()}>
              <div
                ref={(el) => (helpPopoverRef = el)}
                id="dashboard-search-help"
                role="dialog"
                aria-label="Search tips"
                class="absolute right-0 z-50 mt-2 w-72 overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-xl dark:border-gray-600 dark:bg-gray-800"
              >
                <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-gray-700">
                  <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">Search tips</span>
                  <button
                    type="button"
                    class="rounded p-1 text-gray-400 transition-colors hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
                    onClick={closeSearchHelp}
                    aria-label="Close search tips"
                  >
                    <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
                <div class="px-3 py-3 text-xs text-gray-600 dark:text-gray-300">
                  <p class="mb-3 text-[11px] uppercase tracking-wide text-gray-400 dark:text-gray-500">Combine filters to narrow results</p>
                  <div class="space-y-2">
                    <div class="flex items-start gap-2">
                      <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">media</code>
                      <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">Guests with "media" in the name</span>
                    </div>
                    <div class="flex items-start gap-2">
                      <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">cpu&gt;80</code>
                      <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">Highlight guests using more than 80% CPU</span>
                    </div>
                    <div class="flex items-start gap-2">
                      <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">memory&lt;20</code>
                      <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">Find guests under 20% memory usage</span>
                    </div>
                    <div class="flex items-start gap-2">
                      <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">tags:prod</code>
                      <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">Filter by tag</span>
                    </div>
                    <div class="flex items-start gap-2">
                      <code class="rounded bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-gray-700 dark:text-gray-100">node:pve1</code>
                      <span class="text-[12px] leading-snug text-gray-500 dark:text-gray-400">Show guests on a specific node</span>
                    </div>
                  </div>
                  <div class="mt-3 rounded-md bg-blue-50 px-3 py-2 text-[11px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                    Try `node:pve1 cpu&gt;60` to stack filters.
                  </div>
                </div>
              </div>
            </Show>
          </div>
        </div>

        {/* Filters */}
        <div class="flex flex-wrap items-center gap-2">
          {/* Type Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setViewMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'vm' ? 'all' : 'vm')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'vm'
                  ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              VMs
            </button>
            <button
              type="button"
              onClick={() => props.setViewMode(props.viewMode() === 'lxc' ? 'all' : 'lxc')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.viewMode() === 'lxc'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              LXCs
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Status Filter */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setStatusMode('all')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              All
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'running' ? 'all' : 'running')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'running'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Running
            </button>
            <button
              type="button"
              onClick={() => props.setStatusMode(props.statusMode() === 'stopped' ? 'all' : 'stopped')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.statusMode() === 'stopped'
                  ? 'bg-white dark:bg-gray-800 text-red-600 dark:text-red-400 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Stopped
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Grouping Mode Toggle */}
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
            <button
              type="button"
              onClick={() => props.setGroupingMode('grouped')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupingMode() === 'grouped'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
              title="Group by node"
            >
              Grouped
            </button>
            <button
              type="button"
              onClick={() => props.setGroupingMode(props.groupingMode() === 'flat' ? 'grouped' : 'flat')}
              class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                props.groupingMode() === 'flat'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
              title="Flat list view"
            >
              List
            </button>
          </div>

          <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block"></div>

          {/* Reset Button */}
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
            class={`flex items-center justify-center px-2.5 py-1 text-xs font-medium rounded-lg transition-colors ${
              props.search() ||
              props.viewMode() !== 'all' ||
              props.statusMode() !== 'all' ||
              props.groupingMode() !== 'grouped'
                ? 'text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70'
                : 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600'
            }`}
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
            <span class="ml-1 hidden sm:inline">Reset</span>
          </button>
        </div>
      </div>
    </Card>
  );
};
