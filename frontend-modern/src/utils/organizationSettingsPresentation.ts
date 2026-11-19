import type { OrganizationRole, OrganizationShare, OrganizationShareStatus } from '@/api/orgs';
import { formatOrgDate } from '@/utils/orgUtils';

export const ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE =
  'Organization settings are not available on this server.';
export const ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS = 'p-4 text-sm text-slate-500';
export const ORGANIZATION_ACCESS_UNCONFIRMED_CHANGE_MESSAGE =
  'The last access change could not be confirmed. Reload saved access before making another change.';
export const ORGANIZATION_ACCESS_RELOAD_LABEL = 'Reload access';
export const ORGANIZATION_ACCESS_UNAVAILABLE_TITLE = 'Organization access unavailable';
export function getOrganizationAccessReloadMessage(orgId: string): string {
  return `Reload checks saved members and invitations for ${orgId}. It does not repeat an access change.`;
}

// Names may repeat or change; action names keep the saved party's ID.
export function getShareOrganizationLabel(id: string, name?: string): string {
  return name && name !== id ? `${name} (${id})` : id;
}

export function getOrganizationInvitationPresentation(orgId: string, displayName?: string) {
  const organization = displayName && displayName !== orgId ? `${displayName} (${orgId})` : orgId;
  return {
    organization,
    groupLabel: `Invitation to ${organization}`,
    acceptLabel: `Accept invitation to ${organization}`,
    declineLabel: `Decline invitation to ${organization}`,
  };
}

export const ORGANIZATION_SHARING_UNCONFIRMED_CHANGE_MESSAGE =
  'The last sharing change could not be confirmed. Reload saved sharing before making another change.';
export const ORGANIZATION_SHARING_RELOAD_LABEL = 'Reload sharing';
export const ORGANIZATION_SHARING_UNAVAILABLE_TITLE = 'Organization sharing unavailable';
export const ORGANIZATION_SHARING_READ_ONLY_MESSAGE =
  'This session is read-only. Sharing changes are disabled.';
export const ORGANIZATION_SHARING_MANAGER_REQUIRED_MESSAGE =
  'Admin or owner role required to create or remove organization shares.';
export const ORGANIZATION_SHARING_QUICK_PICK_UNAVAILABLE_MESSAGE =
  'The selected resource is no longer available. Choose another resource or enter it manually.';
export const ORGANIZATION_SHARING_QUICK_PICK_GUIDANCE =
  'Choose a discovered resource, or switch to manual entry.';
export const ORGANIZATION_SHARING_MANUAL_ENTRY_HIDDEN_MESSAGE =
  'Manual entry is hidden while quick pick is active.';

type ShareResourceKey = Pick<OrganizationShare, 'resourceType' | 'resourceId'>;

export function getOrganizationSharingPartialReadMessage(
  canManage: boolean,
  outgoingUnavailable?: boolean,
): string {
  if (outgoingUnavailable) {
    return 'Sharing changes are paused. Reload checks saved shares without repeating a change.';
  }
  return canManage
    ? 'Sharing changes are paused until the missing information loads.'
    : 'Some sharing information could not be loaded.';
}

export function getOrganizationSharingTargetPlaceholder(directoryUnavailable?: boolean): string {
  return directoryUnavailable ? 'Organization list unavailable' : 'Select organization';
}

export function getOrganizationShareActionLabel(
  action: 'Accept' | 'Decline' | 'Remove',
  share: ShareResourceKey,
  direction: 'from' | 'to',
  orgID: string,
  orgName?: string,
): string {
  return `${action} share of ${share.resourceType}:${share.resourceId} ${direction} ${getShareOrganizationLabel(orgID, orgName)}`;
}

export function getOrganizationOutgoingShareRemoveConfirmMessage(share: ShareResourceKey): string {
  return `Remove share for ${share.resourceType}:${share.resourceId}?`;
}

export function getOrganizationShareResourceLabel(
  share: ShareResourceKey & Pick<OrganizationShare, 'resourceName'>,
): string {
  const resourceName = share.resourceName?.trim();
  if (resourceName) return resourceName;
  if (share.resourceId.trim() !== '') return share.resourceId;
  return `${share.resourceType}:${share.resourceId}`;
}

export function getOrganizationShareResourceOptionLabel(resource: {
  type: string;
  id: string;
  name: string;
}): string {
  const identity = `${resource.type}:${resource.id}`;
  return resource.name && resource.name !== resource.id
    ? `${resource.name} (${identity})`
    : identity;
}
export function getOrganizationSharingReloadMessage(orgId: string): string {
  return `Reload checks saved incoming and outgoing shares for ${orgId}. It does not repeat a sharing change.`;
}

export const ORGANIZATION_OVERVIEW_UNCONFIRMED_CHANGE_MESSAGE =
  'The last name change could not be confirmed. Reload saved organization details before making another change.';

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

export function getOrganizationOverviewManageRequiredMessage(readOnly = false): string {
  if (readOnly) return 'Organization details are read-only for this session.';
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

export function getOrganizationAccessManageRequiredMessage(readOnly = false): string {
  if (readOnly) return 'Organization access is read-only for this session.';
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

export function getOrganizationOutgoingSharesEmptyState(unavailable?: boolean): string {
  return unavailable ? 'Outgoing shares could not be loaded.' : 'No outgoing shares configured.';
}

export function getOrganizationIncomingSharesEmptyState(unavailable?: boolean): string {
  return unavailable
    ? 'Incoming shares could not be loaded.'
    : 'No incoming shares from other organizations.';
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
