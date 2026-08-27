import { createMemo, createSignal, onMount, type Accessor } from 'solid-js';

import { NotificationsAPI, type AppriseConfig, type Webhook } from '@/api/notifications';
import type { Setter } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import {
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsDeliveryDismissConfirmation,
  getAlertDestinationsDeliveryRetryConfirmation,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
  getAlertDestinationsTestPausedWarning,
} from '@/utils/alertDestinationsPresentation';

import { parseAppriseTargets } from './helpers';
import type { UIAppriseConfig, UIEmailConfig } from './types';
import { useNotificationDeliveryHealth } from './useNotificationDeliveryHealth';
import { useNotificationDeliveryLog } from './useNotificationDeliveryLog';
import { useAlertWebhookDestinationsState } from './useAlertWebhookDestinationsState';

export interface AlertDestinationsTabStateProps {
  emailConfig: Accessor<UIEmailConfig>;
  appriseConfig: Accessor<UIAppriseConfig>;
  setAppriseConfig: (config: UIAppriseConfig) => void;
  configLoadError: Accessor<string | null>;
  isRetrying: Accessor<boolean>;
  isLoadingDestinations: Accessor<boolean>;
  onRetryLoad: () => void;
  webhooks?: Accessor<Webhook[]>;
  setWebhooks?: Setter<Webhook[]>;
}

export function useAlertDestinationsTabState(props: AlertDestinationsTabStateProps) {
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingApprise, setTestingApprise] = createSignal(false);
  const [retryingTerminalFailures, setRetryingTerminalFailures] = createSignal(false);
  const [dismissingTerminalFailures, setDismissingTerminalFailures] = createSignal(false);
  const {
    deliveryHealth,
    deliveryHealthUnavailable,
    refreshingDeliveryHealth,
    deliveryNeedsAttention,
    loadDeliveryHealth,
  } = useNotificationDeliveryHealth();
  const {
    deliveryLog,
    deliveryLogUnavailable,
    refreshingDeliveryLog,
    heldEvents,
    loadDeliveryLog,
  } = useNotificationDeliveryLog();
  const webhookState = useAlertWebhookDestinationsState({
    webhooks: props.webhooks,
    setWebhooks: props.setWebhooks,
    autoLoad: props.webhooks === undefined,
  });

  const isLoading = createMemo(
    () => props.isLoadingDestinations() || webhookState.isLoadingWebhooks() || props.isRetrying(),
  );
  const hasLoadError = createMemo(() => props.configLoadError() || webhookState.webhookLoadError());
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

  const testEmailConfig = async () => {
    setTestingEmail(true);
    try {
      const result = await NotificationsAPI.testNotification({
        type: 'email',
        config: { ...props.emailConfig() } as Record<string, unknown>,
      });
      if (result.deliveryPaused) {
        notificationStore.warning(getAlertDestinationsTestPausedWarning());
      } else {
        notificationStore.success(getAlertDestinationsEmailTestSuccess());
      }
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

      const result = await NotificationsAPI.testNotification({
        type: 'apprise',
        config,
      });
      if (result.deliveryPaused) {
        notificationStore.warning(getAlertDestinationsTestPausedWarning());
      } else {
        notificationStore.success(getAlertDestinationsAppriseTestSuccess());
      }
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

  const handleRetry = () => {
    props.onRetryLoad();
    if (props.webhooks === undefined) {
      void webhookState.loadWebhooks();
    }
    void loadDeliveryHealth();
    void loadDeliveryLog();
  };

  const retryTerminalFailures = async () => {
    const count = deliveryHealth()?.queue.attentionRequired ?? 0;
    if (count <= 0 || !confirm(getAlertDestinationsDeliveryRetryConfirmation(count))) {
      return;
    }
    setRetryingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.retryTerminalFailures();
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'delivery' : 'deliveries'} queued for retry.`,
      );
      await Promise.all([loadDeliveryHealth(), loadDeliveryLog()]);
    } catch (error) {
      logger.error('Failed to retry retained notification deliveries', error);
      notificationStore.error('Unable to retry retained notification deliveries.');
    } finally {
      setRetryingTerminalFailures(false);
    }
  };

  const dismissTerminalFailures = async () => {
    const count = deliveryHealth()?.queue.attentionRequired ?? 0;
    if (count <= 0 || !confirm(getAlertDestinationsDeliveryDismissConfirmation(count))) {
      return;
    }
    setDismissingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.dismissTerminalFailures();
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'failure' : 'failures'} dismissed.`,
      );
      await Promise.all([loadDeliveryHealth(), loadDeliveryLog()]);
    } catch (error) {
      logger.error('Failed to dismiss retained notification failures', error);
      notificationStore.error('Unable to dismiss retained notification failures.');
    } finally {
      setDismissingTerminalFailures(false);
    }
  };

  onMount(() => {
    void loadDeliveryHealth();
    void loadDeliveryLog();
  });

  return {
    appriseState,
    deliveryHealth,
    deliveryHealthUnavailable,
    deliveryLog,
    deliveryLogUnavailable,
    deliveryNeedsAttention,
    heldEvents,
    dismissTerminalFailures,
    dismissingTerminalFailures,
    handleRetry,
    hasLoadError,
    isLoading,
    loadDeliveryHealth,
    loadDeliveryLog,
    refreshingDeliveryHealth,
    refreshingDeliveryLog,
    retryTerminalFailures,
    retryingTerminalFailures,
    testApprise,
    testEmailConfig,
    testingApprise,
    testingEmail,
    updateApprise,
    ...webhookState,
  };
}
