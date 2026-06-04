import { createEffect, createMemo, Accessor } from 'solid-js';
import type { JSX } from 'solid-js';
import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';
import { usePersistentSignal } from './usePersistentSignal';

export interface ColumnDef {
  id: string;
  label: string;
  icon?: JSX.Element; // Optional icon for compact column headers
  toggleable?: boolean;
  defaultHidden?: boolean;
  width?: string; // Fixed width for consistent column sizing
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  sortKey?: string;
  // Canonical column kind for header / cell alignment. See
  // `frontend-modern/src/features/platformPage/columnAlignment.ts` for the
  // kind → align mapping and rationale. Optional so consumers that don't
  // care about platform-table alignment can omit it.
  kind?: PlatformTableColumnKind;
}

const normalizePersistedColumnIds = (
  values: unknown,
  persistedIdAliases: Record<string, string>,
): { ids: string[]; migrated: boolean } => {
  if (!Array.isArray(values)) return { ids: [], migrated: false };

  const ids: string[] = [];
  let migrated = false;

  for (const value of values) {
    if (typeof value !== 'string') {
      migrated = true;
      continue;
    }

    const trimmed = value.trim();
    if (!trimmed) {
      migrated = true;
      continue;
    }

    const canonicalId = persistedIdAliases[trimmed] || trimmed;
    if (canonicalId !== trimmed) migrated = true;
    if (ids.includes(canonicalId)) {
      migrated = true;
      continue;
    }

    ids.push(canonicalId);
  }

  return { ids, migrated };
};

const defaultHiddenMigrationStorageKey = (storageKey: string): string =>
  `${storageKey}:default-hidden-applied`;

const readAppliedDefaultHiddenMigrations = (
  storageKey: string,
  persistedIdAliases: Record<string, string>,
): string[] => {
  if (typeof window === 'undefined') return [];

  try {
    const raw = window.localStorage.getItem(defaultHiddenMigrationStorageKey(storageKey));
    if (raw === null) return [];
    return normalizePersistedColumnIds(JSON.parse(raw), persistedIdAliases).ids;
  } catch {
    return [];
  }
};

const persistAppliedDefaultHiddenMigrations = (
  storageKey: string,
  ids: string[],
): void => {
  if (typeof window === 'undefined') return;

  try {
    window.localStorage.setItem(
      defaultHiddenMigrationStorageKey(storageKey),
      JSON.stringify(Array.from(new Set(ids))),
    );
  } catch {
    // localStorage may be full or unavailable; the hidden-column preference still works.
  }
};

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
 * @param defaultHiddenMigrationIds - New default-hidden columns that should be hidden once for users with
 *   existing preferences. After the user shows the column, the migration marker prevents re-hiding it.
 */
export function useColumnVisibility(
  storageKey: string,
  columns: ColumnDef[],
  defaultHidden: string[] = [],
  relevantColumns?: Accessor<Set<string> | null>,
  persistedIdAliases: Record<string, string> = {},
  defaultHiddenMigrationIds: string[] = [],
) {
  const defaultHiddenFromColumns = columns
    .filter((c) => c.defaultHidden)
    .map((c) => c.id);
  const effectiveDefaultHidden = Array.from(new Set([...defaultHiddenFromColumns, ...defaultHidden]));

  // Get list of toggleable column IDs
  const toggleableIds = columns.filter((c) => c.toggleable).map((c) => c.id);

  // Check if user has any saved preference
  const hasUserPreference =
    typeof window !== 'undefined' && window.localStorage.getItem(storageKey) !== null;
  let persistedIdsMigrated = false;
  const appliedDefaultHiddenMigrations = hasUserPreference
    ? readAppliedDefaultHiddenMigrations(storageKey, persistedIdAliases)
    : [];
  const pendingDefaultHiddenMigrations = hasUserPreference
    ? Array.from(
        new Set(
          defaultHiddenMigrationIds
            .map((id) => id.trim())
            .filter(
              (id) =>
                id &&
                effectiveDefaultHidden.includes(id) &&
                toggleableIds.includes(id) &&
                !appliedDefaultHiddenMigrations.includes(id),
            ),
        ),
      )
    : [];
  let defaultHiddenMigrationsPersisted = false;

  // Persist hidden columns to localStorage
  // Use defaultHidden only if no user preference exists yet
  const [hiddenColumns, setHiddenColumns] = usePersistentSignal<string[]>(
    storageKey,
    hasUserPreference ? [] : effectiveDefaultHidden,
    {
      serialize: (arr) => JSON.stringify(arr),
      deserialize: (str) => {
        try {
          const parsed = JSON.parse(str);
          const normalized = normalizePersistedColumnIds(parsed, persistedIdAliases);
          persistedIdsMigrated = normalized.migrated;
          for (const id of pendingDefaultHiddenMigrations) {
            if (normalized.ids.includes(id)) continue;
            normalized.ids.push(id);
            persistedIdsMigrated = true;
          }
          return normalized.ids;
        } catch {
          return [];
        }
      },
    },
  );

  createEffect(() => {
    const hasUnpersistedDefaultHiddenMigrations =
      pendingDefaultHiddenMigrations.length > 0 && !defaultHiddenMigrationsPersisted;
    if (!hasUserPreference || (!persistedIdsMigrated && !hasUnpersistedDefaultHiddenMigrations)) {
      return;
    }
    persistedIdsMigrated = false;
    if (hasUnpersistedDefaultHiddenMigrations) {
      defaultHiddenMigrationsPersisted = true;
      persistAppliedDefaultHiddenMigrations(storageKey, [
        ...appliedDefaultHiddenMigrations,
        ...pendingDefaultHiddenMigrations,
      ]);
    }
    setHiddenColumns([...hiddenColumns()]);
  });

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
    setHiddenColumns(effectiveDefaultHidden);
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
