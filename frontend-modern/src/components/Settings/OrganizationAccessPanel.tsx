import { Component, For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { OrgsAPI, type Organization, type OrganizationMember, type OrganizationRole } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { canManageOrg, normalizeRole, roleBadgeClass } from '@/utils/orgUtils';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Users from 'lucide-solid/icons/users';
import Trash2 from 'lucide-solid/icons/trash-2';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

interface OrganizationAccessPanelProps {
  currentUser?: string;
}

const roleOptions: Array<{ value: Exclude<OrganizationRole, 'member'>; label: string }> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'editor', label: 'Editor' },
  { value: 'admin', label: 'Admin' },
  { value: 'owner', label: 'Owner' },
];

export const OrganizationAccessPanel: Component<OrganizationAccessPanelProps> = (props) => {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [org, setOrg] = createSignal<Organization | null>(null);
  const [members, setMembers] = createSignal<OrganizationMember[]>([]);
  const [inviteUserID, setInviteUserID] = createSignal('');
  const [inviteRole, setInviteRole] = createSignal<Exclude<OrganizationRole, 'member'>>('viewer');

  const activeOrgId = () => getOrgID() || 'default';

  const loadOrganizationAccess = async () => {
    setLoading(true);
    try {
      const orgId = activeOrgId();
      const [orgData, memberData] = await Promise.all([OrgsAPI.get(orgId), OrgsAPI.listMembers(orgId)]);
      setOrg(orgData);
      setMembers(memberData);
    } catch (error) {
      logger.error('Failed to load organization access data', error);
      const msg = error instanceof Error ? error.message : '';
      if (msg.includes('402')) {
        notificationStore.error('Multi-tenant requires an Enterprise license');
      } else if (msg.includes('501')) {
        notificationStore.error('Multi-tenant is not enabled on this server');
      } else {
        notificationStore.error('Failed to load organization access settings');
      }
    } finally {
      setLoading(false);
    }
  };

  const updateRole = async (member: OrganizationMember, role: Exclude<OrganizationRole, 'member'>) => {
    const currentOrg = org();
    if (!currentOrg) return;
    const currentRole = normalizeRole(member.role);
    if (currentRole === role) return;

    if (member.userId === currentOrg.ownerUserId && role !== 'owner') {
      notificationStore.error('Current owner can only remain owner. Transfer ownership instead.');
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.updateMemberRole(currentOrg.id, { userId: member.userId, role });
      notificationStore.success(`Updated ${member.userId} to ${role}`);
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to update member role', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to update member role');
    } finally {
      setSaving(false);
    }
  };

  const inviteMember = async () => {
    const currentOrg = org();
    if (!currentOrg) return;

    const userId = inviteUserID().trim();
    if (!userId) {
      notificationStore.error('User ID is required');
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.inviteMember(currentOrg.id, { userId, role: inviteRole() });
      notificationStore.success(`Added ${userId} as ${inviteRole()}`);
      setInviteUserID('');
      setInviteRole('viewer');
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to add organization member', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to add member');
    } finally {
      setSaving(false);
    }
  };

  const removeMember = async (member: OrganizationMember) => {
    const currentOrg = org();
    if (!currentOrg) return;
    if (!confirm(`Remove ${member.userId} from ${currentOrg.displayName || currentOrg.id}?`)) {
      return;
    }

    setSaving(true);
    try {
      await OrgsAPI.removeMember(currentOrg.id, member.userId);
      notificationStore.success(`Removed ${member.userId}`);
      await loadOrganizationAccess();
    } catch (error) {
      logger.error('Failed to remove organization member', error);
      notificationStore.error(error instanceof Error ? error.message : 'Failed to remove member');
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadOrganizationAccess();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadOrganizationAccess();
    });
    onCleanup(unsubscribe);
  });

  return (
    <Show when={isMultiTenantEnabled()} fallback={<div class="p-4 text-sm text-slate-500">This feature is not available.</div>}>
      <div class="space-y-6">
        <SettingsPanel
          title="Organization Access"
          description="Manage organization member roles and ownership transfers."
          icon={<Users class="w-5 h-5" />}
          bodyClass="space-y-5"
        >
          <Show
            when={!loading()}
            fallback={
              <div class="space-y-5">
                <div class="rounded-md border border-border p-4 space-y-3">
                  <div class="h-4 w-24 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                  <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
                    <div class="h-10 w-full animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                    <div class="h-10 w-28 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                    <div class="h-10 w-16 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                  </div>
                </div>

                <div class="overflow-hidden rounded-md border border-border">
                  <div class="h-10 w-full animate-pulse bg-slate-100 dark:bg-slate-800" />
                  {Array.from({ length: 4 }).map(() => (
                    <div class="border-t border-border-subtle px-3 py-3">
                      <div class="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3">
                        <div class="h-4 w-40 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                        <div class="h-7 w-24 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                        <div class="h-4 w-20 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                        <div class="ml-auto h-6 w-16 animate-pulse rounded bg-slate-200 dark:bg-slate-700" />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            }
          >
            <Show when={org()}>
              {(currentOrg) => (
                <>
                  <Show when={canManageOrg(currentOrg(), props.currentUser)}>
                    <div class="rounded-md border border-border p-4 space-y-3">
                      <h4 class="text-sm font-semibold text-base-content">Add Member</h4>
                      <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
                        <input
                          type="text"
                          value={inviteUserID()}
                          onInput={(event) => setInviteUserID(event.currentTarget.value)}
                          placeholder="username"
                          class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                        />
                        <select
                          value={inviteRole()}
                          onChange={(event) => setInviteRole(event.currentTarget.value as Exclude<OrganizationRole, 'member'>)}
                          class="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                        >
                          <For each={roleOptions.filter((option) => option.value !== 'owner' || currentOrg().ownerUserId === props.currentUser)}>
                            {(option) => <option value={option.value}>{option.label}</option>}
                          </For>
                        </select>
                        <button
                          type="button"
                          onClick={inviteMember}
                          disabled={saving()}
                          class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {saving() ? 'Saving...' : 'Add'}
                        </button>
                      </div>
                    </div>
                  </Show>

                  <Show when={!canManageOrg(currentOrg(), props.currentUser)}>
                    <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300">
                      Admin or owner role required to manage organization access.
                    </div>
                  </Show>

                  <div class="mt-4">
                    <PulseDataGrid
                      data={members()}
                      columns={[
                        {
                          key: 'userId',
                          label: 'User',
                          render: (member) => <span class="text-base-content">{member.userId}</span>
                        },
                        {
                          key: 'role',
                          label: 'Role',
                          render: (member) => {
                            const role = normalizeRole(member.role);
                            const isOwner = () => member.userId === currentOrg().ownerUserId;
                            return (
                              <Show
                                when={canManageOrg(currentOrg(), props.currentUser)}
                                fallback={
                                  <span class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}>
                                    {role}
                                  </span>
                                }
                              >
                                <select
                                  value={role}
                                  onChange={(event) => {
                                    void updateRole(member, event.currentTarget.value as Exclude<OrganizationRole, 'member'>);
                                  }}
                                  disabled={saving() || (isOwner() && props.currentUser !== currentOrg().ownerUserId)}
                                  class="rounded-md border border-slate-300 bg-white px-2 py-1 text-xs text-slate-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                                >
                                  <For
                                    each={roleOptions.filter(
                                      (option) => option.value !== 'owner' || props.currentUser === currentOrg().ownerUserId,
                                    )}
                                  >
                                    {(option) => <option value={option.value}>{option.label}</option>}
                                  </For>
                                </select>
                              </Show>
                            );
                          }
                        },
                        {
                          key: 'addedAt',
                          label: 'Added',
                          render: (member) => <span class="text-muted">{new Date(member.addedAt).toLocaleDateString()}</span>
                        },
                        {
                          key: 'actions',
                          label: 'Actions',
                          align: 'right',
                          render: (member) => {
                            const isOwner = () => member.userId === currentOrg().ownerUserId;
                            return (
                              <Show when={canManageOrg(currentOrg(), props.currentUser) && !isOwner()}>
                                <button
                                  type="button"
                                  onClick={() => {
                                    void removeMember(member);
                                  }}
                                  disabled={saving()}
                                  class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60"
                                >
                                  <Trash2 class="w-3.5 h-3.5" />
                                  Remove
                                </button>
                              </Show>
                            );
                          }
                        }
                      ]}
                      keyExtractor={(member) => member.userId}
                      emptyState="No organization members found."
                      desktopMinWidth="700px"
                    />
                  </div>
                </>
              )}
            </Show>
          </Show>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationAccessPanel;
