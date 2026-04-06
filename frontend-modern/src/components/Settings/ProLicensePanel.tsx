import type { Component } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import { MonitoredSystemLedgerPanel } from './MonitoredSystemLedgerPanel';
import { CommercialBillingShell, CommercialSection } from './CommercialBillingSections';
import { ProLicensePlanSection } from './ProLicensePlanSection';
import { SelfHostedCommercialActivationSection } from './SelfHostedCommercialActivationSection';
import { useProLicensePanelState } from './useProLicensePanelState';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import { getMonitoredSystemBriefSummary } from '@/utils/monitoredSystemPresentation';
import {
  SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID,
} from '@/utils/pricingHandoff';

export const ProLicensePanel: Component = () => {
  const state = useProLicensePanelState();

  return (
    <div class="space-y-6">
      <CommercialBillingShell
        title={SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle}
        description={SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription}
        icon={<ShieldCheck class="w-5 h-5" />}
        action={
          <button
            class="inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60"
            disabled={state.loading()}
            onClick={state.loadPanelData}
          >
            <RefreshCw class={`w-3.5 h-3.5 ${state.loading() ? 'animate-spin' : ''}`} />
            {SELF_HOSTED_PRO_BILLING_PRESENTATION.refreshLabel}
          </button>
        }
        loading={false}
      >
        <div class="space-y-6">
          <CommercialSection
            id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}
            title={SELF_HOSTED_PRO_BILLING_PRESENTATION.planSectionTitle}
            description={SELF_HOSTED_PRO_BILLING_PRESENTATION.planSectionDescription}
          >
            <ProLicensePlanSection
              commercialMigrationNotice={state.commercialMigrationNotice()}
              commercialPlanModel={state.commercialPlanModel()}
              entitlements={state.entitlements()}
              formattedFeatures={state.formattedFeatures()}
              grandfatheredPriceNotice={state.grandfatheredPriceNotice()}
              hasLicenseDetails={state.hasLicenseDetails()}
              hasPaidFeatures={state.hasPaidFeatures()}
              loading={state.loading()}
              onReload={() => void state.loadPanelData()}
              statusPresentation={state.statusPresentation()}
              trialActivationNotice={state.trialActivationNotice()}
              trialEnded={state.trialEnded()}
            />
          </CommercialSection>

          <CommercialSection
            id={SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}
            title={SELF_HOSTED_PRO_BILLING_PRESENTATION.usageSectionTitle}
            description={getMonitoredSystemBriefSummary()}
          >
            <MonitoredSystemLedgerPanel embedded />
          </CommercialSection>

          <SelfHostedCommercialActivationSection
            licenseKey={state.licenseKey()}
            activating={state.activating()}
            clearing={state.clearing()}
            loading={state.loading()}
            hasLicenseDetails={state.hasLicenseDetails()}
            showTrialStart={state.showTrialStart()}
            startingTrial={state.startingTrial()}
            looksLikeLegacyLicenseKey={state.looksLikeLegacyLicenseKey()}
            onLicenseKeyInput={state.setLicenseKey}
            onActivate={state.handleActivate}
            onClear={state.handleClear}
            onStartTrial={() => void state.handleStartTrial()}
          />
        </div>
      </CommercialBillingShell>
    </div>
  );
};

export default ProLicensePanel;
