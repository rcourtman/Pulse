import { batch, createMemo, createSignal, onCleanup } from 'solid-js';

import { NotificationsAPI, type NotificationHealth } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import type { APIContextOptions } from '@/utils/apiClient';
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
  requestOptions?: APIContextOptions;
  isCurrentReader?: () => boolean;
  onAfterQueueAction?: () => Promise<unknown> | unknown;
}) {
  const [deliveryHealth, setDeliveryHealth] = createSignal<NotificationHealth | null>(null);
  const [deliveryHealthUnavailable, setDeliveryHealthUnavailable] = createSignal(false);
  const [refreshingDeliveryHealth, setRefreshingDeliveryHealth] = createSignal(false);
  const [loadedOnce, setLoadedOnce] = createSignal(false);
  // A request already sent is not cancelled on navigation. Its retired view
  // must not publish a toast or start follow-up reads in the replacement view.
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });

  // Mount, configuration retry and queue actions can overlap. Only the latest
  // requested snapshot owns health and loading state, regardless of completion order.
  let latestHealthRequest = 0;
  const isCurrentReader = () => !disposed && options?.isCurrentReader?.() !== false;
  const ownsHealthRequest = (request: number) =>
    isCurrentReader() && request === latestHealthRequest;
  const loadDeliveryHealth = async () => {
    if (!isCurrentReader()) return;
    const request = ++latestHealthRequest;
    setRefreshingDeliveryHealth(true);
    try {
      const health = await NotificationsAPI.getHealth(options?.requestOptions);
      if (!ownsHealthRequest(request)) return;
      // Snapshot and availability change together. Set one at a time and
      // deliveryNeedsAttention reads a half-updated pair (no health, not yet
      // unavailable), briefly unmounting the warning and everything it holds.
      batch(() => {
        setDeliveryHealth(health);
        setDeliveryHealthUnavailable(health.queue.status === 'unavailable');
      });
    } catch (error) {
      if (!ownsHealthRequest(request)) return;
      logger.error('Failed to load notification delivery health', error);
      batch(() => {
        setDeliveryHealth(null);
        setDeliveryHealthUnavailable(true);
      });
    } finally {
      if (ownsHealthRequest(request)) {
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
    if (!isCurrentReader()) return;
    ++latestAction;
    setQueueActionFeedback(null);
  };
  const refreshAfterAction = async (action: number) => {
    if (!isCurrentReader()) return;
    await Promise.all([
      loadDeliveryHealth(),
      Promise.resolve()
        .then(() => (!isCurrentReader() ? undefined : options?.onAfterQueueAction?.()))
        .catch((error) => {
          if (!isCurrentReader()) return;
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
    if (!isCurrentReader()) return;
    const queue = deliveryHealth()?.queue;
    const count = (queue?.failed ?? 0) + (queue?.deadLetter ?? 0);
    if (count <= 0 || !confirm(getAlertDestinationsDeliveryRetryConfirmation(count))) {
      return;
    }
    const action = ++latestAction;
    setQueueActionFeedback(null);
    setRetryingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.retryTerminalFailures(options?.requestOptions);
      if (!isCurrentReader()) return;
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'delivery' : 'deliveries'} queued for retry.`,
      );
      await refreshAfterAction(action);
    } catch (error) {
      if (!isCurrentReader()) return;
      logger.error('Failed to retry retained notification deliveries', error);
      if (action === latestAction)
        setQueueActionFeedback('Unable to retry retained notification deliveries.');
      notificationStore.error('Unable to retry retained notification deliveries.');
    } finally {
      if (isCurrentReader()) setRetryingTerminalFailures(false);
    }
  };

  const dismissTerminalFailures = async () => {
    if (!isCurrentReader()) return;
    const queue = deliveryHealth()?.queue;
    const count = (queue?.failed ?? 0) + (queue?.deadLetter ?? 0);
    if (count <= 0 || !confirm(getAlertDestinationsDeliveryDismissConfirmation(count))) {
      return;
    }
    const action = ++latestAction;
    setQueueActionFeedback(null);
    setDismissingTerminalFailures(true);
    try {
      const result = await NotificationsAPI.dismissTerminalFailures(options?.requestOptions);
      if (!isCurrentReader()) return;
      notificationStore.success(
        `${result.affected} retained ${result.affected === 1 ? 'failure' : 'failures'} dismissed.`,
      );
      await refreshAfterAction(action);
    } catch (error) {
      if (!isCurrentReader()) return;
      logger.error('Failed to dismiss retained notification failures', error);
      if (action === latestAction)
        setQueueActionFeedback('Unable to dismiss retained notification failures.');
      notificationStore.error('Unable to dismiss retained notification failures.');
    } finally {
      if (isCurrentReader()) setDismissingTerminalFailures(false);
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
