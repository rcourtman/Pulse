import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import {
  OrgsAPI,
  type Organization,
  type OrganizationMember,
  type OrganizationRole,
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
  getOrganizationAccessMemberAddedMessage,
  getOrganizationAccessMemberRemovedMessage,
  getOrganizationAccessRoleUpdatedMessage,
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
  const [inviteUserID, setInviteUserID] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal<OrganizationRole>('viewer');

  const activeOrgId = () => normalizeOrgScope(getOrgID());
  const canManageCurrentOrg = createMemo(() => canManageOrg(org(), props.currentUser));

  const loadOrganizationAccess = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();
      const [orgData, memberData] = await Promise.all([
        OrgsAPI.get(orgId),
        OrgsAPI.listMembers(orgId),
      ]);
      setOrg(orgData);
      setMembers(memberData);
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
      await OrgsAPI.inviteMember(currentOrg.id, { userId, role });
      notificationStore.success(getOrganizationAccessMemberAddedMessage(userId, role));
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
        getOrganizationRemoveMemberErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
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
    onCleanup(unsubscribe);
  });

  return {
    canManageCurrentOrg,
    inviteMember,
    inviteRole,
    inviteUserID,
    loading,
    members,
    org,
    removeMember,
    saving,
    setInviteRole,
    setInviteUserID,
    updateRole,
  };
}
