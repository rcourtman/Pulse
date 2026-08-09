/**
 * useCollapsedSections Hook
 *
 * Manages collapsed/expanded state for accordion sections with localStorage persistence.
 * Provides a clean interface for toggling sections and remembering user preferences.
 */

import { createSignal, createEffect } from 'solid-js';

const STORAGE_KEY = 'pulse-thresholds-collapsed-sections';

interface CollapsedSectionsState {
  [sectionId: string]: boolean;
}

/**
 * Every resource section starts collapsed so the thresholds page opens as an
 * overview instead of a wall of tables. Explicit user choices still win when
 * they are restored from localStorage.
 */
const DEFAULT_COLLAPSED: CollapsedSectionsState = {
  nodes: true,
  pbs: true,
  guests: true,
  guestDisks: true,
  'guest-filtering': true,
  backups: true,
  snapshots: true,
  storage: true,
  pmg: true,
  agents: true,
  agentDisks: true,
  dockerHosts: true,
  dockerContainers: true,
  kubernetesClusters: true,
  kubernetesNodes: true,
  kubernetesNamespaces: true,
  kubernetesDeployments: true,
  kubernetesPods: true,
  trueNASSystems: true,
  trueNASPools: true,
  trueNASDatasets: true,
  trueNASDisks: true,
  vmwareHosts: true,
  vmwareVMs: true,
  vmwareDatastores: true,
  vmwareNetworks: true,
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
  const [collapsedState, setCollapsedState] =
    createSignal<CollapsedSectionsState>(loadFromStorage());

  // Persist to localStorage when state changes
  createEffect(() => {
    saveToStorage(collapsedState());
  });

  const isCollapsed = (sectionId: string): boolean => {
    const state = collapsedState();
    // Unknown future resource sections follow the same overview-first default.
    if (sectionId in state) {
      return state[sectionId];
    }
    return DEFAULT_COLLAPSED[sectionId] ?? true;
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
