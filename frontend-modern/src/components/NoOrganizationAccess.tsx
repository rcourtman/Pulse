import { For, Show, createResource, createSignal, type Component } from 'solid-js';
import { OrgsAPI, type UserOrganizationInvitation } from '@/api/orgs';
import { Button } from '@/components/shared/Button';
import { OrganizationRoleBadge } from '@/components/shared/OrganizationBadges';
import { logger } from '@/utils/logger';
import { normalizeRole } from '@/utils/orgUtils';

interface NoOrganizationAccessProps {
  onSignOut: () => void | Promise<void>;
  /** Reloads the app after an invitation is accepted. */
  onReload?: () => void;
}

type InvitationReadResult =
  | { kind: 'loaded'; invitations: UserOrganizationInvitation[] }
  | { kind: 'unavailable'; invitations: [] };

/**
 * Shown to a signed-in user who can use no organization yet, typically someone
 * invited to an organization who has not accepted. Accepting an invitation
 * reloads the app into that organization.
 */
export const NoOrganizationAccess: Component<NoOrganizationAccessProps> = (props) => {
  const [invitationRead, { refetch }] = createResource<InvitationReadResult>(async () => {
    try {
      const list = await OrgsAPI.listMyInvitations();
      if (!Array.isArray(list)) throw new Error('Invalid organization invitation list');
      return { kind: 'loaded', invitations: list };
    } catch (error) {
      logger.warn('Failed to load organization invitations', error);
      // A failed read says nothing about whether an invitation exists. Keep
      // that separate from an accepted empty list, including after a decline.
      return { kind: 'unavailable', invitations: [] };
    }
  });
  const [busyOrgID, setBusyOrgID] = createSignal<string | null>(null);
  const [actionError, setActionError] = createSignal<string | null>(null);

  const reload = () => (props.onReload ? props.onReload() : window.location.reload());

  const respond = async (orgId: string, accept: boolean) => {
    if (busyOrgID()) return;
    setBusyOrgID(orgId);
    setActionError(null);
    try {
      if (accept) {
        await OrgsAPI.acceptMyInvitation(orgId);
        reload();
        return;
      }
      await OrgsAPI.declineMyInvitation(orgId);
      await refetch();
    } catch (error) {
      logger.warn('Failed to answer organization invitation', error);
      setActionError(
        accept ? 'Could not accept the invitation.' : 'Could not decline the invitation.',
      );
    } finally {
      setBusyOrgID(null);
    }
  };

  return (
    <main
      class="flex min-h-screen items-center justify-center px-4 py-8 text-base-content"
      aria-labelledby="no-organization-access-title"
    >
      <div class="w-full max-w-md rounded-md border border-border bg-surface p-6 shadow-xs sm:p-8">
        <h1 id="no-organization-access-title" class="text-lg font-semibold tracking-tight">
          No organization access yet
        </h1>
        <p class="mt-2 text-sm leading-6 text-muted">
          You are signed in, but you are not a member of an organization on this Pulse server.
        </p>

        <Show
          when={!invitationRead.loading}
          fallback={
            <p class="mt-4 text-sm leading-6 text-muted" role="status">
              Checking invitations…
            </p>
          }
        >
          <Show
            when={invitationRead()?.kind === 'loaded'}
            fallback={
              <div class="mt-4">
                <p class="text-sm leading-6 text-red-600 dark:text-red-400" role="alert">
                  Could not check your invitations.
                </p>
                <Button variant="secondary" size="sm" class="mt-2" onClick={() => void refetch()}>
                  Retry invitations
                </Button>
              </div>
            }
          >
            <Show
              when={(invitationRead()?.invitations ?? []).length > 0}
              fallback={
                <p class="mt-4 text-sm leading-6 text-muted">
                  Ask a Pulse administrator to add you to an organization.
                </p>
              }
            >
              <ul class="mt-4 divide-y divide-border rounded-md border border-border">
                <For each={invitationRead()?.invitations}>
                  {(invitation) => (
                    <li class="flex items-center gap-3 px-3 py-2">
                      <span class="min-w-0 flex-1 truncate text-sm">
                        {invitation.orgDisplayName || invitation.orgId}
                      </span>
                      <OrganizationRoleBadge role={normalizeRole(invitation.role)} />
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={busyOrgID() !== null}
                        onClick={() => void respond(invitation.orgId, true)}
                      >
                        Accept
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={busyOrgID() !== null}
                        onClick={() => void respond(invitation.orgId, false)}
                      >
                        Decline
                      </Button>
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </Show>
        </Show>

        <Show when={actionError()}>
          {(message) => (
            <p class="mt-3 text-sm text-red-600 dark:text-red-400" role="alert">
              {message()}
            </p>
          )}
        </Show>

        <div class="mt-5 flex flex-wrap gap-2 border-t border-border pt-5">
          <Button variant="secondary" size="md" onClick={reload}>
            Check again
          </Button>
          <Button variant="ghost" size="md" onClick={() => void props.onSignOut()}>
            Sign out
          </Button>
        </div>
      </div>
    </main>
  );
};

export default NoOrganizationAccess;
