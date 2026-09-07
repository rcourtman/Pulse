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

  // Mount, configuration retry and queue actions can overlap. Only the latest
  // requested snapshot owns health and loading state, regardless of completion order.
  let latestHealthRequest = 0;
  const loadDeliveryHealth = async () => {
    const request = ++latestHealthRequest;
    setRefreshingDeliveryHealth(true);
    try {
      const health = await NotificationsAPI.getHealth();
      if (request !== latestHealthRequest) return;
      setDeliveryHealth(health);
      setDeliveryHealthUnavailable(health.queue.status === 'unavailable');
    } catch (error) {
      if (request !== latestHealthRequest) return;
      logger.error('Failed to load notification delivery health', error);
      setDeliveryHealth(null);
      setDeliveryHealthUnavailable(true);
    } finally {
      if (request === latestHealthRequest) {
        setLoadedOnce(true);
        setRefreshingDeliveryHealth(false);
      }
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

  // Action failure is independent of current queue health. A healthy read is
  // not evidence that a previously rejected action succeeded.
  const [queueActionFeedback, setQueueActionFeedback] = createSignal<string | null>(null);
  let latestAction = 0;
  const clearQueueActionFeedback = () => {
    ++latestAction;
    setQueueActionFeedback(null);
  };
  const refreshAfterAction = async (action: number) => {
    await Promise.all([
      loadDeliveryHealth(),
      Promise.resolve()
        .then(() => options?.onAfterQueueAction?.())
        .catch((error) => {
          logger.error(
            'Failed to refresh notification activity after accepted queue action',
            error,
          );
          if (action === latestAction) {
            setQueueActionFeedback(
              'The queue action succeeded, but notification activity could not be refreshed. Reload this view to check activity.',
            );
          }
        }),
    ]);
  };

  const [retryingTerminalFailures, setRetryingTerminalFailures] = createSignal(false);
  const [dismissingTerminalFailures, setDismissingTerminalFailures] = createSignal(false);

  const retryTerminalFailures = async () => {
    const count = deliveryHealth()?.queue.attentionRequired ?? 0;
    if (count <= 0 || !confirm(getAlertDestinationsDeliveryRetryConfirmation(count))) {
      return;
    }
    const action = ++latestAction;
    setQueueActionFeedback(null);
    setRetryingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.retryTerminalFailures();
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'delivery' : 'deliveries'} queued for retry.`,
      );
      await refreshAfterAction(action);
    } catch (error) {
      logger.error('Failed to retry retained notification deliveries', error);
      if (action === latestAction)
        setQueueActionFeedback('Unable to retry retained notification deliveries.');
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
    const action = ++latestAction;
    setQueueActionFeedback(null);
    setDismissingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.dismissTerminalFailures();
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'failure' : 'failures'} dismissed.`,
      );
      await refreshAfterAction(action);
    } catch (error) {
      logger.error('Failed to dismiss retained notification failures', error);
      if (action === latestAction)
        setQueueActionFeedback('Unable to dismiss retained notification failures.');
      notificationStore.error('Unable to dismiss retained notification failures.');
    } finally {
      setDismissingTerminalFailures(false);
    }
  };

  return {
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
  };
}
