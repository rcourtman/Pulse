import { describe, expect, it } from 'vitest';
import {
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
  getUserIdentityProviderLabel,
} from '@/utils/rbacPresentation';

describe('rbacPresentation', () => {
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
