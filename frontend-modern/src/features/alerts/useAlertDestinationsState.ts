import { createEffect, createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';

import type { AppriseConfig, EmailConfig } from '@/api/notifications';
import { NotificationsAPI } from '@/api/notifications';
import { getAlertDestinationsConfigLoadError } from '@/utils/alertDestinationsPresentation';
import { logger } from '@/utils/logger';

import {
  createDefaultAppriseConfig,
  createDefaultEmailConfig,
  formatAppriseTargets,
  normalizeEmailConfigFromAPI,
  parseAppriseTargets,
} from './helpers';
import type { AlertTab, UIAppriseConfig, UIEmailConfig } from './types';

interface AlertDestinationsStateOptions {
  activeTab: Accessor<AlertTab>;
}

function normalizeAppriseConfig(config: Partial<AppriseConfig> | null | undefined): UIAppriseConfig {
  return {
    enabled: config?.enabled ?? false,
    mode: config?.mode === 'http' ? 'http' : 'cli',
    targetsText: formatAppriseTargets(config?.targets),
    cliPath: config?.cliPath || 'apprise',
    timeoutSeconds:
      typeof config?.timeoutSeconds === 'number' && config.timeoutSeconds > 0
        ? config.timeoutSeconds
        : 15,
    serverUrl: config?.serverUrl || '',
    configKey: config?.configKey || '',
    apiKey: config?.apiKey || '',
    apiKeyHeader: config?.apiKeyHeader || 'X-API-KEY',
    skipTlsVerify: Boolean(config?.skipTlsVerify),
  };
}

export function useAlertDestinationsState(options: AlertDestinationsStateOptions) {
  const [isLoadingDestinations, setIsLoadingDestinations] = createSignal(false);
  const [destConfigLoadError, setDestConfigLoadError] = createSignal<string | null>(null);
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>(createDefaultEmailConfig());
  const [appriseConfig, setAppriseConfig] = createSignal<UIAppriseConfig>(
    createDefaultAppriseConfig(),
  );

  let reloadVersion = 0;
  let lastActiveTab: AlertTab | null = null;

  const resetDestinations = () => {
    setDestConfigLoadError(null);
    setEmailConfig(createDefaultEmailConfig());
    setAppriseConfig(createDefaultAppriseConfig());
  };

  const loadDestinations = async (options: { indicateLoading?: boolean } = {}) => {
    const indicateLoading = options.indicateLoading ?? false;
    const thisVersion = ++reloadVersion;

    if (indicateLoading) {
      setIsLoadingDestinations(true);
    }

    const results = await Promise.allSettled([
      NotificationsAPI.getEmailConfig(),
      NotificationsAPI.getAppriseConfig(),
    ]);

    if (thisVersion !== reloadVersion) {
      return;
    }

    const [emailResult, appriseResult] = results;

    if (emailResult.status === 'fulfilled') {
      setEmailConfig(normalizeEmailConfigFromAPI(emailResult.value));
    }

    if (appriseResult.status === 'fulfilled') {
      setAppriseConfig(normalizeAppriseConfig(appriseResult.value));
    }

    const failures = results.filter(
      (result): result is PromiseRejectedResult => result.status === 'rejected',
    );

    if (failures.length > 0) {
      failures.forEach((result) => {
        logger.error('Failed to load notification configuration:', result.reason);
      });
      setDestConfigLoadError(getAlertDestinationsConfigLoadError());
    } else {
      setDestConfigLoadError(null);
    }

    if (indicateLoading) {
      setIsLoadingDestinations(false);
    }
  };

  const saveDestinations = async () => {
    const currentEmailConfig = emailConfig();
    await NotificationsAPI.updateEmailConfig({
      enabled: currentEmailConfig.enabled,
      provider: currentEmailConfig.provider,
      server: currentEmailConfig.server,
      port: currentEmailConfig.port,
      username: currentEmailConfig.username,
      password: currentEmailConfig.password,
      from: currentEmailConfig.from,
      to: currentEmailConfig.to,
      tls: currentEmailConfig.tls,
      startTLS: currentEmailConfig.startTLS,
    } as EmailConfig);

    const currentAppriseConfig = appriseConfig();
    const updatedApprise = await NotificationsAPI.updateAppriseConfig({
      enabled: currentAppriseConfig.enabled,
      mode: currentAppriseConfig.mode,
      targets: parseAppriseTargets(currentAppriseConfig.targetsText),
      cliPath: currentAppriseConfig.cliPath,
      timeoutSeconds: currentAppriseConfig.timeoutSeconds,
      serverUrl: currentAppriseConfig.serverUrl,
      configKey: currentAppriseConfig.configKey,
      apiKey: currentAppriseConfig.apiKey,
      apiKeyHeader: currentAppriseConfig.apiKeyHeader,
      skipTlsVerify: currentAppriseConfig.skipTlsVerify,
    } as AppriseConfig);

    setAppriseConfig(normalizeAppriseConfig(updatedApprise));
  };

  createEffect(() => {
    const current = options.activeTab();
    const previous = lastActiveTab;
    lastActiveTab = current;

    if (current !== 'destinations' || previous === null) {
      return;
    }

    void loadDestinations({ indicateLoading: true });
  });

  return {
    isLoadingDestinations,
    destConfigLoadError,
    emailConfig,
    setEmailConfig,
    appriseConfig,
    setAppriseConfig,
    resetDestinations,
    loadDestinations,
    saveDestinations,
  };
}
