import { Component, createSignal, createMemo, onMount, Show, For } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { RBACAPI } from '@/api/rbac';
import type { Role, UserRoleAssignment, Permission } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';
import Users from 'lucide-solid/icons/users';
import Shield from 'lucide-solid/icons/shield';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import X from 'lucide-solid/icons/x';
import Pencil from 'lucide-solid/icons/pencil';
import Search from 'lucide-solid/icons/search';

export const UserAssignmentsPanel: Component = () => {
    const [assignments, setAssignments] = createSignal<UserRoleAssignment[]>([]);
    const [roles, setRoles] = createSignal<Role[]>([]);
    const [loading, setLoading] = createSignal(true);
    const [searchQuery, setSearchQuery] = createSignal('');
    const [showModal, setShowModal] = createSignal(false);
    const [editingUser, setEditingUser] = createSignal<UserRoleAssignment | null>(null);
    const [saving, setSaving] = createSignal(false);
    const [userPermissions, setUserPermissions] = createSignal<Permission[]>([]);
    const [loadingPermissions, setLoadingPermissions] = createSignal(false);

    // Form state
    const [formRoleIds, setFormRoleIds] = createSignal<string[]>([]);

    const loadData = async () => {
        setLoading(true);
        try {
            const [usersData, rolesData] = await Promise.all([
                RBACAPI.getUsers(),
                RBACAPI.getRoles()
            ]);
            setAssignments(usersData || []);
            setRoles(rolesData || []);
        } catch (err) {
            logger.error('Failed to load user assignments', err);
            notificationStore.error('Failed to load user assignments');
        } finally {
            setLoading(false);
        }
    };

    onMount(() => {
        loadLicenseStatus();
        loadData();
    });

    const filteredAssignments = createMemo(() => {
        const query = searchQuery().toLowerCase();
        if (!query) return assignments();
        return assignments().filter(a => a.username.toLowerCase().includes(query));
    });

    const handleEditRoles = async (assignment: UserRoleAssignment) => {
        setEditingUser(assignment);
        setFormRoleIds([...assignment.roleIds]);
        setShowModal(true);
        await loadUserPermissions(assignment.username);
    };

    const loadUserPermissions = async (username: string) => {
        setLoadingPermissions(true);
        try {
            const perms = await RBACAPI.getUserPermissions(username);
            setUserPermissions(perms || []);
        } catch (err) {
            logger.error('Failed to load user permissions', err);
        } finally {
            setLoadingPermissions(false);
        }
    };

    const handleSave = async () => {
        const user = editingUser();
        if (!user) return;

        setSaving(true);
        try {
            await RBACAPI.updateUserRoles(user.username, formRoleIds());
            notificationStore.success(`Roles updated for ${user.username}`);
            setShowModal(false);
            await loadData();
        } catch (err) {
            logger.error('Failed to update user roles', err);
            notificationStore.error('Failed to update user roles');
        } finally {
            setSaving(false);
        }
    };

    const toggleRole = (roleId: string) => {
        const current = formRoleIds();
        if (current.includes(roleId)) {
            setFormRoleIds(current.filter(id => id !== roleId));
        } else {
            setFormRoleIds([...current, roleId]);
        }
    };

    const getRoleName = (id: string) => {
        return roles().find(r => r.id === id)?.name || id;
    };

    return (
        <div class="space-y-6">
            <Card padding="lg" class="space-y-4">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <div class="flex items-center justify-center w-10 h-10 rounded-lg bg-teal-100 dark:bg-teal-900/30">
                            <Users class="w-5 h-5 text-teal-600 dark:text-teal-400" />
                        </div>
                        <div>
                            <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">User Access</h3>
                            <p class="text-sm text-gray-600 dark:text-gray-400">Manage user role assignments and view effective permissions</p>
                        </div>
                    </div>
                    <div class="relative">
                        <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                        <input
                            type="text"
                            placeholder="Search users..."
                            value={searchQuery()}
                            onInput={(e) => setSearchQuery(e.currentTarget.value)}
                            class="pl-9 pr-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-teal-200 dark:focus:ring-teal-800/60"
                        />
                    </div>
                </div>

                <Show when={licenseLoaded() && !hasFeature('rbac') && !loading()}>
                    <div class="p-4 bg-teal-50 dark:bg-teal-900/20 border border-teal-100 dark:border-teal-800 rounded-xl">
                        <div class="flex flex-col sm:flex-row items-center gap-4">
                            <div class="flex-1">
                                <h4 class="text-base font-semibold text-gray-900 dark:text-white">Centralized Access Control (Pro)</h4>
                                <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                    Assign multi-tier roles to users and manage infrastructure-wide security policies.
                                </p>
                            </div>
                            <a
                                href="https://pulserelay.pro/"
                                target="_blank"
                                rel="noopener noreferrer"
                                class="px-5 py-2.5 text-sm font-semibold bg-teal-600 text-white rounded-lg hover:bg-teal-700 transition-colors"
                            >
                                Upgrade to Pro
                            </a>
                        </div>
                    </div>
                </Show>

                <Show when={loading()}>
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-teal-500" />
                    </div>
                </Show>

                <Show when={!loading() && filteredAssignments().length === 0}>
                    <div class="text-center py-12 px-6">
                        <Users class="w-12 h-12 mx-auto text-gray-300 dark:text-gray-600 mb-4" />
                        <h4 class="text-base font-medium text-gray-900 dark:text-gray-100 mb-2">No users yet</h4>
                        <p class="text-sm text-gray-500 dark:text-gray-400 max-w-md mx-auto">
                            Users appear here automatically when they sign in via SSO (OIDC/SAML) or proxy authentication.
                            Once they've logged in, you can assign roles to control their access.
                        </p>
                        <div class="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3 text-xs text-gray-400 dark:text-gray-500">
                            <span class="flex items-center gap-1.5">
                                <Shield class="w-3.5 h-3.5" />
                                Configure SSO in Security settings
                            </span>
                            <span class="hidden sm:inline">•</span>
                            <span>Users sync on first login</span>
                        </div>
                    </div>
                </Show>

                <Show when={!loading() && filteredAssignments().length > 0}>
                    <div class="overflow-x-auto">
                        <table class="w-full text-sm">
                            <thead>
                                <tr class="border-b border-gray-200 dark:border-gray-700">
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Username</th>
                                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Assigned Roles</th>
                                    <th class="text-right py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                <For each={filteredAssignments()}>
                                    {(assignment) => (
                                        <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                                            <td class="py-3 px-3">
                                                <span class="font-medium text-gray-900 dark:text-gray-100">{assignment.username}</span>
                                            </td>
                                            <td class="py-3 px-3">
                                                <div class="flex flex-wrap gap-1">
                                                    <Show when={assignment.roleIds.length === 0}>
                                                        <span class="text-xs text-gray-400 italic">No roles assigned</span>
                                                    </Show>
                                                    <For each={assignment.roleIds}>
                                                        {(roleId) => (
                                                            <span class="inline-flex items-center gap-1 rounded-md bg-indigo-50 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300 border border-indigo-100 dark:border-indigo-800">
                                                                <Shield class="w-3 h-3" />
                                                                {getRoleName(roleId)}
                                                            </span>
                                                        )}
                                                    </For>
                                                </div>
                                            </td>
                                            <td class="py-3 px-3 text-right">
                                                <button
                                                    type="button"
                                                    onClick={() => handleEditRoles(assignment)}
                                                    class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                                                >
                                                    <Pencil class="w-4 h-4" />
                                                    Manage Access
                                                </button>
                                            </td>
                                        </tr>
                                    )}
                                </For>
                            </tbody>
                        </table>
                    </div>
                </Show>
            </Card>

            {/* Assignments Modal */}
            <Show when={showModal()}>
                <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
                    <div class="w-full max-w-2xl bg-white dark:bg-gray-900 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 mx-4">
                        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                            <div>
                                <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                                    Manage Access: {editingUser()?.username}
                                </h3>
                                <p class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold mt-0.5">Role Assignments</p>
                            </div>
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="p-1.5 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-800"
                            >
                                <X class="w-5 h-5" />
                            </button>
                        </div>

                        <div class="px-6 py-6 space-y-8 max-h-[70vh] overflow-y-auto">
                            {/* Role Selection */}
                            <div class="space-y-4">
                                <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                                    <Shield class="w-4 h-4 text-indigo-500" />
                                    Select Roles
                                </h4>
                                <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                    <For each={roles()}>
                                        {(role) => (
                                            <label class={`flex flex-col p-3 rounded-xl border transition-all cursor-pointer ${formRoleIds().includes(role.id)
                                                ? 'bg-indigo-50 border-indigo-200 dark:bg-indigo-900/20 dark:border-indigo-800'
                                                : 'bg-white border-gray-200 hover:border-indigo-100 dark:bg-gray-800 dark:border-gray-700 dark:hover:border-indigo-900'
                                                }`}>
                                                <div class="flex items-center justify-between mb-1">
                                                    <div class="flex items-center gap-2 shadow-sm">
                                                        <input
                                                            type="checkbox"
                                                            checked={formRoleIds().includes(role.id)}
                                                            onChange={() => toggleRole(role.id)}
                                                            class="w-4 h-4 text-indigo-600 rounded border-gray-300 focus:ring-indigo-500 dark:border-gray-600"
                                                        />
                                                        <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                                            {role.name}
                                                        </span>
                                                    </div>
                                                    <Show when={role.isBuiltIn}>
                                                        <BadgeCheck class="w-4 h-4 text-blue-500" />
                                                    </Show>
                                                </div>
                                                <p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 leading-relaxed pl-6">
                                                    {role.description}
                                                </p>
                                            </label>
                                        )}
                                    </For>
                                </div>
                            </div>

                            {/* Effective Permissions Preview */}
                            <div class="space-y-4 pt-4 border-t border-gray-100 dark:border-gray-800">
                                <div class="flex items-center justify-between">
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                                        <BadgeCheck class="w-4 h-4 text-teal-500" />
                                        Effective Permissions Preview
                                    </h4>
                                    <Show when={loadingPermissions()}>
                                        <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-teal-500" />
                                    </Show>
                                </div>
                                <div class="bg-gray-50 dark:bg-gray-950 rounded-xl p-4 border border-gray-100 dark:border-gray-800">
                                    <Show when={!loadingPermissions() && userPermissions().length === 0}>
                                        <p class="text-xs text-gray-500 dark:text-gray-400 italic text-center py-2">
                                            No effective permissions. This user will have no access.
                                        </p>
                                    </Show>
                                    <div class="flex flex-wrap gap-2">
                                        <For each={userPermissions()}>
                                            {(perm) => (
                                                <span class="inline-flex items-center rounded-md bg-white px-2.5 py-1 text-xs font-semibold text-gray-700 dark:bg-gray-900 dark:text-gray-300 border border-gray-200 dark:border-gray-700 shadow-sm">
                                                    <span class="text-indigo-600 dark:text-indigo-400">{perm.action}</span>
                                                    <span class="mx-1 text-gray-400">:</span>
                                                    <span class="text-teal-600 dark:text-teal-400">{perm.resource}</span>
                                                </span>
                                            )}
                                        </For>
                                    </div>
                                    <p class="mt-4 text-[10px] text-gray-400 dark:text-gray-500 uppercase tracking-widest font-bold">
                                        Note: Permissions are recalculated on save. This preview shows current server-side state.
                                    </p>
                                </div>
                            </div>
                        </div>

                        <div class="flex items-center justify-end gap-3 px-6 py-5 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/50 rounded-b-xl">
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSave}
                                disabled={saving()}
                                class="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-6 py-2 text-sm font-bold text-white shadow-lg shadow-indigo-200 dark:shadow-none transition-all hover:bg-indigo-700 hover:-translate-y-0.5 active:translate-y-0 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:translate-y-0"
                            >
                                {saving() ? 'Applying...' : 'Save Changes'}
                            </button>
                        </div>
                    </div>
                </div>
            </Show>
        </div>
    );
};

export default UserAssignmentsPanel;
