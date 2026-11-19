import { describe, expect, it } from 'vitest';
import {
  sameRoleIDs,
  getRoleIdentityLabel,
  getRBACFeatureGateCopy,
  getRolesDeleteErrorMessage,
  getRolesEmptyState,
  getRolesLoadErrorMessage,
  getRolesRequiredFieldsMessage,
  getRolesSaveErrorMessage,
  getUserAssignmentsLoadErrorMessage,
  getUserAssignmentsDeleteErrorMessage,
  getUserAssignmentsEmptyStateCopy,
  getUserAssignmentsUpdateErrorMessage,
  getUserIdentityDisplayName,
  getUserIdentityLabel,
  getUserIdentityProviderLabel,
  isCompleteRoleEditorInventory,
  isCompleteUserAccessInventory,
} from '@/utils/rbacPresentation';

describe('saved role inventory validation', () => {
  const identity = Object.freeze({ id: 'scope/Reader ', name: 'Fleet reader', description: '' });
  const complete = Object.freeze({
    ...identity,
    permissions: [{ action: 'future:inspect', resource: 'scope/nodes', effect: 'unknown' }],
  });
  const users = [{ username: 'fixture-reader', roleIds: [identity.id, 'saved/unlisted'] }];

  it.each([
    { name: 'missing list', roles: null },
    { name: 'non-list response', roles: {} },
    { name: 'missing role', roles: [null] },
    { name: 'blank ID', roles: [{ ...complete, id: ' ' }] },
    { name: 'non-string ID', roles: [{ ...complete, id: 42 }] },
    { name: 'non-string name', roles: [{ ...complete, name: null }] },
    { name: 'missing description', roles: [{ ...complete, description: undefined }] },
    { name: 'duplicate exact ID', roles: [complete, { ...complete, name: 'Renamed' }] },
  ])('rejects $name in both consumers', ({ roles }) => {
    expect(isCompleteRoleEditorInventory(roles)).toBe(false);
    expect(isCompleteUserAccessInventory(users, roles)).toBe(false);
  });

  it.each([
    { name: 'identity only', role: identity },
    { name: 'non-boolean built-in status', role: { ...complete, isBuiltIn: 'true' } },
    { name: 'non-array rules', role: { ...complete, permissions: {} } },
    { name: 'missing rule', role: { ...complete, permissions: [null] } },
    { name: 'blank action', role: { ...complete, permissions: [{ action: ' ', resource: 'x' }] } },
    { name: 'missing resource', role: { ...complete, permissions: [{ action: 'read' }] } },
  ])('requires rules for Roles, but not assignment choices: $name', ({ role }) => {
    expect(isCompleteRoleEditorInventory([role])).toBe(false);
    expect(isCompleteUserAccessInventory(users, [role])).toBe(true);
  });

  it('accepts empty inventories and keeps distinct opaque IDs, built-ins and uncatalogued targets', () => {
    const roles = Object.freeze([
      complete,
      { ...identity, id: 'scope/Reader', isBuiltIn: true, permissions: [] },
      { ...identity, id: 'scope/reader ', isBuiltIn: false, permissions: [] },
    ]);
    expect(isCompleteRoleEditorInventory(roles)).toBe(true);
    expect(isCompleteUserAccessInventory(users, roles)).toBe(true);
    expect(isCompleteRoleEditorInventory([])).toBe(true);
    expect(isCompleteUserAccessInventory([], [])).toBe(true);
    expect(isCompleteUserAccessInventory([users[0], users[0]], roles)).toBe(false);
    expect(roles.map((role) => role.id)).toEqual([
      'scope/Reader ',
      'scope/Reader',
      'scope/reader ',
    ]);
  });
});

