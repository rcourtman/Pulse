import { describe, expect, it } from 'vitest';
import {
  createHiddenCanonicalTypeColumn,
  createVisibleCanonicalTypeColumn,
} from '@/utils/typeColumnDefinition';
import {
  TYPE_COLUMN_MAX_WIDTH,
  TYPE_COLUMN_MIN_WIDTH,
  TYPE_COLUMN_SORT_KEY,
  TYPE_COLUMN_WIDTH,
} from '@/utils/typeColumnContract';

describe('typeColumnDefinition', () => {
  it('creates the canonical visible Type column by default', () => {
    expect(createVisibleCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: false,
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
    });
  });

  it('passes through canonical size and sort options', () => {
    expect(createVisibleCanonicalTypeColumn({ width: '72px' })).toMatchObject({
      width: '72px',
      sortKey: TYPE_COLUMN_SORT_KEY,
      defaultHidden: false,
    });
  });

  it('provides a visible preset helper for standard type columns', () => {
    expect(createVisibleCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: TYPE_COLUMN_WIDTH,
      defaultHidden: false,
    });
  });

  it('provides a hidden preset helper for recovery-style type columns', () => {
    expect(createHiddenCanonicalTypeColumn({ width: '72px' })).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: '72px',
      defaultHidden: true,
    });
  });

  it('exports the canonical responsive type sizing contract', () => {
    expect(TYPE_COLUMN_MIN_WIDTH).toBe('60px');
    expect(TYPE_COLUMN_MAX_WIDTH).toBe('80px');
  });
});
