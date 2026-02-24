import { describe, expect, it } from 'vitest';
import { normalizeRole, canManageOrg, roleBadgeClass, formatOrgDate } from '@/utils/orgUtils';
import type { Organization } from '@/api/orgs';

describe('normalizeRole', () => {
  it('returns viewer for member role', () => {
    expect(normalizeRole('member')).toBe('viewer');
  });

  it('returns owner for owner role', () => {
    expect(normalizeRole('owner')).toBe('owner');
  });

  it('returns admin for admin role', () => {
    expect(normalizeRole('admin')).toBe('admin');
  });

  it('returns editor for editor role', () => {
    expect(normalizeRole('editor')).toBe('editor');
  });

  it('returns viewer for viewer role', () => {
    expect(normalizeRole('viewer')).toBe('viewer');
  });
});

describe('canManageOrg', () => {
  const createOrg = (overrides: Partial<Organization> = {}): Organization => ({
    id: 'org-1',
    displayName: 'Test Org',
    ownerUserId: 'owner-1',
    members: [],
    createdAt: '2024-01-01',
    ...overrides,
  });

  it('returns false when org is null', () => {
    expect(canManageOrg(null, 'user-1')).toBe(false);
  });

  it('returns false when org is null/undefined', () => {
    expect(canManageOrg(undefined as unknown as Organization, 'user-1')).toBe(false);
  });

  it('returns false when currentUser is not provided', () => {
    const org = createOrg();
    expect(canManageOrg(org)).toBe(false);
  });

  it('returns true when user is owner', () => {
    const org = createOrg({ ownerUserId: 'user-1' });
    expect(canManageOrg(org, 'user-1')).toBe(true);
  });

  it('returns true when user has admin role', () => {
    const org = createOrg({
      members: [{ userId: 'user-1', role: 'admin', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(true);
  });

  it('returns true when user has owner role via member', () => {
    const org = createOrg({
      members: [{ userId: 'user-1', role: 'owner', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(true);
  });

  it('returns false when user has viewer role', () => {
    const org = createOrg({
      members: [{ userId: 'user-1', role: 'viewer', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });

  it('returns false when user has editor role', () => {
    const org = createOrg({
      members: [{ userId: 'user-1', role: 'editor', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });

  it('treats member role as viewer', () => {
    const org = createOrg({
      members: [{ userId: 'user-1', role: 'member', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });

  it('returns false when user is not in members list', () => {
    const org = createOrg({
      members: [{ userId: 'other-user', role: 'admin', addedAt: '2024-01-01T00:00:00Z' }],
    });
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });

  it('returns false when members array is empty', () => {
    const org = createOrg({ members: [] });
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });

  it('returns false when members is undefined', () => {
    const org = createOrg({ members: undefined } as unknown as Organization);
    expect(canManageOrg(org, 'user-1')).toBe(false);
  });
});

describe('roleBadgeClass', () => {
  it('returns owner badge class', () => {
    const result = roleBadgeClass('owner');
    expect(result).toContain('bg-purple-100');
    expect(result).toContain('text-purple-800');
  });

  it('returns admin badge class', () => {
    const result = roleBadgeClass('admin');
    expect(result).toContain('bg-blue-100');
    expect(result).toContain('text-blue-800');
  });

  it('returns editor badge class', () => {
    const result = roleBadgeClass('editor');
    expect(result).toContain('bg-emerald-100');
    expect(result).toContain('text-emerald-800');
  });

  it('returns viewer badge class', () => {
    const result = roleBadgeClass('viewer');
    expect(result).toContain('bg-slate-100');
    expect(result).toContain('text-slate-700');
  });

  it('returns member badge class', () => {
    const result = roleBadgeClass('member');
    expect(result).toContain('bg-slate-100');
    expect(result).toContain('text-slate-700');
  });

  it('returns default badge class for unknown role', () => {
    const result = roleBadgeClass('unknown-role');
    expect(result).toContain('bg-slate-100');
    expect(result).toContain('text-slate-700');
  });

  it('returns default badge class for empty string', () => {
    const result = roleBadgeClass('');
    expect(result).toContain('bg-slate-100');
    expect(result).toContain('text-slate-700');
  });
});

describe('formatOrgDate', () => {
  it('returns Unknown for undefined', () => {
    expect(formatOrgDate(undefined)).toBe('Unknown');
  });

  it('returns Unknown for empty string', () => {
    expect(formatOrgDate('')).toBe('Unknown');
  });

  it('returns the value for invalid date string', () => {
    expect(formatOrgDate('not-a-date')).toBe('not-a-date');
  });

  it('formats valid ISO date string', () => {
    const result = formatOrgDate('2024-06-15T10:30:00Z');
    expect(result).toMatch(/\d{1,2}\/\d{1,2}\/\d{4}/);
  });

  it('formats date with only date part', () => {
    const result = formatOrgDate('2024-12-25');
    expect(result).toMatch(/\d{1,2}\/\d{1,2}\/\d{4}/);
  });
});
