import { Component, Show } from 'solid-js';
import { formField, formHelpText, labelClass, controlClass } from '@/components/shared/Form';
import { SELF_HOSTED_RECOVERY_PRESENTATION } from '@/utils/licensePresentation';
import { TERMS_DOC_URL } from '@/utils/docsLinks';
import { CommercialSection } from './CommercialBillingSections';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';

interface SelfHostedCommercialRecoverySectionProps {
  licenseKey: string;
  activating: boolean;
  clearing: boolean;
  loading: boolean;
  hasLicenseDetails: boolean;
  looksLikeLegacyLicenseKey: boolean;
  onLicenseKeyInput: (value: string) => void;
  onActivate: () => void;
  onClear: () => void;
}

export const SelfHostedCommercialRecoverySection: Component<
  SelfHostedCommercialRecoverySectionProps
> = (props) => (
  <CommercialSection
    title={SELF_HOSTED_PRO_BILLING_PRESENTATION.recoverySectionTitle}
    description={SELF_HOSTED_PRO_BILLING_PRESENTATION.recoverySectionDescription}
  >
    <details class="group rounded-md border border-border bg-surface-alt p-4">
      <summary class="cursor-pointer list-none">
        <div class="flex items-start justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-base-content">
              {SELF_HOSTED_RECOVERY_PRESENTATION.disclosureLabel}
            </p>
            <p class="mt-1 text-xs text-muted">
              {SELF_HOSTED_RECOVERY_PRESENTATION.disclosureDescription}
            </p>
          </div>
          <span class="text-xs font-medium text-primary group-open:hidden">Show</span>
          <span class="hidden text-xs font-medium text-primary group-open:inline">Hide</span>
        </div>
      </summary>

      <div class="mt-4 space-y-4">
        <div class={formField}>
          <label class={labelClass()} for="pulse-pro-license-key">
            {SELF_HOSTED_RECOVERY_PRESENTATION.fieldLabel}
          </label>
          <textarea
            id="pulse-pro-license-key"
            class={controlClass('min-h-[120px] font-mono')}
            placeholder={SELF_HOSTED_RECOVERY_PRESENTATION.fieldPlaceholder}
            value={props.licenseKey}
            onInput={(event) => props.onLicenseKeyInput(event.currentTarget.value)}
          />
          <p class={formHelpText}>
            {SELF_HOSTED_RECOVERY_PRESENTATION.helpTextBeforeTerms}{' '}
            <a
              href={TERMS_DOC_URL}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 text-blue-600 hover:underline"
            >
              {SELF_HOSTED_RECOVERY_PRESENTATION.termsLabel}
            </a>
            {SELF_HOSTED_RECOVERY_PRESENTATION.helpTextAfterTerms}
          </p>
          <Show when={props.looksLikeLegacyLicenseKey}>
            <div class="mt-3 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
              <p class="font-medium">{SELF_HOSTED_RECOVERY_PRESENTATION.legacyNotice.title}</p>
              <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                {SELF_HOSTED_RECOVERY_PRESENTATION.legacyNotice.body}
              </p>
            </div>
          </Show>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <button
            class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={props.onActivate}
            disabled={props.activating || !props.licenseKey.trim()}
          >
            {props.activating
              ? SELF_HOSTED_RECOVERY_PRESENTATION.activatePendingLabel
              : SELF_HOSTED_RECOVERY_PRESENTATION.activateIdleLabel}
          </button>
          <button
            class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={props.onClear}
            disabled={props.clearing || props.loading || !props.hasLicenseDetails}
          >
            {props.clearing
              ? SELF_HOSTED_RECOVERY_PRESENTATION.clearPendingLabel
              : SELF_HOSTED_RECOVERY_PRESENTATION.clearIdleLabel}
          </button>
        </div>
      </div>
    </details>
  </CommercialSection>
);
