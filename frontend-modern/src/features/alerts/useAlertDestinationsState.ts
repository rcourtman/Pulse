import { createEffect, createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import { RelayAPI } from '@/api/relay';
import { hasFeature } from '@/stores/license';
import { getAlertDestinationsConfigLoadError } from '@/utils/alertDestinationsPresentation';
import { logger } from '@/utils/logger';

import {
  buildAppriseConfigPayload,
  buildEmailConfigPayload,
  normalizeAppriseConfig,
  normalizeEmailConfigFromAPI,
} from './alertDestinationsModel';
import { createDefaultAppriseConfig, createDefaultEmailConfig } from './helpers';
import type { AlertTab, UIAppriseConfig, UIEmailConfig } from './types';

interface AlertDestinationsStateOptions {
  activeTab: Accessor<AlertTab>;
}

export function useAlertDestinationsState(options: AlertDestinationsStateOptions) {
  const [isLoadingDestinations, setIsLoadingDestinations] = createSignal(false);
  const [destConfigLoadError, setDestConfigLoadError] = createSignal<string | null>(null);
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>(createDefaultEmailConfig());
  const [appriseConfig, setAppriseConfig] = createSignal<UIAppriseConfig>(
    createDefaultAppriseConfig(),
  );
  const [deadManPingUrl, setDeadManPingUrl] = createSignal('');
  const [pushMinimumSeverity, setPushMinimumSeverity] = createSignal<'all' | 'critical'>('all');

  let reloadVersion = 0;
  let lastActiveTab: AlertTab | null = null;

  const resetDestinations = () => {
    setDestConfigLoadError(null);
    setEmailConfig(createDefaultEmailConfig());
    setAppriseConfig(createDefaultAppriseConfig());
    setDeadManPingUrl('');
    setPushMinimumSeverity('all');
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
      AlertsAPI.getDeadManConfig(),
      hasFeature('relay') ? RelayAPI.getConfig() : Promise.resolve(null),
    ]);

    if (thisVersion !== reloadVersion) {
      return;
    }

    const [emailResult, appriseResult, deadManResult, relayResult] = results;

    if (emailResult.status === 'fulfilled') {
      setEmailConfig(normalizeEmailConfigFromAPI(emailResult.value));
    }

    if (appriseResult.status === 'fulfilled') {
      setAppriseConfig(normalizeAppriseConfig(appriseResult.value));
    }

    if (deadManResult.status === 'fulfilled') {
      setDeadManPingUrl(deadManResult.value.pingUrl || '');
    }

    if (relayResult.status === 'fulfilled' && relayResult.value) {
      setPushMinimumSeverity(
        relayResult.value.alert_minimum_severity === 'critical' ? 'critical' : 'all',
      );
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
    await NotificationsAPI.updateEmailConfig(buildEmailConfigPayload(emailConfig()));

    const updatedApprise = await NotificationsAPI.updateAppriseConfig(
      buildAppriseConfigPayload(appriseConfig()),
    );

    await AlertsAPI.updateDeadManConfig(deadManPingUrl());

    if (hasFeature('relay')) {
      await RelayAPI.updateConfig({ alert_minimum_severity: pushMinimumSeverity() });
    }

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
    deadManPingUrl,
    setDeadManPingUrl,
    pushMinimumSeverity,
    setPushMinimumSeverity,
    resetDestinations,
    loadDestinations,
    saveDestinations,
  };
}
