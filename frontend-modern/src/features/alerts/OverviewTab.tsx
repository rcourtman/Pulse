import { createSignal, onCleanup, createEffect, onMount, Show } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Alert } from '@/types/api';

import { AlertDeliveryHealthCard } from './AlertDeliveryHealthCard';
import { AlertOverviewActiveAlertsSection } from './AlertOverviewActiveAlertsSection';
import { AlertOverviewStatsCards } from './AlertOverviewStatsCards';
import type { Override } from './types';
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
      <Show when={deliveryHealthState.deliveryNeedsAttention()}>
        <AlertDeliveryHealthCard
          health={deliveryHealthState.deliveryHealth()?.queue ?? null}
          unavailable={deliveryHealthState.deliveryHealthUnavailable()}
          refreshing={deliveryHealthState.refreshingDeliveryHealth()}
          onRefresh={() => void deliveryHealthState.loadDeliveryHealth()}
          detailsHref="/alerts/notifications"
        />
      </Show>
      <AlertOverviewStatsCards state={overviewState} />
      <AlertOverviewActiveAlertsSection
        state={overviewState}
        timelineState={timelineState}
        activeAlerts={props.activeAlerts}
        alertsDisabled={props.alertsDisabled()}
        showAcknowledged={props.showAcknowledged()}
        setShowAcknowledged={props.setShowAcknowledged}
      />
    </div>
  );
}
