import { Show, type Component } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import { MonitoredSystemLedgerPanel } from './MonitoredSystemLedgerPanel';
import { CommercialBillingShell, CommercialSection } from './CommercialBillingSections';
import { ProLicensePlanSection } from './ProLicensePlanSection';
import { SelfHostedCommercialRecoverySection } from './SelfHostedCommercialRecoverySection';
import { useProLicensePanelState } from './useProLicensePanelState';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import { Subtabs } from '@/components/shared/Subtabs';
import { getMonitoredSystemBriefSummary } from '@/utils/monitoredSystemPresentation';
import {
  presentationPolicyHidesCommercialSurfaces,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import {
  SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID,
} from '@/utils/pricingHandoff';

const ProLicensePolicyLoadingPanel: Component = () => (
  <CommercialBillingShell
    title={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellTitle}
    description={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellDescription}
    icon={<ShieldCheck class="w-5 h-5" />}
    loading={false}
  >
    <div class="rounded-lg border border-border bg-surface px-4 py-4 text-sm">
      <p class="font-semibold text-base-content">
        {SELF_HOSTED_PRO_BILLING_PRESENTATION.policyLoadingTitle}
      </p>
      <p class="mt-1 text-muted">{SELF_HOSTED_PRO_BILLING_PRESENTATION.policyLoadingBody}</p>
    </div>
  </CommercialBillingShell>
);

const ProLicenseHiddenPanel: Component = () => (
  <CommercialBillingShell
    title={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellTitle}
    description={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellDescription}
    icon={<ShieldCheck class="w-5 h-5" />}
    loading={false}
  >
    <div class="rounded-lg border border-border bg-surface px-4 py-4 text-sm">
      <p class="font-semibold text-base-content">
        {SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenStateTitle}
      </p>
      <p class="mt-1 text-muted">{SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenStateBody}</p>
    </div>
  </CommercialBillingShell>
);

const ProLicensePanelContent: Component = () => {
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
          <Subtabs
            value={state.activeSection()}
            onChange={state.setActiveSection}
            ariaLabel={SELF_HOSTED_PRO_BILLING_PRESENTATION.sectionSelectorAriaLabel}
            tabs={[
              {
                value: 'plan',
                label: SELF_HOSTED_PRO_BILLING_PRESENTATION.planTabLabel,
              },
              {
                value: 'usage',
                label: SELF_HOSTED_PRO_BILLING_PRESENTATION.usageTabLabel,
              },
            ]}
          />

          <Show when={state.activeSection() === 'plan'}>
            <CommercialSection
              id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}
              title={SELF_HOSTED_PRO_BILLING_PRESENTATION.planSectionTitle}
              description={SELF_HOSTED_PRO_BILLING_PRESENTATION.planSectionDescription}
            >
              <div class="space-y-6">
                <ProLicensePlanSection
                  commercialMigrationNotice={state.commercialMigrationNotice()}
                  commercialPlanModel={state.commercialPlanModel()}
                  entitlements={state.entitlements()}
                  formattedFeatures={state.formattedFeatures()}
                  grandfatheredPriceNotice={state.grandfatheredPriceNotice()}
                  hasLicenseDetails={state.hasLicenseDetails()}
                  hasPaidFeatures={state.hasPaidFeatures()}
                  loading={state.loading()}
                  monitoredSystemContinuityNotice={state.monitoredSystemContinuityNotice()}
                  onReload={() => void state.loadPanelData()}
                  purchaseActivationAction={state.purchaseActivationAction()}
                  purchaseActivationNotice={state.purchaseActivationNotice()}
                  showMonitoredSystemUpgradeArrival={state.showMonitoredSystemUpgradeArrival()}
                  showTrialStart={state.showTrialStart()}
                  startingTrial={state.startingTrial()}
                  statusPresentation={state.statusPresentation()}
                  onStartTrial={() => void state.handleStartTrial()}
                  trialActivationNotice={state.trialActivationNotice()}
                  trialEnded={state.trialEnded()}
                />

                <SelfHostedCommercialRecoverySection
                  sectionId={SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID}
                  open={state.showRecoveryByDefault()}
                  licenseKey={state.licenseKey()}
                  activating={state.activating()}
                  clearing={state.clearing()}
                  loading={state.loading()}
                  hasLicenseDetails={state.hasLicenseDetails()}
                  looksLikeLegacyLicenseKey={state.looksLikeLegacyLicenseKey()}
                  onLicenseKeyInput={state.setLicenseKey}
                  onActivate={state.handleActivate}
                  onClear={state.handleClear}
                />
              </div>
            </CommercialSection>
          </Show>

          <Show when={state.activeSection() === 'usage'}>
            <CommercialSection
              id={SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}
              title={SELF_HOSTED_PRO_BILLING_PRESENTATION.usageSectionTitle}
              description={getMonitoredSystemBriefSummary()}
            >
              <MonitoredSystemLedgerPanel
                embedded
                monitoredSystemContinuity={state.entitlements()?.monitored_system_continuity}
                monitoredSystemLimit={
                  state
                    .entitlements()
                    ?.limits?.find((entry) => entry.key === 'max_monitored_systems') ?? null
                }
                showCountingRulesByDefault={state.showCountingRulesByDefault()}
              />
            </CommercialSection>
          </Show>
        </div>
      </CommercialBillingShell>
    </div>
  );
};

export const ProLicensePanel: Component = () => (
  <div class="space-y-6">
    <Show when={sessionPresentationPolicyResolved()} fallback={<ProLicensePolicyLoadingPanel />}>
      <Show
        when={!presentationPolicyHidesCommercialSurfaces()}
        fallback={<ProLicenseHiddenPanel />}
      >
        <ProLicensePanelContent />
      </Show>
    </Show>
  </div>
);

export default ProLicensePanel;
