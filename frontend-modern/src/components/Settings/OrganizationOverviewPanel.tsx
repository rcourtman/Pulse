import { Component, Show, createSignal, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { OrgsAPI, type Organization, type OrganizationMember, type OrganizationRole } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Building2 from 'lucide-solid/icons/building-2';

interface OrganizationOverviewPanelProps {
  currentUser?: string;
}

const roleBadgeClass: Record<OrganizationRole, string> = {
  owner: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-200',
  admin: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200',
  editor: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200',
  viewer: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  member: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
};

const normalizeRole = (role: OrganizationRole): OrganizationRole => {
  if (role === 'member') return 'viewer';
  return role;
};

const formatDate = (value?: string) => {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

const canManageOrg = (org: Organization | null, currentUser?: string) => {
  if (!org || !currentUser) return false;
  if (org.ownerUserId === currentUser) return true;
  const role = normalizeRole(org.members?.find((m) => m.userId === currentUser)?.role ?? 'viewer');
  return role === 'admin' || role === 'owner';
};

export const OrganizationOverviewPanel: Component<OrganizationOverviewPanelProps> = (props) => {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [displayNameDraft, setDisplayNameDraft] = createSignal('');

  const activeOrgId = () => getOrgID() || 'default';

  const loadOrganization = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();
      const [orgData, memberData] = await Promise.all([OrgsAPI.get(orgId), OrgsAPI.listMembers(orgId)]);
      setOrg(orgData);
      setMembers(memberData);
      setDisplayNameDraft(orgData.displayName || '');
    } catch (error) {
      logger.error('Failed to load organization overview', error);
      notificationStore.error('Failed to load organization details');
    } finally {
      setLoading(false);
    }
  };

  const saveDisplayName = async () => {
    const currentOrg = org();
    if (!currentOrg) return;
    const nextName = displayNameDraft().trim();
    if (!nextName) {
      notificationStore.error('Display name is required');
      return;
    }
    if (nextName === currentOrg.displayName) {
      return;
    }

    setSaving(true);
    try {
      const updated = await OrgsAPI.update(currentOrg.id, { displayName: nextName });
      setOrg(updated);
      setDisplayNameDraft(updated.displayName || '');
      notificationStore.success('Organization name updated');
    } catch (error) {
      logger.error('Failed to update organization display name', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to update organization name');
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    void loadOrganization();
  });

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Organization Overview"
        description="Review organization metadata, membership footprint, and edit the display name."
        icon={<Building2 class="w-5 h-5" />}
        bodyClass="space-y-5"
      >
        <Show when={!loading()} fallback={<p class="text-sm text-gray-500 dark:text-gray-400">Loading organization details...</p>}>
          <Show when={org()}>
            {(currentOrg) => (
              <>
                <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
                    <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Organization</p>
                    <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{currentOrg().displayName || currentOrg().id}</p>
                  </div>
                  <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
                    <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Org ID</p>
                    <p class="mt-1 text-sm font-mono text-gray-900 dark:text-gray-100">{currentOrg().id}</p>
                  </div>
                  <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
                    <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Created</p>
                    <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{formatDate(currentOrg().createdAt)}</p>
                  </div>
                  <div class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
                    <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Members</p>
                    <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{members().length}</p>
                  </div>
                </div>

                <div class="space-y-2">
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="org-display-name-input">
                    Display Name
                  </label>
                  <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <input
                      id="org-display-name-input"
                      type="text"
                      value={displayNameDraft()}
                      onInput={(event) => setDisplayNameDraft(event.currentTarget.value)}
                      disabled={!canManageOrg(currentOrg(), props.currentUser) || saving()}
                      class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    />
                    <button
                      type="button"
                      onClick={saveDisplayName}
                      disabled={!canManageOrg(currentOrg(), props.currentUser) || saving()}
                      class="inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {saving() ? 'Saving...' : 'Save'}
                    </button>
                  </div>
                  <Show when={!canManageOrg(currentOrg(), props.currentUser)}>
                    <p class="text-xs text-gray-500 dark:text-gray-400">Admin or owner role required to update organization details.</p>
                  </Show>
                </div>

                <div class="space-y-2">
                  <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Membership</h4>
                  <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
                    <table class="w-full text-sm">
                      <thead class="bg-gray-50 dark:bg-gray-800/70">
                        <tr>
                          <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">User</th>
                          <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Role</th>
                          <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-300">Added</th>
                        </tr>
                      </thead>
                      <tbody>
                        <Show
                          when={members().length > 0}
                          fallback={
                            <tr>
                              <td colSpan={3} class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                                No members found.
                              </td>
                            </tr>
                          }
                        >
                          {members().map((member) => {
                            const role = normalizeRole(member.role);
                            return (
                              <tr class="border-t border-gray-100 dark:border-gray-800">
                                <td class="px-3 py-2 text-gray-900 dark:text-gray-100">{member.userId}</td>
                                <td class="px-3 py-2">
                                  <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass[role]}`}>
                                    {role}
                                  </span>
                                </td>
                                <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{formatDate(member.addedAt)}</td>
                              </tr>
                            );
                          })}
                        </Show>
                      </tbody>
                    </table>
                  </div>
                </div>
              </>
            )}
          </Show>
        </Show>
      </SettingsPanel>
    </div>
  );
};

export default OrganizationOverviewPanel;
