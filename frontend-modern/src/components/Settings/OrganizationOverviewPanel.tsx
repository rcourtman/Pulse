import { Component, Show, createSignal, onCleanup, onMount } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { OrgsAPI, type Organization, type OrganizationMember } from '@/api/orgs';
import { getOrgID } from '@/utils/apiClient';
import { canManageOrg, formatOrgDate, normalizeRole, roleBadgeClass } from '@/utils/orgUtils';
import { isMultiTenantEnabled } from '@/stores/license';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Building2 from 'lucide-solid/icons/building-2';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

interface OrganizationOverviewPanelProps {
  currentUser?: string;
}

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
      const [orgData, memberData] = await Promise.all([
        OrgsAPI.get(orgId),
        OrgsAPI.listMembers(orgId),
      ]);
      setOrg(orgData);
      setMembers(memberData);
      setDisplayNameDraft(orgData.displayName || '');
    } catch (error) {
      logger.error('Failed to load organization overview', error);
      const msg = error instanceof Error ? error.message : '';
      if (msg.includes('402')) {
        notificationStore.error('Multi-tenant requires an Enterprise license');
      } else if (msg.includes('501')) {
        notificationStore.error('Multi-tenant is not enabled on this server');
      } else {
        notificationStore.error('Failed to load organization details');
      }
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
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to update organization name',
      );
    } finally {
      setSaving(false);
    }
  };

  onMount(() => {
    if (!isMultiTenantEnabled()) return;
    void loadOrganization();

    const unsubscribe = eventBus.on('org_switched', () => {
      void loadOrganization();
    });
    onCleanup(unsubscribe);
  });

  return (
    <Show
      when={isMultiTenantEnabled()}
      fallback={<div class="p-4 text-sm ">This feature is not available.</div>}
    >
      <div class="space-y-6">
        <SettingsPanel
          title="Organization Overview"
          description="Review organization metadata, membership footprint, and edit the display name."
          icon={<Building2 class="w-5 h-5" />}
          noPadding
          bodyClass="divide-y divide-border"
        >
          <Show
            when={!loading()}
            fallback={
              <div class="space-y-5 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  {Array.from({ length: 4 }).map(() => (
                    <div class="rounded-md border border-border p-3 space-y-2">
                      <div class="h-3 w-20 animate-pulse rounded bg-surface-hover" />
                      <div class="h-5 w-28 animate-pulse rounded bg-surface-hover" />
                    </div>
                  ))}
                </div>

                <div class="space-y-2">
                  <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                  <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <div class="h-10 w-full animate-pulse rounded bg-surface-hover" />
                    <div class="h-10 w-20 animate-pulse rounded bg-surface-hover" />
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                  <div class="overflow-hidden rounded-md border border-border">
                    <div class="h-10 w-full animate-pulse bg-surface-alt" />
                    {Array.from({ length: 3 }).map(() => (
                      <div class="border-t border-border-subtle px-3 py-3">
                        <div class="flex items-center gap-3">
                          <div class="h-4 w-40 animate-pulse rounded bg-surface-hover" />
                          <div class="h-4 w-14 animate-pulse rounded-full bg-surface-hover" />
                          <div class="h-4 w-24 animate-pulse rounded bg-surface-hover" />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            }
          >
            <Show when={org()}>
              {(currentOrg) => (
                <>
                  <div class="space-y-6 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                      <div class="rounded-md border border-border p-3">
                        <p class="text-xs uppercase tracking-wide text-muted">Organization</p>
                        <p class="mt-1 text-sm font-medium text-base-content">
                          {currentOrg().displayName || currentOrg().id}
                        </p>
                      </div>
                      <div class="rounded-md border border-border p-3">
                        <p class="text-xs uppercase tracking-wide text-muted">Org ID</p>
                        <p class="mt-1 text-sm font-mono break-all text-base-content">
                          {currentOrg().id}
                        </p>
                      </div>
                      <div class="rounded-md border border-border p-3">
                        <p class="text-xs uppercase tracking-wide text-muted">Created</p>
                        <p class="mt-1 text-sm font-medium text-base-content">
                          {formatOrgDate(currentOrg().createdAt)}
                        </p>
                      </div>
                      <div class="rounded-md border border-border p-3">
                        <p class="text-xs uppercase tracking-wide text-muted">Members</p>
                        <p class="mt-1 text-sm font-medium text-base-content">{members().length}</p>
                      </div>
                    </div>

                    <div class="space-y-2">
                      <label
                        class="block text-sm font-medium text-base-content"
                        for="org-display-name-input"
                      >
                        Display Name
                      </label>
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                        <input
                          id="org-display-name-input"
                          type="text"
                          value={displayNameDraft()}
                          onInput={(event) => setDisplayNameDraft(event.currentTarget.value)}
                          disabled={!canManageOrg(currentOrg(), props.currentUser) || saving()}
                          class="w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 "
                        />
                        <button
                          type="button"
                          onClick={saveDisplayName}
                          disabled={!canManageOrg(currentOrg(), props.currentUser) || saving()}
                          class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {saving() ? 'Saving...' : 'Save'}
                        </button>
                      </div>
                      <Show when={!canManageOrg(currentOrg(), props.currentUser)}>
                        <p class="text-xs text-muted">
                          Admin or owner role required to update organization details.
                        </p>
                      </Show>
                    </div>
                  </div>

                  <div class="space-y-2 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
                    <h4 class="text-sm font-semibold text-base-content">Membership</h4>
                    <div class="mt-4 -mx-4 sm:mx-0 overflow-x-auto w-full">
                      <PulseDataGrid
                        data={members()}
                        columns={[
                          {
                            key: 'userId',
                            label: 'User',
                            render: (member) => (
                              <span class="text-base-content">{member.userId}</span>
                            ),
                          },
                          {
                            key: 'role',
                            label: 'Role',
                            render: (member) => {
                              const role = normalizeRole(member.role);
                              return (
                                <span
                                  class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}
                                >
                                  {role}
                                </span>
                              );
                            },
                          },
                          {
                            key: 'addedAt',
                            label: 'Added',
                            render: (member) => (
                              <span class="text-muted">{formatOrgDate(member.addedAt)}</span>
                            ),
                          },
                        ]}
                        keyExtractor={(member) => member.userId}
                        emptyState="No members found."
                        desktopMinWidth="560px"
                        class="border-x-0 sm:border-x sm:border-t sm:border-b sm:rounded-md border-y border-border"
                      />
                    </div>
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

export default OrganizationOverviewPanel;
