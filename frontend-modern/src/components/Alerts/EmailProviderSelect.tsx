import { Show, For } from 'solid-js';
import type { UIEmailConfig } from '@/features/alerts/types';
import {
  ALERT_EMAIL_FROM_ADDRESS_LABEL,
  ALERT_EMAIL_FROM_ADDRESS_PLACEHOLDER,
  ALERT_EMAIL_MANUAL_CONFIGURATION_LABEL,
  ALERT_EMAIL_MAX_RETRIES_LABEL,
  ALERT_EMAIL_PASSWORD_LABEL,
  ALERT_EMAIL_PASSWORD_PLACEHOLDER,
  ALERT_EMAIL_PROVIDER_LABEL,
  ALERT_EMAIL_RATE_LIMIT_LABEL,
  ALERT_EMAIL_RATE_LIMIT_SUFFIX,
  ALERT_EMAIL_REAPPLY_DEFAULTS_LABEL,
  ALERT_EMAIL_RECIPIENTS_LABEL,
  ALERT_EMAIL_REPLY_TO_LABEL,
  ALERT_EMAIL_REPLY_TO_PLACEHOLDER,
  ALERT_EMAIL_RETRY_DELAY_LABEL,
  ALERT_EMAIL_SECURITY_LABEL,
  ALERT_EMAIL_SECURITY_NONE_LABEL,
  ALERT_EMAIL_SECURITY_STARTTLS_LABEL,
  ALERT_EMAIL_SECURITY_TLS_LABEL,
  ALERT_EMAIL_SMTP_PORT_LABEL,
  ALERT_EMAIL_SMTP_PORT_PLACEHOLDER,
  ALERT_EMAIL_SMTP_SERVER_LABEL,
  ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER,
  ALERT_EMAIL_USERNAME_LABEL,
  getAlertEmailAdvancedToggleLabel,
  getAlertEmailProviderOptionLabel,
  getAlertEmailRecipientsPlaceholder,
  getAlertEmailSetupInstructionsToggleLabel,
  getAlertEmailTestButtonLabel,
  getAlertEmailUsernamePlaceholder,
} from '@/utils/alertEmailPresentation';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import {
  useEmailProviderSelectState,
  type EmailProviderSelectStateProps,
} from './useEmailProviderSelectState';
import { FormSelect } from '@/components/shared/FormSelect';

interface EmailProviderSelectProps extends EmailProviderSelectStateProps {
  config: UIEmailConfig;
  onTest: () => void;
  testing?: boolean;
}

