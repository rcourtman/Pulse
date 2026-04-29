export const GROUPED_TABLE_ROW_CLASS = 'grouped-table-row';
export const GROUPED_TABLE_ROW_BASE_CLASS = `${GROUPED_TABLE_ROW_CLASS} transition-colors`;
export const GROUPED_TABLE_ROW_INTERACTIVE_CLASS = `${GROUPED_TABLE_ROW_BASE_CLASS} cursor-pointer select-none duration-150`;
export const GROUPED_TABLE_ROW_CELL_CLASS =
  '!px-2 !py-1 align-middle text-[12px] sm:text-sm font-semibold text-base-content';
export const GROUPED_TABLE_ROW_META_CLASS = 'text-[10px] font-medium text-muted';
export const GROUPED_TABLE_ROW_BADGE_CLASS =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';

export const getGroupedTableRowClass = (className = ''): string =>
  `${GROUPED_TABLE_ROW_BASE_CLASS} ${className}`.trim();

export const getInteractiveGroupedTableRowClass = (className = ''): string =>
  `${GROUPED_TABLE_ROW_INTERACTIVE_CLASS} ${className}`.trim();

export const getGroupedTableRowCellClass = (className = ''): string =>
  `${GROUPED_TABLE_ROW_CELL_CLASS} ${className}`.trim();
