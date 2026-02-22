import { Component, Show } from 'solid-js';
import type { ColumnConfig, SortState } from '@/types/responsive';

export interface ResponsiveHeaderProps {
  /** Column configuration */
  column: ColumnConfig;

  /** Current sort state */
  sortState?: SortState;

  /** Sort handler - called with the column's sortKey or id */
  onSort?: (key: string) => void;

  /** Whether to show mobile (icon) view */
  showMobile?: boolean;

  /** Additional CSS classes */
  class?: string;
}

/**
 * A responsive table header cell that shows an icon on mobile
 * and the full label on larger screens.
 *
 * @example
 * ```tsx
 * <ResponsiveHeader
 *   column={STANDARD_COLUMNS.cpu}
 *   sortState={sortState()}
 *   onSort={handleSort}
 *   showMobile={isMobile()}
 * />
 * ```
 */
export const ResponsiveHeader: Component<ResponsiveHeaderProps> = (props) => {
  const sortKey = () => props.column.sortKey || props.column.id;
  const isSorted = () => props.sortState?.key === sortKey();
  const sortIndicator = () => {
    if (!isSorted()) return null;
    return props.sortState?.direction === 'asc' ? '▲' : '▼';
  };

  const handleClick = () => {
    if (props.column.sortable && props.onSort) {
      props.onSort(sortKey());
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && props.column.sortable && props.onSort) {
      props.onSort(sortKey());
    }
  };

  const alignmentClass = () => {
    switch (props.column.align) {
      case 'center': return 'justify-center text-center';
      case 'right': return 'justify-end text-right';
      default: return 'justify-start text-left';
    }
  };

  const baseClass = () => {
    const classes = [
      'px-0.5 md:px-2 py-1',
      'text-[11px] sm:text-xs font-medium uppercase tracking-wider',
      'flex items-center h-full',
      alignmentClass(),
    ];

    if (props.column.sortable) {
      classes.push('cursor-pointer hover:bg-slate-200 dark:hover:bg-slate-600');
    }

    if (props.class) {
      classes.push(props.class);
    }

    return classes.join(' ');
  };

  const Icon = props.column.icon;

  return (
    <div
      class={baseClass()}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      tabIndex={props.column.sortable ? 0 : undefined}
      role={props.column.sortable ? 'button' : undefined}
      aria-label={props.column.sortable
        ? `Sort by ${props.column.label} ${isSorted() ? (props.sortState?.direction === 'asc' ? 'ascending' : 'descending') : ''}`
        : undefined
      }
    >
      {/* Mobile: Show icon if available */}
      <Show when={props.showMobile && Icon}>
        <span class="md:hidden" title={props.column.label}>
          {Icon && <Icon class="w-4 h-4" />}
        </span>
      </Show>

      {/* Desktop: Show label (or always if no icon) */}
      <span class={props.showMobile && Icon ? 'hidden md:inline' : ''}>
        {props.column.label}
      </span>

      {/* Sort indicator */}
      <Show when={sortIndicator()}>
        <span class="ml-0.5">{sortIndicator()}</span>
      </Show>
    </div>
  );
};

/**
 * Sticky header variant for name columns
 */
export const StickyHeader: Component<ResponsiveHeaderProps & { width?: string }> = (props) => {
  return (
    <div
      class={`sticky left-0 z-30 bg-surface-alt border-r md:border-r-0 border-border ${props.width || 'w-[160px] sm:w-[200px] md:w-full'}`}
    >
      <ResponsiveHeader {...props} />
    </div>
  );
};
