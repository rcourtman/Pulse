import type { OrganizationRole } from '@/api/orgs';

export const ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE =
  'Organization settings are not available on this server.';
export const ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS = 'p-4 text-sm text-slate-500';

export type OrganizationSettingsLoadContext =
  | 'access'
  | 'overview'
  | 'sharing'
  | 'billing'
  | 'billing-admin';

const ORGANIZATION_SETTINGS_LOAD_FALLBACKS: Record<OrganizationSettingsLoadContext, string> = {
  access: 'Unable to load organization access settings.',
  overview: 'Unable to load organization details.',
  sharing: 'Unable to load organization sharing details.',
  billing: 'Unable to load billing and plan details.',
  'billing-admin': 'Unable to load the organization list.',
};

export function getOrganizationSettingsLoadErrorMessage(
  message: string,
  context: OrganizationSettingsLoadContext,
): string {
  if (message.includes('402')) {
    return 'Organization settings require an Enterprise license.';
  }
  if (message.includes('501')) {
    return 'Organization settings are not enabled on this server.';
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

export function getOrganizationDisplayNameUpdatedMessage(): string {
  return 'Organization name updated';
}

export function getOrganizationOverviewManageRequiredMessage(): string {
  return 'Admin or owner role required to update organization details.';
}

export function getOrganizationOwnerRoleLockedMessage(): string {
  return 'Current owner can only remain owner. Transfer ownership instead.';
}

export function getOrganizationMemberUserIdRequiredMessage(): string {
  return 'User ID is required';
}

export function getOrganizationAccessManageRequiredMessage(): string {
  return 'Admin or owner role required to manage organization access.';
}

export function getOrganizationAccessRoleUpdatedMessage(
  userId: string,
  role: OrganizationRole,
): string {
  return `Updated ${userId} to ${role}`;
}

export function getOrganizationMemberRoleUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update member role';
}

export function getOrganizationAccessMemberAddedMessage(
  userId: string,
  role: OrganizationRole,
): string {
  return `Added ${userId} as ${role}`;
}

export function getOrganizationAddMemberErrorMessage(message?: string): string {
  return message || 'Failed to add member';
}

export function getOrganizationMemberRemoveConfirmMessage(
  userId: string,
  organizationName: string,
): string {
  return `Remove ${userId} from ${organizationName}?`;
}

export function getOrganizationAccessMemberRemovedMessage(userId: string): string {
  return `Removed ${userId}`;
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
