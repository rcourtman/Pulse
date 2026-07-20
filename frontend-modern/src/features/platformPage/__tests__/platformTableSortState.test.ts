import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import {
  createPlatformTableSortState,
  getPlatformTableDateTimeSortValue,
  type PlatformTableSortValue,
} from '../sharedPlatformPage';

const SORT_KEYS = ['name', 'cpu'] as const;
type SortKey = (typeof SORT_KEYS)[number];

type Row = { name: string; cpu: number | null };

const getSortValue = (row: Row, key: SortKey): PlatformTableSortValue =>
  key === 'name' ? row.name : row.cpu;

const ROWS: readonly Row[] = [
  { name: 'bravo', cpu: 40 },
  { name: 'alpha', cpu: null },
  { name: 'charlie', cpu: 90 },
];

const withSortState = <T>(
  fn: (sort: ReturnType<typeof createPlatformTableSortState<SortKey>>) => T,
): T =>
  createRoot((dispose) => {
    const sort = createPlatformTableSortState({
      storageKey: 'testTable',
      sortKeys: SORT_KEYS,
      descendingFirst: ['cpu'],
    });
    const result = fn(sort);
    dispose();
    return result;
  });

afterEach(() => {
  window.localStorage.clear();
});

describe('createPlatformTableSortState', () => {
  it('keeps the built-in row order until a header is clicked', () => {
    withSortState((sort) => {
      expect(sort.sortKey()).toBeNull();
      expect(sort.sortRows(ROWS, getSortValue)).toEqual(ROWS);
      expect(sort.getAriaSort('name')).toBeUndefined();
    });
  });

  it('cycles a text column asc → desc → built-in order', () => {
    withSortState((sort) => {
      sort.handleSort('name');
      expect(sort.getAriaSort('name')).toBe('ascending');
      expect(sort.sortRows(ROWS, getSortValue).map((row) => row.name)).toEqual([
        'alpha',
        'bravo',
        'charlie',
      ]);

      sort.handleSort('name');
      expect(sort.getAriaSort('name')).toBe('descending');
      expect(sort.sortRows(ROWS, getSortValue).map((row) => row.name)).toEqual([
        'charlie',
        'bravo',
        'alpha',
      ]);

      sort.handleSort('name');
      expect(sort.sortKey()).toBeNull();
      expect(sort.sortRows(ROWS, getSortValue)).toEqual(ROWS);
    });
  });

  it('starts metric columns descending and keeps missing values last both ways', () => {
    withSortState((sort) => {
      sort.handleSort('cpu');
      expect(sort.getAriaSort('cpu')).toBe('descending');
      expect(sort.sortRows(ROWS, getSortValue).map((row) => row.name)).toEqual([
        'charlie',
        'bravo',
        'alpha',
      ]);

      sort.handleSort('cpu');
      expect(sort.getAriaSort('cpu')).toBe('ascending');
      // alpha has no CPU value, so it stays at the bottom in either direction.
      expect(sort.sortRows(ROWS, getSortValue).map((row) => row.name)).toEqual([
        'bravo',
        'charlie',
        'alpha',
      ]);
    });
  });

  it('switching columns applies the new column first-click direction', () => {
    withSortState((sort) => {
      sort.handleSort('name');
      sort.handleSort('cpu');
      expect(sort.sortKey()).toBe('cpu');
      expect(sort.getAriaSort('cpu')).toBe('descending');
      expect(sort.getAriaSort('name')).toBeUndefined();
    });
  });

  it('restores a persisted sort and ignores stale persisted keys', () => {
    window.localStorage.setItem('testTableSortKey', 'name');
    window.localStorage.setItem('testTableSortDirection', 'desc');
    withSortState((sort) => {
      expect(sort.sortKey()).toBe('name');
      expect(sort.sortDirection()).toBe('desc');
    });

    window.localStorage.setItem('testTableSortKey', 'removed-column');
    withSortState((sort) => {
      expect(sort.sortKey()).toBeNull();
    });
  });
});

describe('getPlatformTableDateTimeSortValue', () => {
  it('maps timestamps to epoch millis and unparsable input to null', () => {
    expect(getPlatformTableDateTimeSortValue('2026-01-02T03:04:05Z')).toBe(
      Date.parse('2026-01-02T03:04:05Z'),
    );
    expect(getPlatformTableDateTimeSortValue('not a date')).toBeNull();
    expect(getPlatformTableDateTimeSortValue(undefined)).toBeNull();
    expect(getPlatformTableDateTimeSortValue('')).toBeNull();
  });
});
