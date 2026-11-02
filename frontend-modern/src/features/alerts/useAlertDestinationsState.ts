import { createEffect, createSignal, onCleanup, untrack } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import type { Webhook } from '@/api/notifications';
import { RelayAPI } from '@/api/relay';
import { hasFeature } from '@/stores/license';
import { getAlertDestinationsConfigLoadError } from '@/utils/alertDestinationsPresentation';
import { logger } from '@/utils/logger';
import { getOrgID } from '@/utils/apiClient';

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
  isCurrentReader?: Accessor<boolean>;
}

export function useAlertDestinationsState(options: AlertDestinationsStateOptions) {
  let hasLoadedDestinations = false;
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
  let disposed = false;
  let readController: AbortController | undefined;
  const org = getOrgID() || 'default';
  const requestOptions = { expectedOrgID: org, retry: false } as const;
  const isCurrentReader = () =>
    !disposed && options.isCurrentReader?.() !== false && (getOrgID() || 'default') === org;

  onCleanup(() => {
    disposed = true;
    ++reloadVersion;
    readController?.abort();
  });

  const resetDestinations = () => {
    readController?.abort();
    readController = undefined;
    hasLoadedDestinations = false;
    ++reloadVersion;
    setIsLoadingDestinations(false);
    setDestConfigLoadError(null);
    setEmailConfig(createDefaultEmailConfig());
    setAppriseConfig(createDefaultAppriseConfig());
    setDeadManPingUrl('');
    setPushMinimumSeverity('all');
    setWebhooks([]);
  };

  const loadDestinations = async (_options: { indicateLoading?: boolean } = {}) => {
    if (!isCurrentReader()) return;
    // A refresh withdraws the previous snapshot, including unsaved private
    // values. A partial read is not permission to edit or test empty defaults.
    resetDestinations();
    const thisVersion = ++reloadVersion;
    const controller = new AbortController();
    readController = controller;
    const readOptions = {
      ...requestOptions,
      cache: 'no-store' as const,
      signal: controller.signal,
    };
    setIsLoadingDestinations(true);

    const results = await Promise.allSettled([
      NotificationsAPI.getEmailConfig(readOptions),
      NotificationsAPI.getAppriseConfig(readOptions),
      AlertsAPI.getDeadManConfig(readOptions),
      hasFeature('relay') ? RelayAPI.getConfig(readOptions) : Promise.resolve(null),
      NotificationsAPI.getWebhooks(readOptions),
    ]);

    if (thisVersion !== reloadVersion || !isCurrentReader()) {
      return;
    }
    readController = undefined;

    const failures = results.filter(
      (result): result is PromiseRejectedResult => result.status === 'rejected',
    );
    if (failures.length > 0) {
      // Do not put provider-controlled details (URLs or credentials) in feedback.
      logger.error('Failed to load complete notification configuration');
      setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      setIsLoadingDestinations(false);
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

    setDestConfigLoadError(null);
    hasLoadedDestinations = true;
    setIsLoadingDestinations(false);
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
    const ownsWrite = () =>
      isCurrentReader() &&
      hasLoadedDestinations &&
      ownsSave() &&
      snapshot.reloadVersion === reloadVersion;
    if (!ownsWrite()) return;
    await NotificationsAPI.updateEmailConfig(snapshot.email, requestOptions);
    if (!ownsWrite()) return;

    const updatedApprise = await NotificationsAPI.updateAppriseConfig(
      snapshot.apprise,
      requestOptions,
    );
    if (!ownsWrite()) return;

    await AlertsAPI.updateDeadManConfig(snapshot.pingUrl, requestOptions);
    if (!ownsWrite()) return;

    if (snapshot.relayEnabled && hasFeature('relay')) {
      await RelayAPI.updateConfig(
        { alert_minimum_severity: snapshot.pushMinimumSeverity },
        requestOptions,
      );
    }

    // An acknowledgement is not a reload: it may replace the submitted draft
    // with masked fields, but must not overwrite edits or another context.
    if (
      ownsWrite() &&
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
