import { Component, For, Show } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { ORGANIZATION_MEMBER_ROLE_OPTIONS } from '@/utils/organizationRolePresentation';
import { formatOrgDate, normalizeRole, roleBadgeClass } from '@/utils/orgUtils';
import { getOrganizationAccessEmptyState } from '@/utils/organizationSettingsPresentation';
import Trash2 from 'lucide-solid/icons/trash-2';
import type { OrganizationMember, OrganizationRole } from '@/api/orgs';
import type { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

interface OrganizationAccessMembersSectionProps {
  currentUser?: string;
  state: ReturnType<typeof useOrganizationAccessPanelState>;
}

export const OrganizationAccessMembersSection: Component<
  OrganizationAccessMembersSectionProps
> = (props) => (
  <Show when={props.state.org()}>
    {(currentOrg) => (
      <div class="mt-4">
        <PulseDataGrid
          data={props.state.members()}
          columns={[
            {
              key: 'userId',
              label: 'User',
              render: (member) => <span class="text-base-content">{member.userId}</span>,
            },
            {
              key: 'role',
              label: 'Role',
              render: (member) => {
                const role = normalizeRole(member.role);
                const isOwner = () => member.userId === currentOrg().ownerUserId;
                return (
                  <Show
                    when={props.state.canManageCurrentOrg()}
                    fallback={
                      <span
                        class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(role)}`}
                      >
                        {role}
                      </span>
                    }
                  >
                    <select
                      value={role}
                      onChange={(event) => {
                        void props.state.updateRole(
                          member,
                          event.currentTarget.value as OrganizationRole,
                        );
                      }}
                      disabled={
                        props.state.saving() ||
                        (isOwner() && props.currentUser !== currentOrg().ownerUserId)
                      }
                      class="rounded-md border border-border bg-surface px-2 py-1 text-xs text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <For
                        each={ORGANIZATION_MEMBER_ROLE_OPTIONS.filter(
                          (option) =>
                            option.value !== 'owner' ||
                            props.currentUser === currentOrg().ownerUserId,
                        )}
                      >
                        {(option) => <option value={option.value}>{option.label}</option>}
                      </For>
                    </select>
                  </Show>
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
            {
              key: 'actions',
              label: 'Actions',
              align: 'right',
              render: (member: OrganizationMember) => {
                const isOwner = () => member.userId === currentOrg().ownerUserId;
                return (
                  <Show when={props.state.canManageCurrentOrg() && !isOwner()}>
                    <button
                      type="button"
                      onClick={() => {
                        void props.state.removeMember(member);
                      }}
                      disabled={props.state.saving()}
                      class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                      Remove
                    </button>
                  </Show>
                );
              },
            },
          ]}
          keyExtractor={(member) => member.userId}
          emptyState={getOrganizationAccessEmptyState()}
          desktopMinWidth="700px"
        />
      </div>
    )}
  </Show>
);
