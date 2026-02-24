import { createMemo, Accessor } from 'solid-js';
import type { ColumnConfig } from '@/types/responsive';
import { useBreakpoint, type Breakpoint, PRIORITY_BREAKPOINTS } from '@/hooks/useBreakpoint';

export interface GridTemplateOptions {
  /** Column configurations - can be a static array or reactive accessor */
  columns: ColumnConfig[] | Accessor<ColumnConfig[]>;

  /** Whether to use a sticky first column on mobile */
  hasStickyColumn?: boolean;

  /** Custom gap between columns (CSS value) */
  gap?: string;

  /** If true, show all columns regardless of breakpoint (use horizontal scroll instead) */
  disableColumnHiding?: boolean;
}

export interface GridTemplateResult {
  /** CSS grid-template-columns value */
  gridTemplate: Accessor<string>;

  /** Columns that are currently visible */
  visibleColumns: Accessor<ColumnConfig[]>;

  /** Check if a specific column is visible */
  isColumnVisible: (columnId: string) => boolean;

  /** Current breakpoint */
  breakpoint: Accessor<Breakpoint>;

  /** Whether currently in mobile view */
  isMobile: Accessor<boolean>;
}

/**
 * Generate a CSS grid-template-columns value from column configs
 */
function generateGridTemplate(columns: ColumnConfig[]): string {
  return columns
    .map((col) => {
      const min = col.minWidth || '50px';
      const max = col.maxWidth || `${col.flex || 1}fr`;

      // If both min and max are 'auto', use just 'auto' for content-based sizing
      if (min === 'auto' && max === 'auto') {
        return 'auto';
      }

      // If only max is 'auto', use minmax with auto
      if (max === 'auto') {
        return `minmax(${min}, auto)`;
      }

      // If we have both min and max as fixed values, use the appropriate one
      if (col.maxWidth && !col.maxWidth.includes('fr')) {
        return `minmax(${min}, ${col.maxWidth})`;
      }

      // Use minmax for flexible columns
      return `minmax(${min}, ${max})`;
    })
    .join(' ');
}

/**
 * Filter columns based on current breakpoint
 */
function getVisibleColumns(columns: ColumnConfig[], breakpoint: Breakpoint): ColumnConfig[] {
  const breakpointOrder: Breakpoint[] = ['xs', 'sm', 'md', 'lg', 'xl', '2xl'];
  const currentIndex = breakpointOrder.indexOf(breakpoint);

  return columns.filter((col) => {
    const colBreakpoint = PRIORITY_BREAKPOINTS[col.priority];
    const colIndex = breakpointOrder.indexOf(colBreakpoint);
    return colIndex <= currentIndex;
  });
}

/**
 * Hook to generate responsive CSS grid templates based on column configuration.
 *
 * This hook automatically adjusts the grid template based on the current
 * viewport width, showing/hiding columns according to their priority.
 *
 * @example
 * ```tsx
 * const columns = [
 *   { id: 'name', label: 'Name', priority: 'essential', minWidth: '150px', flex: 1.5 },
 *   { id: 'cpu', label: 'CPU', priority: 'essential', minWidth: '100px', flex: 1 },
 *   { id: 'diskRead', label: 'D Read', priority: 'detailed', minWidth: '100px', flex: 1 },
 * ];
 *
 * const { gridTemplate, visibleColumns, isMobile } = useGridTemplate({ columns });
 *
 * return (
 *   <div
 *     class="grid"
 *     style={{ 'grid-template-columns': gridTemplate() }}
 *   >
 *     <For each={visibleColumns()}>
 *       {col => <div>{col.label}</div>}
 *     </For>
 *   </div>
 * );
 * ```
 */
export function useGridTemplate(options: GridTemplateOptions): GridTemplateResult {
  const { breakpoint, isMobile } = useBreakpoint();

  // Support both static arrays and reactive accessors
  const getColumns = (): ColumnConfig[] => {
    const cols = options.columns;
    return typeof cols === 'function' ? cols() : cols;
  };

  const visibleColumns = createMemo(() => {
    const cols = getColumns();
    if (options.disableColumnHiding) {
      return cols;
    }
    return getVisibleColumns(cols, breakpoint());
  });

  const gridTemplate = createMemo(() => {
    const visible = visibleColumns();

    // On mobile with sticky column, we handle layout differently
    // The first column is sticky and outside the grid
    if (isMobile() && options.hasStickyColumn && visible.length > 1) {
      // Skip the first column (it's sticky) and generate template for the rest
      return generateGridTemplate(visible.slice(1));
    }

    return generateGridTemplate(visible);
  });

  const isColumnVisible = (columnId: string): boolean => {
    return visibleColumns().some((col) => col.id === columnId);
  };

  return {
    gridTemplate,
    visibleColumns,
    isColumnVisible,
    breakpoint,
    isMobile,
  };
}

/**
 * Simple utility to generate a static grid template (non-reactive)
 */
export function generateStaticGridTemplate(
  columns: ColumnConfig[],
  breakpoint: Breakpoint = 'xl',
): string {
  const visible = getVisibleColumns(columns, breakpoint);
  return generateGridTemplate(visible);
}

/**
 * Generate responsive grid template classes for Tailwind
 * Returns an object with breakpoint-specific templates
 */
export function generateResponsiveTemplates(columns: ColumnConfig[]): Record<Breakpoint, string> {
  const breakpoints: Breakpoint[] = ['xs', 'sm', 'md', 'lg', 'xl', '2xl'];

  return Object.fromEntries(
    breakpoints.map((bp) => [bp, generateStaticGridTemplate(columns, bp)]),
  ) as Record<Breakpoint, string>;
}

/**
 * Get Tailwind CSS classes for column visibility based on priority
 */
export function getColumnVisibilityClass(
  priority: ColumnConfig['priority'],
  displayType: 'flex' | 'block' | 'table-cell' | 'grid' = 'flex',
): string {
  const bp = PRIORITY_BREAKPOINTS[priority];

  if (bp === 'xs') {
    // Always visible
    return displayType;
  }

  return `hidden ${bp}:${displayType}`;
}
