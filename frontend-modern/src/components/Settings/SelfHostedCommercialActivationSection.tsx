import { Component, Show } from 'solid-js';
import { formField, formHelpText, labelClass, controlClass } from '@/components/shared/Form';
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
    title="Activation"
    description="Activate, clear, or start a trial for the self-hosted Pulse Pro entitlement that controls paid features on this instance."
  >
    <div class={formField}>
      <label class={labelClass()} for="pulse-pro-license-key">
        License / Activation Key
      </label>
      <textarea
        id="pulse-pro-license-key"
        class={controlClass('min-h-[120px] font-mono')}
        placeholder="Paste your license key or activation key"
        value={props.licenseKey}
        onInput={(event) => props.onLicenseKeyInput(event.currentTarget.value)}
      />
      <p class={formHelpText}>
        Paste the Pulse v6 activation key shown on the hosted checkout success page. A backup copy
        is also sent by email, but the hosted success page is the primary handoff. You can also
        paste a legacy Pulse v5 Pro/Lifetime license key and Pulse will exchange it automatically
        during activation when migration is available. By activating a license, you agree to the{' '}
        <a
          href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 text-blue-600 hover:underline"
        >
          Terms of Service
        </a>
        .
      </p>
      <Show when={props.looksLikeLegacyLicenseKey}>
        <div class="mt-3 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
          <p class="font-medium">Legacy v5 license detected</p>
          <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
            Pulse will try to exchange this key into the v6 activation model automatically. If the
            exchange cannot complete immediately, retry from this panel or use the self-serve
            retrieval flow to get the current v6 activation key.
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
        {props.activating ? 'Activating...' : 'Activate License'}
      </button>
      <button
        class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
        onClick={props.onClear}
        disabled={props.clearing || props.loading || !props.hasLicenseDetails}
      >
        {props.clearing ? 'Clearing...' : 'Clear License'}
      </button>
    </div>

    <Show when={props.showTrialStart}>
      <div class="rounded-md border border-border bg-surface-alt p-3">
        <p class="text-sm font-medium text-base-content">Try Pro for free</p>
        <p class="text-xs text-muted mt-1">Start a 14-day Pro trial for this organization.</p>
        <button
          type="button"
          class="mt-3 inline-flex min-h-10 sm:min-h-9 items-center justify-center px-4 py-2.5 text-sm font-medium rounded-md bg-emerald-600 text-white hover:bg-emerald-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
          disabled={props.startingTrial}
          onClick={props.onStartTrial}
        >
          {props.startingTrial ? 'Starting...' : 'Start 14-day Pro Trial'}
        </button>
      </div>
    </Show>
  </CommercialSection>
);
