import { createMemo, createSignal } from 'solid-js';

import { NotificationsAPI, type NotificationHealth } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import {
  getAlertDestinationsDeliveryDismissConfirmation,
  getAlertDestinationsDeliveryRetryConfirmation,
} from '@/utils/alertDestinationsPresentation';

// Delivery health is the only evidence a user has that configured destinations
// are actually reaching them. It is shared rather than owned by the
// destinations tab so the warning can also reach the alerts overview, which is
// where someone looks when they are wondering about their alerting at all.
// The retained-queue actions live here for the same reason: the warning must
// be clearable wherever it is shown, not only on the destinations tab.
export function useNotificationDeliveryHealth(options?: {
  onAfterQueueAction?: () => Promise<unknown> | unknown;
}) {
  const [deliveryHealth, setDeliveryHealth] = createSignal<NotificationHealth | null>(null);
  const [deliveryHealthUnavailable, setDeliveryHealthUnavailable] = createSignal(false);
  const [refreshingDeliveryHealth, setRefreshingDeliveryHealth] = createSignal(false);
  const [loadedOnce, setLoadedOnce] = createSignal(false);

  const loadDeliveryHealth = async () => {
    setRefreshingDeliveryHealth(true);
    try {
      const health = await NotificationsAPI.getHealth();
      setDeliveryHealth(health);
      setDeliveryHealthUnavailable(health.queue.status === 'unavailable');
    } catch (error) {
      logger.error('Failed to load notification delivery health', error);
      setDeliveryHealth(null);
      setDeliveryHealthUnavailable(true);
    } finally {
      setLoadedOnce(true);
      setRefreshingDeliveryHealth(false);
    }
  };

  // Only a queue the server itself calls degraded, or one it cannot report on,
  // is worth interrupting someone over. Stay silent until the first load
  // resolves so a slow request cannot flash a warning.
  const deliveryNeedsAttention = createMemo(
    () =>
      loadedOnce() &&
      (deliveryHealthUnavailable() || deliveryHealth()?.queue.status === 'degraded'),
  );

  const [retryingTerminalFailures, setRetryingTerminalFailures] = createSignal(false);
  const [dismissingTerminalFailures, setDismissingTerminalFailures] = createSignal(false);

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
      await Promise.all([loadDeliveryHealth(), Promise.resolve(options?.onAfterQueueAction?.())]);
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
      await Promise.all([loadDeliveryHealth(), Promise.resolve(options?.onAfterQueueAction?.())]);
    } catch (error) {
      logger.error('Failed to dismiss retained notification failures', error);
      notificationStore.error('Unable to dismiss retained notification failures.');
    } finally {
      setDismissingTerminalFailures(false);
    }
  };

  return {
    deliveryHealth,
    deliveryHealthUnavailable,
    refreshingDeliveryHealth,
    deliveryNeedsAttention,
    loadDeliveryHealth,
    retryTerminalFailures,
    retryingTerminalFailures,
    dismissTerminalFailures,
    dismissingTerminalFailures,
  };
}
