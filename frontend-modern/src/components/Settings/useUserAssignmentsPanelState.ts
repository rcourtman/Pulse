import { createEffect, createMemo, createSignal } from 'solid-js';
import { RBACAPI } from '@/api/rbac';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  getUserAssignmentsLoadErrorMessage,
  getUserAssignmentsUpdateErrorMessage,
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
  const [userPermissions, setUserPermissions] = createSignal<Permission[]>([]);
  const [loadingPermissions, setLoadingPermissions] = createSignal(false);
  const [formRoleIds, setFormRoleIds] = createSignal<string[]>([]);

  const featureGate = useRBACFeatureGateState({
    kind: 'user-assignments',
    loading,
    paywallLocation: 'settings_user_assignments_panel',
  });

  const loadData = async () => {
    if (!featureGate.rbacEnabled()) {
      setAssignments([]);
      setRoles([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const [usersData, rolesData] = await Promise.all([RBACAPI.getUsers(), RBACAPI.getRoles()]);
      setAssignments(usersData || []);
      setRoles(rolesData || []);
    } catch (err) {
      if (err instanceof Error && /feature not included in license/i.test(err.message)) {
        setAssignments([]);
        setRoles([]);
        return;
      }
      logger.error('Failed to load user assignments', err);
      notificationStore.error(getUserAssignmentsLoadErrorMessage());
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    if (!featureGate.licenseReady()) {
      setLoading(true);
      return;
    }
    if (!featureGate.rbacEnabled()) {
      setAssignments([]);
      setRoles([]);
      setLoading(false);
      return;
    }
    void loadData();
  });

  const filteredAssignments = createMemo(() => {
    const query = searchQuery().toLowerCase();
    if (!query) return assignments();
    return assignments().filter((assignment) => assignment.username.toLowerCase().includes(query));
  });

  const loadUserPermissions = async (username: string) => {
    if (!featureGate.rbacEnabled()) {
      setUserPermissions([]);
      return;
    }
    setLoadingPermissions(true);
    try {
      const permissions = await RBACAPI.getUserPermissions(username);
      setUserPermissions(permissions || []);
    } catch (err) {
      if (err instanceof Error && /feature not included in license/i.test(err.message)) {
        setUserPermissions([]);
        return;
      }
      logger.error('Failed to load user permissions', err);
    } finally {
      setLoadingPermissions(false);
    }
  };

  const closeModal = () => {
    setShowModal(false);
  };

  const openManageAccess = async (assignment: UserRoleAssignment) => {
    setEditingUser(assignment);
    setFormRoleIds([...assignment.roleIds]);
    setShowModal(true);
    await loadUserPermissions(assignment.username);
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
    closeModal,
    editingUser,
    featureGate,
    filteredAssignments,
    formRoleIds,
    getRoleName,
    handleSaveAssignments,
    loading,
    loadingPermissions,
    openManageAccess,
    roles,
    saving,
    searchQuery,
    setSearchQuery,
    showModal,
    toggleRole,
    userPermissions,
  };
}
