import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { RBACAPI } from '@/api/rbac';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  getUserAssignmentsDeleteErrorMessage,
  getUserAssignmentsLoadErrorMessage,
  getUserAssignmentsUpdateErrorMessage,
  getUserIdentityDisplayName,
} from '@/utils/rbacPresentation';
import { useRBACFeatureGateState } from './useRBACFeatureGateState';

export function useUserAssignmentsPanelState() {
  const [assignments, setAssignments] = createSignal<UserRoleAssignment[]>([]);
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [searchQuery, setSearchQuery] = createSignal('');
  const [showModal, setShowModal] = createSignal(false);
  const [editingUser, setEditingUser] = createSignal<UserRoleAssignment | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [userPermissions, setUserPermissions] = createSignal<Permission[] | null>(null);
  const [loadingPermissions, setLoadingPermissions] = createSignal(false);
  const [permissionsError, setPermissionsError] = createSignal<string | null>(null);
  let permissionsRequest = 0;
  const [formRoleIds, setFormRoleIds] = createSignal<string[]>([]);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [deletingUser, setDeletingUser] = createSignal(false);
  const [userPendingDeletion, setUserPendingDeletion] = createSignal<UserRoleAssignment | null>(
    null,
  );

  const featureGate = useRBACFeatureGateState({
    kind: 'user-assignments',
    loading,
    paywallLocation: 'settings_user_assignments_panel',
  });

  const loadData = async () => {
    if (!featureGate.rbacEnabled()) {
      setAssignments([]);
      setRoles([]);
      setLoadError(null);
      setLoading(false);
      return;
    }

    setLoading(true);
    setLoadError(null);
    try {
      const [usersData, rolesData] = await Promise.all([RBACAPI.getUsers(), RBACAPI.getRoles()]);
      setAssignments(usersData || []);
      setRoles(rolesData || []);
    } catch (err) {
      if (err instanceof Error && /feature not included in license/i.test(err.message)) {
        setAssignments([]);
        setRoles([]);
        setLoadError(null);
        return;
      }
      setAssignments([]);
      setRoles([]);
      setLoadError(getUserAssignmentsLoadErrorMessage());
      logger.error('Failed to load user assignments', err);
      notificationStore.error(getUserAssignmentsLoadErrorMessage());
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    if (!featureGate.licenseReady()) {
      closeModal();
      setLoading(true);
      return;
    }
    if (!featureGate.rbacEnabled()) {
      closeModal();
      setAssignments([]);
      setRoles([]);
      setLoadError(null);
      setLoading(false);
      return;
    }
    void loadData();
  });

  const filteredAssignments = createMemo(() => {
    const query = searchQuery().toLowerCase();
    if (!query) return assignments();
    return assignments().filter((assignment) =>
      [
        assignment.username,
        assignment.displayName,
        assignment.email,
        assignment.providerType,
        assignment.providerId,
      ].some((value) => value?.toLowerCase().includes(query)),
    );
  });

  const loadUserPermissions = async () => {
    const username = editingUser()?.username;
    if (!username || !showModal() || !featureGate.rbacEnabled() || loadingPermissions()) return;

    const request = ++permissionsRequest;
    const isCurrentRead = () =>
      request === permissionsRequest &&
      showModal() &&
      editingUser()?.username === username &&
      featureGate.rbacEnabled();
    setUserPermissions(null);
    setPermissionsError(null);
    setLoadingPermissions(true);
    try {
      const permissions = await RBACAPI.getUserPermissions(username);
      if (isCurrentRead()) setUserPermissions(permissions);
    } catch (err) {
      if (!isCurrentRead()) return;
      setPermissionsError('Saved permissions could not be loaded. Access is unknown.');
      logger.error('Failed to load user permissions', err);
    } finally {
      if (isCurrentRead()) setLoadingPermissions(false);
    }
  };

  const closeModal = () => {
    permissionsRequest += 1;
    setUserPermissions(null);
    setPermissionsError(null);
    setLoadingPermissions(false);
    setShowModal(false);
  };

  onCleanup(() => {
    permissionsRequest += 1;
  });

  const openManageAccess = async (assignment: UserRoleAssignment) => {
    closeModal();
    setEditingUser(assignment);
    setFormRoleIds([...assignment.roleIds]);
    setShowModal(true);
    await loadUserPermissions();
  };

  const openDeleteUser = (assignment: UserRoleAssignment) => {
    setUserPendingDeletion(assignment);
  };

  const closeDeleteUser = () => {
    if (!deletingUser()) setUserPendingDeletion(null);
  };

  const handleDeleteUser = async () => {
    const user = userPendingDeletion();
    if (!user) return;

    setDeletingUser(true);
    try {
      await RBACAPI.deleteUser(user.username);
      notificationStore.success(`User access removed for ${getUserIdentityDisplayName(user)}`);
      if (editingUser()?.username === user.username) closeModal();
      setUserPendingDeletion(null);
      await loadData();
    } catch (err) {
      logger.error('Failed to remove user access', err);
      notificationStore.error(getUserAssignmentsDeleteErrorMessage(err));
    } finally {
      setDeletingUser(false);
    }
  };

  const handleSaveAssignments = async () => {
    const user = editingUser();
    if (!user) return;

    setSaving(true);
    try {
      await RBACAPI.updateUserRoles(user.username, formRoleIds());
      notificationStore.success(`Roles updated for ${user.username}`);
      closeModal();
      await loadData();
    } catch (err) {
      logger.error('Failed to update user roles', err);
      notificationStore.error(getUserAssignmentsUpdateErrorMessage());
    } finally {
      setSaving(false);
    }
  };

  const toggleRole = (roleId: string) => {
    const currentRoleIds = formRoleIds();
    if (currentRoleIds.includes(roleId)) {
      setFormRoleIds(currentRoleIds.filter((id) => id !== roleId));
      return;
    }
    setFormRoleIds([...currentRoleIds, roleId]);
  };

  const getRoleName = (roleId: string) =>
    roles().find((role) => role.id === roleId)?.name || roleId;

  return {
    closeDeleteUser,
    closeModal,
    deletingUser,
    editingUser,
    featureGate,
    filteredAssignments,
    formRoleIds,
    getRoleName,
    handleSaveAssignments,
    handleDeleteUser,
    loading,
    loadData,
    loadError,
    loadingPermissions,
    loadUserPermissions,
    openManageAccess,
    openDeleteUser,
    permissionsError,
    roles,
    saving,
    searchQuery,
    setSearchQuery,
    showModal,
    toggleRole,
    userPendingDeletion,
    userPermissions,
  };
}
