import { Component, For, Show } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { OrganizationRoleBadge } from '@/components/shared/OrganizationBadges';
import { formatOrgDate } from '@/utils/orgUtils';
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
                  <OrganizationRoleBadge role={invitation.role} />
                </div>
                <div class="mt-3 flex gap-2">
                  <Button
                    variant="primary"
                    size="mdCompact"
                    onClick={() => void props.state.acceptInvitation(invitation.orgId)}
                    disabled={props.state.saving()}
                  >
                    Accept
                  </Button>
                  <Button
                    variant="dangerOutline"
                    size="mdCompact"
                    onClick={() => void props.state.declineInvitation(invitation.orgId)}
                    disabled={props.state.saving()}
                  >
                    Decline
                  </Button>
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
                  <OrganizationRoleBadge role={invitation.role} />
                  <Button
                    variant="dangerOutline"
                    size="mdCompact"
                    onClick={() => void props.state.revokeInvitation(invitation.userId)}
                    disabled={props.state.saving()}
                  >
                    Revoke
                  </Button>
                </div>
              </div>
            )}
          </For>
        </div>
      </section>
    </Show>
  </div>
);
