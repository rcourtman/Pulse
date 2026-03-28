import { describe, expect, it } from 'vitest';

import { normalizePortalRole, portalRoleCapabilityCopy, portalRoleLabel } from './account_roles';

describe('account roles', function() {
  it('normalizes internal and legacy read-only roles into product labels', function() {
    expect(normalizePortalRole('member')).toBe('read_only');
    expect(normalizePortalRole('read_only')).toBe('read_only');
    expect(portalRoleLabel('member')).toBe('Read-only');
    expect(portalRoleLabel('read_only')).toBe('Read-only');
  });

  it('returns product copy for read-only operators', function() {
    expect(portalRoleCapabilityCopy('read_only')).toContain('review workspace status');
  });
});
