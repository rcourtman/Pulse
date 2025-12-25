import { createMemo, Accessor } from 'solid-js';
import type { JSX } from 'solid-js';
import { usePersistentSignal } from './usePersistentSignal';
import { useBreakpoint, type ColumnPriority, PRIORITY_BREAKPOINTS, type Breakpoint } from './useBreakpoint';

export interface ColumnDef {
  id: string;
  label: string;
  icon?: JSX.Element; // Optional icon for compact column headers
  priority: ColumnPriority;
  toggleable?: boolean;
  width?: string;      // Fixed width for consistent column sizing
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  sortKey?: string;
}

const BREAKPOINT_ORDER: Breakpoint[] = ['xs', 'sm', 'md', 'lg', 'xl', '2xl'];

function breakpointIndex(bp: Breakpoint): number {
  return BREAKPOINT_ORDER.indexOf(bp);
}

/**
 * Hook for managing column visibility with persistence and responsive behavior.
 *
 * Columns are shown if:
 * 1. The current breakpoint supports their priority level, AND
 * 2. The user hasn't explicitly hidden them (for toggleable columns)
 *
 * @param storageKey - localStorage key for persisting user preferences
 * @param columns - Array of column definitions
 * @param defaultHidden - Optional array of column IDs to hide by default (only used if no user preference exists)
 */
export function useColumnVisibility(
  storageKey: string,
  columns: ColumnDef[],
  defaultHidden: string[] = []
) {
  const { breakpoint } = useBreakpoint();

  // Get list of toggleable column IDs
  const toggleableIds = columns.filter(c => c.toggleable).map(c => c.id);

  // Check if user has any saved preference
  const hasUserPreference = typeof window !== 'undefined' && window.localStorage.getItem(storageKey) !== null;

  // Persist hidden columns to localStorage
  // Use defaultHidden only if no user preference exists yet
  const [hiddenColumns, setHiddenColumns] = usePersistentSignal<string[]>(
    storageKey,
    hasUserPreference ? [] : defaultHidden,
    {
      serialize: (arr) => JSON.stringify(arr),
      deserialize: (str) => {
        try {
          const parsed = JSON.parse(str);
          return Array.isArray(parsed) ? parsed : [];
        } catch {
          return [];
        }
      },
    }
  );

  // Check if a column is hidden by user preference
  const isHiddenByUser = (id: string): boolean => {
    return hiddenColumns().includes(id);
  };

  // Check if breakpoint supports showing this column
  const hasSpaceForColumn = (col: ColumnDef): boolean => {
    const minBreakpoint = PRIORITY_BREAKPOINTS[col.priority];
    return breakpointIndex(breakpoint()) >= breakpointIndex(minBreakpoint);
  };

  // Toggle a column's visibility
  const toggle = (id: string) => {
    const current = hiddenColumns();
    if (current.includes(id)) {
      setHiddenColumns(current.filter(c => c !== id));
    } else {
      setHiddenColumns([...current, id]);
    }
  };

  // Show a column (remove from hidden)
  const show = (id: string) => {
    setHiddenColumns(hiddenColumns().filter(c => c !== id));
  };

  // Hide a column (add to hidden)
  const hide = (id: string) => {
    if (!hiddenColumns().includes(id)) {
      setHiddenColumns([...hiddenColumns(), id]);
    }
  };

  // Reset to defaults (restore default hidden columns)
  const resetToDefaults = () => {
    setHiddenColumns(defaultHidden);
  };

  // Compute visible columns based on breakpoint and user preferences
  const visibleColumns: Accessor<ColumnDef[]> = createMemo(() => {
    return columns.filter(col => {
      // Always show essential columns regardless of breakpoint
      // However, check for toggle status later (removed early return)

      // Check if screen has space for this priority level
      if (!hasSpaceForColumn(col)) return false;

      // If toggleable, check user preference
      if (col.toggleable && isHiddenByUser(col.id)) return false;

      return true;
    });
  });

  // Get columns that could be toggled at the current breakpoint
  // (i.e., screen is wide enough to show them)
  const availableToggles: Accessor<ColumnDef[]> = createMemo(() => {
    return columns.filter(col => {
      if (!col.toggleable) return false;
      return hasSpaceForColumn(col);
    });
  });

  // Check if a specific column is currently visible
  const isColumnVisible = (id: string): boolean => {
    return visibleColumns().some(col => col.id === id);
  };

  return {
    visibleColumns,
    availableToggles,
    toggleableIds,
    hiddenColumns,
    isColumnVisible,
    isHiddenByUser,
    toggle,
    show,
    hide,
    resetToDefaults,
  };
}
