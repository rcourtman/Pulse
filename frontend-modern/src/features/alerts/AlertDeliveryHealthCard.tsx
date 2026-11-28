import { For, Show } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type { NotificationQueueHealth } from '@/api/notifications';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import {
  ALERT_DESTINATIONS_DELIVERY_RECOVERY_NOTE,
  getAlertDestinationsDeliveryDismissLabel,
  getAlertDestinationsDeliveryFailureBreakdown,
  getAlertDestinationsDeliveryHealthDescription,
  getAlertDestinationsDeliveryHealthSummary,
  getAlertDestinationsDeliveryHealthTitle,
  getAlertDestinationsDeliveryLeadSummary,
  getAlertDestinationsDeliveryQueueMetrics,
  getAlertDestinationsDeliveryRefreshLabel,
  getAlertDestinationsDeliveryReviewLabel,
  getAlertDestinationsDeliveryRetryLabel,
  type AlertDeliveryQueueMetric,
} from '@/utils/alertDestinationsPresentation';

const QUEUE_METRIC_TONE_CLASSES: Record<AlertDeliveryQueueMetric['tone'], string> = {
  danger:
    'border-red-300 bg-red-100/80 text-red-900 dark:border-red-800 dark:bg-red-950/50 dark:text-red-200',
  warning:
    'border-amber-300 bg-amber-100/80 text-amber-900 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200',
  info: 'border-blue-300 bg-blue-100/80 text-blue-900 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-200',
  success:
    'border-emerald-300 bg-emerald-100/80 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-200',
};

interface AlertDeliveryHealthCardProps {
  health: NotificationQueueHealth | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  retryingFailures?: boolean;
  dismissingFailures?: boolean;
  onRetryFailures?: () => void;
  onDismissFailures?: () => void;
  detailsHref?: string;
  detailLevel?: 'summary' | 'full';
  showRefresh?: boolean;
}

