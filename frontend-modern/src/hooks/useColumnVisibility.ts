import { createMemo, Accessor } from 'solid-js';
import type { JSX } from 'solid-js';
import { usePersistentSignal } from './usePersistentSignal';

export interface ColumnDef {
  id: string;
  label: string;
  icon?: JSX.Element; // Optional icon for compact column headers
  toggleable?: boolean;
  width?: string; // Fixed width for consistent column sizing
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  sortKey?: string;
}

/**
 * Hook for managing column visibility with persistence.
 *
 * Columns are shown if:
 * 1. They are relevant to the current view mode (when relevantColumns is provided), AND
 * 2. The user hasn't explicitly hidden them (for toggleable columns)
 *
 * @param storageKey - localStorage key for persisting user preferences
 * @param columns - Array of column definitions
 * @param defaultHidden - Optional array of column IDs to hide by default (only used if no user preference exists)
 * @param relevantColumns - Optional reactive accessor returning the set of column IDs relevant to the current view.
 *   When non-null, columns not in the set are excluded from visibleColumns and availableToggles.
 */
export function useColumnVisibility(
  storageKey: string,
  columns: ColumnDef[],
  defaultHidden: string[] = [],
  relevantColumns?: Accessor<Set<string> | null>,
) {
  // Get list of toggleable column IDs
  const toggleableIds = columns.filter((c) => c.toggleable).map((c) => c.id);

  // Check if user has any saved preference
  const hasUserPreference =
    typeof window !== 'undefined' && window.localStorage.getItem(storageKey) !== null;

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
    },
  );

  // Check if a column is hidden by user preference
  const isHiddenByUser = (id: string): boolean => {
    return hiddenColumns().includes(id);
  };

  // Toggle a column's visibility
  const toggle = (id: string) => {
    const current = hiddenColumns();
    if (current.includes(id)) {
      setHiddenColumns(current.filter((c) => c !== id));
    } else {
      setHiddenColumns([...current, id]);
    }
  };

  // Show a column (remove from hidden)
  const show = (id: string) => {
    setHiddenColumns(hiddenColumns().filter((c) => c !== id));
  };

  // Hide a column (add to hidden)
  const hide = (id: string) => {
    if (!hiddenColumns().includes(id)) {
      setHiddenColumns([...hiddenColumns(), id]);
    }
  };

  // Show all toggleable columns (clear user hidden list)
  const resetToDefaults = () => {
    setHiddenColumns([]);
  };

  // Compute visible columns based on user preferences and view-mode relevance
  const visibleColumns: Accessor<ColumnDef[]> = createMemo(() => {
    const relevant = relevantColumns?.();
    return columns.filter((col) => {
      // If a relevance set is active, exclude columns not in it
      if (relevant && !relevant.has(col.id)) return false;

      // If toggleable, check user preference
      if (col.toggleable && isHiddenByUser(col.id)) return false;

      return true;
    });
  });

  // Get columns that could be toggled (relevant to the current view)
  const availableToggles: Accessor<ColumnDef[]> = createMemo(() => {
    const relevant = relevantColumns?.();
    return columns.filter((col) => {
      if (!col.toggleable) return false;
      if (relevant && !relevant.has(col.id)) return false;
      return true;
    });
  });

  // Check if a specific column is currently visible
  const isColumnVisible = (id: string): boolean => {
    return visibleColumns().some((col) => col.id === id);
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
