import { createSignal, onCleanup, onMount, type Accessor, type Setter } from 'solid-js';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import { getOrgID } from '@/utils/apiClient';
import {
  getAlertDestinationsTestPausedWarning,
  getAlertDestinationsWebhookLoadError,
} from '@/utils/alertDestinationsPresentation';
import {
  getAlertWebhookMutationFailure,
  getAlertWebhookMutationSuccess,
  getAlertWebhookTestFailure,
  getAlertWebhookTestSuccess,
} from '@/utils/alertWebhookPresentation';

const normalizeWebhook = (webhook: Webhook): Webhook => ({
  ...webhook,
  service: webhook.service || 'generic',
});

interface AlertWebhookDestinationsStateOptions {
  webhooks?: Accessor<Webhook[]>;
  setWebhooks?: Setter<Webhook[]>;
  autoLoad?: boolean;
}

export function useAlertWebhookDestinationsState(
  options: AlertWebhookDestinationsStateOptions = {},
) {
  const [localWebhooks, setLocalWebhooks] = createSignal<Webhook[]>([]);
  const webhooks = options.webhooks ?? localWebhooks;
  const setWebhooks = options.setWebhooks ?? setLocalWebhooks;
  const [webhookLoadError, setWebhookLoadError] = createSignal<string | null>(null);
  const [isLoadingWebhooks, setIsLoadingWebhooks] = createSignal(options.autoLoad !== false);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);
  const org = getOrgID() || 'default';
  const requestOptions = { expectedOrgID: org, retry: false } as const;
  let disposed = false;
  const isCurrentReader = () => !disposed && (getOrgID() || 'default') === org;
  onCleanup(() => {
    disposed = true;
  });

  const loadWebhooks = async () => {
    if (!isCurrentReader()) return;
    setWebhookLoadError(null);
    setWebhooks([]);
    setIsLoadingWebhooks(true);
    try {
      const hooks = await NotificationsAPI.getWebhooks(requestOptions);
      if (!isCurrentReader()) return;
      setWebhooks(hooks.map(normalizeWebhook));
    } catch {
      if (!isCurrentReader()) return;
      logger.error('Failed to load webhooks');
      setWebhookLoadError(getAlertDestinationsWebhookLoadError());
    } finally {
      if (isCurrentReader()) setIsLoadingWebhooks(false);
    }
  };

  onMount(() => {
    if (options.autoLoad !== false) {
      void loadWebhooks();
    }
  });

  const testWebhook = async (webhookId: string, webhookData?: Omit<Webhook, 'id'>) => {
    if (!isCurrentReader() || webhookLoadError() || isLoadingWebhooks() || testingWebhook()) return;
    setTestingWebhook(webhookId);
    try {
      const result = webhookData
        ? await NotificationsAPI.testWebhook(webhookData, requestOptions)
        : await NotificationsAPI.testNotification({ type: 'webhook', webhookId }, requestOptions);
      if (!isCurrentReader()) return;
      if (result.deliveryPaused) {
        notificationStore.warning(getAlertDestinationsTestPausedWarning());
      } else {
        notificationStore.success(getAlertWebhookTestSuccess());
      }
    } catch (error) {
      if (!isCurrentReader()) return;
      const message = error instanceof Error ? error.message : getAlertWebhookTestFailure();
      const detail = (error as Error & { detail?: string })?.detail;
      showErrorWithDetail(message, detail);
    } finally {
      if (isCurrentReader()) setTestingWebhook(null);
    }
  };

  const addWebhook = async (webhook: Omit<Webhook, 'id'>) => {
    if (!isCurrentReader()) return false;
    try {
      const created = await NotificationsAPI.createWebhook(webhook, requestOptions);
      if (!isCurrentReader()) return false;
      setWebhooks((current) => [...current, normalizeWebhook(created)]);
      notificationStore.success(getAlertWebhookMutationSuccess('add'));
      return true;
    } catch (error) {
      if (!isCurrentReader()) return false;
      logger.error('Failed to add webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('add'),
      );
      return false;
    }
  };

  const updateWebhook = async (webhook: Webhook) => {
    if (!isCurrentReader()) return false;
    try {
      const updated = await NotificationsAPI.updateWebhook(webhook.id, webhook, requestOptions);
      if (!isCurrentReader()) return false;
      setWebhooks((current) =>
        current.map((entry) => (entry.id === webhook.id ? normalizeWebhook(updated) : entry)),
      );
      notificationStore.success(getAlertWebhookMutationSuccess('update'));
      return true;
    } catch (error) {
      if (!isCurrentReader()) return false;
      logger.error('Failed to update webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('update'),
      );
      return false;
    }
  };

  const deleteWebhook = async (id: string) => {
    if (!isCurrentReader()) return false;
    try {
      const result = await NotificationsAPI.deleteWebhook(id, requestOptions);
      if (!isCurrentReader()) return false;
      if (result.success !== true) {
        throw new Error(getAlertWebhookMutationFailure('delete'));
      }
      setWebhooks((current) => current.filter((entry) => entry.id !== id));
      notificationStore.success(getAlertWebhookMutationSuccess('delete'));
      return true;
    } catch (error) {
      if (!isCurrentReader()) return false;
      logger.error('Failed to delete webhook:', error);
      notificationStore.error(
        error instanceof Error ? error.message : getAlertWebhookMutationFailure('delete'),
      );
      return false;
    }
  };

  return {
    addWebhook,
    deleteWebhook,
    isLoadingWebhooks,
    loadWebhooks,
    testWebhook,
    testingWebhook,
    updateWebhook,
    webhookLoadError,
    webhooks,
  };
}
