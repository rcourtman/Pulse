export const ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE = 'This feature is not available.';
export const ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS = 'p-4 text-sm text-slate-500';

export type OrganizationSettingsLoadContext =
  | 'access'
  | 'overview'
  | 'sharing'
  | 'billing'
  | 'billing-admin';

const ORGANIZATION_SETTINGS_LOAD_FALLBACKS: Record<OrganizationSettingsLoadContext, string> = {
  access: 'Failed to load organization access settings',
  overview: 'Failed to load organization details',
  sharing: 'Failed to load organization sharing details',
  billing: 'Failed to load billing and plan details',
  'billing-admin': 'Failed to list organizations',
};

export function getOrganizationSettingsLoadErrorMessage(
  message: string,
  context: OrganizationSettingsLoadContext,
): string {
  if (message.includes('402')) {
    return 'Multi-tenant requires an Enterprise license';
  }
  if (message.includes('501')) {
    return 'Multi-tenant is not enabled on this server';
  }
  return ORGANIZATION_SETTINGS_LOAD_FALLBACKS[context];
}

export function getOrganizationAccessEmptyState(): string {
  return 'No organization members found.';
}

export function getOrganizationOverviewMembersEmptyState(): string {
  return 'No members found.';
}

export function getOrganizationDisplayNameRequiredMessage(): string {
  return 'Display name is required';
}

export function getOrganizationDisplayNameUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update organization name';
}

export function getOrganizationOwnerRoleLockedMessage(): string {
  return 'Current owner can only remain owner. Transfer ownership instead.';
}

export function getOrganizationMemberUserIdRequiredMessage(): string {
  return 'User ID is required';
}

export function getOrganizationAddMemberErrorMessage(message?: string): string {
  return message || 'Failed to add member';
}

export function getOrganizationRemoveMemberErrorMessage(message?: string): string {
  return message || 'Failed to remove member';
}

export function getOrganizationOutgoingSharesEmptyState(): string {
  return 'No outgoing shares configured.';
}

export function getOrganizationIncomingSharesEmptyState(): string {
  return 'No incoming shares from other organizations.';
}

export function getOrganizationShareTargetOrgRequiredMessage(): string {
  return 'Target organization is required';
}

export function getOrganizationShareResourceIdRequiredMessage(): string {
  return 'Resource ID is required';
}

export function getOrganizationShareInvalidResourceMessage(): string {
  return 'Valid resource type and resource ID are required';
}

export function getOrganizationShareTargetOrgDifferentMessage(): string {
  return 'Target organization must differ from the current organization';
}

export function getOrganizationShareCreateSuccessMessage(): string {
  return 'Resource shared successfully';
}

export function getOrganizationShareCreateErrorMessage(message?: string): string {
  return message || 'Failed to create share';
}

export function getOrganizationShareDeleteSuccessMessage(): string {
  return 'Share removed';
}

export function getOrganizationShareDeleteErrorMessage(message?: string): string {
  return message || 'Failed to delete share';
}
