import { describe, expect, it } from 'vitest';
import { DEFAULT_ORG_SCOPE, normalizeOrgScope } from '@/utils/orgScope';

describe('orgScope', () => {
  it('normalizes empty org ids to the default scope', () => {
    expect(normalizeOrgScope(undefined)).toBe(DEFAULT_ORG_SCOPE);
    expect(normalizeOrgScope(null)).toBe(DEFAULT_ORG_SCOPE);
    expect(normalizeOrgScope('')).toBe(DEFAULT_ORG_SCOPE);
    expect(normalizeOrgScope('   ')).toBe(DEFAULT_ORG_SCOPE);
  });

  it('trims explicit org ids without changing their meaning', () => {
    expect(normalizeOrgScope(' acme ')).toBe('acme');
    expect(normalizeOrgScope('tenant-1')).toBe('tenant-1');
  });
});
