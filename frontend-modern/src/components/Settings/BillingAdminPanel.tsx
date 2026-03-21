import { Component, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS as ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE as ORGANIZATION_SETTINGS_PANEL_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import CreditCard from 'lucide-solid/icons/credit-card';
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
      <SettingsPanel
        title="Billing Admin"
        description="View and manage billing state across all tenants (hosted mode only)."
        icon={<CreditCard class="w-5 h-5" />}
        action={
          <button
            type="button"
            onClick={() => {
              void state.refreshOrganizations();
            }}
            disabled={state.loadingOrgs()}
            class="w-full sm:w-auto px-3 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50"
          >
            Refresh
          </button>
        }
        bodyClass="space-y-4"
      >
        <Show when={state.orgsError()}>
          <div class="rounded-md border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 p-3 text-sm text-red-800 dark:text-red-200">
            {state.orgsError()}
          </div>
        </Show>

        <div class="mt-4">
          <BillingAdminOrganizationsTable
            orgs={state.orgs()}
            isLoading={state.loadingOrgs()}
            billingByOrgID={state.billingByOrgID()}
            billingLoadingByOrgID={state.billingLoadingByOrgID()}
            savingByOrgID={state.savingByOrgID()}
            expandedOrgID={state.expandedOrgID()}
            onToggleOrganization={(orgID) => {
              void state.toggleExpandedOrganization(orgID);
            }}
            onSuspendOrganization={(orgID) => {
              void state.updateSubscriptionState(orgID, 'suspended');
            }}
            onActivateOrganization={(orgID) => {
              void state.updateSubscriptionState(orgID, 'active');
            }}
            onReloadOrganization={(orgID) => {
              void state.reloadOrganization(orgID);
            }}
          />
        </div>
      </SettingsPanel>
    </Show>
  );
};

export default BillingAdminPanel;
