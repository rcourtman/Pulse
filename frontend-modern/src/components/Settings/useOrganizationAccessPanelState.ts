import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import {
  OrgsAPI,
  type Organization,
  type OrganizationInvitation,
  type OrganizationMember,
  type OrganizationRole,
  type UserOrganizationInvitation,
} from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import { getOrgID } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
  getOrganizationAddMemberErrorMessage,
  getOrganizationAccessInvitationAcceptedMessage,
  getOrganizationAccessInvitationDeclinedMessage,
  getOrganizationAccessInvitationRevokedMessage,
  getOrganizationAccessInvitationSentMessage,
  getOrganizationAccessMemberAddedMessage,
  getOrganizationAccessMemberRemovedMessage,
  getOrganizationAccessOwnerTransferMemberRequiredMessage,
  getOrganizationAccessPendingInvitationsEmptyState,
  getOrganizationAccessRoleUpdatedMessage,
  getOrganizationAccessYourInvitationsEmptyState,
  getOrganizationInvitationActionErrorMessage,
  getOrganizationMemberRoleUpdateErrorMessage,
  getOrganizationMemberRemoveConfirmMessage,
  getOrganizationMemberUserIdRequiredMessage,
  getOrganizationOwnerRoleLockedMessage,
  getOrganizationRemoveMemberErrorMessage,
  getOrganizationSettingsLoadErrorMessage,
} from '@/utils/organizationSettingsPresentation';
import { canManageOrg, normalizeRole } from '@/utils/orgUtils';

interface OrganizationAccessPanelProps {
  currentUser?: string;
}

export function useOrganizationAccessPanelState(props: OrganizationAccessPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [pendingInvitations, setPendingInvitations] = createSignal<OrganizationInvitation[]>([]);
  const [myInvitations, setMyInvitations] = createSignal<UserOrganizationInvitation[]>([]);
  const [inviteUserID, setInviteUserID] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal<OrganizationRole>('viewer');

  const activeOrgId = () => normalizeOrgScope(getOrgID());
  const canManageCurrentOrg = createMemo(() => canManageOrg(org(), props.currentUser));

  const loadOrganizationAccess = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();
      const orgData = await OrgsAPI.get(orgId);
      const manageable = canManageOrg(orgData, props.currentUser);
      const [memberData, invitationData, myInvitationData] = await Promise.all([
        OrgsAPI.listMembers(orgId),
        manageable ? OrgsAPI.listPendingInvitations(orgId) : Promise.resolve([]),
        OrgsAPI.listMyInvitations(),
      ]);
      setOrg(orgData);
      setMembers(memberData);
      setPendingInvitations(invitationData);
      setMyInvitations(myInvitationData);
    } catch (error) {
      logger.error('Failed to load organization access data', error);
      const message = error instanceof Error ? error.message : '';
      notificationStore.error(getOrganizationSettingsLoadErrorMessage(message, 'access'));
    } finally {
      setLoading(false);
    }
  };

  const updateRole = async (member: OrganizationMember, role: OrganizationRole) => {
    const currentOrg = org();
    if (!currentOrg) return;
    const currentRole = normalizeRole(member.role);
    if (currentRole === role) return;

    if (member.userId === currentOrg.ownerUserId && role !== 'owner') {
      notificationStore.error(getOrganizationOwnerRoleLockedMessage());
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.updateMemberRole(currentOrg.id, { userId: member.userId, role });
      notificationStore.success(getOrganizationAccessRoleUpdatedMessage(member.userId, role));
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to update member role', error);
      notificationStore.error(
        getOrganizationMemberRoleUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const inviteMember = async () => {
    const currentOrg = org();
    if (!currentOrg) return;

    const userId = inviteUserID().trim();
    if (!userId) {
      notificationStore.error(getOrganizationMemberUserIdRequiredMessage());
      return;
    }

    setSaving(true);
    try {
      const role = inviteRole();
      if (role === 'owner' && !members().some((member) => member.userId === userId)) {
        notificationStore.error(getOrganizationAccessOwnerTransferMemberRequiredMessage());
        return;
      }

      const result = await OrgsAPI.inviteMember(currentOrg.id, { userId, role });
      if (result.kind === 'invitation') {
        notificationStore.success(
          getOrganizationAccessInvitationSentMessage(userId, result.invitation.role),
        );
      } else {
        notificationStore.success(getOrganizationAccessMemberAddedMessage(userId, role));
      }
      setInviteUserID('');
      setInviteRole('viewer');
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to add organization member', error);
      notificationStore.error(
        getOrganizationAddMemberErrorMessage(error instanceof Error ? error.message : undefined),
      );
    } finally {
      setSaving(false);
    }
  };

  const acceptInvitation = async (orgId: string) => {
    setSaving(true);
    try {
      await OrgsAPI.acceptMyInvitation(orgId);
      notificationStore.success(
        getOrganizationAccessInvitationAcceptedMessage(props.currentUser || 'user'),
      );
      eventBus.emit('organizations_changed');
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to accept organization invitation', error);
      notificationStore.error(
        getOrganizationInvitationActionErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const declineInvitation = async (orgId: string) => {
    setSaving(true);
    try {
      await OrgsAPI.declineMyInvitation(orgId);
      notificationStore.success(getOrganizationAccessInvitationDeclinedMessage(orgId));
      eventBus.emit('organizations_changed');
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to decline organization invitation', error);
      notificationStore.error(
        getOrganizationInvitationActionErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const revokeInvitation = async (userId: string) => {
    const currentOrg = org();
    if (!currentOrg) return;

    setSaving(true);
    try {
      await OrgsAPI.revokeInvitation(currentOrg.id, userId);
      notificationStore.success(getOrganizationAccessInvitationRevokedMessage(userId));
      eventBus.emit('organizations_changed');
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to revoke organization invitation', error);
      notificationStore.error(
        getOrganizationInvitationActionErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const removeMember = async (member: OrganizationMember) => {
    const currentOrg = org();
    if (!currentOrg) return;
    const organizationName = currentOrg.displayName || currentOrg.id;
    if (!confirm(getOrganizationMemberRemoveConfirmMessage(member.userId, organizationName))) {
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.removeMember(currentOrg.id, member.userId);
      notificationStore.success(getOrganizationAccessMemberRemovedMessage(member.userId));
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to remove organization member', error);
      notificationStore.error(
        getOrganizationRemoveMemberErrorMessage(error instanceof Error ? error.message : undefined),
      );
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled() || presentationPolicyHidesOrganizationSurfaces()) {
      setLoading(false);
      return;
    }
    void loadOrganizationAccess();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadOrganizationAccess();
    });
    const unsubscribeOrganizationsChanged = eventBus.on('organizations_changed', () => {
      void loadOrganizationAccess();
    });
    onCleanup(() => {
      unsubscribe();
      unsubscribeOrganizationsChanged();
    });
  });

  return {
    acceptInvitation,
    canManageCurrentOrg,
    declineInvitation,
    inviteMember,
    inviteRole,
    inviteUserID,
    loading,
    members,
    myInvitations,
    org,
    pendingInvitations,
    removeMember,
    revokeInvitation,
    saving,
    setInviteRole,
    setInviteUserID,
    updateRole,
    pendingInvitationsEmptyState: getOrganizationAccessPendingInvitationsEmptyState,
    yourInvitationsEmptyState: getOrganizationAccessYourInvitationsEmptyState,
  };
}
