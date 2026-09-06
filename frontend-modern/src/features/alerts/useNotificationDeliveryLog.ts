import { createSignal, onCleanup } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI, type NotificationDeliveryLog } from '@/api/notifications';
import type { AlertEvent } from '@/types/api';
import { logger } from '@/utils/logger';

const HELD_EVENT_TYPES = ['notification_suppressed', 'notification_deferred'];
const HELD_EVENT_WINDOW_DAYS = 7;
const HELD_EVENT_LIMIT = 100;
const DELIVERY_LOG_LIMIT = 200;

// The delivery log is the positive half of delivery evidence: health warns
// when something is wrong, the log shows what actually fired and where it
// went. A log that cannot be read is reported as unavailable, never as empty,
// because "no attempts" and "cannot tell" mean opposite things to someone
// deciding whether to trust their alerting.
//
// Held events are the third half of that story: notifications that were
// deliberately not attempted, with the mechanism that held them. They come
// from the alert event log and degrade independently — a failed events read
// hides the held rows without marking delivery attempts unavailable.
export function useNotificationDeliveryLog() {
  const [deliveryLog, setDeliveryLog] = createSignal<NotificationDeliveryLog | null>(null);
  const [deliveryLogUnavailable, setDeliveryLogUnavailable] = createSignal(false);
  const [refreshingDeliveryLog, setRefreshingDeliveryLog] = createSignal(false);
  const [heldEvents, setHeldEvents] = createSignal<AlertEvent[]>([]);

  // Both reads belong to the same refresh, but held events never block the log.
  let latestRequest = 0;
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });
  const ownsRequest = (request: number) => !disposed && request === latestRequest;

  const loadHeldEvents = async (request: number) => {
    try {
      const since = new Date(
        Date.now() - HELD_EVENT_WINDOW_DAYS * 24 * 60 * 60 * 1000,
      ).toISOString();
      const events = await AlertsAPI.getEvents({
        types: HELD_EVENT_TYPES,
        since,
        limit: HELD_EVENT_LIMIT,
      });
      if (ownsRequest(request)) setHeldEvents(events);
    } catch (error) {
      if (!ownsRequest(request)) return;
      logger.error('Failed to load held alert notification events', error);
      setHeldEvents([]);
    }
  };

  const loadDeliveryLog = async () => {
    if (disposed) return;
    const request = ++latestRequest;
    setRefreshingDeliveryLog(true);
    // Held events refresh independently: they must never delay or fail the
    // primary delivery-attempt log.
    void loadHeldEvents(request);
    try {
      // Request the server's bounded maximum. A degraded queue can retain more
      // than the default page of 50 failures, and the evidence view should not
      // hide them behind unrelated successful attempts when space is available.
      const log = await NotificationsAPI.getDeliveryLog(DELIVERY_LOG_LIMIT);
      if (!ownsRequest(request)) return;
      setDeliveryLog(log);
      setDeliveryLogUnavailable(false);
    } catch (error) {
      if (!ownsRequest(request)) return;
      logger.error('Failed to load notification delivery log', error);
      setDeliveryLog(null);
      setDeliveryLogUnavailable(true);
    } finally {
      if (ownsRequest(request)) setRefreshingDeliveryLog(false);
    }
  };

  return {
    deliveryLog,
    deliveryLogUnavailable,
    refreshingDeliveryLog,
    heldEvents,
    loadDeliveryLog,
  };
}
