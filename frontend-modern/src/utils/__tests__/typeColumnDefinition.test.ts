import { describe, expect, it } from 'vitest';
import {
  createCanonicalTypeColumn,
  createHiddenCanonicalTypeColumn,
  createVisibleCanonicalTypeColumn,
} from '@/utils/typeColumnDefinition';

describe('typeColumnDefinition', () => {
  it('creates the canonical visible Type column by default', () => {
    expect(createCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: false,
    });
  });

  it('supports an explicit hidden-by-default visibility preset', () => {
    expect(createCanonicalTypeColumn({ defaultVisibility: 'hidden' })).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: true,
    });
  });

  it('passes through canonical size and sort options', () => {
    expect(createCanonicalTypeColumn({ width: '60px', sortKey: 'type' })).toMatchObject({
      width: '60px',
      sortKey: 'type',
      defaultHidden: false,
    });
  });

  it('provides a visible preset helper for standard type columns', () => {
    expect(createVisibleCanonicalTypeColumn({ width: '60px' })).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: '60px',
      defaultHidden: false,
    });
  });

  it('provides a hidden preset helper for recovery-style type columns', () => {
    expect(createHiddenCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: true,
    });
  });
});
