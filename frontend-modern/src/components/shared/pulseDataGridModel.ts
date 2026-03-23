import type { JSX } from 'solid-js';

export interface TableColumn<T> {
  key: keyof T | string;
  label: string;
  /** Custom render function for the cell content. Useful for badges, links, or complex nested data */
  render?: (row: T) => JSX.Element;
  /** Optional alignment for the header and cell content */
  align?: 'left' | 'center' | 'right';
  /** Optional fixed width or width class */
  width?: string;
  /** Hidden on mobile via CSS class */
  hiddenOnMobile?: boolean;
}

export interface PulseDataGridProps<T> {
  /** The rows of data to display */
  data: T[];
  /** Definitions for each column */
  columns: TableColumn<T>[];

  /**
   * A unique identifier function for each row to help SolidJS optimize rendering.
   * Typically `(row) => row.id`
   */
  keyExtractor: (row: T) => string | number;

  /** Triggers when a row is clicked. */
  onRowClick?: (row: T) => void;

  /** What to display when the data array is empty */
  emptyState?: JSX.Element;

  /** Set to true to show a loading state */
  isLoading?: boolean;

  /** Determines if the current row should be expanded */
  isRowExpanded?: (row: T) => boolean;

  /** Render function for the expanded content of a row */
  expandedRender?: (row: T) => JSX.Element;

  /** Minimum width on desktop before horizontal scrolling kicks in */
  desktopMinWidth?: string;

  /**
   * Minimum width on mobile.
   * Defaults to '100%' so the table flexes into horizontal scroll natively
   * without artificially breaking the screen width.
   */
  mobileMinWidth?: string;

  /** Custom classes applied to the root container */
  class?: string;
}

export type PulseDataGridStableRow<T> = {
  __pulseKey: string | number;
  value: T;
};

export const getPulseDataGridAlignClass = (align?: 'left' | 'center' | 'right') => {
  switch (align) {
    case 'center':
      return 'text-center justify-center';
    case 'right':
      return 'text-right justify-end';
    case 'left':
    default:
      return 'text-left justify-start';
  }
};

export const isPulseDataGridInteractiveTarget = (target: EventTarget | null) =>
  target instanceof Element &&
  Boolean(
    target.closest(
      'button, a, input, select, textarea, summary, [role="button"], [data-row-action]',
    ),
  );
