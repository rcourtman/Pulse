import { createMemo, createSignal } from 'solid-js';

import { NotificationsAPI, type NotificationHealth } from '@/api/notifications';
import { logger } from '@/utils/logger';

// Delivery health is the only evidence a user has that configured destinations
// are actually reaching them. It is shared rather than owned by the
// destinations tab so the warning can also reach the alerts overview, which is
// where someone looks when they are wondering about their alerting at all.
export function useNotificationDeliveryHealth() {
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

  return {
    deliveryHealth,
    deliveryHealthUnavailable,
    refreshingDeliveryHealth,
    deliveryNeedsAttention,
    loadDeliveryHealth,
  };
}
