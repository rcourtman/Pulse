import { createSignal, onMount, Show } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
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
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
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
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestLabel,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
  getAlertDestinationsLoadErrorBanner,
  getAlertDestinationsRetryLabel,
  getAlertDestinationsStatusLabel,
  getAlertDestinationsWebhookLoadError,
} from '@/utils/alertDestinationsPresentation';
import {
  getAlertWebhookMutationFailure,
  getAlertWebhookMutationSuccess,
  getAlertWebhookTestFailure,
  getAlertWebhookTestSuccess,
  getAlertWebhooksSectionDescription,
  getAlertWebhooksSectionTitle,
} from '@/utils/alertWebhookPresentation';

import { parseAppriseTargets } from '../helpers';
import type { AppriseConfig } from '@/api/notifications';
import type { UIAppriseConfig, UIEmailConfig } from '../types';

export interface DestinationsTabProps {
  setHasUnsavedChanges: (value: boolean) => void;
  emailConfig: () => UIEmailConfig;
  setEmailConfig: (config: UIEmailConfig) => void;
  appriseConfig: () => UIAppriseConfig;
  setAppriseConfig: (config: UIAppriseConfig) => void;
  configLoadError: () => string | null;
  isRetrying: () => boolean;
  isLoadingDestinations: () => boolean;
  onRetryLoad: () => void;
}

