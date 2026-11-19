import { Component, Show, createEffect, createUniqueId, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  getOrganizationAccessReloadMessage,
  ORGANIZATION_ACCESS_RELOAD_LABEL,
  ORGANIZATION_ACCESS_UNAVAILABLE_TITLE,
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { OrganizationAccessLoadingState } from './OrganizationAccessLoadingState';
import { OrganizationAccessInvitationsSection } from './OrganizationAccessInvitationsSection';
import { OrganizationAccessManagementSection } from './OrganizationAccessManagementSection';
import { OrganizationAccessMembersSection } from './OrganizationAccessMembersSection';
import { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

export interface OrganizationAccessPanelProps {
  currentUser?: string;
}

const unreadInvitationLists = {
  pending: 'Pending Invitations',
  personal: 'Your Invitations',
  both: 'Pending Invitations and Your Invitations',
};

export const OrganizationAccessPanel: Component<OrganizationAccessPanelProps> = (props) => {
  const state = useOrganizationAccessPanelState(props);
  const showOrganizationSurface = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const warningMessage = () => {
    const issue = state.invitationReadIssue();
    return (
      state.loadError() ??
      (issue
        ? `${unreadInvitationLists[issue]} couldn’t be loaded. Access changes stay paused until the lists load.`
        : null)
    );
  };
  const warningTitleID = createUniqueId();
  const checkingID = createUniqueId();
  let panel: HTMLDivElement | undefined;
  let warning: HTMLDivElement | undefined;
  let restartButton: HTMLButtonElement | undefined;
  let summary: HTMLParagraphElement | undefined;
  let lastFocused:
    | {
        element: HTMLElement;
        orgID: string;
        user: string | undefined;
        action?: string;
        targets: string[];
      }
    | undefined;
  let focusContext: [string, string | undefined, boolean, boolean] | undefined;
  const controls = () =>
    Array.from(panel?.querySelectorAll<HTMLElement>('[data-org-access-action]') ?? []);
  let reloadIntent: { orgID: string; user: string | undefined } | undefined;
  let focusTimer: number | undefined;

  const reloadAccess = () => {
    // Pointer activation need not focus the button (notably in Safari).
    reloadIntent = { orgID: state.selectedOrgID(), user: props.currentUser };
    void state.loadOrganizationAccess();
  };

  createEffect(() => {
    window.clearTimeout(focusTimer);
    const orgID = state.selectedOrgID();
    const user = props.currentUser;
    const enabled = showOrganizationSurface();
    const context: typeof focusContext = [orgID, user, enabled, state.readOnly()];
    // A round trip must not revive the old reader's keyboard intent.
    if (focusContext?.some((value, index) => value !== context[index])) {
      lastFocused = undefined;
      reloadIntent = undefined;
    }
    focusContext = context;
    const loading = state.loading();
    const error = warningMessage();
    const org = state.org();
    if (!enabled || reloadIntent?.orgID !== orgID || reloadIntent.user !== user) {
      reloadIntent = undefined;
    }
    const checking = loading && !error;
    if (!enabled || (checking ? !reloadIntent : !error && !org)) return;

    focusTimer = window.setTimeout(() => {
      if (
        !panel?.isConnected ||
        !showOrganizationSurface() ||
        state.loading() !== loading ||
        state.selectedOrgID() !== orgID ||
        props.currentUser !== user ||
        warningMessage() !== error ||
        state.org() !== org
      )
        return;
      const explicitReload = !!reloadIntent;
      // An explicit reload can itself stall. Give its replacement control
      // focus while waiting but keep the intent for the eventual result.
      if (!checking) reloadIntent = undefined;
      // Disabling a pending control can blur it before its failed response.
      // Only recover lost panel focus, never a deliberate move elsewhere.
      const lostPanelFocus =
        lastFocused?.orgID === orgID &&
        lastFocused.user === user &&
        !lastFocused.element.isConnected;
      if (
        (explicitReload || lostPanelFocus) &&
        (!document.activeElement || document.activeElement === document.body)
      ) {
        // The refreshed controls are new DOM nodes. Match exact data values,
        // never interpolate a saved identity into a CSS selector. If a row was
        // removed, continue with the next (then previous) same-kind control.
        const candidates = controls().filter(
          (control) =>
            control.dataset.orgAccessAction === lastFocused?.action &&
            !control.matches(':disabled'),
        );
        const replacement = lastFocused?.targets
          .map((id) => candidates.find((control) => control.dataset.orgAccessTarget === id))
          .find(Boolean);
        const target = error
          ? warning
          : checking
            ? restartButton
            : explicitReload
              ? summary
              : (replacement ?? summary);
        target?.focus();
      }
    }, 0);
  });
  onCleanup(() => window.clearTimeout(focusTimer));

  return (
    <Show
      when={showOrganizationSurface()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div
        ref={panel}
        class="space-y-6"
        onFocusIn={(event) => {
          if (event.target instanceof HTMLElement) {
            const action = event.target.dataset.orgAccessAction;
            const ids = controls()
              .filter((control) => control.dataset.orgAccessAction === action)
              .map((control) => control.dataset.orgAccessTarget!);
            const index = ids.indexOf(event.target.dataset.orgAccessTarget!);
            lastFocused = {
              element: event.target,
              orgID: state.selectedOrgID(),
              user: props.currentUser,
              action,
              targets: index < 0 ? [] : [...ids.slice(index), ...ids.slice(0, index).reverse()],
            };
          }
        }}
        onFocusOut={(event) => {
          if (
            event.relatedTarget instanceof Node &&
            !event.currentTarget.contains(event.relatedTarget)
          ) {
            lastFocused = undefined;
          }
        }}
      >
        <SettingsPanel title="Organization Access" bodyClass="space-y-5">
          <Show when={warningMessage()}>
            {(message) => (
              <Card tone="warning" padding="md">
                <div
                  ref={warning}
                  role="alert"
                  aria-labelledby={warningTitleID}
                  tabIndex={-1}
                  class="space-y-3 text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  <p id={warningTitleID} class="font-semibold">
                    {state.loadError()
                      ? ORGANIZATION_ACCESS_UNAVAILABLE_TITLE
                      : 'Organization invitations incomplete'}
                  </p>
                  <p>{message()}</p>
                  <p>{getOrganizationAccessReloadMessage(state.selectedOrgID())}</p>
                  <Button onClick={reloadAccess} disabled={state.saving()} class="w-full sm:w-auto">
                    {ORGANIZATION_ACCESS_RELOAD_LABEL}
                  </Button>
                </div>
              </Card>
            )}
          </Show>
          <Show when={state.loading() && !warningMessage()}>
            <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
              <p id={checkingID} class="min-w-0 max-w-full break-words text-sm text-muted">
                Checking saved members and invitations for {state.selectedOrgID()}. Restarting does
                not repeat a change.
              </p>
              <Button
                ref={restartButton}
                aria-describedby={checkingID}
                onClick={reloadAccess}
                class="min-h-11 w-full sm:min-h-0 sm:w-auto"
              >
                Restart access check
              </Button>
            </div>
          </Show>
          <Show
            when={!state.loading()}
            fallback={<OrganizationAccessLoadingState showFormPlaceholder={!!warningMessage()} />}
          >
            <Show when={!state.loadError() && state.org()}>
              {(currentOrg) => (
                <>
                  <p
                    ref={summary}
                    tabIndex={-1}
                    class="text-sm text-muted break-words focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                  >
                    Access for{' '}
                    <span class="font-medium text-base-content">
                      {currentOrg().displayName || currentOrg().id}
                    </span>{' '}
                    ({currentOrg().id})
                  </p>
                  <OrganizationAccessInvitationsSection state={state} />
                  <OrganizationAccessManagementSection
                    state={state}
                    currentUser={props.currentUser}
                  />
                  <OrganizationAccessMembersSection state={state} currentUser={props.currentUser} />
                </>
              )}
            </Show>
          </Show>
        </SettingsPanel>
        <Show keyed when={state.ownershipTransfer()}>
          {(transfer) => (
            <Dialog
              isOpen={true}
              ariaLabel="Transfer organization ownership"
              onClose={() => state.cancelOwnershipTransfer(transfer)}
              panelClass="max-w-lg"
            >
              <Card padding="lg" class="min-h-0 overflow-y-auto space-y-4">
                <h3 class="text-lg font-semibold text-base-content">
                  Transfer organization ownership?
                </h3>
                <p class="text-sm text-base-content break-words">
                  Make <strong>{transfer.member.userId}</strong> the Owner of{' '}
                  <strong>{transfer.organization.displayName || transfer.organization.id}</strong> (
                  {transfer.organization.id})?
                </p>
                <p class="text-sm text-muted break-words">
                  {transfer.owner} will become an Admin. Only {transfer.member.userId} will be able
                  to transfer ownership again.
                </p>
                <div class="flex flex-col gap-2 sm:flex-row sm:justify-end">
                  <Button onClick={() => state.cancelOwnershipTransfer(transfer)}>Cancel</Button>
                  <Button
                    variant="danger"
                    onClick={() => void state.confirmOwnershipTransfer(transfer)}
                  >
                    Transfer ownership
                  </Button>
                </div>
              </Card>
            </Dialog>
          )}
        </Show>
      </div>
    </Show>
  );
};

export default OrganizationAccessPanel;
