import { AlertQueueActionFeedback } from './AlertQueueActionFeedback';
import { createSignal, onCleanup, createEffect, onMount, Show } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { AlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert } from '@/types/api';

import { AlertDeliveryHealthCard } from './AlertDeliveryHealthCard';
import { AlertDeliveryPausedCard } from './AlertDeliveryPausedCard';
import { AlertNoDestinationsCard } from './AlertNoDestinationsCard';
import { AlertOverviewActiveAlertsSection } from './AlertOverviewActiveAlertsSection';
import type { Override } from './types';
import { useAlertDeliveryPausedReason } from './useAlertDeliveryPausedReason';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { useAlertOverviewState } from './useAlertOverviewState';
import { useNotificationDeliveryHealth } from './useNotificationDeliveryHealth';
import { createLocalStorageBooleanSignal } from '@/utils/localStorage';

const ALERTS_NO_DESTINATIONS_DISMISSED_KEY = 'pulseAlertsNoDestinationsDismissed';

export function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
  acknowledgementRequests: AlertAcknowledgementRequests;
  showQuickTip: () => boolean;
  dismissQuickTip: () => void;
  showAcknowledged: () => boolean;
  setShowAcknowledged: (value: boolean) => void;
  alertsDisabled: () => boolean;
  hasMonitoredResources?: () => boolean;
  canAddInfrastructure?: boolean;
}) {
  const location = useLocation();
  let hashScrollRafId: number | undefined;
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const overviewState = useAlertOverviewState({
    activeAlerts: () => props.activeAlerts,
    overrides: () => props.overrides,
    showAcknowledged: props.showAcknowledged,
    updateAlert: props.updateAlert,
    acknowledgementRequests: props.acknowledgementRequests,
  });
  const timelineState = useAlertIncidentTimelineState();
  // A destination that stopped delivering is invisible by nature: the failure
  // is the channel that would have reported it. Surface it on the tab people
  // actually open, not only on the destinations config tab.
  const deliveryHealthState = useNotificationDeliveryHealth();
  // Delivery being off applies to every alert at once, so it is said once here
  // rather than on each card.
  const deliveryPaused = useAlertDeliveryPausedReason();
  const alertsActivation = useAlertsActivation();
  // No pause banner can also mean the config has not confirmed activation.
  // The setup hint needs a positive read, not the store's legacy default.
  const deliveryConfirmedActive = () => {
    const config = alertsActivation.config();
    return config?.enabled === true && config.activationState === 'active';
  };
  const [noDestinationsDismissed, setNoDestinationsDismissed] = createLocalStorageBooleanSignal(
    ALERTS_NO_DESTINATIONS_DISMISSED_KEY,
    false,
  );
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
      <Show
        when={
          deliveryConfirmedActive() &&
          deliveryHealthState.hasNoEnabledDestinations() &&
          !deliveryHealthState.deliveryNeedsAttention() &&
          !props.alertsDisabled() &&
          !noDestinationsDismissed()
        }
      >
        <AlertNoDestinationsCard
          setupHref="/alerts/notifications"
          onDismiss={() => setNoDestinationsDismissed(true)}
        />
      </Show>
      <AlertOverviewActiveAlertsSection
        state={overviewState}
        timelineState={timelineState}
        activeAlerts={props.activeAlerts}
        alertsDisabled={props.alertsDisabled()}
        showAcknowledged={props.showAcknowledged()}
        setShowAcknowledged={props.setShowAcknowledged}
        deliveryPausedGlobally={deliveryPaused.pausedReason() !== null}
        hasMonitoredResources={props.hasMonitoredResources ? props.hasMonitoredResources() : true}
        canAddInfrastructure={props.canAddInfrastructure ?? false}
      />
    </div>
  );
}