export function AlertDeliveryHealthCard(props: AlertDeliveryHealthCardProps) {
  const status = (): 'degraded' | 'unavailable' =>
    props.unavailable || props.health?.status === 'unavailable' ? 'unavailable' : 'degraded';
  const failed = () => props.health?.failed ?? 0;
  const deadLetter = () => props.health?.deadLetter ?? 0;
  const knownTerminalFailures = () =>
    (!props.unavailable || props.health?.status === 'unavailable') && failed() + deadLetter() > 0;
  const completedRetentionDays = () => props.health?.completedRetentionDays ?? 7;
  const deadLetterRetentionDays = () => props.health?.deadLetterRetentionDays ?? 30;
  const failureWindowDays = () => props.health?.failureClassWindowDays || 7;
  const isSummary = () => props.detailLevel === 'summary';
  const isDegradedFull = () => status() === 'degraded' && !isSummary();
  const healthInput = () => ({
    status: status(),
    failed: failed(),
    deadLetter: deadLetter(),
    completedRetentionDays: completedRetentionDays(),
    deadLetterRetentionDays: deadLetterRetentionDays(),
    failureClasses7d: props.health?.failureClasses7d,
    failureClassesAvailable: props.health?.failureClassesAvailable ?? false,
    storageUnsettled: props.health?.reasonCodes?.includes('notification_storage_unsettled'),
    configurationUnavailable: props.health?.reasonCodes?.includes(
      'notification_configuration_unavailable',
    ),
    storageBootstrapUnavailable: props.health?.reasonCodes?.includes(
      'notification_storage_bootstrap_unavailable',
    ),
    occurrenceUnavailable: props.health?.reasonCodes?.includes('occurrence_lifecycle_unavailable'),
  });
  const description = () =>
    isSummary()
      ? getAlertDestinationsDeliveryHealthSummary(healthInput())
      : getAlertDestinationsDeliveryHealthDescription(healthInput());
  const failureBreakdown = () =>
    status() === 'degraded'
      ? getAlertDestinationsDeliveryFailureBreakdown(
          props.health?.failureClasses7d,
          props.health?.failureClassesAvailable ?? false,
        )
      : [];
  const queueMetrics = () => {
    const health = props.health;
    if (!health) return [];
    if (status() === 'unavailable') {
      return knownTerminalFailures()
        ? getAlertDestinationsDeliveryQueueMetrics(health, { terminalOnly: true })
        : [];
    }
    return getAlertDestinationsDeliveryQueueMetrics(health);
  };

  return (
    <Card tone="danger" padding="sm" class="border-red-200 dark:border-red-800 sm:p-4" role="alert">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex min-w-0 flex-1 basis-72 items-start gap-3">
          <AlertTriangleIcon
            class="mt-0.5 h-4 w-4 shrink-0 text-red-700 dark:text-red-300"
            aria-hidden="true"
          />
          <div class="min-w-0 flex-1">
            <h3 class="text-sm font-semibold text-base-content">
              {getAlertDestinationsDeliveryHealthTitle(status())}
            </h3>
            <Show
              when={isDegradedFull()}
              fallback={
                <p class="mt-1 text-sm leading-6 text-red-800 dark:text-red-200">{description()}</p>
              }
            >
              <p class="mt-1 text-sm leading-6 text-red-900 dark:text-red-200">
                {getAlertDestinationsDeliveryLeadSummary(healthInput())}
              </p>
            </Show>
          </div>
        </div>
        <div class="flex max-w-full flex-wrap items-center gap-2">
          {props.detailsHref ? (
            <ButtonLink
              variant="secondary"
              size="sm"
              class="min-h-11 sm:min-h-0"
              href={props.detailsHref}
            >
              {getAlertDestinationsDeliveryReviewLabel()}
            </ButtonLink>
          ) : null}
          {props.onRetryFailures && knownTerminalFailures() ? (
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 sm:min-h-0"
              isLoading={props.retryingFailures}
              disabled={props.dismissingFailures}
              onClick={props.onRetryFailures}
            >
              {getAlertDestinationsDeliveryRetryLabel()}
            </Button>
          ) : null}
          {props.onDismissFailures && knownTerminalFailures() ? (
            <Button
              variant="danger"
              size="sm"
              class="min-h-11 sm:min-h-0"
              isLoading={props.dismissingFailures}
              disabled={props.retryingFailures}
              onClick={props.onDismissFailures}
            >
              {getAlertDestinationsDeliveryDismissLabel()}
            </Button>
          ) : null}
          {props.showRefresh !== false ? (
            <Button
              variant="ghost"
              size="sm"
              class="min-h-11 sm:min-h-0"
              disabled={props.refreshing || props.retryingFailures || props.dismissingFailures}
              onClick={props.onRefresh}
            >
              <RefreshCwIcon
                class={`mr-2 h-4 w-4 ${props.refreshing ? 'animate-spin' : ''}`}
                aria-hidden="true"
              />
              {getAlertDestinationsDeliveryRefreshLabel()}
            </Button>
          ) : null}
        </div>
      </div>

      <Show when={queueMetrics().length > 0}>
        <dl
          class="mt-3 flex flex-wrap gap-2"
          data-testid="delivery-health-queue-metrics"
          aria-label="Notification queue counts"
        >
          <For each={queueMetrics()}>
            {(metric) => (
              <div
                class={`inline-flex items-center gap-1.5 rounded-sm border px-2 py-1 text-xs ${QUEUE_METRIC_TONE_CLASSES[metric.tone]}`}
              >
                <dt class="font-medium">{metric.label}</dt>
                <dd class="font-semibold tabular-nums">{metric.count}</dd>
              </div>
            )}
          </For>
        </dl>
      </Show>

      <Show when={status() === 'degraded' && isSummary() && failureBreakdown().length > 1}>
        <div
          class="mt-2.5 flex flex-wrap items-center gap-1.5"
          data-testid="delivery-health-failure-summary"
        >
          <span class="text-xs font-medium text-muted">
            Failure causes ({failureWindowDays()}d):
          </span>
          <For each={failureBreakdown()}>
            {(item) => (
              <span class="inline-flex items-center gap-1 rounded-sm border border-border bg-surface px-2 py-0.5 text-xs text-base-content">
                <span>{item.label}</span>
                <span class="font-semibold tabular-nums text-red-700 dark:text-red-300">
                  {item.count}
                </span>
              </span>
            )}
          </For>
        </div>
      </Show>

      <Show when={isDegradedFull()}>
        <Show
          when={failureBreakdown().length > 0}
          fallback={
            <p class="mt-2.5 text-xs text-base-content">
              Check each enabled destination and send a test.
            </p>
          }
        >
          <div
            class="mt-3 rounded-sm border border-border bg-surface p-3"
            data-testid="delivery-health-failure-breakdown"
          >
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h4 class="text-xs font-semibold uppercase tracking-wide text-base-content">
                Failure causes (last {failureWindowDays()} days)
              </h4>
              <span class="text-xs text-muted">
                Most recent terminal failures were classified as{' '}
                {failureBreakdown()[0].failureClass.replace('_', ' ')} (
                {failureBreakdown()[0].count})
              </span>
            </div>
            <ul class="mt-2 divide-y divide-border-subtle">
              <For each={failureBreakdown()}>
                {(item) => (
                  <li class="flex flex-col gap-0.5 py-1.5 first:pt-0 last:pb-0 sm:flex-row sm:items-baseline sm:gap-2.5">
                    <span class="inline-flex shrink-0 items-center gap-1.5 text-xs font-semibold text-base-content">
                      <span>{item.label}</span>
                      <span class="rounded-sm bg-red-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-red-800 dark:bg-red-900/40 dark:text-red-200">
                        {item.count}
                      </span>
                    </span>
                    <span class="text-xs leading-relaxed text-muted">{item.guidance}</span>
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>

        <p class="mt-3 text-xs leading-relaxed text-muted">
          {ALERT_DESTINATIONS_DELIVERY_RECOVERY_NOTE}
        </p>
      </Show>
    </Card>
  );
}
