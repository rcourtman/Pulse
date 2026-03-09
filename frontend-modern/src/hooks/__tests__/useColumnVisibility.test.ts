import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createRoot } from 'solid-js';
import { useColumnVisibility, type ColumnDef } from '@/hooks/useColumnVisibility';

describe('useColumnVisibility', () => {
  const storageKey = 'test-column-visibility';

  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it('hides columns marked defaultHidden when no user preference exists', () => {
    createRoot((dispose) => {
      const columns: ColumnDef[] = [
        { id: 'name', label: 'Name' },
        { id: 'type', label: 'Type', toggleable: true, defaultHidden: true },
      ];

      const visibility = useColumnVisibility(storageKey, columns);

      expect(visibility.hiddenColumns()).toEqual(['type']);
      expect(visibility.visibleColumns().map((col) => col.id)).toEqual(['name']);

      dispose();
    });
  });

  it('merges explicit defaultHidden ids with column-level defaults', () => {
    createRoot((dispose) => {
      const columns: ColumnDef[] = [
        { id: 'name', label: 'Name' },
        { id: 'type', label: 'Type', toggleable: true, defaultHidden: true },
        { id: 'ip', label: 'IP', toggleable: true },
      ];

      const visibility = useColumnVisibility(storageKey, columns, ['ip']);

      expect(visibility.hiddenColumns()).toEqual(['type', 'ip']);
      expect(visibility.visibleColumns().map((col) => col.id)).toEqual(['name']);

      dispose();
    });
  });

  it('prefers saved user visibility state over defaultHidden configuration', () => {
    window.localStorage.setItem(storageKey, JSON.stringify(['ip']));

    createRoot((dispose) => {
      const columns: ColumnDef[] = [
        { id: 'name', label: 'Name' },
        { id: 'type', label: 'Type', toggleable: true, defaultHidden: true },
        { id: 'ip', label: 'IP', toggleable: true },
      ];

      const visibility = useColumnVisibility(storageKey, columns, ['ip']);

      expect(visibility.hiddenColumns()).toEqual(['ip']);
      expect(visibility.visibleColumns().map((col) => col.id)).toEqual(['name', 'type']);

      dispose();
    });
  });
});
