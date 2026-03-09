import { describe, expect, it } from 'vitest';
import {
  createHiddenCanonicalTypeColumn,
  createVisibleCanonicalTypeColumn,
} from '@/utils/typeColumnDefinition';

describe('typeColumnDefinition', () => {
  it('creates the canonical visible Type column by default', () => {
    expect(createVisibleCanonicalTypeColumn()).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      defaultHidden: false,
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
    expect(createVisibleCanonicalTypeColumn({ width: '60px', sortKey: 'type' })).toMatchObject({
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
    expect(createHiddenCanonicalTypeColumn({ width: '72px' })).toMatchObject({
      id: 'type',
      label: 'Type',
      toggleable: true,
      width: '72px',
      defaultHidden: true,
    });
  });
});
