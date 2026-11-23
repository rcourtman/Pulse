import { Show, type Component } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import { CommercialBillingShell } from './CommercialBillingSections';
import { ProLicensePlanSection } from './ProLicensePlanSection';
import { SelfHostedCommercialRecoverySection } from './SelfHostedCommercialRecoverySection';
import { useProLicensePanelState } from './useProLicensePanelState';
import {
  PULSE_MSP_REFERRAL_URL,
  SELF_HOSTED_PRO_BILLING_PRESENTATION,
} from './selfHostedBillingPresentation';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { isHostedModeEnabled } from '@/stores/license';
import {
  presentationPolicyHidesCommercialSurfaces,
  sessionPresentationPolicyResolved,
} from '@/stores/sessionPresentationPolicy';
import {
  SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID,
  SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID,
} from '@/utils/pricingHandoff';

const ProLicensePolicyLoadingPanel: Component = () => (
  <CommercialBillingShell
    title={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellTitle}
    description={SELF_HOSTED_PRO_BILLING_PRESENTATION.hiddenShellDescription}
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
        description={SELF_HOSTED_PRO_BILLING_PRESENTATION.planSectionDescription}
        action={
          <Button
            variant="outline"
            size="settingsAction"
            class="gap-2"
            disabled={state.loading()}
            onClick={state.loadPanelData}
          >
            <RefreshCw class={`w-3.5 h-3.5 ${state.loading() ? 'animate-spin' : ''}`} />
            {SELF_HOSTED_PRO_BILLING_PRESENTATION.refreshLabel}
          </Button>
        }
        loading={false}
      >
        <section id={SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID} class="scroll-mt-24 space-y-6">
          <ProLicensePlanSection
            activationSuccessSummary={state.activationSuccessSummary()}
            commercialMigrationNotice={state.commercialMigrationNotice()}
            commercialPlanModel={state.commercialPlanModel()}
            currentPlanSummary={state.currentPlanSummary()}
            entitlements={state.entitlements()}
            grandfatheredPriceNotice={state.grandfatheredPriceNotice()}
            hasLicenseDetails={state.hasLicenseDetails()}
            loading={state.loading()}
            onReload={() => void state.loadPanelData()}
            planComparisonSummary={state.planComparisonSummary()}
            planSelectionPrompt={state.planSelectionPrompt()}
            planStatus={state.planStatus()}
            proRuntimeSwitchState={{
              info: state.proRuntimeSwitch(),
              loading: state.proRuntimeSwitchLoading(),
              loadError: state.proRuntimeSwitchLoadError(),
              switching: state.switchingProRuntime(),
              status: state.proRuntimeSwitchStatus(),
              error: state.proRuntimeSwitchError(),
              onRefresh: () => void state.loadProRuntimeSwitch(),
              onSwitch: () => void state.handleSwitchToProRuntime(),
            }}
            purchaseActivationAction={state.purchaseActivationAction()}
            purchaseActivationNotice={state.purchaseActivationNotice()}
          />

          <SelfHostedCommercialRecoverySection
            sectionId={SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID}
            open={state.showRecoveryByDefault()}
            isProRuntime={state.isProRuntime()}
            licenseKey={state.licenseKey()}
            activating={state.activating()}
            clearing={state.clearing()}
            loading={state.loading()}
            hasLicenseDetails={state.hasLicenseDetails()}
            looksLikeLegacyLicenseKey={state.looksLikeLegacyLicenseKey()}
            feedback={state.recoveryFeedback()}
            onLicenseKeyInput={state.setLicenseKey}
            onActivate={state.handleActivate}
            onClear={state.requestClear}
          />

          {/* Opening Plans & Billing is the opt-in, so this follows the
              panel's commercial gate rather than the upgrade-prompt policy
              that keeps free installs free of nudges elsewhere. Hosted
              runtimes include MSP client workspaces, which already belong
              to a provider. */}
          <Show when={!isHostedModeEnabled()}>
            <div
              class="flex flex-col gap-3 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between"
              data-testid="pulse-msp-referral"
            >
              <p class="text-sm text-muted">
                {SELF_HOSTED_PRO_BILLING_PRESENTATION.mspReferralBody}
              </p>
              <ButtonLink
                href={PULSE_MSP_REFERRAL_URL}
                target="_blank"
                variant="outline"
                size="settingsActionXs"
                class="w-fit shrink-0"
              >
                {SELF_HOSTED_PRO_BILLING_PRESENTATION.mspReferralActionLabel}
              </ButtonLink>
            </div>
          </Show>
        </section>
      </CommercialBillingShell>

      <Dialog
        isOpen={state.showClearConfirm()}
        onClose={() => {
          if (!state.clearing()) state.setShowClearConfirm(false);
        }}
        panelClass="max-w-md"
        closeOnBackdrop={!state.clearing()}
        ariaLabel="Clear Pro license"
      >
        <div class="w-full">
          <div class="border-b border-border px-6 py-4">
            <h3 class="text-base font-semibold text-base-content">
              Clear the current Pro license?
            </h3>
          </div>
          <div class="px-6 py-4 text-sm text-muted">
            This removes the stored activation key from this Pulse instance. Pro-only features will
            be disabled until a valid license is activated again.
          </div>
          <div class="flex items-center justify-end gap-3 border-t border-border bg-surface-alt px-6 py-4">
            <Button
              type="button"
              variant="ghost"
              size="md"
              disabled={state.clearing()}
              onClick={() => state.setShowClearConfirm(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="danger"
              size="md"
              disabled={state.clearing()}
              onClick={() => void state.handleClear()}
            >
              {state.clearing() ? 'Clearing...' : 'Clear license'}
            </Button>
          </div>
        </div>
      </Dialog>
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