describe('rbacPresentation', () => {
  it.each([
    { first: [], second: [], same: true },
    { first: ['reader', 'writer'], second: ['writer', 'reader'], same: true },
    { first: ['reader'], second: [], same: false },
    { first: [], second: ['reader'], same: false },
    { first: ['reader'], second: ['Reader'], same: false },
    { first: ['reader'], second: [' reader'], same: false },
    { first: ['reader', 'reader'], second: ['reader', 'reader'], same: true },
    { first: ['reader', 'reader'], second: ['reader'], same: false },
    { first: ['a,b', 'c'], second: ['a', 'b,c'], same: false },
    { first: ['a\u0000b', 'c'], second: ['a', 'b\u0000c'], same: false },
    { first: ['saved-unlisted', 'reader'], second: ['reader', 'saved-unlisted'], same: true },
  ])(
    'compares exact unordered role IDs without changing either input: %j',
    ({ first, second, same }) => {
      const firstCopy = [...first];
      const secondCopy = [...second];
      expect(sameRoleIDs(Object.freeze(first), Object.freeze(second))).toBe(same);
      expect(first).toEqual(firstCopy);
      expect(second).toEqual(secondCopy);
    },
  );

  it('identifies namesake roles by their unchanged saved keys, not mutable names', () => {
    expect(getRoleIdentityLabel({ id: 'policy/north:read', name: 'Fleet auditor' })).toBe(
      'Fleet auditor (Role ID: policy/north:read)',
    );
    expect(getRoleIdentityLabel({ id: 'policy/south:read', name: 'Fleet auditor' })).toBe(
      'Fleet auditor (Role ID: policy/south:read)',
    );
    expect(getRoleIdentityLabel({ id: 'viewer', name: 'viewer' })).toBe('Role ID: viewer');
    expect(getRoleIdentityLabel({ id: 'ops ', name: '' })).toBe('Role ID: ops ');
    expect(getRoleIdentityLabel({ id: 'ops', name: '  ' })).toBe('Role ID: ops');
    expect(getRoleIdentityLabel({ id: 'actual/key', name: 'Viewer (policy/admin)' })).toBe(
      'Viewer (policy/admin) (Role ID: actual/key)',
    );
  });

  it('returns canonical feature gate copy', () => {
    expect(getRBACFeatureGateCopy('roles')).toMatchObject({
      title: 'Custom Roles',
      body: expect.stringContaining('paid self-hosted and hosted plans'),
    });
    expect(getRBACFeatureGateCopy('user-assignments')).toMatchObject({
      title: 'Centralized Access Control',
      body: expect.stringContaining('paid self-hosted and hosted plans'),
    });
  });

  it('returns neutral feature gate copy when commercial prompts are hidden', () => {
    expect(getRBACFeatureGateCopy('roles', { showCommercialCopy: false })).toMatchObject({
      title: 'Custom Roles',
      body: expect.not.stringContaining('Pro'),
    });
    expect(getRBACFeatureGateCopy('user-assignments', { showCommercialCopy: false })).toMatchObject(
      {
        title: 'Centralized Access Control',
        body: expect.not.stringContaining('Pro'),
      },
    );
  });

  it('returns canonical user assignments empty state copy', () => {
    expect(getUserAssignmentsEmptyStateCopy()).toMatchObject({
      title: 'No users yet',
      ssoHint: 'Configure SSO in Security settings',
      syncHint: 'Users sync on first login',
    });
  });

  it('returns canonical roles empty state copy', () => {
    expect(getRolesEmptyState()).toBe('No roles available.');
  });

  it('presents mutable identity claims without replacing the stable principal', () => {
    const user = {
      username: 'sso:oidc:okta:stable',
      displayName: 'Alice Example',
      email: 'alice@example.com',
      providerType: 'oidc',
      providerId: 'okta',
    };
    expect(getUserIdentityDisplayName(user)).toBe('Alice Example');
    expect(getUserIdentityProviderLabel(user)).toBe('OIDC · okta');
    expect(getUserIdentityDisplayName({ username: 'local-admin' })).toBe('local-admin');
  });

  it('distinguishes namesake identities without duplicating a local username', () => {
    const first = { username: 'sso:oidc:workforce:subject-01', displayName: 'Alex Morgan' };
    const second = { ...first, username: 'sso:oidc:workforce:subject-02' };
    expect(getUserIdentityLabel(first)).toBe('Alex Morgan (sso:oidc:workforce:subject-01)');
    expect(getUserIdentityLabel(second)).toBe('Alex Morgan (sso:oidc:workforce:subject-02)');
    expect(getUserIdentityLabel({ username: 'local-admin' })).toBe('local-admin');
    expect(getUserIdentityLabel({ username: 'subject', email: '  alex@example.invalid  ' })).toBe(
      'alex@example.invalid (subject)',
    );
  });

  it('returns canonical RBAC admin error copy', () => {
    expect(getRolesLoadErrorMessage()).toBe('Failed to load roles');
    expect(getRolesDeleteErrorMessage()).toBe('Failed to delete role');
    expect(getRolesRequiredFieldsMessage()).toBe('ID and Name are required');
    expect(getRolesSaveErrorMessage()).toBe('Failed to save role');
    expect(getUserAssignmentsLoadErrorMessage()).toBe('Failed to load user assignments');
    expect(getUserAssignmentsUpdateErrorMessage()).toBe('Failed to update user roles');
    expect(getUserAssignmentsDeleteErrorMessage()).toBe('Failed to remove user access');
    expect(
      getUserAssignmentsDeleteErrorMessage({
        status: 403,
        code: 'self_deprovision_denied',
      }),
    ).toBe('You cannot remove your own user access.');
  });
});
