import { createSignal } from 'solid-js';

import { NotificationsAPI, type NotificationDeliveryLog } from '@/api/notifications';
import { logger } from '@/utils/logger';

// The delivery log is the positive half of delivery evidence: health warns
// when something is wrong, the log shows what actually fired and where it
// went. A log that cannot be read is reported as unavailable, never as empty,
// because "no attempts" and "cannot tell" mean opposite things to someone
// deciding whether to trust their alerting.
export function useNotificationDeliveryLog() {
  const [deliveryLog, setDeliveryLog] = createSignal<NotificationDeliveryLog | null>(null);
  const [deliveryLogUnavailable, setDeliveryLogUnavailable] = createSignal(false);
  const [refreshingDeliveryLog, setRefreshingDeliveryLog] = createSignal(false);

  const loadDeliveryLog = async () => {
    setRefreshingDeliveryLog(true);
    try {
      const log = await NotificationsAPI.getDeliveryLog();
      setDeliveryLog(log);
      setDeliveryLogUnavailable(false);
    } catch (error) {
      logger.error('Failed to load notification delivery log', error);
      setDeliveryLog(null);
      setDeliveryLogUnavailable(true);
    } finally {
      setRefreshingDeliveryLog(false);
    }
  };

  return {
    deliveryLog,
    deliveryLogUnavailable,
    refreshingDeliveryLog,
    loadDeliveryLog,
  };
}
