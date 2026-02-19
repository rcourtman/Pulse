import { Component, createSignal, createMemo, createEffect, onMount, Show, For } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACAPI } from '@/api/rbac';
import type { Role, UserRoleAssignment, Permission } from '@/types/rbac';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
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

    createEffect((wasPaywallVisible) => {
        const isPaywallVisible = licenseLoaded() && !hasFeature('rbac') && !loading();
        if (isPaywallVisible && !wasPaywallVisible) {
            trackPaywallViewed('rbac', 'settings_user_assignments_panel');
        }
        return isPaywallVisible;
    }, false);

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
            <SettingsPanel
                title="User Access"
                description="Assign roles to users and review effective permissions."
                icon={<Users class="w-5 h-5" />}
                action={
                    <div class="relative">
                        <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
                        <input
                            type="text"
                            placeholder="Search users..."
                            value={searchQuery()}
                            onInput={(e) => setSearchQuery(e.currentTarget.value)}
                            class="min-h-10 sm:min-h-9 pl-9 pr-3 py-2.5 text-sm rounded-md border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:ring-blue-900/40"
                        />
                    </div>
                }
                bodyClass="space-y-4"
            >

                <Show when={licenseLoaded() && !hasFeature('rbac') && !loading()}>
                    <div class="p-4 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-md">
                        <div class="flex flex-col sm:flex-row items-center gap-4">
                            <div class="flex-1">
                                <h4 class="text-base font-semibold text-slate-900 dark:text-white">Centralized Access Control (Pro)</h4>
                                <p class="text-sm text-slate-600 dark:text-slate-400 mt-1">
                                    Assign multi-tier roles to users and manage infrastructure-wide security policies.
                                </p>
                            </div>
                            <a
                                href={getUpgradeActionUrlOrFallback('rbac')}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
                                onClick={() => trackUpgradeClicked('settings_user_assignments_panel', 'rbac')}
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

                <Show when={!loading() && filteredAssignments().length === 0}>
                    <div class="text-center py-12 px-6">
                        <Users class="w-12 h-12 mx-auto text-slate-300 dark:text-slate-600 mb-4" />
                        <h4 class="text-base font-medium text-slate-900 dark:text-slate-100 mb-2">No users yet</h4>
                        <p class="text-sm text-slate-500 dark:text-slate-400 max-w-md mx-auto">
                            Users appear here automatically when they sign in via SSO (OIDC/SAML) or proxy authentication.
                            Once they've logged in, you can assign roles to control their access.
                        </p>
                        <div class="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3 text-xs text-slate-400 dark:text-slate-500">
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
                        <table class="min-w-[620px] w-full text-sm">
                            <thead>
                                <tr class="border-b border-slate-200 dark:border-slate-700">
                                    <th class="text-left py-2 px-3 font-medium text-slate-600 dark:text-slate-400">Username</th>
                                    <th class="text-left py-2 px-3 font-medium text-slate-600 dark:text-slate-400">Assigned Roles</th>
                                    <th class="text-right py-2 px-3 font-medium text-slate-600 dark:text-slate-400">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                <For each={filteredAssignments()}>
                                    {(assignment) => (
                                        <tr class="border-b border-slate-100 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/50">
                                            <td class="py-3 px-3">
                                                <span class="font-medium text-slate-900 dark:text-slate-100">{assignment.username}</span>
                                            </td>
                                            <td class="py-3 px-3">
                                                <div class="flex flex-wrap gap-1">
                                                    <Show when={assignment.roleIds.length === 0}>
                                                        <span class="text-xs text-slate-400 italic">No roles assigned</span>
                                                    </Show>
                                                    <For each={assignment.roleIds}>
                                                        {(roleId) => (
                                                            <span class="inline-flex items-center gap-1 rounded-md bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300 border border-slate-200 dark:border-slate-700">
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
                                                    class="inline-flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
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
            </SettingsPanel>

            {/* Assignments Modal */}
            <Show when={showModal()}>
                <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
                    <div class="w-full max-w-2xl bg-white dark:bg-slate-900 rounded-md shadow-sm border border-slate-200 dark:border-slate-700 mx-4 max-h-[92vh] overflow-hidden">
                        <div class="flex items-start justify-between gap-3 px-4 sm:px-6 py-4 border-b border-slate-200 dark:border-slate-700">
                            <div>
                                <h3 class="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                    Manage Access: {editingUser()?.username}
                                </h3>
                                <p class="text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wider font-semibold mt-0.5">Role Assignments</p>
                            </div>
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="p-1.5 rounded-md text-slate-500 hover:text-slate-700 hover:bg-slate-100 dark:hover:text-slate-300 dark:hover:bg-slate-800"
                            >
                                <X class="w-5 h-5" />
                            </button>
                        </div>

                        <div class="px-4 sm:px-6 py-6 space-y-8 max-h-[70vh] overflow-y-auto">
                            {/* Role Selection */}
                            <div class="space-y-4">
                                <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                                    <Shield class="w-4 h-4 text-blue-500" />
                                    Select Roles
                                </h4>
                                <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                    <For each={roles()}>
                                        {(role) => (
                                            <label class={`flex flex-col p-3 rounded-md border transition-all cursor-pointer ${formRoleIds().includes(role.id)
                                                ? 'bg-blue-50 border-blue-200 dark:bg-blue-900/20 dark:border-blue-800'
                                                : 'bg-white border-slate-200 hover:border-blue-100 dark:bg-slate-800 dark:border-slate-700 dark:hover:border-blue-900'
                                                }`}>
                                                <div class="flex items-start justify-between gap-2 mb-1">
                                                    <div class="flex items-center gap-2 shadow-sm">
                                                        <input
                                                            type="checkbox"
                                                            checked={formRoleIds().includes(role.id)}
                                                            onChange={() => toggleRole(role.id)}
                                                            class="w-4 h-4 text-blue-600 rounded border-slate-300 focus:ring-blue-500 dark:border-slate-600"
                                                        />
                                                        <span class="text-sm font-semibold text-slate-900 dark:text-slate-100">
                                                            {role.name}
                                                        </span>
                                                    </div>
                                                    <Show when={role.isBuiltIn}>
                                                        <BadgeCheck class="w-4 h-4 text-blue-500" />
                                                    </Show>
                                                </div>
                                                <p class="text-xs text-slate-500 dark:text-slate-400 line-clamp-2 leading-relaxed pl-6">
                                                    {role.description}
                                                </p>
                                            </label>
                                        )}
                                    </For>
                                </div>
                            </div>

                            {/* Effective Permissions Preview */}
                            <div class="space-y-4 pt-4 border-t border-slate-100 dark:border-slate-800">
                                <div class="flex items-center justify-between">
                                    <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                                        <BadgeCheck class="w-4 h-4 text-blue-500" />
                                        Effective Permissions Preview
                                    </h4>
                                    <Show when={loadingPermissions()}>
                                        <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500" />
                                    </Show>
                                </div>
                                <div class="bg-slate-50 dark:bg-slate-950 rounded-md p-4 border border-slate-100 dark:border-slate-800">
                                    <Show when={!loadingPermissions() && userPermissions().length === 0}>
                                        <p class="text-xs text-slate-500 dark:text-slate-400 italic text-center py-2">
                                            No effective permissions. This user will have no access.
                                        </p>
                                    </Show>
                                    <div class="flex flex-wrap gap-2">
                                        <For each={userPermissions()}>
                                            {(perm) => (
                                                <span class="inline-flex items-center rounded-md bg-white px-2.5 py-1 text-xs font-semibold text-slate-700 dark:bg-slate-900 dark:text-slate-300 border border-slate-200 dark:border-slate-700 shadow-sm">
                                                    <span class="text-blue-600 dark:text-blue-400">{perm.action}</span>
                                                    <span class="mx-1 text-slate-400">:</span>
                                                    <span class="text-blue-600 dark:text-blue-400">{perm.resource}</span>
                                                </span>
                                            )}
                                        </For>
                                    </div>
                                    <p class="mt-4 text-[10px] text-slate-400 dark:text-slate-500 uppercase tracking-widest font-bold">
                                        Note: Permissions are recalculated on save. This preview shows current server-side state.
                                    </p>
                                </div>
                            </div>
                        </div>

                        <div class="grid grid-cols-1 sm:flex sm:items-center sm:justify-end gap-3 px-4 sm:px-6 py-5 border-t border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-800 rounded-b-xl">
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                class="w-full sm:w-auto rounded-md px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSave}
                                disabled={saving()}
                                class="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-md bg-blue-600 px-6 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
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
