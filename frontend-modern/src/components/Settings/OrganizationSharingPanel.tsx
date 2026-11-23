import { Component, Show, createEffect, createMemo, createUniqueId, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { isMultiTenantEnabled } from '@/stores/license';
import { presentationPolicyHidesOrganizationSurfaces } from '@/stores/sessionPresentationPolicy';
import {
  getOrganizationSharingPartialReadMessage,
  getOrganizationSharingReloadMessage,
  ORGANIZATION_SHARING_MANAGER_REQUIRED_MESSAGE,
  ORGANIZATION_SHARING_READ_ONLY_MESSAGE,
  ORGANIZATION_SHARING_RELOAD_LABEL,
  ORGANIZATION_SHARING_UNAVAILABLE_TITLE,
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { OrganizationIncomingSharesSection } from './OrganizationIncomingSharesSection';
import { OrganizationOutgoingSharesSection } from './OrganizationOutgoingSharesSection';
import OrganizationSharingCreateSection from './OrganizationSharingCreateSection';
import { OrganizationSharingLoadingState } from './OrganizationSharingLoadingState';
import { useOrganizationSharingPanelState } from './useOrganizationSharingPanelState';

export interface OrganizationSharingPanelProps {
  currentUser?: string;
}

export const OrganizationSharingPanel: Component<OrganizationSharingPanelProps> = (props) => {
  const state = useOrganizationSharingPanelState(props);
  const showOrganizationSurface = () =>
    isMultiTenantEnabled() && !presentationPolicyHidesOrganizationSurfaces();
  const warningTitleID = createUniqueId();
  const checkingID = createUniqueId();
  let panel: HTMLDivElement | undefined;
  let warning: HTMLDivElement | undefined;
  let summary: HTMLParagraphElement | undefined;
  // A real scope change retires focus. A refreshed but equivalent status does not.
  const focusScope = createMemo(
    () =>
      [
        showOrganizationSurface(),
        state.selectedOrgID(),
        props.currentUser,
        state.readOnly(),
      ] as const,
    undefined,
    { equals: (previous, next) => previous.every((value, index) => value === next[index]) },
  );
  let lastFocused:
    | {
        element: HTMLElement;
        scope: ReturnType<typeof focusScope>;
        action?: string;
        targets: string[];
      }
    | undefined;
  const controls = (action?: string) =>
    action
      ? Array.from(panel?.querySelectorAll<HTMLElement>('[data-org-sharing-action]') ?? []).filter(
          (control) => control.dataset.orgSharingAction === action,
        )
      : [];

  const replacementControl = ({ action, targets }: NonNullable<typeof lastFocused>) => {
    if (!targets.length) return;
    // Reverse insertion keeps the first enabled match for an opaque ID.
    const byTarget = new Map<string | undefined, HTMLElement>();
    for (const control of controls(action).reverse()) {
      if (!control.matches(':disabled')) byTarget.set(control.dataset.orgSharingTarget, control);
    }
    for (const id of targets) {
      const control = byTarget.get(id);
      if (control) return control;
    }
  };
  const reloadSharing = () => {
    // The explicit read replaces its button. Keep a stable keyboard position
    // through loading instead of losing focus and moving it after the response.
    // Safari pointer activation need not focus the button first.
    panel?.focus();
    void state.loadSharingData();
  };

  createEffect(() => {
    const scope = focusScope();
    if (lastFocused?.scope !== scope) lastFocused = undefined;
    const loading = state.loading();
    const error = state.loadError();
    if (!scope[0] || loading || (!error && !state.org())) return;

    // Effect cleanup cancels this timer whenever the saved view changes or unmounts.
    const timer = window.setTimeout(() => {
      const previous = lastFocused;
      // A reread replaces controls. Recover only the focus it removed, never
      // an untouched initial read or a deliberate move to another control.
      if (
        panel?.isConnected &&
        previous?.scope === scope &&
        !previous.element.isConnected &&
        (!document.activeElement || document.activeElement === document.body)
      ) {
        const target = error ? warning : (replacementControl(previous) ?? summary);
        target?.focus();
      }
    }, 0);
    onCleanup(() => window.clearTimeout(timer));
  });

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
        aria-label="Organization sharing"
        tabIndex={-1}
        class="space-y-6 rounded-lg focus:outline-2 focus:-outline-offset-2 focus:outline-blue-500 dark:focus:outline-blue-400"
        onFocusIn={(event) => {
          if (event.target instanceof HTMLElement) {
            const action = event.target.dataset.orgSharingAction;
            const ids = controls(action).map((control) => control.dataset.orgSharingTarget!);
            const index = ids.indexOf(event.target.dataset.orgSharingTarget!);
            lastFocused = {
              element: event.target,
              scope: focusScope(),
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
        <SettingsPanel
          title="Organization Sharing"
          action={
            state.sharingChangesPaused() ? (
              <Button onClick={reloadSharing}>{ORGANIZATION_SHARING_RELOAD_LABEL}</Button>
            ) : undefined
          }
          noPadding
        >
          <Show when={state.loading()}>
            <div class="px-4 pt-4 sm:px-6 sm:pt-6">
              <p id={checkingID} class="text-sm text-muted">
                Checking saved shares for{' '}
                <bdi class="font-medium text-base-content [overflow-wrap:anywhere]">
                  {`${state.selectedOrgID()}.`}
                </bdi>{' '}
                Restarting does not repeat a sharing change.
              </p>
              <Button
                aria-describedby={checkingID}
                onClick={() => {
                  if (state.restartSharingRead()) panel?.focus({ preventScroll: true });
                }}
                class="mt-2 min-h-11 w-full sm:min-h-0 sm:w-auto"
              >
                Restart sharing check
              </Button>
            </div>
          </Show>
          <div
            role="group"
            aria-label="Saved shares"
            aria-busy={state.loading()}
            class="divide-y divide-border"
          >
            <Show when={!state.loading()} fallback={<OrganizationSharingLoadingState />}>
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
                        {ORGANIZATION_SHARING_UNAVAILABLE_TITLE}
                      </p>
                      <p>{state.loadError()}</p>
                      <p>{getOrganizationSharingReloadMessage(state.selectedOrgID())}</p>
                      <Button
                        onClick={reloadSharing}
                        disabled={state.loading() || state.saving()}
                        class="w-full sm:w-auto"
                      >
                        {ORGANIZATION_SHARING_RELOAD_LABEL}
                      </Button>
                    </div>
                  </Card>
                </div>
              </Show>
              <Show when={!state.loadError() && state.org()}>
                {(currentOrg) => (
                  <>
                    <div class="p-4 sm:px-6 space-y-2 text-sm text-muted break-words">
                      <p
                        ref={summary}
                        tabIndex={-1}
                        class="focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                      >
                        Sharing for{' '}
                        <span class="font-medium text-base-content" data-sharing-organization>
                          {currentOrg().displayName || currentOrg().id}
                        </span>{' '}
                        ({currentOrg().id})
                      </p>
                      <Show when={state.readOnly()}>
                        <p>{ORGANIZATION_SHARING_READ_ONLY_MESSAGE}</p>
                      </Show>
                      <Show when={state.sharingChangesPaused()}>
                        <p role="status">
                          {getOrganizationSharingPartialReadMessage(
                            state.canManageCurrentOrg(),
                            state.unavailableLists()?.outgoing,
                          )}
                        </p>
                      </Show>
                    </div>
                    <Show
                      when={state.canManageCurrentOrg()}
                      fallback={
                        <Show when={!state.readOnly()}>
                          <div class="p-4 sm:p-6">
                            <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/25 dark:text-amber-300">
                              {ORGANIZATION_SHARING_MANAGER_REQUIRED_MESSAGE}
                            </div>
                          </div>
                        </Show>
                      }
                    >
                      <Show when={!state.unavailableLists()?.outgoing}>
                        <OrganizationSharingCreateSection state={state} />
                      </Show>
                    </Show>
                    <OrganizationOutgoingSharesSection state={state} />
                    <OrganizationIncomingSharesSection state={state} />
                  </>
                )}
              </Show>
            </Show>
          </div>
        </SettingsPanel>
      </div>
    </Show>
  );
};

export default OrganizationSharingPanel;
