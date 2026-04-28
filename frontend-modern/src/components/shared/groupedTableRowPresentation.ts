export const GROUPED_TABLE_ROW_CLASS = 'grouped-table-row';
export const GROUPED_TABLE_ROW_BASE_CLASS = `${GROUPED_TABLE_ROW_CLASS} transition-colors`;
export const GROUPED_TABLE_ROW_INTERACTIVE_CLASS = `${GROUPED_TABLE_ROW_BASE_CLASS} cursor-pointer select-none duration-150`;

export const getGroupedTableRowClass = (className = ''): string =>
  `${GROUPED_TABLE_ROW_BASE_CLASS} ${className}`.trim();

export const getInteractiveGroupedTableRowClass = (className = ''): string =>
  `${GROUPED_TABLE_ROW_INTERACTIVE_CLASS} ${className}`.trim();
