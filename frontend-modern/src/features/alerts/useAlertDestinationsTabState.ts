import { createMemo, createSignal, onMount, type Accessor } from 'solid-js';

import { NotificationsAPI, type AppriseConfig, type Webhook } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import {
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
  getAlertDestinationsWebhookLoadError,
} from '@/utils/alertDestinationsPresentation';
import {
  getAlertWebhookMutationFailure,
  getAlertWebhookMutationSuccess,
  getAlertWebhookTestFailure,
  getAlertWebhookTestSuccess,
} from '@/utils/alertWebhookPresentation';

import { parseAppriseTargets } from './helpers';
import type { UIAppriseConfig, UIEmailConfig } from './types';

export interface AlertDestinationsTabStateProps {
  emailConfig: Accessor<UIEmailConfig>;
  appriseConfig: Accessor<UIAppriseConfig>;
  setAppriseConfig: (config: UIAppriseConfig) => void;
  configLoadError: Accessor<string | null>;
  isRetrying: Accessor<boolean>;
  isLoadingDestinations: Accessor<boolean>;
  onRetryLoad: () => void;
}

const normalizeWebhook = (webhook: Webhook): Webhook => ({
  ...webhook,
  service: webhook.service || 'generic',
});

export function useAlertDestinationsTabState(props: AlertDestinationsTabStateProps) {
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);
  const [webhookLoadError, setWebhookLoadError] = createSignal<string | null>(null);
  const [isLoadingWebhooks, setIsLoadingWebhooks] = createSignal(true);
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingApprise, setTestingApprise] = createSignal(false);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);

  const isLoading = createMemo(
    () => props.isLoadingDestinations() || isLoadingWebhooks() || props.isRetrying(),
  );
  const hasLoadError = createMemo(() => props.configLoadError() || webhookLoadError());
  const appriseState = createMemo(() => props.appriseConfig());

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
      setWebhooks(hooks.map(normalizeWebhook));
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

  const addWebhook = async (webhook: Omit<Webhook, 'id'>) => {
    try {
      const created = await NotificationsAPI.createWebhook(webhook);
      setWebhooks((current) => [...current, normalizeWebhook(created)]);
      notificationStore.success(getAlertWebhookMutationSuccess('add'));
    } catch (error) {
      logger.error('Failed to add webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('add'),
      );
    }
  };

  const updateWebhook = async (webhook: Webhook) => {
    try {
      const updated = await NotificationsAPI.updateWebhook(webhook.id, webhook);
      setWebhooks((current) =>
        current.map((entry) => (entry.id === webhook.id ? normalizeWebhook(updated) : entry)),
      );
      notificationStore.success(getAlertWebhookMutationSuccess('update'));
    } catch (error) {
      logger.error('Failed to update webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('update'),
      );
    }
  };

  const deleteWebhook = async (id: string) => {
    try {
      await NotificationsAPI.deleteWebhook(id);
      setWebhooks((current) => current.filter((entry) => entry.id !== id));
      notificationStore.success(getAlertWebhookMutationSuccess('delete'));
    } catch (error) {
      logger.error('Failed to delete webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('delete'),
      );
    }
  };

  const handleRetry = () => {
    props.onRetryLoad();
    void loadWebhooks();
  };

  return {
    addWebhook,
    appriseState,
    deleteWebhook,
    handleRetry,
    hasLoadError,
    isLoading,
    isLoadingWebhooks,
    loadWebhooks,
    testApprise,
    testEmailConfig,
    testWebhook,
    testingApprise,
    testingEmail,
    testingWebhook,
    updateApprise,
    updateWebhook,
    webhookLoadError,
    webhooks,
  };
}
