import { createSignal, onCleanup, createEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Resource } from '@/types/resource';
import { useWebSocket } from '@/contexts/appRuntime';
import { useBreakpoint } from '@/hooks/useBreakpoint';

import { AlertHistoryAdministrationCard } from '../AlertHistoryAdministrationCard';
import { AlertHistoryFiltersCard } from '../AlertHistoryFiltersCard';
import { AlertHistoryFrequencyCard } from '../AlertHistoryFrequencyCard';
import { AlertHistoryTableSection } from '../AlertHistoryTableSection';
import { AlertResourceIncidentsPanel } from '../AlertResourceIncidentsPanel';
import { useAlertHistoryState } from '../useAlertHistoryState';

export interface HistoryTabProps {
  hasAIAlertsFeature: () => boolean;
  licenseLoading: () => boolean;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

export function HistoryTab(props: HistoryTabProps) {
  const location = useLocation();
  const { activeAlerts } = useWebSocket();
  const { isMobile } = useBreakpoint();
  let hashScrollRafId: number | undefined;
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);

  const historyState = useAlertHistoryState({
    activeAlerts: () => activeAlerts || {},
    getResource: props.getResource,
    allResources: props.allResources,
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
    historyState.alertData().length;
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
    <div class="space-y-4">
      <AlertHistoryFrequencyCard state={historyState} />
      <AlertHistoryFiltersCard state={historyState} isMobile={isMobile()} />
      <AlertResourceIncidentsPanel state={historyState} getResource={props.getResource} />
      <AlertHistoryTableSection
        state={historyState}
        hasAIAlertsFeature={props.hasAIAlertsFeature}
        licenseLoading={props.licenseLoading}
      />
      <AlertHistoryAdministrationCard state={historyState} />
    </div>
  );
}
