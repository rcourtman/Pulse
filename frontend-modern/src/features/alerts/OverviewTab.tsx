import { createSignal, onCleanup, createEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Alert } from '@/types/api';

import { AlertOverviewActiveAlertsSection } from './AlertOverviewActiveAlertsSection';
import { AlertOverviewStatsCards } from './AlertOverviewStatsCards';
import type { Override } from './types';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { useAlertOverviewState } from './useAlertOverviewState';

export function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
  showQuickTip: () => boolean;
  dismissQuickTip: () => void;
  showAcknowledged: () => boolean;
  setShowAcknowledged: (value: boolean) => void;
  alertsDisabled: () => boolean;
  hasAIAlertsFeature: () => boolean;
  runtimeCapabilitiesLoading: () => boolean;
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
      <AlertOverviewStatsCards state={overviewState} />
      <AlertOverviewActiveAlertsSection
        state={overviewState}
        timelineState={timelineState}
        activeAlerts={props.activeAlerts}
        alertsDisabled={props.alertsDisabled()}
        hasAIAlertsFeature={props.hasAIAlertsFeature()}
        runtimeCapabilitiesLoading={props.runtimeCapabilitiesLoading()}
        showAcknowledged={props.showAcknowledged()}
        setShowAcknowledged={props.setShowAcknowledged}
      />
    </div>
  );
}
