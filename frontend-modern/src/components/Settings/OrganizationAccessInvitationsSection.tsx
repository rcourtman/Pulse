import { Component, For, Show } from 'solid-js';
import { formatOrgDate, roleBadgeClass } from '@/utils/orgUtils';
import type { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

interface OrganizationAccessInvitationsSectionProps {
  state: ReturnType<typeof useOrganizationAccessPanelState>;
}

export const OrganizationAccessInvitationsSection: Component<
  OrganizationAccessInvitationsSectionProps
> = (props) => (
  <div class="space-y-4">
    <Show when={props.state.myInvitations().length > 0}>
      <section class="rounded-md border border-border p-4 space-y-3">
        <div>
          <h4 class="text-sm font-semibold text-base-content">Your Invitations</h4>
          <p class="text-sm text-muted">Accept or decline pending organization access.</p>
        </div>
        <div class="space-y-3">
          <For each={props.state.myInvitations()}>
            {(invitation) => (
              <div class="rounded-md border border-border bg-surface px-3 py-3">
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-base-content">
                      {invitation.orgDisplayName || invitation.orgId}
                    </div>
                    <div class="text-xs text-muted">
                      Invited by {invitation.invitedBy || 'an admin'} on{' '}
                      {formatOrgDate(invitation.invitedAt)}
                    </div>
                  </div>
                  <span
                    class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(invitation.role)}`}
                  >
                    {invitation.role}
                  </span>
                </div>
                <div class="mt-3 flex gap-2">
                  <button
                    type="button"
                    onClick={() => void props.state.acceptInvitation(invitation.orgId)}
                    disabled={props.state.saving()}
                    class="inline-flex items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    Accept
                  </button>
                  <button
                    type="button"
                    onClick={() => void props.state.declineInvitation(invitation.orgId)}
                    disabled={props.state.saving()}
                    class="inline-flex items-center justify-center rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:border-red-300 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    Decline
                  </button>
                </div>
              </div>
            )}
          </For>
        </div>
      </section>
    </Show>

    <Show when={props.state.canManageCurrentOrg() && props.state.pendingInvitations().length > 0}>
      <section class="rounded-md border border-border p-4 space-y-3">
        <div>
          <h4 class="text-sm font-semibold text-base-content">Pending Invitations</h4>
          <p class="text-sm text-muted">These users still need to accept access.</p>
        </div>
        <div class="space-y-3">
          <For each={props.state.pendingInvitations()}>
            {(invitation) => (
              <div class="flex flex-col gap-2 rounded-md border border-border bg-surface px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="space-y-1">
                  <div class="text-sm font-medium text-base-content">{invitation.userId}</div>
                  <div class="text-xs text-muted">
                    Invited by {invitation.invitedBy || 'an admin'} on{' '}
                    {formatOrgDate(invitation.invitedAt)}
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${roleBadgeClass(invitation.role)}`}
                  >
                    {invitation.role}
                  </span>
                  <button
                    type="button"
                    onClick={() => void props.state.revokeInvitation(invitation.userId)}
                    disabled={props.state.saving()}
                    class="inline-flex items-center justify-center rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:border-red-300 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    Revoke
                  </button>
                </div>
              </div>
            )}
          </For>
        </div>
      </section>
    </Show>
  </div>
);
