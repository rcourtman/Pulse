import { createMemo, createSignal, onCleanup, onMount, type Accessor } from 'solid-js';

import { NotificationsAPI, type AppriseConfig, type Webhook } from '@/api/notifications';
import type { Setter } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import { getOrgID } from '@/utils/apiClient';
import {
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
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
  const org = getOrgID() || 'default';
  const requestOptions = { expectedOrgID: org, retry: false } as const;
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });
  const isCurrentReader = () => !disposed && (getOrgID() || 'default') === org;
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingApprise, setTestingApprise] = createSignal(false);
  const {
    queueActionFeedback,
    clearQueueActionFeedback,
    deliveryHealth,
    deliveryHealthUnavailable,
    refreshingDeliveryHealth,
    deliveryNeedsAttention,
    loadDeliveryHealth,
    retryTerminalFailures,
    retryingTerminalFailures,
    dismissTerminalFailures,
    dismissingTerminalFailures,
  } = useNotificationDeliveryHealth({
    requestOptions,
    isCurrentReader,
    onAfterQueueAction: () => loadDeliveryLog(),
  });
  const {
    deliveryLog,
    deliveryLogUnavailable,
    refreshingDeliveryLog,
    heldEvents,
    heldEventsUnavailable,
    refreshingHeldEvents,
    loadDeliveryLog,
  } = useNotificationDeliveryLog({ requestOptions, isCurrentReader });
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
    if (!isCurrentReader() || hasLoadError() || isLoading() || testingEmail()) return;
    setTestingEmail(true);
    try {
      const result = await NotificationsAPI.testNotification(
        {
          type: 'email',
          config: { ...props.emailConfig() } as Record<string, unknown>,
        },
        requestOptions,
      );
      if (!isCurrentReader()) return;
      if (result.deliveryPaused) {
        notificationStore.warning(getAlertDestinationsTestPausedWarning());
      } else {
        notificationStore.success(getAlertDestinationsEmailTestSuccess());
      }
    } catch (error) {
      if (!isCurrentReader()) return;
      logger.error(getAlertDestinationsEmailTestFailure(), error);
      const message =
        error instanceof Error ? error.message : getAlertDestinationsEmailTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      if (isCurrentReader()) setTestingEmail(false);
    }
  };

  const testApprise = async () => {
    if (!isCurrentReader() || hasLoadError() || isLoading() || testingApprise()) return;
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

      const result = await NotificationsAPI.testNotification(
        {
          type: 'apprise',
          config,
        },
        requestOptions,
      );
      if (!isCurrentReader()) return;
      if (result.deliveryPaused) {
        notificationStore.warning(getAlertDestinationsTestPausedWarning());
      } else {
        notificationStore.success(getAlertDestinationsAppriseTestSuccess());
      }
    } catch (error) {
      if (!isCurrentReader()) return;
      logger.error(getAlertDestinationsAppriseTestFailure(), error);
      const message =
        error instanceof Error ? error.message : getAlertDestinationsAppriseTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      if (isCurrentReader()) setTestingApprise(false);
    }
  };

  const handleRetry = () => {
    if (!isCurrentReader()) return;
    props.onRetryLoad();
    if (props.webhooks === undefined) {
      void webhookState.loadWebhooks();
    }
    void loadDeliveryHealth();
    void loadDeliveryLog();
  };

  onMount(() => {
    void loadDeliveryHealth();
    void loadDeliveryLog();
  });

  return {
    appriseState,
    queueActionFeedback,
    clearQueueActionFeedback,
    deliveryHealth,
    deliveryHealthUnavailable,
    deliveryLog,
    deliveryLogUnavailable,
    deliveryNeedsAttention,
    heldEvents,
    heldEventsUnavailable,
    refreshingHeldEvents,
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
