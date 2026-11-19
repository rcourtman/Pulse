import { Component, Show, createEffect, createUniqueId, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import {
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { OrganizationOverviewDetailsSection } from './OrganizationOverviewDetailsSection';
import { OrganizationOverviewLoadingState } from './OrganizationOverviewLoadingState';
import { OrganizationOverviewMembersSection } from './OrganizationOverviewMembersSection';
import { useOrganizationOverviewPanelState } from './useOrganizationOverviewPanelState';

export interface OrganizationOverviewPanelProps {
  currentUser?: string;
}

export const OrganizationOverviewPanel: Component<OrganizationOverviewPanelProps> = (props) => {
  const state = useOrganizationOverviewPanelState(props);
  const showOrganizationSurface = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const warningTitleID = createUniqueId();
  const checkingID = createUniqueId();
  let panel: HTMLDivElement | undefined;
  let warning: HTMLDivElement | undefined;
  let lastFocused: { element: HTMLElement; orgID: string; user: string | undefined } | undefined;
  let focusContext: [string, string | undefined, boolean, boolean] | undefined;
  let focusTimer: number | undefined;

  const reloadOrganization = () => {
    if (state.loading() || state.saving()) return;
    // Keep a stable keyboard anchor before the read replaces its Reload button,
    // including pointer activation that did not focus the button in Safari.
    panel?.focus();
    state.reloadOrganization();
  };

  createEffect(() => {
    window.clearTimeout(focusTimer);
    const orgID = state.selectedOrgID();
    const user = props.currentUser;
    const enabled = showOrganizationSurface();
    const context: typeof focusContext = [orgID, user, enabled, presentationPolicyIsReadOnly()];
    // Reader/authority round trips must not revive a previous keyboard intent.
    if (focusContext?.some((value, index) => value !== context[index])) {
      lastFocused = undefined;
    }
    focusContext = context;
    const loading = state.loading();
    const error = state.loadError();
    const org = state.org();
    if (!enabled || loading) return;
    if (!error && !org) return;

    focusTimer = window.setTimeout(() => {
      if (
        !panel?.isConnected ||
        !showOrganizationSurface() ||
        state.loading() ||
        state.selectedOrgID() !== orgID ||
        props.currentUser !== user ||
        state.loadError() !== error ||
        state.org() !== org
      )
        return;
      const lostPanelFocus =
        lastFocused?.orgID === orgID &&
        lastFocused.user === user &&
        !lastFocused.element.isConnected;
      // Recover only replaced local controls, never deliberate outside focus.
      if (lostPanelFocus && (!document.activeElement || document.activeElement === document.body)) {
        const input = panel.querySelector<HTMLInputElement>('#org-display-name-input');
        const target = error
          ? warning
          : input && !input.disabled
            ? input
            : panel.querySelector<HTMLElement>('[data-org-overview-name]');
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
        role="region"
        aria-label="Organization details"
        tabIndex={-1}
        class="space-y-6 focus:outline-2 focus:-outline-offset-2 focus:outline-blue-500"
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
        <SettingsPanel title="Organization Overview" noPadding>
          <Show when={state.loading()}>
            <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 pt-4 sm:px-6 sm:pt-6">
              <p id={checkingID} class="min-w-0 max-w-full text-sm text-muted">
                Checking saved details for{' '}
                <bdi class="font-medium text-base-content [overflow-wrap:anywhere]">
                  {`${state.selectedOrgID()}.`}
                </bdi>{' '}
                Restarting does not repeat a name change.
              </p>
              <Button
                aria-describedby={checkingID}
                onClick={(event) => {
                  // Pointer activation in Safari may not focus the control itself.
                  if (state.restartOrganizationRead()) event.currentTarget.focus();
                }}
                class="min-h-11 w-full sm:min-h-0 sm:w-auto"
              >
                Restart details check
              </Button>
            </div>
          </Show>
          <div
            role="group"
            aria-label="Saved organization details"
            aria-busy={state.loading()}
            class="divide-y divide-border"
          >
            <Show when={!state.loading()} fallback={<OrganizationOverviewLoadingState />}>
              <Show when={state.loadError()}>
                <div class="p-4 sm:p-6">
                  <Card tone="warning" padding="md">
                    <div
                      ref={warning}
                      role="alert"
                      aria-labelledby={warningTitleID}
                      tabIndex={-1}
                      class="space-y-3 text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                    >
                      <p id={warningTitleID} class="font-semibold">
                        Organization details unavailable
                      </p>
                      <p>{state.loadError()}</p>
                      <p class="break-words">
                        Reload checks saved details for {state.selectedOrgID()}. It does not repeat
                        a name change.
                      </p>
                      <Button onClick={reloadOrganization} class="w-full sm:w-auto">
                        Reload organization
                      </Button>
                    </div>
                  </Card>
                </div>
              </Show>
              <Show when={!state.loadError() && state.org()}>
                <OrganizationOverviewDetailsSection state={state} />
                <OrganizationOverviewMembersSection state={state} />
              </Show>
            </Show>
          </div>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationOverviewPanel;
