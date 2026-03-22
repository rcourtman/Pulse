import { createMemo, createSignal, type Accessor } from 'solid-js';

import { NotificationsAPI, type AppriseConfig } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import {
  getAlertDestinationsAppriseTestFailure,
  getAlertDestinationsAppriseTestSuccess,
  getAlertDestinationsAppriseValidationError,
  getAlertDestinationsEmailTestFailure,
  getAlertDestinationsEmailTestSuccess,
} from '@/utils/alertDestinationsPresentation';

import { parseAppriseTargets } from './helpers';
import type { UIAppriseConfig, UIEmailConfig } from './types';
import { useAlertWebhookDestinationsState } from './useAlertWebhookDestinationsState';

export interface AlertDestinationsTabStateProps {
  emailConfig: Accessor<UIEmailConfig>;
  appriseConfig: Accessor<UIAppriseConfig>;
  setAppriseConfig: (config: UIAppriseConfig) => void;
  configLoadError: Accessor<string | null>;
  isRetrying: Accessor<boolean>;
  isLoadingDestinations: Accessor<boolean>;
  onRetryLoad: () => void;
}

export function useAlertDestinationsTabState(props: AlertDestinationsTabStateProps) {
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingApprise, setTestingApprise] = createSignal(false);
  const webhookState = useAlertWebhookDestinationsState();

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

  const handleRetry = () => {
    props.onRetryLoad();
    void webhookState.loadWebhooks();
  };

  return {
    appriseState,
    handleRetry,
    hasLoadError,
    isLoading,
    testApprise,
    testEmailConfig,
    testingApprise,
    testingEmail,
    updateApprise,
    ...webhookState,
  };
}
