import { Show } from 'solid-js';

import { StatusDot } from '@/components/shared/StatusDot';
import {
  getAlertOverviewSeverityCountLabel,
  getAlertOverviewStatsLabels,
} from '@/utils/alertOverviewPresentation';

import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewStatsCardsProps {
  state: AlertOverviewState;
}

// One line beside the active-alerts heading: what is open now by severity,
// then how much fired in the last day. It replaces a three-row table whose
// override count was configuration, not status.
export function AlertOverviewStatsCards(props: AlertOverviewStatsCardsProps) {
  const labels = () => getAlertOverviewStatsLabels();
  const stats = () => props.state.alertStats();

  return (
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
      <Show when={stats().activeCritical > 0}>
        <span class="inline-flex items-center gap-1.5 font-medium text-red-700 dark:text-red-300">
          <StatusDot variant="danger" size="sm" ariaHidden />
          {getAlertOverviewSeverityCountLabel('critical', stats().activeCritical)}
        </span>
      </Show>
      <Show when={stats().activeWarning > 0}>
        <span class="inline-flex items-center gap-1.5 font-medium text-yellow-700 dark:text-yellow-300">
          <StatusDot variant="warning" size="sm" ariaHidden />
          {getAlertOverviewSeverityCountLabel('warning', stats().activeWarning)}
        </span>
      </Show>
      <span class="inline-flex items-center gap-1" data-alert-overview-stat="triggered24h">
        <span>{labels().last24Hours}</span>
        <span
          class="font-semibold tabular-nums text-base-content"
          data-testid="alert-overview-stat-value"
        >
          {props.state.alertStats().total24h}
        </span>
      </span>
      <Show when={props.state.alertStats().acknowledged > 0}>
        <span class="inline-flex items-center gap-1" data-alert-overview-stat="acknowledged">
          <span>{labels().acknowledged}</span>
          <span
            class="font-semibold tabular-nums text-base-content"
            data-testid="alert-overview-stat-value"
          >
            {props.state.alertStats().acknowledged}
          </span>
        </span>
      </Show>
    </div>
  );
}
