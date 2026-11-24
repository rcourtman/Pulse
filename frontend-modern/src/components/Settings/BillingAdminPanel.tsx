import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS as ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE as ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import { BillingAdminOrganizationsTable } from './BillingAdminOrganizationsTable';
import { useBillingAdminPanelState } from './useBillingAdminPanelState';

export const BillingAdminPanel: Component = () => {
  const state = useBillingAdminPanelState();

  return (
    <Show
      when={state.hostedEnabled()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <>
        <SettingsPanel
          title="Billing Admin"
          action={
            <Button
              variant="secondary"
              size="sm"
              class="w-full sm:w-auto"
              onClick={() => {
                void state.refreshOrganizations();
              }}
              disabled={state.loadingOrgs()}
            >
              Refresh
            </Button>
          }
          bodyClass="space-y-4"
        >
          <Show when={state.orgsError()}>
            <div
              role="alert"
              class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900/25 dark:bg-red-900/25 dark:text-red-200"
            >
              <span>{state.orgsError()}</span>
              <Button
                variant="secondary"
                size="xs"
                onClick={() => {
                  void state.refreshOrganizations();
                }}
                disabled={state.loadingOrgs()}
              >
                Retry
              </Button>
            </div>
          </Show>

          <Show when={!state.orgsError()}>
            <div class="mt-4" aria-busy={state.loadingOrgs()}>
              <BillingAdminOrganizationsTable
                orgs={state.orgs()}
                isLoading={state.loadingOrgs()}
                billingByOrgID={state.billingByOrgID()}
                billingLoadingByOrgID={state.billingLoadingByOrgID()}
                billingErrorByOrgID={state.billingErrorByOrgID()}
                canManageBilling={state.canManageBilling()}
                savingByOrgID={state.savingByOrgID()}
                expandedOrgID={state.expandedOrgID()}
                onToggleOrganization={(orgID) => {
                  void state.toggleExpandedOrganization(orgID);
                }}
                onSuspendOrganization={(orgID) => {
                  state.requestSuspendOrganization(orgID);
                }}
                onActivateOrganization={(orgID) => {
                  void state.updateSubscriptionState(orgID, 'active');
                }}
                onReloadOrganization={(orgID) => {
                  void state.reloadOrganization(orgID);
                }}
              />
            </div>
          </Show>
        </SettingsPanel>

        <Show keyed when={state.pendingSuspension()}>
          {(confirmation) => {
            const label =
              confirmation.displayName && confirmation.displayName !== confirmation.orgID
                ? `${confirmation.displayName} (${confirmation.orgID})`
                : confirmation.orgID;
            return (
              <Dialog
                isOpen={true}
                ariaLabel="Suspend organization"
                onClose={() => state.cancelSuspendOrganization(confirmation)}
                panelClass="max-w-lg"
              >
                <Card padding="lg" class="min-h-0 overflow-y-auto space-y-4">
                  <h3 class="text-lg font-semibold text-base-content">Suspend organization?</h3>
                  <p class="text-sm text-base-content break-words">
                    Suspend billing access for <strong>{label}</strong>?
                  </p>
                  <p class="text-sm text-muted">
                    Members of this organization will lose active subscription access until it is
                    reactivated.
                  </p>
                  <div class="flex flex-col gap-2 sm:flex-row sm:justify-end">
                    <Button onClick={() => state.cancelSuspendOrganization(confirmation)}>
                      Cancel
                    </Button>
                    <Button
                      variant="danger"
                      onClick={() => void state.confirmSuspendOrganization(confirmation)}
                    >
                      Suspend organization
                    </Button>
                  </div>
                </Card>
              </Dialog>
            );
          }}
        </Show>
      </>
    </Show>
  );
};

export default BillingAdminPanel;
