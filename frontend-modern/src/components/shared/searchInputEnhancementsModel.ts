export const SEARCH_HISTORY_MENU_CLASS =
  'absolute left-0 right-0 top-full z-50 mt-2 w-full overflow-hidden rounded-md border border-border bg-surface text-sm shadow-sm';
export const SEARCH_HISTORY_EMPTY_STATE_CLASS = 'px-3 py-2 text-xs text-muted';
export const SEARCH_HISTORY_ROW_CLASS =
  'flex items-center justify-between px-2 py-1.5 hover:bg-blue-50 dark:hover:bg-blue-900';
export const SEARCH_HISTORY_ENTRY_BUTTON_CLASS =
  'flex-1 truncate pr-2 text-left text-sm text-base-content transition-colors hover:text-blue-600 focus:outline-none dark:hover:text-blue-300';
export const SEARCH_HISTORY_CLEAR_LABEL = 'Clear history';

export function getSearchHistoryToggleButtonClass(isOpen: boolean): string {
  return `flex h-7 w-7 items-center justify-center rounded-md transition-colors ${
    isOpen
      ? 'bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400'
      : 'text-muted hover:bg-surface-hover hover:text-base-content'
  }`;
}

export function getSearchHistoryToggleTitle(historyCount: number): string {
  return historyCount > 0 ? 'Show recent searches' : 'No recent searches yet';
}

export function getSearchHistoryDeleteButtonClass(): string {
  return 'ml-1 flex h-6 w-6 items-center justify-center rounded text-slate-400 transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none';
}

export function getSearchHistoryClearButtonClass(): string {
  return 'flex w-full items-center justify-center gap-2 border-t border-border px-3 py-2 text-xs font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content';
}
