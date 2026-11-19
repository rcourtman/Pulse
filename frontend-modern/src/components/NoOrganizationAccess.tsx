import { For, Show, createResource, createSignal, type Component } from 'solid-js';
import { OrgsAPI, type UserOrganizationInvitation } from '@/api/orgs';
import { Button } from '@/components/shared/Button';
import { OrganizationRoleBadge } from '@/components/shared/OrganizationBadges';
import { logger } from '@/utils/logger';
import { getOrganizationInvitationPresentation } from '@/utils/organizationSettingsPresentation';
import { normalizeRole } from '@/utils/orgUtils';

interface NoOrganizationAccessProps {
  onSignOut: () => void | Promise<void>;
  /** Rechecks the session's organization access. */
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
  const [responding, setResponding] = createSignal(false);
  const [unconfirmedDecision, setUnconfirmedDecision] = createSignal<{
    action: 'accept' | 'decline';
    organization: string;
  }>();
  const decisionsDisabled = () => responding() || unconfirmedDecision() !== undefined;
  let invitationRegion: HTMLElement | undefined;

  const retryInvitations = () => {
    // Retry removes its own button while reading. Keep the keyboard in the
    // invitation region now; a late result must not take focus back.
    invitationRegion?.focus();
    void refetch();
  };

  const reload = () => (props.onReload ? props.onReload() : window.location.reload());

  const respond = async (invitation: UserOrganizationInvitation, action: 'accept' | 'decline') => {
    if (decisionsDisabled()) return;
    // A disabled row button can drop keyboard focus. Move it only when it is
    // still inside this region, never from a recovery or sign-out control.
    if (invitationRegion?.contains(document.activeElement)) {
      invitationRegion.focus({ preventScroll: true });
    }
    setResponding(true);
    try {
      if (action === 'accept') {
        await OrgsAPI.acceptMyInvitation(invitation.orgId);
        reload();
        return;
      }
      await OrgsAPI.declineMyInvitation(invitation.orgId);
      await refetch();
    } catch (error) {
      logger.warn('Failed to answer organization invitation', error);
      // The reply can fail after Pulse saved the decision. An inbox read
      // alone cannot tell whether an acceptance granted access; Check again
      // re-enters the existing bootstrap path instead of resending the write.
      setUnconfirmedDecision({
        action,
        organization: getOrganizationInvitationPresentation(
          invitation.orgId,
          invitation.orgDisplayName,
        ).organization,
      });
    } finally {
      setResponding(false);
    }
  };

  return (
    <main
      class="flex min-h-screen items-center justify-center px-4 py-8 text-base-content"
      aria-labelledby="no-organization-access-title"
    >
      <div class="w-full max-w-md rounded-md border border-border bg-surface p-6 shadow-xs sm:p-8">
        <h1 id="no-organization-access-title" class="text-lg font-semibold tracking-tight">
          {unconfirmedDecision() ? 'Check organization access' : 'No organization access yet'}
        </h1>
        <p class="mt-2 text-sm leading-6 text-muted">
          {unconfirmedDecision()
            ? 'You are signed in. Your organization access may have changed.'
            : 'You are signed in, but you are not a member of an organization on this Pulse server.'}
        </p>

        <section
          ref={invitationRegion}
          aria-label="Organization invitations"
          aria-busy={invitationRead.loading}
          tabIndex={-1}
          class="rounded-md focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          <Show when={unconfirmedDecision()} keyed>
            {(decision) => (
              <p
                class="mt-4 text-sm leading-6 text-amber-700 [overflow-wrap:anywhere] dark:text-amber-300"
                role="alert"
              >
                Pulse could not confirm your decision to {decision.action} the invitation to{' '}
                <bdi>{decision.organization}</bdi>. Choose <strong>Check again</strong> to check
                access and invitations without resending your decision. The list below is from
                before your decision.
              </p>
            )}
          </Show>
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
                  <Button variant="secondary" size="sm" class="mt-2" onClick={retryInvitations}>
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
                    {(invitation) => {
                      const presentation = () =>
                        getOrganizationInvitationPresentation(
                          invitation.orgId,
                          invitation.orgDisplayName,
                        );
                      return (
                        <li class="px-3 py-2">
                          <div
                            role="group"
                            aria-label={presentation().groupLabel}
                            class="space-y-2"
                          >
                            <div class="flex items-start gap-3">
                              <div class="min-w-0 flex-1 [overflow-wrap:anywhere]">
                                <Show
                                  when={
                                    invitation.orgDisplayName &&
                                    invitation.orgDisplayName !== invitation.orgId
                                  }
                                >
                                  <bdi class="block text-sm">{invitation.orgDisplayName}</bdi>
                                </Show>
                                <bdi class="block text-xs text-muted">{invitation.orgId}</bdi>
                              </div>
                              <OrganizationRoleBadge role={normalizeRole(invitation.role)} />
                            </div>
                            <div class="flex flex-wrap gap-2">
                              <Button
                                variant="primary"
                                size="sm"
                                aria-label={presentation().acceptLabel}
                                disabled={decisionsDisabled()}
                                onClick={() => void respond(invitation, 'accept')}
                              >
                                Accept
                              </Button>
                              <Button
                                variant="secondary"
                                size="sm"
                                aria-label={presentation().declineLabel}
                                disabled={decisionsDisabled()}
                                onClick={() => void respond(invitation, 'decline')}
                              >
                                Decline
                              </Button>
                            </div>
                          </div>
                        </li>
                      );
                    }}
                  </For>
                </ul>
              </Show>
            </Show>
          </Show>
        </section>

        <div class="mt-5 flex flex-wrap gap-2 border-t border-border pt-5">
          <Button
            variant={unconfirmedDecision() ? 'primary' : 'secondary'}
            size="md"
            onClick={reload}
          >
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
