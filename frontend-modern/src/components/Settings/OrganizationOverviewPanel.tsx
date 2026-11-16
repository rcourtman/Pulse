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
  let panel: HTMLDivElement | undefined;
  let warning: HTMLDivElement | undefined;
  let lastFocused: { element: HTMLElement; orgID: string; user: string | undefined } | undefined;
  let focusContext: [string, string | undefined, boolean, boolean] | undefined;
  let reloadIntent = false;
  let focusTimer: number | undefined;

  const reloadOrganization = () => {
    if (state.loading() || state.saving()) return;
    // Pointer activation need not focus the button, notably in Safari.
    reloadIntent = true;
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
      reloadIntent = false;
    }
    focusContext = context;
    const loading = state.loading();
    const error = state.loadError();
    const org = state.org();
    if (!enabled || loading) return;
    const explicitReload = reloadIntent;
    reloadIntent = false;
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
      if (
        (explicitReload || lostPanelFocus) &&
        (!document.activeElement || document.activeElement === document.body)
      ) {
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
        <SettingsPanel title="Organization Overview" noPadding bodyClass="divide-y divide-border">
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
                      Reload checks saved details for {state.selectedOrgID()}. It does not repeat a
                      name change.
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
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationOverviewPanel;
