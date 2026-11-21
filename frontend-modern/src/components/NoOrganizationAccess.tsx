import {
  For,
  Show,
  batch,
  createResource,
  createSignal,
  onCleanup,
  type Component,
} from 'solid-js';
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

interface InvitationDecision {
  action: 'accept' | 'decline';
  organization: string;
}

const SLOW_INVITATION_DECISION_MS = 10_000;

/**
 * Shown to a signed-in user who can use no organization yet, typically someone
 * invited to an organization who has not accepted. Accepting an invitation
 * reloads the app into that organization.
 */
export const NoOrganizationAccess: Component<NoOrganizationAccessProps> = (props) => {
  let disposed = false;
  let invitationReadController: AbortController | undefined;
  let invitationRegion: HTMLElement | undefined;
  let restartCheckButton: HTMLButtonElement | undefined;
  let slowDecisionTimer: number | undefined;
  const [invitationRead, { refetch }] = createResource<InvitationReadResult>(async () => {
    invitationReadController?.abort();
    const controller = new AbortController();
    invitationReadController = controller;
    try {
      // This inbox belongs to the signed-in user, not an active organization.
      const list = await OrgsAPI.listMyInvitations({ signal: controller.signal, retry: false });
      if (!Array.isArray(list)) throw new Error('Invalid organization invitation list');
      return { kind: 'loaded', invitations: list };
    } catch (error) {
      if (!controller.signal.aborted) logger.warn('Failed to load organization invitations', error);
      // A failed read says nothing about whether an invitation exists. Keep
      // that separate from an accepted empty list, including after a decline.
      return { kind: 'unavailable', invitations: [] };
    } finally {
      if (invitationReadController === controller) {
        invitationReadController = undefined;
        // The result removes Restart. Keep keyboard position if the current
        // read finishes while that button still owns focus.
        if (
          !controller.signal.aborted &&
          restartCheckButton &&
          restartCheckButton.ownerDocument.activeElement === restartCheckButton
        ) {
          invitationRegion?.focus({ preventScroll: true });
        }
      }
    }
  });
  onCleanup(() => {
    disposed = true;
    invitationReadController?.abort();
    window.clearTimeout(slowDecisionTimer);
  });
  const [pendingDecision, setPendingDecision] = createSignal<InvitationDecision>();
  const [decisionSlow, setDecisionSlow] = createSignal(false);
  const [unconfirmedDecision, setUnconfirmedDecision] = createSignal<InvitationDecision>();
  const decisionsDisabled = () => !!(pendingDecision() || unconfirmedDecision());
  const checkAgainDisabled = () => !!pendingDecision() && !decisionSlow();

  const retryInvitations = () => {
    if (disposed || decisionsDisabled()) return;
    // The retry and restart controls can disappear as the read settles. Move
    // focus before that happens, never back from an exit on a late result.
    invitationRegion?.focus();
    void refetch();
  };

  const reload = () => (props.onReload ? props.onReload() : window.location.reload());

  const finishDecision = (decision: InvitationDecision, unconfirmed = false) => {
    window.clearTimeout(slowDecisionTimer);
    batch(() => {
      if (unconfirmed) setUnconfirmedDecision(decision);
      setPendingDecision(undefined);
      setDecisionSlow(false);
    });
  };

  const leaveWhileWaiting = () => {
    const decision = pendingDecision();
    // Stop acting on this response if the user leaves. That does not cancel
    // or undo a server write; the current access has to be read again.
    if (decision) finishDecision(decision, true);
  };

  const checkAgain = () => {
    if (disposed || checkAgainDisabled()) return;
    leaveWhileWaiting();
    reload();
  };

  const respond = async (
    invitation: UserOrganizationInvitation,
    action: InvitationDecision['action'],
  ) => {
    if (disposed || invitationRead.loading || decisionsDisabled()) return;
    // A disabled row button can drop keyboard focus. Move it only when it is
    // still inside this region, never from a recovery or sign-out control.
    if (invitationRegion?.contains(document.activeElement)) {
      invitationRegion.focus({ preventScroll: true });
    }
    const decision = {
      action,
      organization: getOrganizationInvitationPresentation(
        invitation.orgId,
        invitation.orgDisplayName,
      ).organization,
    };
    const isCurrent = () => !disposed && pendingDecision() === decision;
    setPendingDecision(decision);
    // Avoid replacing a healthy in-flight decision with a reload, but keep
    // an escape for an unresponsive server. The timer itself does not retry.
    slowDecisionTimer = window.setTimeout(() => {
      if (isCurrent()) setDecisionSlow(true);
    }, SLOW_INVITATION_DECISION_MS);
    try {
      if (action === 'accept') {
        await OrgsAPI.acceptMyInvitation(invitation.orgId);
        if (isCurrent()) reload();
        return;
      }
      await OrgsAPI.declineMyInvitation(invitation.orgId);
      // The decision is complete. Its readback can be replaced independently.
      if (isCurrent()) void refetch();
    } catch (error) {
      if (!isCurrent()) return;
      logger.warn('Failed to answer organization invitation', error);
      // The reply can fail after Pulse saved the decision. An inbox read
      // alone cannot tell whether an acceptance granted access; Check again
      // re-enters the existing bootstrap path instead of resending the write.
      finishDecision(decision, true);
    } finally {
      if (isCurrent()) finishDecision(decision);
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
        <p
          class="mt-2 text-sm leading-6 text-muted [overflow-wrap:anywhere]"
          aria-live="polite"
          aria-atomic="true"
        >
          <Show
            when={pendingDecision()}
            keyed
            fallback={
              unconfirmedDecision()
                ? 'You are signed in. Your organization access may have changed.'
                : 'You are signed in, but you are not a member of an organization on this Pulse server.'
            }
          >
            {(decision) => (
              <>
                You are signed in. Waiting for Pulse to confirm your decision to {decision.action}{' '}
                the invitation to <bdi>{decision.organization}</bdi>.
                <Show when={decisionSlow()}>
                  {' '}
                  Keep waiting or choose <strong>Check again</strong> to check access without
                  resending your decision.
                </Show>
              </>
            )}
          </Show>
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
              <div class="mt-4">
                <p class="text-sm leading-6 text-muted" role="status">
                  Checking invitations…
                </p>
                <Button
                  ref={restartCheckButton}
                  variant="secondary"
                  size="sm"
                  class="mt-2 min-h-11 sm:min-h-0"
                  onClick={retryInvitations}
                >
                  Restart invitation check
                </Button>
              </div>
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
            disabled={checkAgainDisabled()}
            onClick={checkAgain}
          >
            Check again
          </Button>
          <Button
            variant="ghost"
            size="md"
            onClick={() => {
              leaveWhileWaiting();
              void props.onSignOut();
            }}
          >
            Sign out
          </Button>
        </div>
      </div>
    </main>
  );
};

export default NoOrganizationAccess;
