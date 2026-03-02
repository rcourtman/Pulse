import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useCollapsedSections } from '../useCollapsedSections';

const STORAGE_KEY = 'pulse-thresholds-collapsed-sections';

const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      store = {};
    }),
    get length() {
      return Object.keys(store).length;
    },
  };
})();

describe('useCollapsedSections', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', localStorageMock);
    localStorageMock.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('default state', () => {
    it('returns default collapsed state for predefined sections', () => {
      const { isCollapsed } = useCollapsedSections();

      // These are collapsed by default per DEFAULT_COLLAPSED
      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('backups')).toBe(true);
      expect(isCollapsed('snapshots')).toBe(true);
      expect(isCollapsed('pbs')).toBe(true);
    });

    it('returns expanded (false) for unknown sections', () => {
      const { isCollapsed } = useCollapsedSections();

      expect(isCollapsed('nodes')).toBe(false);
      expect(isCollapsed('vms')).toBe(false);
      expect(isCollapsed('anything-else')).toBe(false);
    });
  });

  describe('localStorage persistence', () => {
    it('loads persisted state from localStorage', () => {
      const persisted = { nodes: true, storage: false };
      localStorageMock.setItem(STORAGE_KEY, JSON.stringify(persisted));

      const { isCollapsed } = useCollapsedSections();

      // Overridden by persisted state
      expect(isCollapsed('nodes')).toBe(true);
      expect(isCollapsed('storage')).toBe(false);
      // Defaults still apply for non-persisted keys
      expect(isCollapsed('backups')).toBe(true);
    });

    it('falls back to defaults when localStorage has invalid JSON', () => {
      localStorageMock.setItem(STORAGE_KEY, 'not-valid-json');

      const { isCollapsed } = useCollapsedSections();

      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('nodes')).toBe(false);
    });

    it('falls back to defaults when localStorage has non-object value', () => {
      localStorageMock.setItem(STORAGE_KEY, JSON.stringify('just-a-string'));

      const { isCollapsed } = useCollapsedSections();

      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('nodes')).toBe(false);
    });

    it('falls back to defaults when localStorage has null', () => {
      localStorageMock.setItem(STORAGE_KEY, JSON.stringify(null));

      const { isCollapsed } = useCollapsedSections();

      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('nodes')).toBe(false);
    });

    it('handles localStorage.getItem throwing an error', () => {
      localStorageMock.getItem.mockImplementationOnce(() => {
        throw new Error('SecurityError');
      });

      const { isCollapsed } = useCollapsedSections();

      // Should fall back to defaults
      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('nodes')).toBe(false);
    });

    it('handles localStorage.setItem throwing on init gracefully', () => {
      localStorageMock.setItem.mockImplementationOnce(() => {
        throw new Error('QuotaExceededError');
      });

      // Hook creation triggers createEffect which calls saveToStorage
      // — should not throw even when setItem fails
      const { isCollapsed } = useCollapsedSections();
      expect(isCollapsed('storage')).toBe(true);
    });

    it('handles localStorage.setItem throwing on mutation gracefully', () => {
      const { toggleSection, isCollapsed } = useCollapsedSections();

      // Make setItem throw once for the mutation write (falls back to original impl after)
      localStorageMock.setItem.mockImplementationOnce(() => {
        throw new Error('QuotaExceededError');
      });

      // Should not throw — saveToStorage catches the error
      expect(() => toggleSection('storage')).not.toThrow();
      // In-memory state still updates even if persistence fails
      expect(isCollapsed('storage')).toBe(false);
    });

    it('handles malformed object values in localStorage (non-boolean values)', () => {
      localStorageMock.setItem(
        STORAGE_KEY,
        JSON.stringify({ storage: 'yes', backups: 0 }),
      );

      const { isCollapsed } = useCollapsedSections();

      // Non-boolean values are returned as-is — truthy string 'yes' is truthy
      expect(isCollapsed('storage')).toBeTruthy();
      // Falsy number 0 is falsy
      expect(isCollapsed('backups')).toBeFalsy();
    });
  });

  describe('toggleSection', () => {
    it('toggles a default-collapsed section to expanded', () => {
      const { isCollapsed, toggleSection } = useCollapsedSections();

      expect(isCollapsed('storage')).toBe(true);
      toggleSection('storage');
      expect(isCollapsed('storage')).toBe(false);
    });

    it('toggles a default-expanded section to collapsed', () => {
      const { isCollapsed, toggleSection } = useCollapsedSections();

      expect(isCollapsed('nodes')).toBe(false);
      toggleSection('nodes');
      expect(isCollapsed('nodes')).toBe(true);
    });

    it('toggles back and forth', () => {
      const { isCollapsed, toggleSection } = useCollapsedSections();

      expect(isCollapsed('nodes')).toBe(false);
      toggleSection('nodes');
      expect(isCollapsed('nodes')).toBe(true);
      toggleSection('nodes');
      expect(isCollapsed('nodes')).toBe(false);
    });

    it('persists toggle to localStorage', () => {
      const { toggleSection } = useCollapsedSections();

      toggleSection('nodes');

      // Verify setItem was called with the updated state
      const calls = localStorageMock.setItem.mock.calls.filter(
        (c: string[]) => c[0] === STORAGE_KEY,
      );
      expect(calls.length).toBeGreaterThan(0);
      const lastWritten = JSON.parse(calls[calls.length - 1][1]);
      expect(lastWritten.nodes).toBe(true);
    });
  });

  describe('setCollapsed', () => {
    it('explicitly sets a section to collapsed', () => {
      const { isCollapsed, setCollapsed } = useCollapsedSections();

      expect(isCollapsed('nodes')).toBe(false);
      setCollapsed('nodes', true);
      expect(isCollapsed('nodes')).toBe(true);
    });

    it('explicitly sets a section to expanded', () => {
      const { isCollapsed, setCollapsed } = useCollapsedSections();

      expect(isCollapsed('storage')).toBe(true);
      setCollapsed('storage', false);
      expect(isCollapsed('storage')).toBe(false);
    });

    it('does not affect other sections', () => {
      const { isCollapsed, setCollapsed } = useCollapsedSections();

      setCollapsed('nodes', true);

      expect(isCollapsed('nodes')).toBe(true);
      expect(isCollapsed('storage')).toBe(true); // default
      expect(isCollapsed('vms')).toBe(false); // default
    });
  });

  describe('expandAll', () => {
    it('expands all default-collapsed sections', () => {
      const { isCollapsed, expandAll } = useCollapsedSections();

      // Verify some are collapsed first
      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('backups')).toBe(true);

      expandAll();

      expect(isCollapsed('storage')).toBe(false);
      expect(isCollapsed('backups')).toBe(false);
      expect(isCollapsed('snapshots')).toBe(false);
      expect(isCollapsed('pbs')).toBe(false);
    });

    it('expands manually collapsed sections', () => {
      const { isCollapsed, toggleSection, expandAll } = useCollapsedSections();

      toggleSection('nodes'); // collapse it
      expect(isCollapsed('nodes')).toBe(true);

      expandAll();

      expect(isCollapsed('nodes')).toBe(false);
    });
  });

  describe('collapseAll', () => {
    it('collapses all default sections', () => {
      const { isCollapsed, expandAll, collapseAll } = useCollapsedSections();

      expandAll(); // expand everything first
      expect(isCollapsed('storage')).toBe(false);

      collapseAll();

      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('backups')).toBe(true);
      expect(isCollapsed('snapshots')).toBe(true);
      expect(isCollapsed('pbs')).toBe(true);
    });

    it('collapses previously expanded sections', () => {
      const { isCollapsed, setCollapsed, collapseAll } = useCollapsedSections();

      setCollapsed('nodes', false);
      setCollapsed('vms', false);

      collapseAll();

      expect(isCollapsed('nodes')).toBe(true);
      expect(isCollapsed('vms')).toBe(true);
    });
  });

  describe('resetToDefaults', () => {
    it('resets to default collapsed state after modifications', () => {
      const { isCollapsed, setCollapsed, resetToDefaults } = useCollapsedSections();

      // Modify state
      setCollapsed('storage', false);
      setCollapsed('nodes', true);
      expect(isCollapsed('storage')).toBe(false);
      expect(isCollapsed('nodes')).toBe(true);

      resetToDefaults();

      expect(isCollapsed('storage')).toBe(true); // back to default
      expect(isCollapsed('nodes')).toBe(false); // back to default (not in defaults)
    });

    it('restores all default-collapsed sections', () => {
      const { isCollapsed, expandAll, resetToDefaults } = useCollapsedSections();

      expandAll();
      resetToDefaults();

      expect(isCollapsed('storage')).toBe(true);
      expect(isCollapsed('backups')).toBe(true);
      expect(isCollapsed('snapshots')).toBe(true);
      expect(isCollapsed('pbs')).toBe(true);
    });
  });

  describe('multiple hook instances', () => {
    it('independent instances do not share in-memory state', () => {
      const hook1 = useCollapsedSections();
      const hook2 = useCollapsedSections();

      hook1.toggleSection('nodes');

      // hook2 should still reflect its own initial state
      // (both loaded from same localStorage at creation time, but signals are independent)
      expect(hook1.isCollapsed('nodes')).toBe(true);
      expect(hook2.isCollapsed('nodes')).toBe(false);
    });
  });

  describe('write-side persistence', () => {
    it('persists setCollapsed changes to localStorage', () => {
      const { setCollapsed } = useCollapsedSections();

      setCollapsed('nodes', true);

      const calls = localStorageMock.setItem.mock.calls.filter(
        (c: string[]) => c[0] === STORAGE_KEY,
      );
      expect(calls.length).toBeGreaterThan(0);
      const lastWritten = JSON.parse(calls[calls.length - 1][1]);
      expect(lastWritten.nodes).toBe(true);
    });

    it('persists expandAll to localStorage', () => {
      const { expandAll } = useCollapsedSections();

      expandAll();

      const calls = localStorageMock.setItem.mock.calls.filter(
        (c: string[]) => c[0] === STORAGE_KEY,
      );
      expect(calls.length).toBeGreaterThan(0);
      const lastWritten = JSON.parse(calls[calls.length - 1][1]);
      expect(lastWritten.storage).toBe(false);
      expect(lastWritten.backups).toBe(false);
      expect(lastWritten.snapshots).toBe(false);
      expect(lastWritten.pbs).toBe(false);
    });

    it('persists collapseAll to localStorage', () => {
      const { expandAll, collapseAll } = useCollapsedSections();

      expandAll();
      collapseAll();

      const calls = localStorageMock.setItem.mock.calls.filter(
        (c: string[]) => c[0] === STORAGE_KEY,
      );
      const lastWritten = JSON.parse(calls[calls.length - 1][1]);
      expect(lastWritten.storage).toBe(true);
      expect(lastWritten.backups).toBe(true);
    });

    it('persists resetToDefaults to localStorage', () => {
      const { setCollapsed, resetToDefaults } = useCollapsedSections();

      setCollapsed('nodes', true);
      resetToDefaults();

      const calls = localStorageMock.setItem.mock.calls.filter(
        (c: string[]) => c[0] === STORAGE_KEY,
      );
      const lastWritten = JSON.parse(calls[calls.length - 1][1]);
      // 'nodes' should not be in reset state (only defaults)
      expect(lastWritten.nodes).toBeUndefined();
      expect(lastWritten.storage).toBe(true);
    });

    it('new instance rehydrates mutated state from storage', () => {
      const hook1 = useCollapsedSections();

      // Mutate via first instance
      hook1.setCollapsed('nodes', true);
      hook1.setCollapsed('storage', false);

      // Create a new instance — it should read from localStorage
      const hook2 = useCollapsedSections();

      expect(hook2.isCollapsed('nodes')).toBe(true);
      expect(hook2.isCollapsed('storage')).toBe(false);
    });
  });
});
