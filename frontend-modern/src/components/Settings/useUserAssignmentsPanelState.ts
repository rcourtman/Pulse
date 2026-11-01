import { batch, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { RBACAPI } from '@/api/rbac';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';
import {
  getUserAssignmentsDeleteErrorMessage,
  getUserAssignmentsLoadErrorMessage,
  getUserAssignmentsUpdateErrorMessage,
  getUserIdentityDisplayName,
} from '@/utils/rbacPresentation';
import { useRBACFeatureGateState } from './useRBACFeatureGateState';

export interface UserAssignmentsPanelProps {
  currentUser?: string;
  canManage?: boolean;
}

export function useUserAssignmentsPanelState(props: UserAssignmentsPanelProps = {}) {
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
  const [formRoleIds, setFormRoleIds] = createSignal<string[]>([]);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [successNotice, setSuccessNotice] = createSignal<string | null>(null);
  const [deletingUser, setDeletingUser] = createSignal(false);
  const [userPendingDeletion, setUserPendingDeletion] = createSignal<UserRoleAssignment | null>(
    null,
  );
  const [inventoryLoaded, setInventoryLoaded] = createSignal(false);

  const featureGate = useRBACFeatureGateState({
    kind: 'user-assignments',
    loading,
    paywallLocation: 'settings_user_assignments_panel',
  });
  const accessAllowed = () => featureGate.rbacEnabled() && props.canManage !== false;
  const currentOrg = () => getOrgID() || 'default';
  let context = { org: currentOrg(), user: props.currentUser, allowed: accessAllowed() };
  let disposed = false;
  let contextVersion = 0;
  let editorVersion = 0;
  let readController: AbortController | undefined;
  let permissionsController: AbortController | undefined;
  let mutationController: AbortController | undefined;
  const contextIsCurrent = () =>
    !disposed &&
    context.org === currentOrg() &&
    context.user === props.currentUser &&
    context.allowed === accessAllowed();
  const canEdit = () =>
    contextIsCurrent() &&
    accessAllowed() &&
    inventoryLoaded() &&
    !loading() &&
    !loadError() &&
    !saving() &&
    !deletingUser();
  const isCurrentUser = (user: UserRoleAssignment) =>
    assignments().some((current) => unwrap(current) === unwrap(user));
  const requestOptions = (controller: AbortController) => ({
    signal: controller.signal,
    expectedOrgID: context.org,
    retry: false,
  });

  const clearEditor = () => {
    editorVersion += 1;
    permissionsController?.abort();
    permissionsController = undefined;
    batch(() => {
      setUserPermissions(null);
      setPermissionsError(null);
      setLoadingPermissions(false);
      setShowModal(false);
      setEditingUser(null);
      setFormRoleIds([]);
      setActionError(null);
    });
  };
  const closeModal = () => {
    if (!saving()) clearEditor();
  };

  const loadData = async () => {
    if (!contextIsCurrent() || !accessAllowed() || readController || mutationController) return;
    const version = contextVersion;
    const controller = new AbortController();
    const isCurrentRead = () =>
      contextIsCurrent() && version === contextVersion && readController === controller;
    readController = controller;
    batch(() => {
      clearEditor();
      setUserPendingDeletion(null);
      setAssignments([]);
      setRoles([]);
      setInventoryLoaded(false);
      setLoading(true);
      setLoadError(null);
    });
    try {
      const [usersData, rolesData] = await Promise.all([
        RBACAPI.getUsers(requestOptions(controller)),
        RBACAPI.getRoles(requestOptions(controller)),
      ]);
      if (!isCurrentRead()) return;
      const users = new Set<string>();
      const roleIds = new Set<string>();
      // A failed or malformed read is unknown, not an empty account directory.
      if (
        !Array.isArray(usersData) ||
        !Array.isArray(rolesData) ||
        usersData.some((user) => {
          if (
            !user ||
            typeof user.username !== 'string' ||
            !user.username.trim() ||
            users.has(user.username) ||
            !Array.isArray(user.roleIds) ||
            user.roleIds.some((id) => typeof id !== 'string' || !id.trim())
          )
            return true;
          users.add(user.username);
          return false;
        }) ||
        rolesData.some((role) => {
          if (
            !role ||
            typeof role.id !== 'string' ||
            !role.id.trim() ||
            roleIds.has(role.id) ||
            typeof role.name !== 'string' ||
            typeof role.description !== 'string'
          )
            return true;
          roleIds.add(role.id);
          return false;
        })
      )
        throw new Error('Invalid user access inventory');
      batch(() => {
        setAssignments(usersData);
        setRoles(rolesData);
        setInventoryLoaded(true);
      });
    } catch {
      if (!isCurrentRead()) return;
      setLoadError(getUserAssignmentsLoadErrorMessage());
    } finally {
      if (isCurrentRead()) {
        controller.abort();
        readController = undefined;
        setLoading(false);
      }
    }
  };

  const resetContext = () => {
    contextVersion += 1;
    readController?.abort();
    mutationController?.abort();
    readController = mutationController = undefined;
    context = { org: currentOrg(), user: props.currentUser, allowed: accessAllowed() };
    batch(() => {
      clearEditor();
      setAssignments([]);
      setRoles([]);
      setInventoryLoaded(false);
      setSearchQuery('');
      setSuccessNotice(null);
      setLoadError(null);
      setUserPendingDeletion(null);
      setSaving(false);
      setDeletingUser(false);
      setLoading(!featureGate.licenseReady() || accessAllowed());
    });
    if (!disposed) void loadData();
  };
  createEffect(
    on([featureGate.licenseReady, accessAllowed, () => props.currentUser], resetContext),
  );
  const unsubscribeOrg = eventBus.on('org_switched', resetContext);
  onCleanup(() => {
    disposed = true;
    unsubscribeOrg();
    resetContext();
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
    const user = editingUser();
    if (!user || !showModal() || !canEdit() || !isCurrentUser(user) || permissionsController)
      return;
    const version = contextVersion;
    const editor = editorVersion;
    const controller = new AbortController();
    permissionsController = controller;
    const isCurrentRead = () =>
      contextIsCurrent() &&
      version === contextVersion &&
      editor === editorVersion &&
      permissionsController === controller &&
      showModal() &&
      editingUser() === user;
    batch(() => {
      setUserPermissions(null);
      setPermissionsError(null);
      setLoadingPermissions(true);
    });
    try {
      const permissions = await RBACAPI.getUserPermissions(
        user.username,
        requestOptions(controller),
      );
      if (isCurrentRead()) setUserPermissions(permissions);
    } catch {
      if (!isCurrentRead()) return;
      setPermissionsError('Saved permissions could not be loaded. Access is unknown.');
    } finally {
      if (isCurrentRead()) {
        permissionsController = undefined;
        setLoadingPermissions(false);
      }
    }
  };

  const openManageAccess = async (assignment: UserRoleAssignment) => {
    if (!canEdit() || !isCurrentUser(assignment)) return;
    clearEditor();
    setSuccessNotice(null);
    setUserPendingDeletion(null);
    setEditingUser(assignment);
    setFormRoleIds([...assignment.roleIds]);
    setShowModal(true);
    await loadUserPermissions();
  };
  const openDeleteUser = (assignment: UserRoleAssignment) => {
    if (!canEdit() || !isCurrentUser(assignment)) return;
    clearEditor();
    setSuccessNotice(null);
    setUserPendingDeletion(assignment);
  };
  const closeDeleteUser = () => {
    if (!deletingUser()) setUserPendingDeletion(null);
  };

  const mutateUser = async (user: UserRoleAssignment, kind: 'save' | 'delete') => {
    if (!canEdit() || !isCurrentUser(user) || mutationController) return;
    const version = contextVersion;
    const controller = new AbortController();
    const isCurrentMutation = () =>
      contextIsCurrent() && version === contextVersion && mutationController === controller;
    mutationController = controller;
    setActionError(null);
    setSuccessNotice(null);
    if (kind === 'save') setSaving(true);
    else setDeletingUser(true);
    let saved = false;
    try {
      if (kind === 'save') {
        await RBACAPI.updateUserRoles(
          user.username,
          [...formRoleIds()],
          requestOptions(controller),
        );
      } else {
        await RBACAPI.deleteUser(user.username, requestOptions(controller));
      }
      if (!isCurrentMutation()) return;
      saved = true;
    } catch (err) {
      if (!isCurrentMutation()) return;
      setActionError(
        kind === 'save'
          ? getUserAssignmentsUpdateErrorMessage()
          : getUserAssignmentsDeleteErrorMessage(err),
      );
    } finally {
      if (isCurrentMutation()) {
        mutationController = undefined;
        setSaving(false);
        setDeletingUser(false);
        if (saved) {
          await loadData();
          if (contextIsCurrent() && version === contextVersion) {
            setSuccessNotice(
              kind === 'save'
                ? `Roles updated for ${user.username}`
                : `User access removed for ${getUserIdentityDisplayName(user)}`,
            );
          }
        }
      }
    }
  };
  const handleDeleteUser = async () => {
    const user = userPendingDeletion();
    if (user) await mutateUser(user, 'delete');
  };
  const handleSaveAssignments = async () => {
    const user = editingUser();
    if (showModal() && user) await mutateUser(user, 'save');
  };
  const toggleRole = (roleId: string) => {
    if (!showModal() || !canEdit() || !roles().some((role) => role.id === roleId)) return;
    const currentRoleIds = formRoleIds();
    setFormRoleIds(
      currentRoleIds.includes(roleId)
        ? currentRoleIds.filter((id) => id !== roleId)
        : [...currentRoleIds, roleId],
    );
  };
  const getRoleName = (roleId: string) =>
    roles().find((role) => role.id === roleId)?.name || roleId;

  return {
    actionError,
    successNotice,
    accessAllowed,
    canEdit,
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
