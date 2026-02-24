import { describe, expect, it } from 'vitest';
import { normalizePublicReleaseTrack } from './publicRelease';

describe('normalizePublicReleaseTrack', () => {
  it('defaults to v5 for missing values', () => {
    expect(normalizePublicReleaseTrack(undefined)).toBe('v5');
    expect(normalizePublicReleaseTrack(null)).toBe('v5');
    expect(normalizePublicReleaseTrack('')).toBe('v5');
  });

  it('accepts v6 values case-insensitively', () => {
    expect(normalizePublicReleaseTrack('v6')).toBe('v6');
    expect(normalizePublicReleaseTrack('V6')).toBe('v6');
    expect(normalizePublicReleaseTrack('  v6  ')).toBe('v6');
  });

  it('falls back to v5 for unknown values', () => {
    expect(normalizePublicReleaseTrack('v5')).toBe('v5');
    expect(normalizePublicReleaseTrack('launch')).toBe('v5');
  });
});
