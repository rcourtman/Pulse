import { describe, expect, it } from 'vitest';
import {
  getOrganizationAccessEmptyState,
  getOrganizationAddMemberErrorMessage,
  getOrganizationDisplayNameRequiredMessage,
  getOrganizationDisplayNameUpdateErrorMessage,
  getOrganizationIncomingSharesEmptyState,
  getOrganizationMemberUserIdRequiredMessage,
  getOrganizationOwnerRoleLockedMessage,
  getOrganizationOutgoingSharesEmptyState,
  getOrganizationOverviewMembersEmptyState,
  getOrganizationRemoveMemberErrorMessage,
  getOrganizationShareCreateErrorMessage,
  getOrganizationShareCreateSuccessMessage,
  getOrganizationShareDeleteErrorMessage,
  getOrganizationShareDeleteSuccessMessage,
  getOrganizationShareInvalidResourceMessage,
  getOrganizationShareResourceIdRequiredMessage,
  getOrganizationShareTargetOrgDifferentMessage,
  getOrganizationShareTargetOrgRequiredMessage,
  getOrganizationSettingsLoadErrorMessage,
  type OrganizationSettingsLoadContext,
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';

describe('organizationSettingsPresentation', () => {
  it('returns canonical multi-tenant fallback presentation', () => {
    expect(ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE).toBe('This feature is not available.');
    expect(ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS).toContain('text-slate-500');
  });

  it('normalizes organization settings load errors', () => {
    expect(getOrganizationSettingsLoadErrorMessage('request failed 402', 'access')).toBe(
      'Multi-tenant requires an Enterprise license',
    );
    expect(getOrganizationSettingsLoadErrorMessage('request failed 501', 'access')).toBe(
      'Multi-tenant is not enabled on this server',
    );
    const expectedFallbacks: Record<OrganizationSettingsLoadContext, string> = {
      access: 'Failed to load organization access settings',
      overview: 'Failed to load organization details',
      sharing: 'Failed to load organization sharing details',
      billing: 'Failed to load billing and plan details',
      'billing-admin': 'Failed to list organizations',
    };

    for (const [context, fallback] of Object.entries(expectedFallbacks)) {
      expect(
        getOrganizationSettingsLoadErrorMessage(
          'request failed',
          context as OrganizationSettingsLoadContext,
        ),
      ).toBe(fallback);
    }
  });

  it('returns canonical organization empty states', () => {
    expect(getOrganizationAccessEmptyState()).toBe('No organization members found.');
    expect(getOrganizationOverviewMembersEmptyState()).toBe('No members found.');
    expect(getOrganizationOutgoingSharesEmptyState()).toBe('No outgoing shares configured.');
    expect(getOrganizationIncomingSharesEmptyState()).toBe(
      'No incoming shares from other organizations.',
    );
  });

  it('returns canonical organization admin validation and error copy', () => {
    expect(getOrganizationDisplayNameRequiredMessage()).toBe('Display name is required');
    expect(getOrganizationDisplayNameUpdateErrorMessage()).toBe(
      'Failed to update organization name',
    );
    expect(getOrganizationDisplayNameUpdateErrorMessage('bad request')).toBe('bad request');
    expect(getOrganizationOwnerRoleLockedMessage()).toBe(
      'Current owner can only remain owner. Transfer ownership instead.',
    );
    expect(getOrganizationMemberUserIdRequiredMessage()).toBe('User ID is required');
    expect(getOrganizationAddMemberErrorMessage()).toBe('Failed to add member');
    expect(getOrganizationAddMemberErrorMessage('duplicate')).toBe('duplicate');
    expect(getOrganizationRemoveMemberErrorMessage()).toBe('Failed to remove member');
    expect(getOrganizationRemoveMemberErrorMessage('forbidden')).toBe('forbidden');
  });

  it('returns canonical organization sharing validation and operational copy', () => {
    expect(getOrganizationShareTargetOrgRequiredMessage()).toBe('Target organization is required');
    expect(getOrganizationShareResourceIdRequiredMessage()).toBe('Resource ID is required');
    expect(getOrganizationShareInvalidResourceMessage()).toBe(
      'Valid resource type and resource ID are required',
    );
    expect(getOrganizationShareTargetOrgDifferentMessage()).toBe(
      'Target organization must differ from the current organization',
    );
    expect(getOrganizationShareCreateSuccessMessage()).toBe('Resource shared successfully');
    expect(getOrganizationShareCreateErrorMessage()).toBe('Failed to create share');
    expect(getOrganizationShareCreateErrorMessage('duplicate')).toBe('duplicate');
    expect(getOrganizationShareDeleteSuccessMessage()).toBe('Share removed');
    expect(getOrganizationShareDeleteErrorMessage()).toBe('Failed to delete share');
    expect(getOrganizationShareDeleteErrorMessage('forbidden')).toBe('forbidden');
  });
});
