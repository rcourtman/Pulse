import type { OrganizationRole } from '@/api/orgs';
import { Component, For, Show } from 'solid-js';
import { ORGANIZATION_MEMBER_ROLE_OPTIONS } from '@/utils/organizationRolePresentation';
import { getOrganizationAccessManageRequiredMessage } from '@/utils/organizationSettingsPresentation';
import type { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

interface OrganizationAccessManagementSectionProps {
  currentUser?: string;
  state: ReturnType<typeof useOrganizationAccessPanelState>;
}

export const OrganizationAccessManagementSection: Component<
  OrganizationAccessManagementSectionProps
> = (props) => (
  <Show when={props.state.org()}>
    {(currentOrg) => (
      <>
        <Show when={props.state.canManageCurrentOrg()}>
          <div class="rounded-md border border-border p-4 space-y-3">
            <h4 class="text-sm font-semibold text-base-content">Add Member</h4>
            <div class="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
              <input
                type="text"
                value={props.state.inviteUserID()}
                onInput={(event) => props.state.setInviteUserID(event.currentTarget.value)}
                placeholder="username"
                class="w-full rounded-md border px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <select
                value={props.state.inviteRole()}
                onChange={(event) =>
                  props.state.setInviteRole(event.currentTarget.value as OrganizationRole)
                }
                class="rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <For
                  each={ORGANIZATION_MEMBER_ROLE_OPTIONS.filter(
                    (option) =>
                      option.value !== 'owner' || currentOrg().ownerUserId === props.currentUser,
                  )}
                >
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
              <button
                type="button"
                onClick={props.state.inviteMember}
                disabled={props.state.saving()}
                class="inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {props.state.saving() ? 'Saving...' : 'Add'}
              </button>
            </div>
          </div>
        </Show>

        <Show when={!props.state.canManageCurrentOrg()}>
          <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300">
            {getOrganizationAccessManageRequiredMessage()}
          </div>
        </Show>
      </>
    )}
  </Show>
);
