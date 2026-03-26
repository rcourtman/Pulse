import { Component, Show } from 'solid-js';
import { formField, formHelpText, labelClass, controlClass } from '@/components/shared/Form';
import { SELF_HOSTED_ACTIVATION_PRESENTATION } from '@/utils/licensePresentation';
import { CommercialSection } from './CommercialBillingSections';

interface SelfHostedCommercialActivationSectionProps {
  licenseKey: string;
  activating: boolean;
  clearing: boolean;
  loading: boolean;
  hasLicenseDetails: boolean;
  showTrialStart: boolean;
  startingTrial: boolean;
  looksLikeLegacyLicenseKey: boolean;
  onLicenseKeyInput: (value: string) => void;
  onActivate: () => void;
  onClear: () => void;
  onStartTrial: () => void;
}

export const SelfHostedCommercialActivationSection: Component<
  SelfHostedCommercialActivationSectionProps
> = (props) => (
  <CommercialSection
    title={SELF_HOSTED_ACTIVATION_PRESENTATION.sectionTitle}
    description={SELF_HOSTED_ACTIVATION_PRESENTATION.sectionDescription}
  >
    <div class={formField}>
      <label class={labelClass()} for="pulse-pro-license-key">
        {SELF_HOSTED_ACTIVATION_PRESENTATION.fieldLabel}
      </label>
      <textarea
        id="pulse-pro-license-key"
        class={controlClass('min-h-[120px] font-mono')}
        placeholder={SELF_HOSTED_ACTIVATION_PRESENTATION.fieldPlaceholder}
        value={props.licenseKey}
        onInput={(event) => props.onLicenseKeyInput(event.currentTarget.value)}
      />
      <p class={formHelpText}>
        {SELF_HOSTED_ACTIVATION_PRESENTATION.helpTextBeforeTerms}{' '}
        <a
          href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 text-blue-600 hover:underline"
        >
          {SELF_HOSTED_ACTIVATION_PRESENTATION.termsLabel}
        </a>
        {SELF_HOSTED_ACTIVATION_PRESENTATION.helpTextAfterTerms}
      </p>
      <Show when={props.looksLikeLegacyLicenseKey}>
        <div class="mt-3 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
          <p class="font-medium">{SELF_HOSTED_ACTIVATION_PRESENTATION.legacyNotice.title}</p>
          <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
            {SELF_HOSTED_ACTIVATION_PRESENTATION.legacyNotice.body}
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
          ? SELF_HOSTED_ACTIVATION_PRESENTATION.activatePendingLabel
          : SELF_HOSTED_ACTIVATION_PRESENTATION.activateIdleLabel}
      </button>
      <button
        class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
        onClick={props.onClear}
        disabled={props.clearing || props.loading || !props.hasLicenseDetails}
      >
        {props.clearing
          ? SELF_HOSTED_ACTIVATION_PRESENTATION.clearPendingLabel
          : SELF_HOSTED_ACTIVATION_PRESENTATION.clearIdleLabel}
      </button>
    </div>

    <Show when={props.showTrialStart}>
      <div class="rounded-md border border-border bg-surface-alt p-3">
        <p class="text-sm font-medium text-base-content">
          {SELF_HOSTED_ACTIVATION_PRESENTATION.trial.title}
        </p>
        <p class="text-xs text-muted mt-1">{SELF_HOSTED_ACTIVATION_PRESENTATION.trial.body}</p>
        <button
          type="button"
          class="mt-3 inline-flex min-h-10 sm:min-h-9 items-center justify-center px-4 py-2.5 text-sm font-medium rounded-md bg-emerald-600 text-white hover:bg-emerald-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
          disabled={props.startingTrial}
          onClick={props.onStartTrial}
        >
          {props.startingTrial
            ? SELF_HOSTED_ACTIVATION_PRESENTATION.trial.pendingActionLabel
            : SELF_HOSTED_ACTIVATION_PRESENTATION.trial.idleActionLabel}
        </button>
      </div>
    </Show>
  </CommercialSection>
);
