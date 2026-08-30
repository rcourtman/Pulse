import { For, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type {
  NotificationDeliveryLog,
  NotificationDeliveryLogEntry,
  Webhook,
} from '@/api/notifications';
import { Card } from '@/components/shared/Card';
import type { AlertEvent } from '@/types/api';
import {
  getAlertDeliveryLogFailureClassLabel,
  getAlertDeliveryLogOutcomeLabel,
  getAlertDestinationsDeliveryLogDescription,
  getAlertDestinationsDeliveryLogEmpty,
  getAlertDestinationsDeliveryLogTitle,
  getAlertDestinationsDeliveryLogUnavailable,
  getAlertDestinationsDeliveryRefreshLabel,
} from '@/utils/alertDestinationsPresentation';
import { formatRelativeTime } from '@/utils/format';

import { describeAlertEventReason } from './deliveryDiagnosisPresentation';

interface AlertDeliveryLogCardProps {
  log: NotificationDeliveryLog | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  webhooks: Webhook[];
  heldEvents?: AlertEvent[];
}

type DeliveryLogRow =
  | { kind: 'attempt'; timestamp: string; entry: NotificationDeliveryLogEntry }
  | { kind: 'held'; timestamp: string; event: AlertEvent };

// mergeDeliveryLogRows interleaves delivery attempts with held-notification
// events, newest first, so the activity list tells the whole story: what was
// sent, what failed, and what was deliberately not attempted and why.
export const mergeDeliveryLogRows = (
  entries: NotificationDeliveryLogEntry[],
  heldEvents: AlertEvent[],
): DeliveryLogRow[] => {
  const rows: DeliveryLogRow[] = [
    ...entries.map((entry): DeliveryLogRow => ({
      kind: 'attempt',
      timestamp: entry.timestamp,
      entry,
    })),
    ...heldEvents.map((event): DeliveryLogRow => ({
      kind: 'held',
      timestamp: event.occurredAt,
      event,
    })),
  ];
  return rows.sort((a, b) => {
    const at = new Date(a.timestamp).getTime() || 0;
    const bt = new Date(b.timestamp).getTime() || 0;
    return bt - at;
  });
};

const WEBHOOK_DESTINATION_PREFIX = 'webhook:';

const outcomeBadgeClasses: Record<NotificationDeliveryLogEntry['outcome'], string> = {
  sent: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  retry: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  dead_letter: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  cancelled: 'bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300',
};

export function AlertDeliveryLogCard(props: AlertDeliveryLogCardProps) {
  // Destination ids are opaque on purpose (hashes for email/apprise, config
  // ids for webhooks); only webhook ids can be resolved to a configured name.
  const destinationLabel = (entry: NotificationDeliveryLogEntry): string => {
    if (entry.destinationId?.startsWith(WEBHOOK_DESTINATION_PREFIX)) {
      const webhookId = entry.destinationId.slice(WEBHOOK_DESTINATION_PREFIX.length);
      const webhook = props.webhooks.find((candidate) => candidate.id === webhookId);
      if (webhook?.name) return webhook.name;
    }
    switch (entry.type) {
      case 'email':
        return 'Email';
      case 'apprise':
        return 'Apprise';
      case 'webhook':
        return 'Webhook';
      default:
        return entry.type || 'Destination';
    }
  };

  const alertSummary = (entry: NotificationDeliveryLogEntry): string => {
    if (entry.alertIds.length === 0) {
      return entry.alertCount === 1 ? '1 alert' : `${entry.alertCount} alerts`;
    }
    const [first, ...rest] = entry.alertIds;
    return rest.length > 0 ? `${first} +${rest.length} more` : first;
  };

  const entries = () => props.log?.entries ?? [];
  const completedRetentionDays = () => props.log?.completedRetentionDays ?? 7;
  const deadLetterRetentionDays = () => props.log?.deadLetterRetentionDays ?? 30;
  const rows = () => mergeDeliveryLogRows(entries(), props.heldEvents ?? []);

  const absoluteTimestamp = (value: string): string => {
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return parsed.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  const heldResourceLabel = (event: AlertEvent): string => {
    const resource = event.resourceName || event.alertId;
    if (event.alertType) return `${resource} (${event.alertType})`;
    return resource;
  };

  return (
    <Card id="notification-delivery-activity" padding="sm" class="scroll-mt-24 sm:p-4">
      <div class="flex flex-col gap-3">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {getAlertDestinationsDeliveryLogTitle()}
            </h3>
            <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-400">
              {getAlertDestinationsDeliveryLogDescription(
                completedRetentionDays(),
                deadLetterRetentionDays(),
              )}
            </p>
          </div>
          <button
            type="button"
            class="inline-flex flex-shrink-0 items-center justify-center gap-2 rounded-md border border-gray-300 bg-transparent px-3 py-1.5 text-sm font-medium text-gray-700 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700/40"
            disabled={props.refreshing}
            onClick={props.onRefresh}
          >
            <RefreshCwIcon class={`h-4 w-4 ${props.refreshing ? 'animate-spin' : ''}`} />
            {getAlertDestinationsDeliveryRefreshLabel()}
          </button>
        </div>

        <Show
          when={!props.unavailable}
          fallback={
            <p class="text-sm text-red-800 dark:text-red-300" role="alert">
              {getAlertDestinationsDeliveryLogUnavailable()}
            </p>
          }
        >
          <Show
            when={rows().length > 0}
            fallback={
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {getAlertDestinationsDeliveryLogEmpty()}
              </p>
            }
          >
            <ul class="max-h-80 divide-y divide-gray-200 overflow-y-auto dark:divide-gray-700">
              <For each={rows()}>
                {(row) =>
                  row.kind === 'attempt' ? (
                    <li class="flex flex-col gap-1 py-2">
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                        <span
                          class={`inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${outcomeBadgeClasses[row.entry.outcome]}`}
                        >
                          {getAlertDeliveryLogOutcomeLabel(row.entry.outcome)}
                        </span>
                        <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                          {destinationLabel(row.entry)}
                        </span>
                        <span
                          class="min-w-0 truncate text-sm text-gray-600 dark:text-gray-400"
                          title={row.entry.alertIds.join(', ')}
                        >
                          {alertSummary(row.entry)}
                        </span>
                        <time
                          class="ml-auto flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
                          dateTime={row.entry.timestamp}
                          title={formatRelativeTime(row.entry.timestamp)}
                        >
                          {absoluteTimestamp(row.entry.timestamp)}
                        </time>
                      </div>
                      <Show
                        when={
                          !row.entry.success && (row.entry.failureClass || row.entry.errorMessage)
                        }
                      >
                        <p class="text-xs leading-5 text-red-700 dark:text-red-300">
                          <Show when={row.entry.failureClass}>
                            {(failureClass) => (
                              <span class="font-medium">
                                {getAlertDeliveryLogFailureClassLabel(failureClass())}
                                {row.entry.errorMessage ? ': ' : ''}
                              </span>
                            )}
                          </Show>
                          {row.entry.errorMessage}
                        </p>
                      </Show>
                    </li>
                  ) : (
                    <li class="flex flex-col gap-1 py-2" title={row.event.message}>
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                        <span
                          class={`inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                            row.event.type === 'notification_deferred'
                              ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300'
                              : 'bg-gray-100 text-gray-600 dark:bg-gray-700/60 dark:text-gray-300'
                          }`}
                        >
                          {row.event.type === 'notification_deferred' ? 'Deferred' : 'Held'}
                        </span>
                        <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                          {heldResourceLabel(row.event)}
                        </span>
                        <span class="min-w-0 truncate text-sm text-gray-600 dark:text-gray-400">
                          {describeAlertEventReason(row.event.reason)}
                        </span>
                        <time
                          class="ml-auto flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
                          dateTime={row.event.occurredAt}
                          title={formatRelativeTime(row.event.occurredAt)}
                        >
                          {absoluteTimestamp(row.event.occurredAt)}
                        </time>
                      </div>
                    </li>
                  )
                }
              </For>
            </ul>
          </Show>
        </Show>
      </div>
    </Card>
  );
}
