import { Component, Show } from 'solid-js';
import { formField, controlClass } from '@/components/shared/Form';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { SELF_HOSTED_RECOVERY_PRESENTATION } from '@/utils/licensePresentation';
import { TERMS_DOC_URL } from '@/utils/docsLinks';
import { CommercialSection } from './CommercialBillingSections';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';

interface SelfHostedCommercialRecoverySectionProps {
  sectionId?: string;
  open?: boolean;
  licenseKey: string;
  activating: boolean;
  clearing: boolean;
  loading: boolean;
  hasLicenseDetails: boolean;
  looksLikeLegacyLicenseKey: boolean;
  feedback?: { type: 'success' | 'error'; message: string } | null;
  onLicenseKeyInput: (value: string) => void;
  onActivate: () => void;
  onClear: () => void;
}

export const SelfHostedCommercialRecoverySection: Component<
  SelfHostedCommercialRecoverySectionProps
> = (props) => (
  <CommercialSection
    id={props.sectionId}
    title={SELF_HOSTED_PRO_BILLING_PRESENTATION.recoverySectionTitle}
    description={SELF_HOSTED_PRO_BILLING_PRESENTATION.recoverySectionDescription}
  >
    <details class="group rounded-md border border-border bg-surface-alt p-4" open={props.open}>
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
          <FormTextarea
            id="pulse-pro-license-key"
            label={SELF_HOSTED_RECOVERY_PRESENTATION.fieldLabel}
            fieldBaseClass="contents"
            textareaBaseClass={controlClass('min-h-[120px] font-mono')}
            placeholder={SELF_HOSTED_RECOVERY_PRESENTATION.fieldPlaceholder}
            value={props.licenseKey}
            disabled={props.activating || props.clearing}
            onInput={(event) => props.onLicenseKeyInput(event.currentTarget.value)}
            help={
              <>
                {SELF_HOSTED_RECOVERY_PRESENTATION.helpTextBeforeTerms}{' '}
                <ExternalTextLink href={TERMS_DOC_URL} variant="compact">
                  {SELF_HOSTED_RECOVERY_PRESENTATION.termsLabel}
                </ExternalTextLink>
                {SELF_HOSTED_RECOVERY_PRESENTATION.helpTextAfterTerms}
              </>
            }
          />
          <div class="mt-3 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/25 p-3 text-sm text-amber-800 dark:text-amber-200">
            <p class="font-medium">
              {SELF_HOSTED_RECOVERY_PRESENTATION.privateRuntimeNotice.title}
            </p>
            <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">
              {SELF_HOSTED_RECOVERY_PRESENTATION.privateRuntimeNotice.body}
            </p>
            <ExternalTextLink
              href={SELF_HOSTED_RECOVERY_PRESENTATION.privateRuntimeNotice.actionUrl}
              variant="compactAction"
              class="mt-2"
            >
              {SELF_HOSTED_RECOVERY_PRESENTATION.privateRuntimeNotice.actionLabel}
            </ExternalTextLink>
          </div>
          <Show when={props.looksLikeLegacyLicenseKey}>
            <div class="mt-3 rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/25 p-3 text-sm text-amber-800 dark:text-amber-200">
              <p class="font-medium">{SELF_HOSTED_RECOVERY_PRESENTATION.legacyNotice.title}</p>
              <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                {SELF_HOSTED_RECOVERY_PRESENTATION.legacyNotice.body}
              </p>
            </div>
          </Show>
        </div>

        <Show when={props.feedback}>
          {(feedback) => (
            <InlineNotice
              tone={feedback().type === 'error' ? 'danger' : 'success'}
              role={feedback().type === 'error' ? 'alert' : 'status'}
              aria-live={feedback().type === 'error' ? 'assertive' : 'polite'}
              aria-atomic="true"
              class="break-words [overflow-wrap:anywhere]"
            >
              {feedback().message}
            </InlineNotice>
          )}
        </Show>

        <div class="flex flex-wrap items-center gap-2">
          <Button
            variant="primary"
            size="settingsAction"
            onClick={props.onActivate}
            disabled={props.activating || props.clearing || !props.licenseKey.trim()}
          >
            {props.activating
              ? SELF_HOSTED_RECOVERY_PRESENTATION.activatePendingLabel
              : SELF_HOSTED_RECOVERY_PRESENTATION.activateIdleLabel}
          </Button>
          <Button
            variant="outline"
            size="settingsAction"
            onClick={props.onClear}
            disabled={
              props.activating || props.clearing || props.loading || !props.hasLicenseDetails
            }
          >
            {props.clearing
              ? SELF_HOSTED_RECOVERY_PRESENTATION.clearPendingLabel
              : SELF_HOSTED_RECOVERY_PRESENTATION.clearIdleLabel}
          </Button>
        </div>
      </div>
    </details>
  </CommercialSection>
);
