import { describe, expect, it } from 'vitest';
import {
  getRBACFeatureGateCopy,
  getRolesDeleteErrorMessage,
  getRolesEmptyState,
  getRolesLoadErrorMessage,
  getRolesRequiredFieldsMessage,
  getRolesSaveErrorMessage,
  getUserAssignmentsLoadErrorMessage,
  getUserAssignmentsEmptyStateCopy,
  getUserAssignmentsUpdateErrorMessage,
} from '@/utils/rbacPresentation';

describe('rbacPresentation', () => {
  it('returns canonical feature gate copy', () => {
    expect(getRBACFeatureGateCopy('roles')).toMatchObject({
      title: 'Custom Roles (Pro)',
    });
    expect(getRBACFeatureGateCopy('user-assignments')).toMatchObject({
      title: 'Centralized Access Control (Pro)',
    });
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

  it('returns canonical RBAC admin error copy', () => {
    expect(getRolesLoadErrorMessage()).toBe('Failed to load roles');
    expect(getRolesDeleteErrorMessage()).toBe('Failed to delete role');
    expect(getRolesRequiredFieldsMessage()).toBe('ID and Name are required');
    expect(getRolesSaveErrorMessage()).toBe('Failed to save role');
    expect(getUserAssignmentsLoadErrorMessage()).toBe('Failed to load user assignments');
    expect(getUserAssignmentsUpdateErrorMessage()).toBe('Failed to update user roles');
  });
});
