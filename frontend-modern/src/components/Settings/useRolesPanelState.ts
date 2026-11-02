import { batch, createEffect, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { RBACAPI } from '@/api/rbac';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Permission, Role } from '@/types/rbac';
import { createDefaultRBACPermission } from '@/utils/rbacPermissions';
import {
  getRolesDeleteErrorMessage,
  getRolesLoadErrorMessage,
  getRolesRequiredFieldsMessage,
  getRolesSaveErrorMessage,
} from '@/utils/rbacPresentation';
import { useRBACFeatureGateState } from './useRBACFeatureGateState';

export interface RolesPanelProps {
  currentUser?: string;
  canManage?: boolean;
}

export function useRolesPanelState(props: RolesPanelProps = {}) {
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [showModal, setShowModal] = createSignal(false);
  const [editingRole, setEditingRole] = createSignal<Role | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [deleting, setDeleting] = createSignal(false);
  const [rolePendingDeletion, setRolePendingDeletion] = createSignal<Role | null>(null);
  const [formId, setFormId] = createSignal('');
  const [formName, setFormName] = createSignal('');
  const [formDescription, setFormDescription] = createSignal('');
  const [formPermissions, setFormPermissions] = createSignal<Permission[]>([]);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [successNotice, setSuccessNotice] = createSignal<string | null>(null);
  const [inventoryLoaded, setInventoryLoaded] = createSignal(false);

  const featureGate = useRBACFeatureGateState({
    kind: 'roles',
    loading,
    paywallLocation: 'settings_roles_panel',
  });
  const accessAllowed = () => featureGate.rbacEnabled() && props.canManage !== false;
  const currentOrg = () => getOrgID() || 'default';
  const [context, setContext] = createSignal({
    org: currentOrg(),
    user: props.currentUser,
    allowed: accessAllowed(),
  });
  let disposed = false;
  let readController: AbortController | undefined;
  let mutationController: AbortController | undefined;
  const contextIsCurrent = () =>
    !disposed &&
    context().org === currentOrg() &&
    context().user === props.currentUser &&
    context().allowed === accessAllowed();
  const canEdit = () =>
    contextIsCurrent() &&
    accessAllowed() &&
    inventoryLoaded() &&
    !loading() &&
    !loadError() &&
    !saving() &&
    !deleting();
  const isCurrentRole = (role: Role) => roles().some((current) => unwrap(current) === unwrap(role));
  const requestOptions = (controller: AbortController) => ({
    signal: controller.signal,
    expectedOrgID: context().org,
    retry: false,
  });

  const clearEditor = () => {
    setShowModal(false);
    setEditingRole(null);
    setFormId('');
    setFormName('');
    setFormDescription('');
    setFormPermissions([]);
    setActionError(null);
  };
  const closeModal = () => {
    if (!saving()) batch(clearEditor);
  };
  const closeDeleteRole = () => {
    if (deleting()) return;
    setRolePendingDeletion(null);
    setActionError(null);
  };

  const loadRoles = async () => {
    if (!contextIsCurrent() || !accessAllowed() || readController || mutationController) return;
    const reader = context();
    const controller = new AbortController();
    readController = controller;
    const isCurrentRead = () =>
      contextIsCurrent() && reader === context() && readController === controller;
    batch(() => {
      clearEditor();
      setRolePendingDeletion(null);
      setRoles([]);
      setInventoryLoaded(false);
      setLoading(true);
      setLoadError(null);
    });
    try {
      const data = await RBACAPI.getRoles(requestOptions(controller));
      if (!isCurrentRead()) return;
      const ids = new Set<string>();
      if (
        !Array.isArray(data) ||
        data.some((role) => {
          if (
            !role ||
            typeof role.id !== 'string' ||
            !role.id.trim() ||
            ids.has(role.id) ||
            typeof role.name !== 'string' ||
            typeof role.description !== 'string' ||
            (role.isBuiltIn !== undefined && typeof role.isBuiltIn !== 'boolean') ||
            !Array.isArray(role.permissions) ||
            role.permissions.some(
              (permission) =>
                !permission ||
                typeof permission.action !== 'string' ||
                !permission.action.trim() ||
                typeof permission.resource !== 'string' ||
                !permission.resource.trim(),
            )
          )
            return true;
          ids.add(role.id);
          return false;
        })
      )
        throw new Error('Invalid role inventory');
      batch(() => {
        setRoles(data);
        setInventoryLoaded(true);
      });
    } catch {
      if (isCurrentRead()) setLoadError(getRolesLoadErrorMessage());
    } finally {
      if (isCurrentRead()) {
        readController = undefined;
        setLoading(false);
      }
    }
  };

  const resetContext = () => {
    readController?.abort();
    mutationController?.abort();
    readController = mutationController = undefined;
    // A new object also invalidates operations when returning to the same reader.
    batch(() => {
      setContext({ org: currentOrg(), user: props.currentUser, allowed: accessAllowed() });
      clearEditor();
      setRoles([]);
      setRolePendingDeletion(null);
      setInventoryLoaded(false);
      setSuccessNotice(null);
      setLoadError(null);
      setSaving(false);
      setDeleting(false);
      setLoading(!featureGate.licenseReady() || accessAllowed());
    });
    if (!disposed) void loadRoles();
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

  const openCreateRole = () => {
    if (!canEdit()) return;
    batch(() => {
      clearEditor();
      setRolePendingDeletion(null);
      setSuccessNotice(null);
      setFormPermissions([createDefaultRBACPermission()]);
      setShowModal(true);
    });
  };
  const openEditRole = (role: Role) => {
    if (!canEdit() || !isCurrentRole(role) || role.isBuiltIn) return;
    batch(() => {
      clearEditor();
      setRolePendingDeletion(null);
      setSuccessNotice(null);
      setEditingRole(role);
      setFormId(role.id);
      setFormName(role.name);
      setFormDescription(role.description);
      setFormPermissions(role.permissions.map((permission) => ({ ...permission })));
      setShowModal(true);
    });
  };
  const openDeleteRole = (role: Role) => {
    if (!canEdit() || !isCurrentRole(role) || role.isBuiltIn) return;
    batch(() => {
      clearEditor();
      setSuccessNotice(null);
      setRolePendingDeletion(role);
    });
  };

  const mutateRole = async (role: Role, remove: boolean) => {
    const reader = context();
    const controller = new AbortController();
    mutationController = controller;
    const isCurrentWrite = () =>
      contextIsCurrent() && reader === context() && mutationController === controller;
    batch(() => {
      setSaving(!remove);
      setDeleting(remove);
      setActionError(null);
      setSuccessNotice(null);
    });
    let saved = false;
    try {
      if (remove) await RBACAPI.deleteRole(role.id, requestOptions(controller));
      else await RBACAPI.saveRole(role, requestOptions(controller));
      if (!isCurrentWrite()) return;
      batch(() => {
        clearEditor();
        setRolePendingDeletion(null);
        setSuccessNotice(`Role "${role.name}" ${remove ? 'deleted' : 'saved'}`);
      });
      saved = true;
    } catch {
      if (isCurrentWrite())
        setActionError(remove ? getRolesDeleteErrorMessage() : getRolesSaveErrorMessage());
    } finally {
      if (isCurrentWrite()) {
        mutationController = undefined;
        setSaving(false);
        setDeleting(false);
      }
    }
    if (saved && contextIsCurrent() && reader === context()) await loadRoles();
  };
  const handleDeleteRole = async () => {
    const role = rolePendingDeletion();
    if (!canEdit() || !role || !isCurrentRole(role) || role.isBuiltIn) return;
    await mutateRole(role, true);
  };
  const handleSaveRole = async () => {
    if (!canEdit() || !showModal()) return;
    const existing = editingRole();
    if (existing && (!isCurrentRole(existing) || existing.isBuiltIn)) return;
    const id = existing?.id ?? formId().trim().toLowerCase().replace(/\s+/g, '-');
    const name = formName().trim();
    if (!id || !name) {
      setActionError(getRolesRequiredFieldsMessage());
      return;
    }
    await mutateRole(
      {
        id,
        name,
        description: formDescription(),
        permissions: formPermissions().map((permission) => ({ ...permission })),
        createdAt: existing?.createdAt,
      },
      false,
    );
  };
  const addPermission = () => {
    if (canEdit() && showModal())
      setFormPermissions([...formPermissions(), createDefaultRBACPermission()]);
  };
  const removePermission = (index: number) => {
    if (canEdit() && showModal())
      setFormPermissions(formPermissions().filter((_, current) => current !== index));
  };
  const updatePermission = (index: number, field: keyof Permission, value: string) => {
    if (canEdit() && showModal())
      setFormPermissions(
        formPermissions().map((permission, current) =>
          current === index ? { ...permission, [field]: value } : permission,
        ),
      );
  };

  return {
    accessAllowed,
    actionError,
    addPermission,
    canEdit,
    closeDeleteRole,
    closeModal,
    deleting,
    editingRole,
    featureGate,
    formDescription,
    formId,
    formName,
    formPermissions,
    handleDeleteRole,
    handleSaveRole,
    loading,
    loadError,
    loadRoles,
    openCreateRole,
    openDeleteRole,
    openEditRole,
    removePermission,
    rolePendingDeletion,
    roles,
    saving,
    setFormDescription,
    setFormId,
    setFormName,
    showModal,
    successNotice,
    updatePermission,
  };
}
