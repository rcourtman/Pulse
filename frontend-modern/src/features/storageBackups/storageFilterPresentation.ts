export const getNextStorageSortDirection = (
  direction: 'asc' | 'desc',
): 'asc' | 'desc' => (direction === 'asc' ? 'desc' : 'asc');

export const getStorageSortDirectionTitle = (direction: 'asc' | 'desc'): string =>
  `Sort ${direction === 'asc' ? 'descending' : 'ascending'}`;

export const getStorageSortDirectionIconClass = (direction: 'asc' | 'desc'): string =>
  direction === 'asc' ? 'rotate-180' : '';

export const STORAGE_FILTER_SORT_SELECT_CLASS =
  'px-2 py-1 text-xs border border-border rounded-md bg-surface text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500';

export const STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS =
  'inline-flex items-center justify-center h-7 w-7 rounded-md border border-border text-muted hover:bg-surface-hover transition-colors';

export const STORAGE_FILTER_SEGMENTED_WRAP_CLASS = 'max-w-full overflow-x-auto scrollbar-hide';
export const STORAGE_FILTER_COMPACT_SELECT_CLASS = 'min-w-[8rem]';
export const STORAGE_FILTER_SORT_WRAP_CLASS = 'flex items-center gap-1.5';
export const STORAGE_FILTER_SORT_ICON_CLASS = 'h-4 w-4 transition-transform';
export const STORAGE_FILTER_RESET_ACTION_CLASS = 'text-base-content';
