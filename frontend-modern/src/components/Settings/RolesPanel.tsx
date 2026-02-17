import { Component, createEffect, createSignal, onMount, Show, For } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACAPI } from '@/api/rbac';
import type { Role, Permission } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Shield from 'lucide-solid/icons/shield';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import X from 'lucide-solid/icons/x';

const ACTIONS = ['read', 'write', 'delete', 'admin', '*'];
const RESOURCES = ['settings', 'audit_logs', 'nodes', 'users', 'license', '*'];

export const RolesPanel: Component = () => {
    const [roles, setRoles] = createSignal<Role[]>([]);
    const [loading, setLoading] = createSignal(true);
    const [showModal, setShowModal] = createSignal(false);
    const [editingRole, setEditingRole] = createSignal<Role | null>(null);
    const [saving, setSaving] = createSignal(false);

    // Form state
    const [formId, setFormId] = createSignal('');
    const [formName, setFormName] = createSignal('');
    const [formDescription, setFormDescription] = createSignal('');
    const [formPermissions, setFormPermissions] = createSignal<Permission[]>([]);

    const loadRoles = async () => {
        setLoading(true);
        try {
            const data = await RBACAPI.getRoles();
            setRoles(data || []);
        } catch (err) {
            logger.error('Failed to load roles', err);
            notificationStore.error('Failed to load roles');
        } finally {
            setLoading(false);
        }
    };

    onMount(() => {
        loadLicenseStatus();
        loadRoles();
    });

    createEffect((wasPaywallVisible) => {
        const isPaywallVisible = licenseLoaded() && !hasFeature('rbac') && !loading();
        if (isPaywallVisible && !wasPaywallVisible) {
            trackPaywallViewed('rbac', 'settings_roles_panel');
        }
        return isPaywallVisible;
    }, false);

    const handleCreate = () => {
        setEditingRole(null);
        setFormId('');
        setFormName('');
        setFormDescription('');
        setFormPermissions([{ action: 'read', resource: 'nodes' }]);
        setShowModal(true);
    };

    const handleEdit = (role: Role) => {
        if (role.isBuiltIn) return;
        setEditingRole(role);
        setFormId(role.id);
        setFormName(role.name);
        setFormDescription(role.description);
        setFormPermissions([...role.permissions]);
        setShowModal(true);
    };

    const handleDelete = async (role: Role) => {
        if (role.isBuiltIn) return;
        if (!confirm(`Are you sure you want to delete the role "${role.name}"?`)) return;

        try {
            await RBACAPI.deleteRole(role.id);
            notificationStore.success(`Role "${role.name}" deleted`);
            await loadRoles();
        } catch (err) {
            logger.error('Failed to delete role', err);
            notificationStore.error('Failed to delete role');
        }
    };

    const handleSave = async () => {
        const id = formId().trim().toLowerCase().replace(/\s+/g, '-');
        const name = formName().trim();
        if (!id || !name) {
            notificationStore.error('ID and Name are required');
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
            setShowModal(false);
            await loadRoles();
        } catch (err) {
            logger.error('Failed to save role', err);
            notificationStore.error('Failed to save role');
        } finally {
            setSaving(false);
        }
    };

    const addPermission = () => {
        setFormPermissions([...formPermissions(), { action: 'read', resource: 'nodes' }]);
    };

    const removePermission = (index: number) => {
        const perms = [...formPermissions()];
        perms.splice(index, 1);
        setFormPermissions(perms);
    };

    const updatePermission = (index: number, field: keyof Permission, value: string) => {
        const perms = [...formPermissions()];
        perms[index] = { ...perms[index], [field]: value };
        setFormPermissions(perms);
    };

    return (
        <div class="space-y-6">
            <SettingsPanel
                title="Roles"
                description="Manage built-in and custom roles with granular permissions."
                icon={<Shield class="w-5 h-5" />}
                action={
                    <button
                        type="button"
                        onClick={handleCreate}
                        class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
                    >
                        <Plus class="w-4 h-4" />
                        New Role
                    </button>
                }
                bodyClass="space-y-4"
            >

                <Show when={licenseLoaded() && !hasFeature('rbac') && !loading()}>
                    <div class="p-4 bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700 rounded-xl">
                        <div class="flex flex-col sm:flex-row items-center gap-4">
                            <div class="flex-1">
                                <h4 class="text-base font-semibold text-gray-900 dark:text-white">Custom Roles (Pro)</h4>
                                <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                    Define granular permissions and custom access tiers for your team.
                                </p>
                            </div>
                            <a
                                href={getUpgradeActionUrlOrFallback('rbac')}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                                onClick={() => trackUpgradeClicked('settings_roles_panel', 'rbac')}
                            >
                                Upgrade to Pro
                            </a>
                        </div>
                    </div>
                </Show>

                <Show when={loading()}>
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
                    </div>
                </Show>

                <Show when={!loading()}>
                    <div class="overflow-x-auto">
                        <table class="w-full text-sm">
                            <thead>
                                <tr class="border-b border-gray-200 dark:border-gray-700">
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Role</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Permissions</th>
                                    <th class="text-right py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                <For each={roles()}>
                                    {(role) => (
                                        <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                            <td class="py-3 px-3">
                                                <div class="flex flex-col">
                                                    <span class="font-medium text-gray-900 dark:text-gray-100 flex items-center gap-1">
                                                        {role.name}
                                                        <Show when={role.isBuiltIn}>
                                                            <BadgeCheck class="w-4 h-4 text-blue-500" />
                                                        </Show>
                                                    </span>
                                                    <span class="text-xs text-gray-500 dark:text-gray-400">{role.description}</span>
                                                </div>
                                            </td>
                                            <td class="py-3 px-3">
                                                <div class="flex flex-wrap gap-1">
                                                    <For each={role.permissions}>
                                                        {(perm) => (
                                                            <span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-400 border border-gray-200 dark:border-gray-700">
                                                                {perm.action}:{perm.resource}
                                                            </span>
                                                        )}
                                                    </For>
                                                </div>
                                            </td>
                                            <td class="py-3 px-3 text-right">
                                                <Show when={!role.isBuiltIn}>
                                                    <div class="inline-flex items-center gap-1">
                                                        <button
                                                            type="button"
                                                            onClick={() => handleEdit(role)}
                                                            class="p-1.5 rounded-md text-gray-500 hover:text-blue-600 hover:bg-gray-100 dark:hover:text-blue-300 dark:hover:bg-gray-800"
                                                            title="Edit role"
                                                        >
                                                            <Pencil class="w-4 h-4" />
                                                        </button>
                                                        <button
                                                            type="button"
                                                            onClick={() => handleDelete(role)}
                                                            class="p-1.5 rounded-md text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900/30"
                                                            title="Delete role"
                                                        >
                                                            <Trash2 class="w-4 h-4" />
                                                        </button>
                                                    </div>
                                                </Show>
                                                <Show when={role.isBuiltIn}>
                                                    <span class="text-xs text-gray-400 italic">Read-only</span>
                                                </Show>
                                            </td>
                                        </tr>
                                    )}
                                </For>
                            </tbody>
                        </table>
                    </div>
                </Show>
            </SettingsPanel>

            {/* Role Modal */}
            <Show when={showModal()}>
                <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
                    <div class="w-full max-w-2xl bg-white dark:bg-gray-900 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 mx-4">
                        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                {editingRole() ? 'Edit Role' : 'New Role'}
                            </h3>
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="p-1.5 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-800"
                            >
                                <X class="w-5 h-5" />
                            </button>
                        </div>

                        <div class="px-6 py-4 space-y-4 max-h-[70vh] overflow-y-auto">
                            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                                <div class="space-y-1">
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                        Role ID
                                    </label>
                                    <input
                                        type="text"
                                        value={formId()}
                                        onInput={(e) => setFormId(e.currentTarget.value)}
                                        placeholder="e.g., custom-auditor"
                                        disabled={!!editingRole()}
                                        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/40 disabled:opacity-50"
                                    />
                                </div>
                                <div class="space-y-1">
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                        Role Name
                                    </label>
                                    <input
                                        type="text"
                                        value={formName()}
                                        onInput={(e) => setFormName(e.currentTarget.value)}
                                        placeholder="e.g., Custom Auditor"
                                        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/40"
                                    />
                                </div>
                            </div>
                            <div class="space-y-1">
                                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                    Description
                                </label>
                                <input
                                    type="text"
                                    value={formDescription()}
                                    onInput={(e) => setFormDescription(e.currentTarget.value)}
                                    placeholder="Brief description of this role's purpose"
                                    class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-900/40"
                                />
                            </div>

                            <div class="space-y-3 pt-2">
                                <div class="flex items-center justify-between">
                                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                                        Permissions
                                    </label>
                                    <button
                                        type="button"
                                        onClick={addPermission}
                                        class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-300 flex items-center gap-1"
                                    >
                                        <Plus class="w-3 h-3" /> Add Permission
                                    </button>
                                </div>

                                <div class="space-y-2">
                                    <For each={formPermissions()}>
                                        {(perm, index) => (
                                            <div class="flex items-center gap-2">
                                                <select
                                                    value={perm.action}
                                                    onChange={(e) => updatePermission(index(), 'action', e.currentTarget.value)}
                                                    class="flex-1 rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                                                >
                                                    <For each={ACTIONS}>
                                                        {(action) => <option value={action}>{action}</option>}
                                                    </For>
                                                </select>
                                                <span class="text-gray-400 text-sm">:</span>
                                                <select
                                                    value={perm.resource}
                                                    onChange={(e) => updatePermission(index(), 'resource', e.currentTarget.value)}
                                                    class="flex-1 rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                                                >
                                                    <For each={RESOURCES}>
                                                        {(resource) => <option value={resource}>{resource}</option>}
                                                    </For>
                                                </select>
                                                <button
                                                    type="button"
                                                    onClick={() => removePermission(index())}
                                                    disabled={formPermissions().length <= 1}
                                                    class="p-1.5 text-gray-400 hover:text-red-500 disabled:opacity-30"
                                                >
                                                    <Trash2 class="w-4 h-4" />
                                                </button>
                                            </div>
                                        )}
                                    </For>
                                </div>
                            </div>
                        </div>

                        <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSave}
                                disabled={saving() || !formName().trim()}
                                class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                                {saving() ? 'Saving...' : editingRole() ? 'Update Role' : 'Create Role'}
                            </button>
                        </div>
                    </div>
                </div>
            </Show>
        </div>
    );
};

export default RolesPanel;
