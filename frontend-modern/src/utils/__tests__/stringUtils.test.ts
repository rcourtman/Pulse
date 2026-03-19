import { describe, expect, it } from 'vitest';

import { asTrimmedString } from '@/utils/stringUtils';

describe('stringUtils', () => {
  it('trims strings and returns undefined for empty values', () => {
    expect(asTrimmedString('  hello  ')).toBe('hello');
    expect(asTrimmedString('')).toBeUndefined();
    expect(asTrimmedString(undefined)).toBeUndefined();
    expect(asTrimmedString(null)).toBeUndefined();
  });
});
