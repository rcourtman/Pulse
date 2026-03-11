import { describe, expect, it } from 'vitest';
import {
  normalizeOrganizationShareRole,
  ORGANIZATION_MEMBER_ROLE_OPTIONS,
  ORGANIZATION_SHARE_ROLE_OPTIONS,
} from '@/utils/organizationRolePresentation';

describe('organizationRolePresentation', () => {
  it('exposes canonical member and share role options', () => {
    expect(ORGANIZATION_MEMBER_ROLE_OPTIONS.map((option) => option.value)).toEqual([
      'viewer',
      'editor',
      'admin',
      'owner',
    ]);
    expect(ORGANIZATION_SHARE_ROLE_OPTIONS.map((option) => option.value)).toEqual([
      'viewer',
      'editor',
      'admin',
    ]);
  });

  it('normalizes organization share roles onto the supported subset', () => {
    expect(normalizeOrganizationShareRole('admin')).toBe('admin');
    expect(normalizeOrganizationShareRole('editor')).toBe('editor');
    expect(normalizeOrganizationShareRole('viewer')).toBe('viewer');
    expect(normalizeOrganizationShareRole('owner')).toBe('viewer');
  });
});