export function EmailProviderSelect(props: EmailProviderSelectProps) {
  const state = useEmailProviderSelectState(props);
  const instructionBoxClass =
    'mt-2 rounded border border-blue-200 bg-blue-50 px-3 py-2 text-xs leading-relaxed text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200';

  return (
    <div class="space-y-4 text-sm overflow-hidden">
      <div class={formField}>
        <div class="flex w-full flex-wrap items-end gap-2 sm:flex-nowrap">
          <FormSelect
            id="alert-email-provider-select"
            label={ALERT_EMAIL_PROVIDER_LABEL}
            value={props.config.provider}
            onChange={(e) => state.handleProviderChange(e.currentTarget.value)}
            selectBaseClass={controlClass('px-2 py-1.5')}
            selectClass="sm:w-auto sm:min-w-[180px]"
          >
            <option value="">{ALERT_EMAIL_MANUAL_CONFIGURATION_LABEL}</option>
            <For each={state.providers()}>
              {(provider) => (
                <option value={provider.name}>{getAlertEmailProviderOptionLabel(provider)}</option>
              )}
            </For>
          </FormSelect>
          <Show when={props.config.provider}>
            <button
              type="button"
              onClick={() => {
                const provider = state.currentProvider();
                if (provider) state.applyProviderDefaults(provider);
              }}
              class="text-xs font-medium text-blue-600 hover:underline dark:text-blue-400"
            >
              {ALERT_EMAIL_REAPPLY_DEFAULTS_LABEL}
            </button>
          </Show>
        </div>
      </div>

      <Show when={state.currentProvider()}>
        <div class="sm:hidden w-full">
          <button
            type="button"
            onClick={state.toggleShowInstructions}
            class="text-xs font-medium text-blue-600 hover:underline dark:text-blue-300"
          >
            {getAlertEmailSetupInstructionsToggleLabel(state.showInstructions())}
          </button>
          <Show when={state.showInstructions()}>
            <div class={instructionBoxClass}>{state.currentProvider()!.instructions}</div>
          </Show>
        </div>
        <div class="hidden w-full sm:block">
          <div class={instructionBoxClass}>{state.currentProvider()!.instructions}</div>
        </div>
      </Show>

      <div class="grid w-full gap-3 sm:grid-cols-2">
        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_SMTP_SERVER_LABEL}</label>
          <input
            type="text"
            value={props.config.server}
            onInput={(e) => props.onChange({ ...props.config, server: e.currentTarget.value })}
            placeholder={ALERT_EMAIL_SMTP_SERVER_PLACEHOLDER}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_SMTP_PORT_LABEL}</label>
          <input
            type="number"
            min="1"
            max="65535"
            value={props.config.port || ''}
            onInput={(e) => {
              const value = e.currentTarget.value;
              // Allow empty field while typing, parse as number when valid
              const port = value === '' ? 0 : parseInt(value, 10);
              props.onChange({ ...props.config, port: isNaN(port) ? 0 : port });
            }}
            onBlur={(e) => {
              // Apply default on blur if empty or invalid
              const value = parseInt(e.currentTarget.value, 10);
              if (!value || isNaN(value)) {
                props.onChange({ ...props.config, port: 587 });
              }
            }}
            placeholder={ALERT_EMAIL_SMTP_PORT_PLACEHOLDER}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_FROM_ADDRESS_LABEL}</label>
          <input
            type="email"
            value={props.config.from}
            onInput={(e) => props.onChange({ ...props.config, from: e.currentTarget.value })}
            placeholder={ALERT_EMAIL_FROM_ADDRESS_PLACEHOLDER}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_REPLY_TO_LABEL}</label>
          <input
            type="email"
            value={props.config.replyTo || ''}
            onInput={(e) => props.onChange({ ...props.config, replyTo: e.currentTarget.value })}
            placeholder={ALERT_EMAIL_REPLY_TO_PLACEHOLDER}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_USERNAME_LABEL}</label>
          <input
            type="text"
            value={props.config.username}
            onInput={(e) => props.onChange({ ...props.config, username: e.currentTarget.value })}
            placeholder={getAlertEmailUsernamePlaceholder(props.config.provider)}
            class={controlClass('px-2 py-1.5')}
          />
        </div>

        <div class={formField}>
          <label class={labelClass()}>{ALERT_EMAIL_PASSWORD_LABEL}</label>
          <input
            type="password"
            value={props.config.password}
            onInput={(e) => props.onChange({ ...props.config, password: e.currentTarget.value })}
            placeholder={ALERT_EMAIL_PASSWORD_PLACEHOLDER}
            class={controlClass('px-2 py-1.5')}
          />
        </div>
      </div>

      <div class={formField}>
        <label class={labelClass()}>{ALERT_EMAIL_RECIPIENTS_LABEL}</label>
        <textarea
          value={props.config.to.join('\n')}
          onInput={(e) => {
            // Keep raw lines while editing — trimming/filtering on every
            // keystroke ate empty trailing lines and stripped typed
            // whitespace, so users couldn't type past line 1. Empty
            // entries are filtered in buildEmailConfigPayload at save.
            props.onChange({ ...props.config, to: e.currentTarget.value.split('\n') });
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.stopPropagation();
            }
          }}
          rows={3}
          class={controlClass('px-2 py-1.5 font-mono leading-snug')}
          placeholder={getAlertEmailRecipientsPlaceholder(props.config.from)}
        />
      </div>

      <div class="border-t border-border pt-3">
        <button
          type="button"
          onClick={state.toggleShowAdvanced}
          class="text-xs font-semibold uppercase tracking-wide transition-colors hover:text-muted"
        >
          {getAlertEmailAdvancedToggleLabel(state.showAdvanced())}
        </button>

        <Show when={state.showAdvanced()}>
          <div class="mt-3 space-y-3 text-xs text-base-content">
            <div class="grid gap-3 sm:grid-cols-3">
              <FormSelect
                id="alert-email-security-select"
                label={ALERT_EMAIL_SECURITY_LABEL}
                value={props.config.tls ? 'tls' : props.config.startTLS ? 'starttls' : 'none'}
                onChange={(e) => {
                  const value = e.currentTarget.value;
                  props.onChange({
                    ...props.config,
                    tls: value === 'tls',
                    startTLS: value === 'starttls',
                  });
                }}
                fieldBaseClass="flex"
                fieldClass="flex-row items-center gap-2"
                labelClass="text-xs uppercase tracking-[0.08em]"
                selectBaseClass={controlClass('px-2 py-1 text-sm')}
                selectClass="min-w-[120px]"
              >
                <option value="none">{ALERT_EMAIL_SECURITY_NONE_LABEL}</option>
                <option value="starttls">{ALERT_EMAIL_SECURITY_STARTTLS_LABEL}</option>
                <option value="tls">{ALERT_EMAIL_SECURITY_TLS_LABEL}</option>
              </FormSelect>
              <div class="flex w-full flex-wrap items-center gap-2 sm:flex-nowrap">
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  {ALERT_EMAIL_RATE_LIMIT_LABEL}
                </label>
                <input
                  type="number"
                  min="1"
                  value={props.config.rateLimit || 60}
                  onInput={(e) =>
                    props.onChange({ ...props.config, rateLimit: parseInt(e.currentTarget.value) })
                  }
                  class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                />
                <span class={formHelpText}>{ALERT_EMAIL_RATE_LIMIT_SUFFIX}</span>
              </div>
            </div>

            <div class="grid w-full gap-3 sm:grid-cols-2">
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  {ALERT_EMAIL_MAX_RETRIES_LABEL}
                </label>
                <input
                  type="number"
                  value={props.config.maxRetries || 3}
                  min={0}
                  max={5}
                  onInput={(e) =>
                    props.onChange({ ...props.config, maxRetries: parseInt(e.currentTarget.value) })
                  }
                  class={controlClass('px-2 py-1 text-sm')}
                />
              </div>
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  {ALERT_EMAIL_RETRY_DELAY_LABEL}
                </label>
                <input
                  type="number"
                  value={props.config.retryDelay || 5}
                  min={1}
                  max={60}
                  onInput={(e) =>
                    props.onChange({ ...props.config, retryDelay: parseInt(e.currentTarget.value) })
                  }
                  class={controlClass('px-2 py-1 text-sm')}
                />
              </div>
            </div>
          </div>
        </Show>
      </div>

      <div class="flex justify-end pt-2">
        <button
          type="button"
          onClick={props.onTest}
          disabled={props.testing || !props.config.enabled}
          class="rounded border border-blue-500 px-3 py-1.5 text-xs font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:opacity-50 disabled:cursor-not-allowed dark:border-blue-400 dark:text-blue-300 dark:hover:bg-blue-900"
        >
          {getAlertEmailTestButtonLabel(Boolean(props.testing))}
        </button>
      </div>
    </div>
  );
}
