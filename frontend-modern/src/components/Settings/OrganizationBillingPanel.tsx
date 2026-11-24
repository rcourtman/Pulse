import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createUniqueId,
  onCleanup,
} from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import {
  CommercialBillingShell,
  CommercialSection,
  CommercialStatGrid,
  CommercialUsageMeters,
} from './CommercialBillingSections';
import { OrganizationBillingLoadingState } from './OrganizationBillingLoadingState';
import {
  useOrganizationBillingPanelState,
  type OrganizationBillingPanelProps,
} from './useOrganizationBillingPanelState';

export const OrganizationBillingPanel: Component<OrganizationBillingPanelProps> = (props) => {
  const {
    commercialPlanModel,
    commercialUsageModel,
    isBillingAvailable,
    loading,
    loadError,
    reloadBillingData,
    selectedOrgID,
    organizationCountUnavailable,
  } = useOrganizationBillingPanelState(props);
  const showHeaderAction = createMemo(() => loading() || organizationCountUnavailable());
  const warningTitleID = createUniqueId();
  const checkingID = createUniqueId();
  let panel: HTMLDivElement | undefined;
  let warning: HTMLDivElement | undefined;
  let headerAction: HTMLButtonElement | undefined;
  let reloadOrgID: string | undefined;
  let focusOrgID: string | undefined;
  let focusTimer: number | undefined;

  const reloadBilling = () => {
    // Record the explicit action even when pointer activation does not focus
    // the button. Background reads must not acquire this keyboard intent.
    reloadOrgID = selectedOrgID();
    if (!reloadBillingData()) reloadOrgID = undefined;
  };

  createEffect(() => {
    window.clearTimeout(focusTimer);
    const orgID = selectedOrgID();
    const available = isBillingAvailable();
    const pending = loading();
    const error = loadError();
    if (!available || (focusOrgID !== undefined && focusOrgID !== orgID)) {
      reloadOrgID = undefined;
    }
    focusOrgID = orgID;
    if (!available || reloadOrgID !== orgID) return;
    if (!pending) reloadOrgID = undefined;
    focusTimer = window.setTimeout(() => {
      if (
        !panel?.isConnected ||
        !isBillingAvailable() ||
        loading() !== pending ||
        selectedOrgID() !== orgID ||
        loadError() !== error
      )
        return;
      // Loading replaces the retry control. Recover only its lost focus,
      // never a deliberate move elsewhere while the read was pending.
      if (!document.activeElement || document.activeElement === document.body) {
        const target = pending ? headerAction : error ? warning : panel;
        target?.focus();
      }
    }, 0);
  });
  onCleanup(() => window.clearTimeout(focusTimer));

  return (
    <Show
      when={isBillingAvailable()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div
        ref={panel}
        role="region"
        aria-label="Billing & Usage"
        tabIndex={-1}
        class="space-y-6 rounded-md focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      >
        <CommercialBillingShell
          title="Billing & Usage"
          action={
            showHeaderAction() ? (
              <Button
                ref={headerAction}
                variant={loading() ? 'ghost' : 'secondary'}
                size={loading() ? 'xs' : 'sm'}
                onClick={reloadBilling}
                aria-describedby={loading() ? checkingID : undefined}
                class="w-full sm:w-auto"
              >
                {loading() ? 'Restart billing check' : 'Reload billing'}
              </Button>
            ) : undefined
          }
          loading={loading()}
          loadingFallback={
            <OrganizationBillingLoadingState orgID={selectedOrgID()} descriptionID={checkingID} />
          }
        >
          <Show when={loadError()}>
            <Card tone="warning" padding="md">
              <div
                ref={warning}
                role="alert"
                aria-labelledby={warningTitleID}
                tabIndex={-1}
                class="space-y-3 text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <p id={warningTitleID} class="font-semibold">
                  Organization billing unavailable
                </p>
                <p>{loadError()}</p>
                <p class="break-words">
                  Reload checks billing and plan details for {selectedOrgID()}.
                </p>
                <Button onClick={reloadBilling} class="w-full sm:w-auto">
                  Reload billing
                </Button>
              </div>
            </Card>
          </Show>
          <Show when={!loadError()}>
            <CommercialSection
              title="Plan"
              description="Review the active organization plan, subscription state, and tenant footprint tied to this billing record."
            >
              <CommercialStatGrid columns="four" items={commercialPlanModel().summary} />

              <div class="grid gap-3 sm:grid-cols-2">
                <For each={commercialPlanModel().details}>
                  {(item) => (
                    <div class="rounded-md border border-border p-3">
                      <p class="text-xs uppercase tracking-wide text-muted">{item.label}</p>
                      <p class="mt-1 text-sm font-medium text-base-content">{item.value}</p>
                    </div>
                  )}
                </For>
              </div>
            </CommercialSection>

            <CommercialSection
              title="Usage"
              description="Review current organization usage for billing and support context."
            >
              <CommercialUsageMeters title="Usage" items={commercialUsageModel().meters} />
            </CommercialSection>
          </Show>
        </CommercialBillingShell>
      </div>
    </Show>
  );
};

export default OrganizationBillingPanel;
