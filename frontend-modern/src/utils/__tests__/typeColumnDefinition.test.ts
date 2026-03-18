import { describe, expect, it } from 'vitest';
import {
  createHiddenCanonicalTypeColumn,
  createVisibleCanonicalTypeColumn,
} from '@/utils/typeColumnDefinition';
import {
  TYPE_COLUMN_MAX_WIDTH,
  TYPE_COLUMN_MIN_WIDTH,
  TYPE_COLUMN_PRIORITY,
  TYPE_COLUMN_SORT_KEY,
  TYPE_COLUMN_SORTABLE,
  TYPE_COLUMN_WIDTH,
} from '@/utils/typeColumnContract';

describe('typeColumnDefinition', () => {
  it('creates the canonical visible Type column by default', () => {
    expect(createVisibleCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: TYPE_COLUMN_WIDTH,
      sortKey: TYPE_COLUMN_SORT_KEY,
    });
  });

  it('supports a hidden-by-default visibility preset', () => {
    expect(createHiddenCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: true,
      width: TYPE_COLUMN_WIDTH,
      sortKey: TYPE_COLUMN_SORT_KEY,
    });
  });

  it('provides a visible preset helper for standard type columns', () => {
    expect(createVisibleCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: TYPE_COLUMN_WIDTH,
    });
  });

  it('provides a hidden preset helper for recovery-style type columns', () => {
    expect(createHiddenCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: true,
      width: TYPE_COLUMN_WIDTH,
      sortKey: TYPE_COLUMN_SORT_KEY,
    });
  });

  it('exports the canonical responsive type sizing contract', () => {
    expect(TYPE_COLUMN_MIN_WIDTH).toBe('60px');
    expect(TYPE_COLUMN_MAX_WIDTH).toBe('80px');
    expect(TYPE_COLUMN_PRIORITY).toBe('essential');
    expect(TYPE_COLUMN_SORTABLE).toBe(true);
  });
});
