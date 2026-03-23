import { Component, For, Show } from 'solid-js';
import { SearchTipsPopover } from '@/components/shared/SearchTipsPopover';
import {
  SEARCH_HISTORY_CLEAR_LABEL,
  SEARCH_HISTORY_EMPTY_STATE_CLASS,
  SEARCH_HISTORY_ENTRY_BUTTON_CLASS,
  SEARCH_HISTORY_MENU_CLASS,
  SEARCH_HISTORY_ROW_CLASS,
  getSearchHistoryClearButtonClass,
  getSearchHistoryDeleteButtonClass,
  getSearchHistoryToggleButtonClass,
  getSearchHistoryToggleTitle,
} from '@/components/shared/searchInputEnhancementsModel';
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
        class={getSearchHistoryToggleButtonClass(props.state.isHistoryOpen())}
        onClick={props.state.toggleHistory}
        onMouseDown={props.state.onClearMouseDown}
        aria-haspopup="listbox"
        aria-expanded={props.state.isHistoryOpen()}
        title={getSearchHistoryToggleTitle(props.state.searchHistory().length)}
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
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
      class={SEARCH_HISTORY_MENU_CLASS}
      role="listbox"
    >
      <Show
        when={props.state.searchHistory().length > 0}
        fallback={
          <div class={SEARCH_HISTORY_EMPTY_STATE_CLASS}>{props.state.emptyHistoryMessage()}</div>
        }
      >
        <div class="max-h-52 overflow-y-auto py-1">
          <For each={props.state.searchHistory()}>
            {(entry) => (
              <div class={SEARCH_HISTORY_ROW_CLASS}>
                <button
                  type="button"
                  class={SEARCH_HISTORY_ENTRY_BUTTON_CLASS}
                  onClick={() => props.state.selectHistoryEntry(entry)}
                  onMouseDown={props.state.onClearMouseDown}
                >
                  {entry}
                </button>
                <button
                  type="button"
                  class={getSearchHistoryDeleteButtonClass()}
                  title="Remove from history"
                  onClick={() => props.state.deleteHistoryEntry(entry)}
                  onMouseDown={props.state.onClearMouseDown}
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
          class={getSearchHistoryClearButtonClass()}
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
          {SEARCH_HISTORY_CLEAR_LABEL}
        </button>
      </Show>
    </div>
  </Show>
);
