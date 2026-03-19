import { describe, expect, it } from 'vitest';

import { formatIdentifierLabel, humanizeToken } from '@/utils/textPresentation';

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

  it('formats identifier labels without title-casing', () => {
    expect(formatIdentifierLabel('pulse_get_container_status', { stripPrefix: 'pulse_' })).toBe(
      'get container status',
    );
    expect(formatIdentifierLabel('some_reason')).toBe('some reason');
    expect(formatIdentifierLabel('pulse_very_long_unknown_tool_name', {
      stripPrefix: 'pulse_',
      maxLength: 12,
    })).toBe('very long un');
    expect(formatIdentifierLabel(undefined, { fallback: 'Unknown' })).toBe('Unknown');
  });
});
