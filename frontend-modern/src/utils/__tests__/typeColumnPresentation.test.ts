import { describe, expect, it } from 'vitest';
import { TYPE_COLUMN_LABEL } from '@/utils/typeColumnContract';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

describe('typeColumnPresentation', () => {
  it('returns the canonical Type column label', () => {
    expect(getTypeColumnLabel()).toBe(TYPE_COLUMN_LABEL);
  });
});
