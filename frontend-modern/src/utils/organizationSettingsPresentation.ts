import type { OrganizationRole, OrganizationShareStatus } from '@/api/orgs';
import { formatOrgDate } from '@/utils/orgUtils';

export const ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE =
  'Organization settings are not available on this server.';
export const ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS = 'p-4 text-sm text-slate-500';

export type OrganizationSettingsLoadContext =
  'access' | 'overview' | 'sharing' | 'billing' | 'billing-admin';

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

export function getOrganizationAccessPendingInvitationsEmptyState(): string {
  return 'No pending invitations for this organization.';
}

export function getOrganizationAccessYourInvitationsEmptyState(): string {
  return 'No invitations are waiting for you.';
}

export function getOrganizationOverviewMembersEmptyState(): string {
  return 'No members found.';
}

export function getOrganizationDisplayNameRequiredMessage(): string {
  return 'Display name is required';
}

export function getOrganizationDisplayNameUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update the organization name.';
}

export function getOrganizationDisplayNameUpdatedMessage(): string {
  return 'Organization name has been updated.';
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

export function getOrganizationAccessOwnerTransferMemberRequiredMessage(): string {
  return 'Ownership can only be transferred to an existing member.';
}

export function getOrganizationAccessManageRequiredMessage(): string {
  return 'Admin or owner role required to manage organization access.';
}

export function getOrganizationAccessRoleUpdatedMessage(
  userId: string,
  role: OrganizationRole,
): string {
  return `Updated ${userId} to the ${role} role.`;
}

export function getOrganizationMemberRoleUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update the member role.';
}

export function getOrganizationAccessMemberAddedMessage(
  userId: string,
  role: OrganizationRole,
): string {
  return `Added ${userId} as ${role}.`;
}

export function getOrganizationAccessInvitationSentMessage(
  userId: string,
  role: Exclude<OrganizationRole, 'owner'>,
): string {
  return `Sent ${userId} an invitation for the ${role} role.`;
}

export function getOrganizationAccessInvitationAcceptedMessage(userId: string): string {
  return `${userId} joined the organization.`;
}

export function getOrganizationAccessInvitationDeclinedMessage(orgId: string): string {
  return `Declined the invitation for ${orgId}.`;
}

export function getOrganizationAccessInvitationRevokedMessage(userId: string): string {
  return `Revoked ${userId}'s pending invitation.`;
}

export function getOrganizationAddMemberErrorMessage(message?: string): string {
  return message || 'Unable to add the member.';
}

export function getOrganizationInvitationActionErrorMessage(message?: string): string {
  return message || 'Unable to update the invitation.';
}

export function getOrganizationMemberRemoveConfirmMessage(
  userId: string,
  organizationName: string,
): string {
  return `Remove ${userId} from ${organizationName}?`;
}

export function getOrganizationAccessMemberRemovedMessage(userId: string): string {
  return `Removed ${userId} from the organization.`;
}

export function getOrganizationRemoveMemberErrorMessage(message?: string): string {
  return message || 'Unable to remove the member.';
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
  return 'Share request sent. A target organization admin must accept it before access becomes active.';
}

export function getOrganizationShareCreateErrorMessage(message?: string): string {
  return message || 'Unable to create the share.';
}

export function getOrganizationShareStatusLabel(status: OrganizationShareStatus): string {
  return status === 'pending' ? 'Pending approval' : 'Active';
}

export function getOrganizationShareStatusDescription(
  status: OrganizationShareStatus,
  acceptedAt?: string,
  acceptedBy?: string,
): string {
  if (status === 'pending') {
    return 'Waiting for a target organization admin to accept.';
  }

  let message = acceptedAt
    ? `Accepted ${formatOrgDate(acceptedAt)}`
    : 'Accepted by the target organization';
  if (acceptedBy) {
    message += ` by ${acceptedBy}`;
  }
  return `${message}.`;
}

export function getOrganizationIncomingShareAcceptSuccessMessage(resourceLabel: string): string {
  return `Accepted shared access to ${resourceLabel}.`;
}

export function getOrganizationIncomingShareAcceptErrorMessage(message?: string): string {
  return message || 'Unable to accept the incoming share.';
}

export function getOrganizationIncomingShareDeclineConfirmMessage(
  resourceLabel: string,
  status: OrganizationShareStatus,
): string {
  if (status === 'pending') {
    return `Decline the share request for ${resourceLabel}?`;
  }
  return `Remove shared access to ${resourceLabel}?`;
}

export function getOrganizationIncomingShareDeclineSuccessMessage(
  resourceLabel: string,
  status: OrganizationShareStatus,
): string {
  if (status === 'pending') {
    return `Declined the share request for ${resourceLabel}.`;
  }
  return `Removed shared access to ${resourceLabel}.`;
}

export function getOrganizationIncomingShareDeclineErrorMessage(message?: string): string {
  return message || 'Unable to remove the incoming share.';
}

export function getOrganizationShareDeleteSuccessMessage(): string {
  return 'Share has been removed.';
}

export function getOrganizationShareDeleteErrorMessage(message?: string): string {
  return message || 'Unable to remove the share.';
}
