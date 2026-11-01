import { Component, For, Show } from 'solid-js';
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
  } = useOrganizationBillingPanelState(props);

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
          loading={loading()}
          loadingFallback={<OrganizationBillingLoadingState />}
        >
          <Show when={loadError()}>
            <Card tone="warning" padding="md">
              <div role="alert" class="space-y-3 text-sm">
                <p class="font-semibold">Organization billing unavailable</p>
                <p>{loadError()}</p>
                <p class="break-words">
                  Reload checks billing and plan details for {selectedOrgID()}.
                </p>
                <Button onClick={reloadBillingData} class="w-full sm:w-auto">
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
