import { describe, expect, it } from 'vitest';

import {
  formatIdentifierLabel,
  formatTrimmedLabel,
  humanizeArrowDelimitedLabel,
  humanizeToken,
  titleCaseDelimitedLabel,
} from '@/utils/textPresentation';

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

  it('formats trimmed labels with a fallback', () => {
    expect(formatTrimmedLabel('  endpoint  ', { fallback: 'Unknown resource' })).toBe('endpoint');
    expect(formatTrimmedLabel('', { fallback: 'Unknown resource' })).toBe('Unknown resource');
  });

  it('title-cases delimited labels with configurable separators', () => {
    expect(titleCaseDelimitedLabel('docker_host')).toBe('Docker Host');
    expect(titleCaseDelimitedLabel('IP_address', { preserveShortAllCaps: true })).toBe(
      'IP Address',
    );
    expect(titleCaseDelimitedLabel('k8s-node', { preserveShortAllCaps: true })).toBe('K8s Node');
    expect(
      titleCaseDelimitedLabel('agent.profile.suggestion', {
        separators: /[._]+/,
      }),
    ).toBe('Agent Profile Suggestion');
    expect(titleCaseDelimitedLabel('', { fallback: 'Unknown' })).toBe('Unknown');
  });

  it('humanizes arrow-delimited labels', () => {
    expect(humanizeArrowDelimitedLabel('disk_full -> restart', { fallback: 'Correlation' })).toBe(
      'Disk Full → Restart',
    );
    expect(humanizeArrowDelimitedLabel('', { fallback: 'Correlation' })).toBe('Correlation');
  });
});
