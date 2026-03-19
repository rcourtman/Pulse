import { describe, expect, it } from 'vitest';

import { humanizeToken } from '@/utils/textPresentation';

describe('textPresentation', () => {
  it('humanizes underscore-separated tokens with a fallback', () => {
    expect(humanizeToken('hello_world')).toBe('Hello World');
    expect(humanizeToken('')).toBe('');
    expect(humanizeToken(undefined, { fallback: 'Unknown' })).toBe('Unknown');
  });

  it('preserves short all-caps tokens when requested', () => {
    expect(humanizeToken('IP', { preserveShortAllCaps: true })).toBe('IP');
    expect(humanizeToken('vm')).toBe('Vm');
  });
});