export function DestinationsTab(props: DestinationsTabProps) {
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);
  const [webhookLoadError, setWebhookLoadError] = createSignal<string | null>(null);
  const [isLoadingWebhooks, setIsLoadingWebhooks] = createSignal(true);
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingApprise, setTestingApprise] = createSignal(false);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);

  const isLoading = () =>
    props.isLoadingDestinations() || isLoadingWebhooks() || props.isRetrying();
  const appriseState = () => props.appriseConfig();

  const updateApprise = (partial: Partial<UIAppriseConfig>) => {
    props.setAppriseConfig({ ...props.appriseConfig(), ...partial });
  };

  const buildAppriseRequestConfig = (): AppriseConfig => {
    const config = appriseState();
    const serverUrl = (config.serverUrl || '').trim();
    const apiKeyHeader = (config.apiKeyHeader || '').trim() || 'X-API-KEY';
    return {
      enabled: config.enabled,
      mode: config.mode,
      targets: parseAppriseTargets(config.targetsText),
      cliPath: config.cliPath?.trim() || 'apprise',
      timeoutSeconds: config.timeoutSeconds,
      serverUrl,
      configKey: config.configKey.trim(),
      apiKey: config.apiKey,
      apiKeyHeader,
      skipTlsVerify: config.skipTlsVerify,
    };
  };

  const loadWebhooks = async () => {
    setWebhookLoadError(null);
    setIsLoadingWebhooks(true);
    try {
      const hooks = await NotificationsAPI.getWebhooks();
      setWebhooks(
        hooks.map((hook) => ({
          ...hook,
          service: hook.service || 'generic',
        })),
      );
    } catch (error) {
      logger.error('Failed to load webhooks:', error);
      setWebhookLoadError(getAlertDestinationsWebhookLoadError());
    } finally {
      setIsLoadingWebhooks(false);
    }
  };

  onMount(() => {
    void loadWebhooks();
  });

  const testEmailConfig = async () => {
    setTestingEmail(true);
    try {
      await NotificationsAPI.testNotification({
        type: 'email',
        config: { ...props.emailConfig() } as Record<string, unknown>,
      });
      notificationStore.success(getAlertDestinationsEmailTestSuccess());
    } catch (error) {
      logger.error(getAlertDestinationsEmailTestFailure(), error);
      const message =
        error instanceof Error ? error.message : getAlertDestinationsEmailTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      setTestingEmail(false);
    }
  };

  const testApprise = async () => {
    setTestingApprise(true);
    try {
      const config = buildAppriseRequestConfig();

      if (!config.enabled) {
        throw new Error(getAlertDestinationsAppriseValidationError('disabled'));
      }

      const targets = config.targets || [];
      if (config.mode === 'cli' && targets.length === 0) {
        throw new Error(getAlertDestinationsAppriseValidationError('missingTargets'));
      }
      if (config.mode === 'http' && !config.serverUrl) {
        throw new Error(getAlertDestinationsAppriseValidationError('missingServerUrl'));
      }

      await NotificationsAPI.testNotification({
        type: 'apprise',
        config,
      });
      notificationStore.success(getAlertDestinationsAppriseTestSuccess());
    } catch (error) {
      logger.error(getAlertDestinationsAppriseTestFailure(), error);
      const message =
        error instanceof Error ? error.message : getAlertDestinationsAppriseTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      setTestingApprise(false);
    }
  };

  const testWebhook = async (webhookId: string, webhookData?: Omit<Webhook, 'id'>) => {
    setTestingWebhook(webhookId);
    try {
      if (webhookData) {
        await NotificationsAPI.testWebhook(webhookData);
      } else {
        await NotificationsAPI.testNotification({ type: 'webhook', webhookId });
      }
      notificationStore.success(getAlertWebhookTestSuccess());
    } catch (error) {
      const message = error instanceof Error ? error.message : getAlertWebhookTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      setTestingWebhook(null);
    }
  };

  const hasLoadError = () => props.configLoadError() || webhookLoadError();

  const handleRetry = () => {
    props.onRetryLoad();
    void loadWebhooks();
  };

  return (
    <div class="flex w-full max-w-full flex-col gap-6 md:gap-8">
      <Show
        when={!isLoading()}
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
        <Show when={hasLoadError()}>
          <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-2 text-red-800 dark:text-red-200">
                <AlertTriangleIcon class="h-4 w-4 flex-shrink-0" />
                <span class="text-sm font-medium">
                  {getAlertDestinationsLoadErrorBanner(
                    props.configLoadError() || webhookLoadError() || '',
                  )}
                </span>
              </div>
              <button
                class="flex-shrink-0 rounded-md border border-red-300 bg-transparent px-3 py-1.5 text-sm font-medium text-red-800 transition hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-700 dark:text-red-200 dark:hover:bg-red-900/30"
                disabled={props.isRetrying()}
                onClick={handleRetry}
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
              onTest={testEmailConfig}
              testing={testingEmail()}
            />
          </div>
        </SettingsPanel>

        <SettingsPanel
          title={ALERT_DESTINATIONS_APPRISE_PANEL_TITLE}
          description={ALERT_DESTINATIONS_APPRISE_PANEL_DESCRIPTION}
          action={
            <div class="flex items-center gap-3 sm:self-start">
              <Toggle
                checked={appriseState().enabled}
                onChange={(event) => {
                  updateApprise({ enabled: event.currentTarget.checked });
                  props.setHasUnsavedChanges(true);
                }}
                label={
                  <span class="text-xs font-medium text-muted">
                    {getAlertDestinationsStatusLabel(appriseState().enabled)}
                  </span>
                }
              />
              <button
                class="rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!appriseState().enabled || testingApprise()}
                onClick={testApprise}
              >
                {getAlertDestinationsAppriseTestLabel(testingApprise())}
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
                value={appriseState().mode}
                onInput={(event) => {
                  updateApprise({ mode: event.currentTarget.value as 'cli' | 'http' });
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
                value={appriseState().targetsText}
                placeholder={ALERT_DESTINATIONS_APPRISE_TARGETS_PLACEHOLDER}
                onInput={(event) => {
                  updateApprise({ targetsText: event.currentTarget.value });
                  props.setHasUnsavedChanges(true);
                }}
              />
              <p class={formHelpText}>
                {getAlertDestinationsAppriseTargetsHelp(appriseState().mode)}
              </p>
            </div>

            <Show when={appriseState().mode === 'cli'}>
              <div class={formField}>
                <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                  {ALERT_DESTINATIONS_APPRISE_CLI_PATH_LABEL}
                </label>
                <input
                  type="text"
                  value={appriseState().cliPath}
                  class={formControl}
                  placeholder={ALERT_DESTINATIONS_APPRISE_CLI_PATH_PLACEHOLDER}
                  onInput={(event) => {
                    updateApprise({ cliPath: event.currentTarget.value });
                    props.setHasUnsavedChanges(true);
                  }}
                />
                <p class={formHelpText}>{ALERT_DESTINATIONS_APPRISE_CLI_PATH_HELP}</p>
              </div>
            </Show>

            <Show when={appriseState().mode === 'http'}>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div class={`${formField} sm:col-span-2`}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_DESTINATIONS_APPRISE_SERVER_URL_LABEL}
                  </label>
                  <input
                    type="text"
                    value={appriseState().serverUrl}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_SERVER_URL_PLACEHOLDER}
                    onInput={(event) => {
                      updateApprise({ serverUrl: event.currentTarget.value });
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
                    value={appriseState().configKey}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_CONFIG_KEY_PLACEHOLDER}
                    onInput={(event) => {
                      updateApprise({ configKey: event.currentTarget.value });
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
                    value={appriseState().apiKey}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_PLACEHOLDER}
                    onInput={(event) => {
                      updateApprise({ apiKey: event.currentTarget.value });
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
                    value={appriseState().apiKeyHeader}
                    class={formControl}
                    placeholder={ALERT_DESTINATIONS_APPRISE_API_KEY_HEADER_PLACEHOLDER}
                    onInput={(event) => {
                      updateApprise({ apiKeyHeader: event.currentTarget.value });
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
                      checked={appriseState().skipTlsVerify}
                      onChange={(event) => {
                        updateApprise({ skipTlsVerify: event.currentTarget.checked });
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
                value={appriseState().timeoutSeconds}
                class={formControl}
                onInput={(event) => {
                  const raw = event.currentTarget.valueAsNumber;
                  const safe = Number.isNaN(raw) ? 15 : Math.min(120, Math.max(5, Math.trunc(raw)));
                  updateApprise({ timeoutSeconds: safe });
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
            <span class="whitespace-nowrap text-xs text-muted">{webhooks().length} configured</span>
          }
          class="min-w-0"
          bodyClass="space-y-4"
        >
          <WebhookConfig
            webhooks={webhooks()}
            onAdd={async (webhook) => {
              try {
                const created = await NotificationsAPI.createWebhook(webhook);
                setWebhooks([...webhooks(), created]);
                notificationStore.success(getAlertWebhookMutationSuccess('add'));
              } catch (error) {
                logger.error('Failed to add webhook:', error);
                notificationStore.error(
                  error instanceof Error ? error.message : getAlertWebhookMutationFailure('add'),
                );
              }
            }}
            onUpdate={async (webhook) => {
              try {
                const updated = await NotificationsAPI.updateWebhook(webhook.id!, webhook);
                setWebhooks(webhooks().map((current) => (current.id === webhook.id ? updated : current)));
                notificationStore.success(getAlertWebhookMutationSuccess('update'));
              } catch (error) {
                logger.error('Failed to update webhook:', error);
                notificationStore.error(
                  error instanceof Error ? error.message : getAlertWebhookMutationFailure('update'),
                );
              }
            }}
            onDelete={async (id) => {
              try {
                await NotificationsAPI.deleteWebhook(id);
                setWebhooks(webhooks().filter((current) => current.id !== id));
                notificationStore.success(getAlertWebhookMutationSuccess('delete'));
              } catch (error) {
                logger.error('Failed to delete webhook:', error);
                notificationStore.error(
                  error instanceof Error ? error.message : getAlertWebhookMutationFailure('delete'),
                );
              }
            }}
            onTest={testWebhook}
            testing={testingWebhook()}
          />
        </SettingsPanel>
      </Show>
    </div>
  );
}
