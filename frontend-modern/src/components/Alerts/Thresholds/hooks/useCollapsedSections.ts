/**
 * useCollapsedSections Hook
 *
 * Manages collapsed/expanded state for accordion sections with localStorage persistence.
 * Provides a clean interface for toggling sections and remembering user preferences.
 */

import { createSignal, createEffect, onMount } from 'solid-js';

const STORAGE_KEY = 'pulse-thresholds-collapsed-sections';

interface CollapsedSectionsState {
    [sectionId: string]: boolean;
}

/**
 * Default collapsed state for sections
 * Sections not listed here default to expanded (false)
 */
const DEFAULT_COLLAPSED: CollapsedSectionsState = {
    // In Proxmox tab, collapse less-frequently-used sections by default
    storage: true,
    backups: true,
    snapshots: true,
    // PBS servers often empty, collapse by default
    pbs: true,
};

/**
 * Load collapsed state from localStorage
 */
const loadFromStorage = (): CollapsedSectionsState => {
    try {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
            const parsed = JSON.parse(stored);
            if (typeof parsed === 'object' && parsed !== null) {
                return { ...DEFAULT_COLLAPSED, ...parsed };
            }
        }
    } catch {
        // Ignore parse errors
    }
    return { ...DEFAULT_COLLAPSED };
};

/**
 * Save collapsed state to localStorage
 */
const saveToStorage = (state: CollapsedSectionsState): void => {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
        // Ignore storage errors (e.g., quota exceeded)
    }
};

export interface UseCollapsedSectionsResult {
    /**
     * Check if a section is collapsed
     */
    isCollapsed: (sectionId: string) => boolean;

    /**
     * Toggle a section's collapsed state
     */
    toggleSection: (sectionId: string) => void;

    /**
     * Set a section's collapsed state explicitly
     */
    setCollapsed: (sectionId: string, collapsed: boolean) => void;

    /**
     * Expand all sections
     */
    expandAll: () => void;

    /**
     * Collapse all sections
     */
    collapseAll: () => void;

    /**
     * Reset to default collapsed state
     */
    resetToDefaults: () => void;
}

/**
 * Hook for managing accordion section collapsed state
 *
 * @example
 * ```tsx
 * const { isCollapsed, toggleSection } = useCollapsedSections();
 *
 * <CollapsibleSection
 *   id="nodes"
 *   collapsed={isCollapsed('nodes')}
 *   onToggle={() => toggleSection('nodes')}
 * >
 *   {content}
 * </CollapsibleSection>
 * ```
 */
export function useCollapsedSections(): UseCollapsedSectionsResult {
    const [collapsedState, setCollapsedState] = createSignal<CollapsedSectionsState>(
        loadFromStorage()
    );

    // Persist to localStorage when state changes
    createEffect(() => {
        saveToStorage(collapsedState());
    });

    const isCollapsed = (sectionId: string): boolean => {
        const state = collapsedState();
        // If not explicitly set, check defaults
        if (sectionId in state) {
            return state[sectionId];
        }
        return DEFAULT_COLLAPSED[sectionId] ?? false;
    };

    const toggleSection = (sectionId: string): void => {
        setCollapsedState((prev) => ({
            ...prev,
            [sectionId]: !isCollapsed(sectionId),
        }));
    };

    const setCollapsed = (sectionId: string, collapsed: boolean): void => {
        setCollapsedState((prev) => ({
            ...prev,
            [sectionId]: collapsed,
        }));
    };

    const expandAll = (): void => {
        setCollapsedState((prev) => {
            const newState: CollapsedSectionsState = {};
            Object.keys(prev).forEach((key) => {
                newState[key] = false;
            });
            // Also expand defaults
            Object.keys(DEFAULT_COLLAPSED).forEach((key) => {
                newState[key] = false;
            });
            return newState;
        });
    };

    const collapseAll = (): void => {
        setCollapsedState((prev) => {
            const newState: CollapsedSectionsState = {};
            Object.keys(prev).forEach((key) => {
                newState[key] = true;
            });
            // Also collapse defaults
            Object.keys(DEFAULT_COLLAPSED).forEach((key) => {
                newState[key] = true;
            });
            return newState;
        });
    };

    const resetToDefaults = (): void => {
        setCollapsedState({ ...DEFAULT_COLLAPSED });
    };

    return {
        isCollapsed,
        toggleSection,
        setCollapsed,
        expandAll,
        collapseAll,
        resetToDefaults,
    };
}

export default useCollapsedSections;
