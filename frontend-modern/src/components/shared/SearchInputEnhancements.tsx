import { Component, For, Show } from 'solid-js';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import type {
  SearchInputEnhancementsState,
  SearchTipsConfig,
} from '@/components/shared/useSearchInputEnhancements';

interface SearchInputTrailingControlsProps {
  state: SearchInputEnhancementsState;
  tips?: SearchTipsConfig;
}

export const SearchInputTrailingControls: Component<SearchInputTrailingControlsProps> = (props) => (
  <>
    <Show when={props.state.hasHistory()}>
      <button
        ref={props.state.setHistoryToggleRef}
        type="button"
        class={`flex h-7 w-7 items-center justify-center rounded-md transition-colors
 ${
   props.state.isHistoryOpen()
     ? 'bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400'
     : 'text-muted hover:bg-surface-hover hover:text-base-content'
 }`}
        onClick={props.state.toggleHistory}
        onMouseDown={props.state.onClearMouseDown}
        aria-haspopup="listbox"
        aria-expanded={props.state.isHistoryOpen()}
        title={
          props.state.searchHistory().length > 0 ? 'Show recent searches' : 'No recent searches yet'
        }
      >
        <svg
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
        <span class="sr-only">Show search history</span>
      </button>
    </Show>
    <Show when={props.state.hasTips() && props.tips}>
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
  </>
);

interface SearchInputHistoryDropdownProps {
  state: SearchInputEnhancementsState;
}

export const SearchInputHistoryDropdown: Component<SearchInputHistoryDropdownProps> = (props) => (
  <Show when={props.state.hasHistory() && props.state.isHistoryOpen()}>
    <div
      ref={props.state.setHistoryMenuRef}
      class="absolute left-0 right-0 top-full z-50 mt-2 w-full overflow-hidden rounded-md border border-border bg-surface text-sm shadow-sm"
      role="listbox"
    >
      <Show
        when={props.state.searchHistory().length > 0}
        fallback={<div class="px-3 py-2 text-xs text-muted">{props.state.emptyHistoryMessage()}</div>}
      >
        <div class="max-h-52 overflow-y-auto py-1">
          <For each={props.state.searchHistory()}>
            {(entry) => (
              <div class="flex items-center justify-between px-2 py-1.5 hover:bg-blue-50 dark:hover:bg-blue-900">
                <button
                  type="button"
                  class="flex-1 truncate pr-2 text-left text-sm text-base-content transition-colors hover:text-blue-600 focus:outline-none dark:hover:text-blue-300"
                  onClick={() => props.state.selectHistoryEntry(entry)}
                  onMouseDown={props.state.onClearMouseDown}
                >
                  {entry}
                </button>
                <button
                  type="button"
                  class="ml-1 flex h-6 w-6 items-center justify-center rounded text-slate-400 transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none"
                  title="Remove from history"
                  onClick={() => props.state.deleteHistoryEntry(entry)}
                  onMouseDown={props.state.onClearMouseDown}
                >
                  <svg
                    class="h-3.5 w-3.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
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
          class="flex w-full items-center justify-center gap-2 border-t border-border px-3 py-2 text-xs font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
          onClick={props.state.clearHistory}
          onMouseDown={props.state.onClearMouseDown}
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
);
