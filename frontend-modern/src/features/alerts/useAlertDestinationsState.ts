import { createEffect, createSignal, onCleanup, untrack } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import type { Webhook } from '@/api/notifications';
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
  canReload?: Accessor<boolean>;
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
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);

  let reloadVersion = 0;
  let lastActiveTab: AlertTab | null = null;

  onCleanup(() => {
    ++reloadVersion;
  });

  const resetDestinations = () => {
    ++reloadVersion;
    setDestConfigLoadError(null);
    setEmailConfig(createDefaultEmailConfig());
    setAppriseConfig(createDefaultAppriseConfig());
    setDeadManPingUrl('');
    setPushMinimumSeverity('all');
    setWebhooks([]);
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
      NotificationsAPI.getWebhooks(),
    ]);

    if (thisVersion !== reloadVersion) {
      return;
    }

    const [emailResult, appriseResult, deadManResult, relayResult, webhooksResult] = results;

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

    if (webhooksResult.status === 'fulfilled') {
      setWebhooks(
        webhooksResult.value.map((webhook) => ({
          ...webhook,
          service: webhook.service || 'generic',
        })),
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

  // Capture every endpoint's payload before any write yields. The alert-policy
  // owner uses the same snapshot even while its preceding PUT is pending.
  const captureDestinations = () => ({
    email: structuredClone(buildEmailConfigPayload(emailConfig())),
    apprise: structuredClone(buildAppriseConfigPayload(appriseConfig())),
    appriseDraft: appriseConfig(),
    pingUrl: deadManPingUrl(),
    pushMinimumSeverity: pushMinimumSeverity(),
    relayEnabled: hasFeature('relay'),
    reloadVersion,
  });

  const saveDestinations = async (snapshot = captureDestinations(), ownsSave = () => true) => {
    if (!ownsSave()) return;
    await NotificationsAPI.updateEmailConfig(snapshot.email);
    if (!ownsSave()) return;

    const updatedApprise = await NotificationsAPI.updateAppriseConfig(snapshot.apprise);
    if (!ownsSave()) return;

    await AlertsAPI.updateDeadManConfig(snapshot.pingUrl);
    if (!ownsSave()) return;

    if (snapshot.relayEnabled && hasFeature('relay')) {
      await RelayAPI.updateConfig({ alert_minimum_severity: snapshot.pushMinimumSeverity });
    }

    // An acknowledgement is not a reload: it may replace the submitted draft
    // with masked fields, but must not overwrite edits or another context.
    if (
      ownsSave() &&
      reloadVersion === snapshot.reloadVersion &&
      appriseConfig() === snapshot.appriseDraft
    ) {
      setAppriseConfig(normalizeAppriseConfig(updatedApprise));
    }
  };

  createEffect(() => {
    const current = options.activeTab();
    const previous = lastActiveTab;
    lastActiveTab = current;

    if (current !== 'destinations' || previous === null || previous === current) {
      return;
    }

    // Entering a tab must not replace drafts or race a pending save. Only tab
    // changes trigger this refresh, not later changes to the admission flags.
    if (!untrack(options.canReload ?? (() => true))) return;
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
    webhooks,
    setWebhooks,
    resetDestinations,
    loadDestinations,
    captureDestinations,
    saveDestinations,
  };
}
