import { AlertQueueActionFeedback } from './AlertQueueActionFeedback';
import { createSignal, onCleanup, createEffect, onMount, Show } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Alert } from '@/types/api';

import { AlertDeliveryHealthCard } from './AlertDeliveryHealthCard';
import { AlertDeliveryPausedCard } from './AlertDeliveryPausedCard';
import { AlertOverviewActiveAlertsSection } from './AlertOverviewActiveAlertsSection';
import type { Override } from './types';
import { useAlertDeliveryPausedReason } from './useAlertDeliveryPausedReason';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { useAlertOverviewState } from './useAlertOverviewState';
import { useNotificationDeliveryHealth } from './useNotificationDeliveryHealth';

export function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
  showQuickTip: () => boolean;
  dismissQuickTip: () => void;
  showAcknowledged: () => boolean;
  setShowAcknowledged: (value: boolean) => void;
  alertsDisabled: () => boolean;
}) {
  const location = useLocation();
  let hashScrollRafId: number | undefined;
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const overviewState = useAlertOverviewState({
    activeAlerts: () => props.activeAlerts,
    overrides: () => props.overrides,
    showAcknowledged: props.showAcknowledged,
    updateAlert: props.updateAlert,
  });
  const timelineState = useAlertIncidentTimelineState();
  // A destination that stopped delivering is invisible by nature: the failure
  // is the channel that would have reported it. Surface it on the tab people
  // actually open, not only on the destinations config tab.
  const deliveryHealthState = useNotificationDeliveryHealth();
  // Delivery being off applies to every alert at once, so it is said once here
  // rather than on each card.
  const deliveryPaused = useAlertDeliveryPausedReason();
  onMount(() => {
    void deliveryHealthState.loadDeliveryHealth();
  });

  const scrollToAlertHash = () => {
    const hash = location.hash;
    if (!hash || !hash.startsWith('#alert-')) {
      setLastHashScrolled(null);
      return;
    }
    if (hash === lastHashScrolled()) {
      return;
    }
    const target = document.getElementById(hash.slice(1));
    if (!target) {
      return;
    }
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setLastHashScrolled(hash);
  };

  createEffect(() => {
    location.hash;
    overviewState.filteredAlerts().length;
    props.showAcknowledged();
    if (hashScrollRafId !== undefined) {
      cancelAnimationFrame(hashScrollRafId);
    }
    hashScrollRafId = requestAnimationFrame(() => {
      hashScrollRafId = undefined;
      scrollToAlertHash();
    });
  });

  onCleanup(() => {
    if (hashScrollRafId !== undefined) {
      cancelAnimationFrame(hashScrollRafId);
      hashScrollRafId = undefined;
    }
  });

  return (
    <div class="space-y-4 sm:space-y-6">
      <AlertQueueActionFeedback
        message={deliveryHealthState.queueActionFeedback()}
        onClear={deliveryHealthState.clearQueueActionFeedback}
      />
      <Show when={deliveryHealthState.deliveryNeedsAttention()}>
        <AlertDeliveryHealthCard
          health={deliveryHealthState.deliveryHealth()?.queue ?? null}
          unavailable={deliveryHealthState.deliveryHealthUnavailable()}
          refreshing={deliveryHealthState.refreshingDeliveryHealth()}
          onRefresh={() => void deliveryHealthState.loadDeliveryHealth()}
          retryingFailures={deliveryHealthState.retryingTerminalFailures()}
          dismissingFailures={deliveryHealthState.dismissingTerminalFailures()}
          onRetryFailures={() => void deliveryHealthState.retryTerminalFailures()}
          onDismissFailures={() => void deliveryHealthState.dismissTerminalFailures()}
          detailsHref="/alerts/notifications#notification-delivery-activity"
          detailLevel="summary"
          showRefresh={deliveryHealthState.deliveryHealthUnavailable()}
        />
      </Show>
      <Show when={deliveryPaused.pausedReason()}>
        {(reason) => (
          <AlertDeliveryPausedCard
            reason={reason()}
            surface="overview"
            setupHref="/alerts/notifications"
            activating={deliveryPaused.activating()}
            onActivate={() => void deliveryPaused.activate()}
          />
        )}
      </Show>
      <AlertOverviewActiveAlertsSection
        state={overviewState}
        timelineState={timelineState}
        activeAlerts={props.activeAlerts}
        alertsDisabled={props.alertsDisabled()}
        showAcknowledged={props.showAcknowledged()}
        setShowAcknowledged={props.setShowAcknowledged}
        deliveryPausedGlobally={deliveryPaused.pausedReason() !== null}
      />
    </div>
  );
}
