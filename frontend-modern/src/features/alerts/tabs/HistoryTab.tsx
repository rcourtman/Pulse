import { Show, createSignal, onCleanup, createEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Resource } from '@/types/resource';
import { useWebSocket } from '@/contexts/appRuntime';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { Button } from '@/components/shared/Button';
import {
  getAlertHistoryFocusNotice,
  getAlertHistoryLoadFailure,
  getAlertOverviewHydrationState,
} from '@/utils/alertOverviewPresentation';

import { AlertHistoryAdministrationCard } from '../AlertHistoryAdministrationCard';
import { AlertHistoryFiltersCard } from '../AlertHistoryFiltersCard';
import { AlertHistoryFrequencyCard } from '../AlertHistoryFrequencyCard';
import { AlertHistoryTableSection } from '../AlertHistoryTableSection';
import { useAlertHistoryState } from '../useAlertHistoryState';

export interface HistoryTabProps {
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

export function HistoryTab(props: HistoryTabProps) {
  const location = useLocation();
  const { activeAlerts, activeAlertsHydrationStatus, refreshActiveAlerts } = useWebSocket();
  const { isMobile } = useBreakpoint();
  let hashScrollRafId: number | undefined;
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);

  const historyState = useAlertHistoryState({
    activeAlerts: () => activeAlerts || {},
    getResource: props.getResource,
    allResources: props.allResources,
    // Only a delivered live list settles it: "unavailable" says nothing about
    // whether the linked alert is still firing.
    activeAlertsSettled: () => activeAlertsHydrationStatus() === 'ready',
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

  const focusNotice = () =>
    getAlertHistoryFocusNotice({
      alertTitle: historyState.focusedAlert()?.title,
      resourceName: historyState.focusedAlert()?.resourceName,
      missing: historyState.focusedAlertMissing(),
      liveAlertsUnavailable: activeAlertsHydrationStatus() === 'unavailable',
    });

  return (
    <div class="space-y-4">
      <Show when={historyState.historyLoadError()}>
        <div
          class="flex flex-col gap-3 rounded-md border border-border bg-surface-alt p-4 sm:flex-row sm:items-center sm:justify-between"
          role="alert"
        >
          <div class="min-w-0">
            <p class="text-sm font-medium text-base-content">
              {getAlertHistoryLoadFailure().title}
            </p>
            <p class="mt-1 text-sm text-muted">{getAlertHistoryLoadFailure().description}</p>
          </div>
          <Button
            size="sm"
            class="min-h-11 shrink-0 self-start"
            disabled={historyState.loading()}
            aria-busy={historyState.loading()}
            onClick={() => void historyState.retryHistory()}
          >
            {historyState.loading()
              ? getAlertHistoryLoadFailure().retryingLabel
              : getAlertHistoryLoadFailure().retryLabel}
          </Button>
        </div>
      </Show>
      <Show when={historyState.focusedAlertId()}>
        <div
          class="flex flex-col gap-3 rounded-md border border-border bg-surface-alt p-4 sm:flex-row sm:items-center sm:justify-between"
          role="status"
          data-alert-history-focus
        >
          <p class="min-w-0 text-sm text-base-content">{focusNotice().text}</p>
          <div class="flex shrink-0 flex-wrap gap-2">
            <Show when={activeAlertsHydrationStatus() === 'unavailable'}>
              <Button
                size="sm"
                variant="secondary"
                class="min-h-11 sm:min-h-0"
                onClick={() => void refreshActiveAlerts()}
              >
                {getAlertOverviewHydrationState('unavailable').retryLabel}
              </Button>
            </Show>
            <Button
              size="sm"
              variant="secondary"
              class="min-h-11 sm:min-h-0"
              onClick={historyState.clearFocusedAlert}
            >
              {focusNotice().clearLabel}
            </Button>
          </div>
        </div>
      </Show>
      <Show when={!historyState.loading() && !historyState.historyLoadError()}>
        <AlertHistoryFrequencyCard state={historyState} />
      </Show>
      <AlertHistoryFiltersCard state={historyState} isMobile={isMobile()} />
      <AlertHistoryTableSection state={historyState} />
      <AlertHistoryAdministrationCard state={historyState} />
    </div>
  );
}
