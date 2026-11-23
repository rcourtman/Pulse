import { Component, Show, createEffect, createUniqueId, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
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
  let focusTimer: number | undefined;
  const controls = () =>
    Array.from(panel?.querySelectorAll<HTMLElement>('[data-org-sharing-action]') ?? []);
  const reloadSharing = () => {
    // The explicit read replaces its button. Keep a stable keyboard position
    // through loading instead of losing focus and moving it after the response.
    // Safari pointer activation need not focus the button first.
    panel?.focus();
    void state.loadSharingData();
  };

  createEffect(() => {
    window.clearTimeout(focusTimer);
    const orgID = state.selectedOrgID();
    const user = props.currentUser;
    const enabled = showOrganizationSurface();
    const context: typeof focusContext = [orgID, user, enabled, state.readOnly()];
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
      // A reread replaces controls. Recover only the focus it removed, never
      // an untouched initial read or a deliberate move to another control.
      if (lostPanelFocus && (!document.activeElement || document.activeElement === document.body)) {
        const candidates = controls().filter(
          (control) =>
            control.dataset.orgSharingAction === lastFocused?.action &&
            !control.matches(':disabled'),
        );
        // Opaque keys are values, not CSS selectors or current row indices.
        const replacement = lastFocused?.targets
          .map((id) => candidates.find((control) => control.dataset.orgSharingTarget === id))
          .find(Boolean);
        const target = error ? warning : (replacement ?? summary);
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
        aria-label="Organization sharing"
        tabIndex={-1}
        class="space-y-6 rounded-lg focus:outline-2 focus:-outline-offset-2 focus:outline-blue-500 dark:focus:outline-blue-400"
        onFocusIn={(event) => {
          if (event.target instanceof HTMLElement) {
            const action = event.target.dataset.orgSharingAction;
            const ids = controls()
              .filter((control) => control.dataset.orgSharingAction === action)
              .map((control) => control.dataset.orgSharingTarget!);
            const index = ids.indexOf(event.target.dataset.orgSharingTarget!);
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
            <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-border/60 bg-surface-alt/50 px-4 py-2.5 sm:px-6">
              <p id={checkingID} class="min-w-0 max-w-full text-xs text-muted">
                Checking saved shares for{' '}
                <bdi class="font-medium text-base-content [overflow-wrap:anywhere]">
                  {`${state.selectedOrgID()}.`}
                </bdi>{' '}
                Restarting does not repeat a sharing change.
              </p>
              <Button
                variant="ghost"
                size="xs"
                aria-describedby={checkingID}
                onClick={() => {
                  if (state.restartSharingRead()) panel?.focus({ preventScroll: true });
                }}
                class="w-full sm:w-auto"
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
                      <Show when={state.mutationError()}>
                        {(message) => (
                          <div
                            role="alert"
                            class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800 dark:border-red-900/40 dark:bg-red-950/40 dark:text-red-200"
                          >
                            {message()}
                          </div>
                        )}
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
        <Show keyed when={state.shareConfirmation()}>
          {(confirmation) => (
            <Dialog
              isOpen={true}
              ariaLabel={confirmation.title}
              onClose={() => state.cancelShareConfirmation(confirmation)}
              panelClass="max-w-lg"
            >
              <Card padding="lg" class="min-h-0 overflow-y-auto space-y-4">
                <h3 class="text-lg font-semibold text-base-content">{confirmation.title}</h3>
                <p class="text-sm text-base-content break-words">{confirmation.message}</p>
                <div class="flex flex-col gap-2 sm:flex-row sm:justify-end">
                  <Button onClick={() => state.cancelShareConfirmation(confirmation)}>
                    Cancel
                  </Button>
                  <Button
                    variant="danger"
                    onClick={() => void state.confirmShareAction(confirmation)}
                  >
                    {confirmation.confirmLabel}
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

export default OrganizationSharingPanel;
