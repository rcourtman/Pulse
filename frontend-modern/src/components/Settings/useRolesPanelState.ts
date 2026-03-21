import { createEffect, createSignal } from 'solid-js';
import { RBACAPI } from '@/api/rbac';
import type { Permission, Role } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { createDefaultRBACPermission } from '@/utils/rbacPermissions';
import {
  getRolesDeleteErrorMessage,
  getRolesLoadErrorMessage,
  getRolesRequiredFieldsMessage,
  getRolesSaveErrorMessage,
} from '@/utils/rbacPresentation';
import { useRBACFeatureGateState } from './useRBACFeatureGateState';

export function useRolesPanelState() {
  const [roles, setRoles] = createSignal<Role[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [showModal, setShowModal] = createSignal(false);
  const [editingRole, setEditingRole] = createSignal<Role | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [formId, setFormId] = createSignal('');
  const [formName, setFormName] = createSignal('');
  const [formDescription, setFormDescription] = createSignal('');
  const [formPermissions, setFormPermissions] = createSignal<Permission[]>([]);

  const featureGate = useRBACFeatureGateState({
    kind: 'roles',
    loading,
    paywallLocation: 'settings_roles_panel',
  });

  const loadRoles = async () => {
    if (!featureGate.rbacEnabled()) {
      setRoles([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const data = await RBACAPI.getRoles();
      setRoles(data || []);
    } catch (err) {
      if (err instanceof Error && /feature not included in license/i.test(err.message)) {
        setRoles([]);
        return;
      }
      logger.error('Failed to load roles', err);
      notificationStore.error(getRolesLoadErrorMessage());
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
      setRoles([]);
      setLoading(false);
      return;
    }
    void loadRoles();
  });

  const closeModal = () => {
    setShowModal(false);
  };

  const openCreateRole = () => {
    setEditingRole(null);
    setFormId('');
    setFormName('');
    setFormDescription('');
    setFormPermissions([createDefaultRBACPermission()]);
    setShowModal(true);
  };

  const openEditRole = (role: Role) => {
    if (role.isBuiltIn) return;
    setEditingRole(role);
    setFormId(role.id);
    setFormName(role.name);
    setFormDescription(role.description);
    setFormPermissions([...role.permissions]);
    setShowModal(true);
  };

  const handleDeleteRole = async (role: Role) => {
    if (role.isBuiltIn) return;
    if (!confirm(`Are you sure you want to delete the role "${role.name}"?`)) return;

    try {
      await RBACAPI.deleteRole(role.id);
      notificationStore.success(`Role "${role.name}" deleted`);
      await loadRoles();
    } catch (err) {
      logger.error('Failed to delete role', err);
      notificationStore.error(getRolesDeleteErrorMessage());
    }
  };

  const handleSaveRole = async () => {
    const id = formId().trim().toLowerCase().replace(/\s+/g, '-');
    const name = formName().trim();
    if (!id || !name) {
      notificationStore.error(getRolesRequiredFieldsMessage());
      return;
    }

    setSaving(true);
    try {
      const role: Role = {
        id,
        name,
        description: formDescription(),
        permissions: formPermissions(),
        createdAt: editingRole()?.createdAt,
      };
      await RBACAPI.saveRole(role);
      notificationStore.success(`Role "${name}" saved`);
      closeModal();
      await loadRoles();
    } catch (err) {
      logger.error('Failed to save role', err);
      notificationStore.error(getRolesSaveErrorMessage());
    } finally {
      setSaving(false);
    }
  };

  const addPermission = () => {
    setFormPermissions([...formPermissions(), createDefaultRBACPermission()]);
  };

  const removePermission = (index: number) => {
    const nextPermissions = [...formPermissions()];
    nextPermissions.splice(index, 1);
    setFormPermissions(nextPermissions);
  };

  const updatePermission = (index: number, field: keyof Permission, value: string) => {
    const nextPermissions = [...formPermissions()];
    nextPermissions[index] = { ...nextPermissions[index], [field]: value };
    setFormPermissions(nextPermissions);
  };

  return {
    addPermission,
    closeModal,
    editingRole,
    featureGate,
    formDescription,
    formId,
    formName,
    formPermissions,
    handleDeleteRole,
    handleSaveRole,
    loading,
    openCreateRole,
    openEditRole,
    removePermission,
    roles,
    saving,
    setFormDescription,
    setFormId,
    setFormName,
    showModal,
    updatePermission,
  };
}
