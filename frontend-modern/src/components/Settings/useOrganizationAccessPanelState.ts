import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import {
  OrgsAPI,
  type Organization,
  type OrganizationAccessRequestOptions,
  type OrganizationInvitation,
  type OrganizationMember,
  type OrganizationRole,
  type UserOrganizationInvitation,
} from '@/api/orgs';
import { eventBus } from '@/stores/events';
import { isMultiTenantEnabled } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import {
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
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
  ORGANIZATION_ACCESS_UNCONFIRMED_CHANGE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { canManageOrg, normalizeRole } from '@/utils/orgUtils';

interface OrganizationAccessPanelProps {
  currentUser?: string;
}

export function useOrganizationAccessPanelState(props: OrganizationAccessPanelProps) {
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [pendingInvitations, setPendingInvitations] = createSignal<OrganizationInvitation[]>([]);
  const [myInvitations, setMyInvitations] = createSignal<UserOrganizationInvitation[]>([]);
  const [inviteUserID, setInviteUserID] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal<OrganizationRole>('viewer');

  const activeOrgId = () => normalizeOrgScope(getOrgID());
  const [selectedOrgID, setSelectedOrgID] = createSignal(activeOrgId());
  const [selectedUser, setSelectedUser] = createSignal(props.currentUser);
  let generation = 0;
  let disposed = false;
  let readController: AbortController | undefined;
  let mutationController: AbortController | undefined;
  const surfaceEnabled = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const selectionIsCurrent = () =>
    !disposed &&
    surfaceEnabled() &&
    selectedOrgID() === activeOrgId() &&
    selectedUser() === props.currentUser;
  const hasCurrentAccess = () =>
    selectionIsCurrent() && !loading() && !loadError() && org()?.id === activeOrgId();
  const canManageCurrentOrg = createMemo(
    () =>
      hasCurrentAccess() &&
      !presentationPolicyIsReadOnly() &&
      canManageOrg(org(), props.currentUser),
  );
  const canChangeAccess = () =>
    hasCurrentAccess() && !!props.currentUser && !presentationPolicyIsReadOnly() && !saving();

  // PulseDataGrid wraps the accepted row in a Solid store proxy. Compare its
  // underlying object, not the proxy, without admitting an older read's row.
  const isCurrentMember = (member: OrganizationMember) => members().includes(unwrap(member));

  const clearAccess = () => {
    setOrg(null);
    setMembers([]);
    setPendingInvitations([]);
    setMyInvitations([]);
  };

  const invalidateRequests = () => {
    generation += 1;
    readController?.abort();
    readController = undefined;
    mutationController?.abort();
    mutationController = undefined;
    setSaving(false);
  };

  const loadOrganizationAccess = async () => {
    if (disposed) return;
    invalidateRequests();
    const orgId = activeOrgId();
    const user = props.currentUser;
    const request = generation;
    const isCurrent = () => request === generation && selectionIsCurrent();
    batch(() => {
      if (selectedOrgID() !== orgId || selectedUser() !== user) {
        setInviteUserID('');
        setInviteRole('viewer');
      }
      setSelectedOrgID(orgId);
      setSelectedUser(user);
      clearAccess();
      setLoadError(null);
      setLoading(surfaceEnabled());
    });
    if (!surfaceEnabled()) return;

    const controller = new AbortController();
    readController = controller;
    const options: OrganizationAccessRequestOptions = {
      expectedOrgID: orgId,
      signal: controller.signal,
      retry: false,
    };
    try {
      const orgData = await OrgsAPI.get(orgId, options);
      if (!isCurrent()) return;
      if (!orgData || orgData.id !== orgId) {
        throw new Error('Organization access returned a different selection');
      }
      const manageable = canManageOrg(orgData, user);
      const [memberData, invitationData, myInvitationData] = await Promise.all([
        OrgsAPI.listMembers(orgId, options),
        manageable ? OrgsAPI.listPendingInvitations(orgId, options) : Promise.resolve([]),
        OrgsAPI.listMyInvitations(options),
      ]);
      if (!isCurrent()) return;
      if (
        !Array.isArray(memberData) ||
        !Array.isArray(invitationData) ||
        !Array.isArray(myInvitationData)
      ) {
        throw new Error('Organization access returned invalid lists');
      }
      batch(() => {
        setOrg(orgData);
        setMembers(memberData);
        setPendingInvitations(invitationData);
        setMyInvitations(myInvitationData);
      });
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to load organization access data', error);
      const message = error instanceof Error ? error.message : '';
      setLoadError(getOrganizationSettingsLoadErrorMessage(message, 'access'));
    } finally {
      if (!disposed && request === generation) {
        // Also cancel sibling reads when only one Promise.all member failed.
        controller.abort();
        readController = undefined;
        setLoading(false);
        if (!selectionIsCurrent()) void loadOrganizationAccess();
      }
    }
  };

  const changeAccess = async <T>(
    change: (options: OrganizationAccessRequestOptions) => Promise<T>,
    successMessage: (result: T) => string,
    errorMessage: (message?: string) => string,
    afterSuccess?: () => void,
  ) => {
    if (!canChangeAccess()) return;
    const request = generation;
    const controller = new AbortController();
    mutationController = controller;
    setSaving(true);
    const isCurrent = () => request === generation && selectionIsCurrent();
    try {
      const result = await change({
        expectedOrgID: selectedOrgID(),
        signal: controller.signal,
        retry: false,
      });
      if (!isCurrent()) return;
      notificationStore.success(successMessage(result));
      afterSuccess?.();
      mutationController = undefined;
      setSaving(false);
      // Refresh the accepted snapshot for every mutation, and notify the other
      // organization surfaces. Our existing subscription owns the single read.
      eventBus.emit('organizations_changed');
    } catch (error) {
      if (!isCurrent()) return;
      logger.error('Failed to change organization access', error);
      notificationStore.error(errorMessage(error instanceof Error ? error.message : undefined));
      // A failed response does not establish whether the server applied the
      // change. Require a read, never automatically repeat a write.
      batch(() => {
        clearAccess();
        setLoadError(ORGANIZATION_ACCESS_UNCONFIRMED_CHANGE_MESSAGE);
      });
    } finally {
      if (mutationController === controller) {
        mutationController = undefined;
        setSaving(false);
        if (!disposed && !selectionIsCurrent()) void loadOrganizationAccess();
      }
    }
  };

  const updateRole = async (member: OrganizationMember, role: OrganizationRole) => {
    if (!canChangeAccess() || !canManageCurrentOrg() || !isCurrentMember(member)) return;
    const currentOrg = org()!;
    if (normalizeRole(member.role) === role) return;
    if (
      (member.userId === currentOrg.ownerUserId && role !== 'owner') ||
      (role === 'owner' && props.currentUser !== currentOrg.ownerUserId)
    ) {
      notificationStore.error(getOrganizationOwnerRoleLockedMessage());
      return;
    }
    await changeAccess(
      (options) =>
        OrgsAPI.updateMemberRole(currentOrg.id, { userId: member.userId, role }, options),
      () => getOrganizationAccessRoleUpdatedMessage(member.userId, role),
      getOrganizationMemberRoleUpdateErrorMessage,
    );
  };

  const inviteMember = async () => {
    if (!canChangeAccess() || !canManageCurrentOrg()) return;
    const currentOrg = org()!;
    const draft = inviteUserID();
    const userId = draft.trim();
    const role = inviteRole();
    if (!userId) {
      notificationStore.error(getOrganizationMemberUserIdRequiredMessage());
      return;
    }
    if (role === 'owner') {
      if (props.currentUser !== currentOrg.ownerUserId) return;
      if (!members().some((member) => member.userId === userId)) {
        notificationStore.error(getOrganizationAccessOwnerTransferMemberRequiredMessage());
        return;
      }
    }
    await changeAccess(
      (options) => OrgsAPI.inviteMember(currentOrg.id, { userId, role }, options),
      (result) =>
        result.kind === 'invitation'
          ? getOrganizationAccessInvitationSentMessage(userId, result.invitation.role)
          : getOrganizationAccessMemberAddedMessage(userId, role),
      getOrganizationAddMemberErrorMessage,
      () => {
        if (inviteUserID() === draft && inviteRole() === role) {
          setInviteUserID('');
          setInviteRole('viewer');
        }
      },
    );
  };

  const acceptInvitation = async (orgId: string) => {
    if (!canChangeAccess() || !myInvitations().some((invitation) => invitation.orgId === orgId))
      return;
    await changeAccess(
      (options) => OrgsAPI.acceptMyInvitation(orgId, options),
      () => getOrganizationAccessInvitationAcceptedMessage(props.currentUser!),
      getOrganizationInvitationActionErrorMessage,
    );
  };

  const declineInvitation = async (orgId: string) => {
    if (!canChangeAccess() || !myInvitations().some((invitation) => invitation.orgId === orgId))
      return;
    await changeAccess(
      (options) => OrgsAPI.declineMyInvitation(orgId, options),
      () => getOrganizationAccessInvitationDeclinedMessage(orgId),
      getOrganizationInvitationActionErrorMessage,
    );
  };

  const revokeInvitation = async (userId: string) => {
    if (
      !canChangeAccess() ||
      !canManageCurrentOrg() ||
      !pendingInvitations().some((invitation) => invitation.userId === userId)
    )
      return;
    const orgId = org()!.id;
    await changeAccess(
      (options) => OrgsAPI.revokeInvitation(orgId, userId, options),
      () => getOrganizationAccessInvitationRevokedMessage(userId),
      getOrganizationInvitationActionErrorMessage,
    );
  };

  const removeMember = async (member: OrganizationMember) => {
    if (!canChangeAccess() || !canManageCurrentOrg() || !isCurrentMember(member)) return;
    const currentOrg = org()!;
    if (member.userId === currentOrg.ownerUserId) return;
    if (
      !confirm(
        getOrganizationMemberRemoveConfirmMessage(
          member.userId,
          currentOrg.displayName || currentOrg.id,
        ),
      )
    )
      return;
    // A confirmation can outlive the snapshot that supplied its member.
    if (org() !== currentOrg || !isCurrentMember(member) || !canManageCurrentOrg()) return;
    await changeAccess(
      (options) => OrgsAPI.removeMember(currentOrg.id, member.userId, options),
      () => getOrganizationAccessMemberRemovedMessage(member.userId),
      getOrganizationRemoveMemberErrorMessage,
    );
  };

  const unsubscribeOrg = eventBus.on('org_switched', () => void loadOrganizationAccess());
  const unsubscribeChanged = eventBus.on(
    'organizations_changed',
    () => void loadOrganizationAccess(),
  );
  createEffect(
    on([surfaceEnabled, () => props.currentUser, presentationPolicyIsReadOnly], () => {
      void loadOrganizationAccess();
    }),
  );
  onCleanup(() => {
    disposed = true;
    invalidateRequests();
    unsubscribeOrg();
    unsubscribeChanged();
  });

  return {
    acceptInvitation,
    canManageCurrentOrg,
    canChangeAccess,
    declineInvitation,
    inviteMember,
    inviteRole,
    inviteUserID,
    loading,
    loadError,
    loadOrganizationAccess,
    members,
    myInvitations,
    org,
    pendingInvitations,
    readOnly: presentationPolicyIsReadOnly,
    removeMember,
    revokeInvitation,
    saving,
    selectedOrgID,
    setInviteRole,
    setInviteUserID,
    updateRole,
    pendingInvitationsEmptyState: getOrganizationAccessPendingInvitationsEmptyState,
    yourInvitationsEmptyState: getOrganizationAccessYourInvitationsEmptyState,
  };
}
