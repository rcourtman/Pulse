import { createMemo, createSignal, type Accessor } from 'solid-js';

import { useAlertsActivation } from '@/stores/alertsActivation';
import type { AlertDestinationsDeliveryPausedReason } from '@/utils/alertDestinationsPresentation';
import { logger } from '@/utils/logger';

export interface AlertDeliveryPausedState {
  pausedReason: Accessor<AlertDestinationsDeliveryPausedReason | null>;
  activating: Accessor<boolean>;
  activate: () => Promise<void>;
}

// Why live alerts are not reaching any destination, or null when delivery is
// on. Shared by the overview and the notifications tab so both surfaces name
// the same gate in the same words.
export function useAlertDeliveryPausedReason(): AlertDeliveryPausedState {
  const alertsActivation = useAlertsActivation();
  const [activating, setActivating] = createSignal(false);

  const pausedReason = createMemo<AlertDestinationsDeliveryPausedReason | null>(() => {
    // Until the alert config resolves, activation state is null and would read
    // as paused; staying quiet avoids a false warning on every page open.
    if (!alertsActivation.config() || alertsActivation.activationState() === null) {
      return null;
    }
    if (alertsActivation.notificationDeliveryEnabled()) {
      return null;
    }
    if (!alertsActivation.detectionEnabled()) {
      return 'detection_off';
    }
    return alertsActivation.activationState() === 'snoozed' ? 'snoozed' : 'not_activated';
  });

  const activate = async () => {
    if (activating()) {
      return;
    }
    setActivating(true);
    try {
      await alertsActivation.activate();
    } catch (error) {
      logger.error('Failed to activate notification delivery', error);
    } finally {
      setActivating(false);
    }
  };

  return { pausedReason, activating, activate };
}
