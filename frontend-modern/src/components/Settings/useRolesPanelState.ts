import { batch, createEffect, createSignal, on, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { RBACAPI } from '@/api/rbac';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import type { Permission, Role } from '@/types/rbac';
import { createDefaultRBACPermission } from '@/utils/rbacPermissions';
import {
  getRoleChangeSuccessMessage,
  getRoleChangeUnconfirmedMessage,
  getRolesDeleteErrorMessage,
  getRolesLoadErrorMessage,
  getRolesRequiredFieldsMessage,
  getRolesSaveErrorMessage,
  isCompleteRoleEditorInventory,
} from '@/utils/rbacPresentation';
import { useRBACFeatureGateState } from './useRBACFeatureGateState';

const ROLE_ID_COLLISION = 'Role ID already exists: ';

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
  const [roleIdTouched, setRoleIdTouched] = createSignal(false);
  const [formName, setFormName] = createSignal('');
  const [formDescription, setFormDescription] = createSignal('');
  const [formPermissions, setFormPermissions] = createSignal<Permission[]>([]);
  const [loadError, setLoadError] = createSignal<string | null>(null);
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [successNotice, setSuccessNotice] = createSignal<string | null>(null);
  const [inventoryLoaded, setInventoryLoaded] = createSignal(false);
  const [unconfirmedChange, setUnconfirmedChange] = createSignal<string | null>(null);

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
    !unconfirmedChange() &&
    !saving() &&
    !deleting();
  const isCurrentRole = (role: Role) => roles().some((current) => unwrap(current) === unwrap(role));
  const requestOptions = (controller: AbortController) => ({
    signal: controller.signal,
    expectedOrgID: context().org,
    retry: false,
  });

  const slugifyRoleId = (name: string) =>
    name
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '');

  const clearEditor = () => {
    setShowModal(false);
    setEditingRole(null);
    setFormId('');
    setRoleIdTouched(false);
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

  const loadRoles = async (replacePending = false) => {
    const previous = readController;
    if (
      !contextIsCurrent() ||
      !accessAllowed() ||
      mutationController ||
      Boolean(previous) !== replacePending
    )
      return;
    const reader = context();
    const controller = new AbortController();
    // Own the replacement before cancelling; the old read must not settle it.
    readController = controller;
    previous?.abort();
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
      if (!isCompleteRoleEditorInventory(data)) throw new Error('Invalid role inventory');
      batch(() => {
        setRoles(data);
        setInventoryLoaded(true);
        setUnconfirmedChange(null);
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
      setUnconfirmedChange(null);
      setLoadError(null);
      setSaving(false);
      setDeleting(false);
      setLoading(!featureGate.licenseReady() || accessAllowed());
    });
    if (!disposed) void loadRoles();
  };
  createEffect(
    on([featureGate.licenseReady, accessAllowed, () => props.currentUser], (current, previous) => {
      // Dynamic props can refresh without changing the admitted reader.
      if (!previous || current.some((value, index) => value !== previous[index])) resetContext();
    }),
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
  const openDuplicateRole = (role: Role) => {
    if (!canEdit() || !isCurrentRole(role)) return;
    const baseId = `${role.id}-copy`;
    let candidateId = baseId;
    let suffix = 2;
    while (roles().some((existing) => existing.id === candidateId)) {
      candidateId = `${baseId}-${suffix}`;
      suffix += 1;
    }
    batch(() => {
      clearEditor();
      setRolePendingDeletion(null);
      setSuccessNotice(null);
      setFormId(candidateId);
      setRoleIdTouched(true);
      setFormName(`${role.name} Copy`);
      setFormDescription(role.description);
      setFormPermissions(
        role.permissions.length > 0
          ? role.permissions.map((permission) => ({ ...permission }))
          : [createDefaultRBACPermission()],
      );
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
        setSuccessNotice(getRoleChangeSuccessMessage(role, remove));
      });
      saved = true;
    } catch (error) {
      if (!isCurrentWrite()) return;
      const status = (error as { status?: number } | null)?.status;
      // A definite rejection can be corrected in the draft. A lost response,
      // server error or unreadable success does not establish rollback.
      if (status && [400, 401, 403, 404, 409, 422, 429].includes(status)) {
        const code = (error as { code?: unknown } | null)?.code;
        if (remove) {
          setActionError(getRolesDeleteErrorMessage());
        } else if (code === 'role_id_exists') {
          setActionError(`${getRolesSaveErrorMessage()}: a role with this ID already exists.`);
        } else if (code === 'validation_error' || code === 'invalid_role') {
          setActionError(
            `${getRolesSaveErrorMessage()}: check the role ID, name, and permissions.`,
          );
        } else {
          setActionError(getRolesSaveErrorMessage());
        }
      } else {
        batch(() => {
          clearEditor();
          setRolePendingDeletion(null);
          setRoles([]);
          setInventoryLoaded(false);
          setUnconfirmedChange(getRoleChangeUnconfirmedMessage(role, remove));
        });
      }
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
    // New Role must not replace a role this reader already knows. Compare the
    // exact normalized ID we would send, not its display name or case-folded
    // inventory IDs. This does not replace a server-side create-only check.
    if (!existing && roles().some((role) => role.id === id)) {
      setActionError(`${ROLE_ID_COLLISION}${id}. Choose a different ID.`);
      return;
    }
    await mutateRole(
      {
        // This form edits only the fields below. Keep saved inheritance,
        // priority and other role metadata instead of resetting it on PUT.
        ...existing,
        id,
        name,
        description: formDescription(),
        permissions: formPermissions().map((permission) => ({ ...permission })),
        createdAt: existing?.createdAt,
      },
      false,
    );
  };
  const updateFormId = (value: string) => {
    setRoleIdTouched(true);
    setFormId(value);
    if (actionError()?.startsWith(ROLE_ID_COLLISION)) setActionError(null);
  };
  const updateFormName = (value: string) => {
    setFormName(value);
    if (!editingRole() && !roleIdTouched()) {
      setFormId(slugifyRoleId(value));
      if (actionError()?.startsWith(ROLE_ID_COLLISION)) setActionError(null);
    }
  };
  const addPermission = () => {
    if (canEdit() && showModal())
      setFormPermissions([...formPermissions(), createDefaultRBACPermission()]);
  };
  const removePermission = (index: number) => {
    if (canEdit() && showModal())
      setFormPermissions(formPermissions().filter((_, current) => current !== index));
  };
  const updatePermission = (index: number, field: 'action' | 'resource', value: string) => {
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
    openDuplicateRole,
    openEditRole,
    removePermission,
    restartRead: () => loadRoles(true),
    rolePendingDeletion,
    roles,
    saving,
    setFormDescription,
    setFormId: updateFormId,
    setFormName: updateFormName,
    showModal,
    successNotice,
    unconfirmedChange,
    updatePermission,
  };
}
