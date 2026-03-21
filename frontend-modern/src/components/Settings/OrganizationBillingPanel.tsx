import { Component, For, Show } from 'solid-js';
import {
  ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS,
  ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE,
} from '@/utils/organizationSettingsPresentation';
import CreditCard from 'lucide-solid/icons/credit-card';
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
  const { commercialPlanModel, commercialUsageModel, isBillingAvailable, loading } =
    useOrganizationBillingPanelState(props);

  return (
    <Show
      when={isBillingAvailable()}
      fallback={
        <div class={ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS}>
          {ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE}
        </div>
      }
    >
      <div class="space-y-6">
        <CommercialBillingShell
          title="Billing & Usage"
          description="Review your organization plan, usage against limits, and available upgrade paths."
          icon={<CreditCard class="w-5 h-5" />}
          loading={loading()}
          loadingFallback={<OrganizationBillingLoadingState />}
        >
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
            description="Compare current agent and guest usage against the active organization allocation."
          >
            <CommercialUsageMeters title="Usage vs Plan Limits" items={commercialUsageModel().meters} />
          </CommercialSection>
        </CommercialBillingShell>
      </div>
    </Show>
  );
};

export default OrganizationBillingPanel;
