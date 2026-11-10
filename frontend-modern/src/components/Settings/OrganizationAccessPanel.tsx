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

export const OrganizationAccessPanel: Component<OrganizationAccessPanelProps> = (props) => {
  const state = useOrganizationAccessPanelState(props);
  const showOrganizationSurface = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const warningTitleID = createUniqueId();
  let warning: HTMLDivElement | undefined;
  let summary: HTMLParagraphElement | undefined;
  let lastFocused: { element: HTMLElement; orgID: string; user: string | undefined } | undefined;
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
    const loading = state.loading();
    const error = state.loadError();
    const org = state.org();
    if (!enabled || reloadIntent?.orgID !== orgID || reloadIntent.user !== user) {
      reloadIntent = undefined;
    }
    if (!enabled || loading) return;
    const explicitReload = !!reloadIntent;
    reloadIntent = undefined;
    if (!error && (!explicitReload || !org)) return;

    focusTimer = window.setTimeout(() => {
      const target = error ? warning : summary;
      if (
        !target?.isConnected ||
        !showOrganizationSurface() ||
        state.loading() ||
        state.selectedOrgID() !== orgID ||
        props.currentUser !== user ||
        state.loadError() !== error ||
        state.org() !== org
      )
        return;
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
        target.focus();
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
        class="space-y-6"
        onFocusIn={(event) => {
          if (event.target instanceof HTMLElement) {
            lastFocused = {
              element: event.target,
              orgID: state.selectedOrgID(),
              user: props.currentUser,
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
          <Show when={!state.loading()} fallback={<OrganizationAccessLoadingState />}>
            <Show when={state.loadError()}>
              <Card tone="warning" padding="md">
                <div
                  ref={warning}
                  role="alert"
                  aria-labelledby={warningTitleID}
                  tabIndex={-1}
                  class="space-y-3 text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  <p id={warningTitleID} class="font-semibold">
                    {ORGANIZATION_ACCESS_UNAVAILABLE_TITLE}
                  </p>
                  <p>{state.loadError()}</p>
                  <p>{getOrganizationAccessReloadMessage(state.selectedOrgID())}</p>
                  <Button
                    onClick={reloadAccess}
                    disabled={state.loading() || state.saving()}
                    class="w-full sm:w-auto"
                  >
                    {ORGANIZATION_ACCESS_RELOAD_LABEL}
                  </Button>
                </div>
              </Card>
            </Show>
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
