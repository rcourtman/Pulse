import { Show } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

import { EmailProviderSelect } from '@/components/Alerts/EmailProviderSelect';
import { WebhookConfig } from '@/components/Alerts/WebhookConfig';
import { Card } from '@/components/shared/Card';
import {
  formControl,
  formField,
  formHelpText,
  labelClass,
} from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_API_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL,
  ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL,
  ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_HELP,
  ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_LABEL,
  ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_APPRISE_PANEL_TITLE,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL,
  ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL,
  ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP,
  ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL,
  ALERT_DESTINATIONS_APPRISE_TLS_HELP,
  ALERT_DESTINATIONS_APPRISE_TLS_LABEL,
  ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION,
  ALERT_DESTINATIONS_EMAIL_PANEL_TITLE,
  getAlertDestinationsAppriseTargetsHelp,
  getAlertDestinationsAppriseTestLabel,
  getAlertDestinationsLoadErrorBanner,
  getAlertDestinationsRetryLabel,
  getAlertDestinationsStatusLabel,
} from '@/utils/alertDestinationsPresentation';
import {
  getAlertWebhooksSectionDescription,
  getAlertWebhooksSectionTitle,
} from '@/utils/alertWebhookPresentation';

import { useAlertDestinationsTabState, type AlertDestinationsTabStateProps } from '../useAlertDestinationsTabState';

export interface DestinationsTabProps extends AlertDestinationsTabStateProps {
  setHasUnsavedChanges: (value: boolean) => void;
  setEmailConfig: (config: ReturnType<AlertDestinationsTabStateProps['emailConfig']>) => void;
}

