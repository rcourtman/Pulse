export type TableSortDirection = 'asc' | 'desc';

/**
 * Canonical visual sort marker for table headers.
 *
 * Inactive headers remain discoverable through their button semantics instead
 * of repeating a dormant icon in every column. Only the active column needs a
 * direction marker.
 */
export function getTableSortIndicator(
  active: boolean,
  direction: TableSortDirection,
): '▲' | '▼' | null {
  if (!active) return null;
  return direction === 'asc' ? '▲' : '▼';
}
