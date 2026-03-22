import { createSignal, onMount } from 'solid-js';

import { NotificationsAPI, type Webhook } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { showErrorWithDetail } from '@/utils/toast';
import { getAlertDestinationsWebhookLoadError } from '@/utils/alertDestinationsPresentation';
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

export function useAlertWebhookDestinationsState() {
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);
  const [webhookLoadError, setWebhookLoadError] = createSignal<string | null>(null);
  const [isLoadingWebhooks, setIsLoadingWebhooks] = createSignal(true);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);

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