export function DestinationsTab(props: DestinationsTabProps) {
  const state = useAlertDestinationsTabState(props);

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <Show
        when={!state.isLoading()}
        fallback={
          <div class="flex w-full flex-col gap-6 animate-pulse pointer-events-none select-none md:gap-8">
            <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
              <div class="flex items-center justify-between">
                <div class="space-y-2">
                  <div class="h-5 w-40 rounded bg-surface-hover" />
                  <div class="h-3 w-64 rounded bg-surface-hover" />
                </div>
                <div class="h-6 w-12 rounded-full bg-surface-hover" />
              </div>
              <div class="space-y-3">
                <div class="h-4 w-24 rounded bg-surface-hover" />
                <div class="h-10 w-full rounded bg-surface-hover" />
                <div class="h-4 w-32 rounded bg-surface-hover" />
                <div class="h-10 w-full rounded bg-surface-hover" />
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
              <div class="flex items-center justify-between">
                <div class="space-y-2">
                  <div class="h-5 w-44 rounded bg-surface-hover" />
                  <div class="h-3 w-72 rounded bg-surface-hover" />
                </div>
                <div class="h-6 w-12 rounded-full bg-surface-hover" />
              </div>
              <div class="space-y-3">
                <div class="h-4 w-28 rounded bg-surface-hover" />
                <div class="h-10 w-full rounded bg-surface-hover" />
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface p-6 space-y-4">
              <div class="flex items-center justify-between">
                <div class="space-y-2">
                  <div class="h-5 w-28 rounded bg-surface-hover" />
                  <div class="h-3 w-56 rounded bg-surface-hover" />
                </div>
                <div class="h-4 w-20 rounded bg-surface-hover" />
              </div>
              <div class="h-10 w-full rounded bg-surface-hover" />
            </div>
          </div>
        }
      >
        <Show when={state.hasLoadError()}>
          <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-2 text-red-800 dark:text-red-200">
                <AlertTriangleIcon class="h-4 w-4 flex-shrink-0" />
                <span class="text-sm font-medium">
                  {getAlertDestinationsLoadErrorBanner(
                    props.configLoadError() || state.webhookLoadError() || '',
                  )}
                </span>
              </div>
              <button
                class="flex-shrink-0 rounded-md border border-red-300 bg-transparent px-3 py-1.5 text-sm font-medium text-red-800 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-700 dark:text-red-200 dark:hover:bg-red-900/30"
                disabled={props.isRetrying()}
                onClick={state.handleRetry}
              >
                {getAlertDestinationsRetryLabel(props.isRetrying())}
              </button>
            </div>
          </Card>
        </Show>

        <SettingsPanel
          title={ALERT_DESTINATIONS_EMAIL_PANEL_TITLE}
          description={ALERT_DESTINATIONS_EMAIL_PANEL_DESCRIPTION}
          action={
            <Toggle
              checked={props.emailConfig().enabled}
              onChange={(event) => {
                props.setEmailConfig({
                  ...props.emailConfig(),
                  enabled: event.currentTarget.checked,
                });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {getAlertDestinationsStatusLabel(props.emailConfig().enabled)}
                </span>
              }
            />
          }
          class="min-w-0"
          bodyClass=""
        >
          <div
            class={`${!props.emailConfig().enabled ? 'pointer-events-none opacity-50 transition-opacity' : 'transition-opacity'}`}
          >
            <EmailProviderSelect
              config={props.emailConfig()}
              onChange={(config) => {
                props.setEmailConfig(config);
                props.setHasUnsavedChanges(true);
              }}
              onTest={state.testEmailConfig}
              testing={state.testingEmail()}
            />
          </div>
        </SettingsPanel>

        <SettingsPanel
          title={ALERT_DESTINATIONS_APPRISE_PANEL_TITLE}
          description={ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION}
          action={
            <div class="flex items-center gap-3 sm:self-start">
              <Toggle
                checked={state.appriseState().enabled}
                onChange={(event) => {
                  state.updateApprise({ enabled: event.currentTarget.checked });
                  props.setHasUnsavedChanges(true);
                }}
                label={
                  <span class="text-xs font-medium text-muted">
                    {getAlertDestinationsStatusLabel(state.appriseState().enabled)}
                  </span>
                }
              />
              <button
                class="rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!state.appriseState().enabled || state.testingApprise()}
                onClick={state.testApprise}
              >
                {getAlertDestinationsAppriseTestLabel(state.testingApprise())}
              </button>
            </div>
          }
          class="min-w-0"
          bodyClass="space-y-4"
        >
          <div class="space-y-4">
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_MODE_LABEL}
              </label>
              <select
                class={formControl}
                value={state.appriseState().mode}
                onInput={(event) => {
                  state.updateApprise({ mode: event.currentTarget.value as 'cli' | 'http' });
                  props.setHasUnsavedChanges(true);
                }}
              >
                <option value="cli">{ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL}</option>
                <option value="http">{ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL}</option>
              </select>
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_MODE_HELP}</p>
            </div>

            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_TARGETS_LABEL}
              </label>
              <textarea
                rows={4}
                class={`${formControl} min-h-[120px] font-mono`}
                value={state.appriseState().targetsText}
                placeholder={ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER}
                onInput={(event) => {
                  state.updateApprise({ targetsText: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>
                {getAlertDestinationsAppriseTargetsHelp(state.appriseState().mode)}
              </p>
            </div>

            <Show when={state.appriseState().mode === 'cli'}>
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  {ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL}
                </label>
                <input
                  type="text"
                  value={state.appriseState().cliPath}
                  class={formControl}
                  placeholder={ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER}
                  onInput={(event) => {
                    state.updateApprise({ cliPath: event.currentTarget.value });
                    props.setHasUnsavedChanges(true);
                  }}
                />
                <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP}</p>
              </div>
            </Show>

            <Show when={state.appriseState().mode === 'http'}>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div class={`${formField} sm:col-span-2`}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL}
                  </label>
                  <input
                    type="text"
                    value={state.appriseState().serverUrl}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER}
                    onInput={(event) => {
                      state.updateApprise({ serverUrl: event.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                  />
                  <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_SERVER_URL_HELP}</p>
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_LABEL}
                  </label>
                  <input
                    type="text"
                    value={state.appriseState().configKey}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER}
                    onInput={(event) => {
                      state.updateApprise({ configKey: event.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                  />
                  <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_HELP}</p>
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_API_KEY_LABEL}
                  </label>
                  <input
                    type="password"
                    value={state.appriseState().apiKey}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER}
                    onInput={(event) => {
                      state.updateApprise({ apiKey: event.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                  />
                  <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_API_KEY_HELP}</p>
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_LABEL}
                  </label>
                  <input
                    type="text"
                    value={state.appriseState().apiKeyHeader}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER}
                    onInput={(event) => {
                      state.updateApprise({ apiKeyHeader: event.currentTarget.value });
                      props.setHasUnsavedChanges(true);
                    }}
                  />
                  <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_HELP}</p>
                </div>
                <div class={`${formField} sm:col-span-2`}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_TLS_LABEL}
                  </label>
                  <label class="inline-flex items-center gap-2">
                    <input
                      type="checkbox"
                      class="h-4 w-4 rounded border border-border"
                      checked={state.appriseState().skipTlsVerify}
                      onChange={(event) => {
                        state.updateApprise({ skipTlsVerify: event.currentTarget.checked });
                        props.setHasUnsavedChanges(true);
                      }}
                    />
                    <span class="text-sm text-muted">
                      {ALERT_DESTINATIONS_APPRISE_TLS_CHECKBOX_LABEL}
                    </span>
                  </label>
                  <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_TLS_HELP}</p>
                </div>
              </div>
            </Show>

            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_DESTINATIONS_APPRISE_TIMEOUT_LABEL}
              </label>
              <input
                type="number"
                min="5"
                max="120"
                value={state.appriseState().timeoutSeconds}
                class={formControl}
                onInput={(event) => {
                  const raw = event.currentTarget.valueAsNumber;
                  const safe = Number.isNaN(raw) ? 15 : Math.min(120, Math.max(5, Math.trunc(raw)));
                  state.updateApprise({ timeoutSeconds: safe });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_TIMEOUT_HELP}</p>
            </div>
          </div>
        </SettingsPanel>

        <SettingsPanel
          title={getAlertWebhooksSectionTitle()}
          description={getAlertWebhooksSectionDescription()}
          action={
            <span class="whitespace-nowrap text-xs text-muted">{state.webhooks().length} configured</span>
          }
          class="min-w-0"
          bodyClass="space-y-4"
        >
          <WebhookConfig
            webhooks={state.webhooks()}
            onAdd={state.addWebhook}
            onUpdate={state.updateWebhook}
            onDelete={state.deleteWebhook}
            onTest={state.testWebhook}
            testing={state.testingWebhook()}
          />
        </SettingsPanel>
      </Show>
    </div>
  );
}
