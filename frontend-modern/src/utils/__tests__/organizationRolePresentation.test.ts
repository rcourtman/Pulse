import { describe, expect, it } from 'vitest';
import {
  getOrganizationShareRoleOption,
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

  it('uses the same audience choices for saved shares without guessing at an unrecognized role', () => {
    for (const option of ORGANIZATION_SHARE_ROLE_OPTIONS) {
      expect(getOrganizationShareRoleOption(option.value)).toBe(option);
    }
    expect(getOrganizationShareRoleOption('owner')).toBeUndefined();
    expect(getOrganizationShareRoleOption(undefined)).toBeUndefined();
  });
});
