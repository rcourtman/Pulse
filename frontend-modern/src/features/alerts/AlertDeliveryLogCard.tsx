import { For, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type {
  NotificationDeliveryLog,
  NotificationDeliveryLogEntry,
  Webhook,
} from '@/api/notifications';
import { Card } from '@/components/shared/Card';
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

interface AlertDeliveryLogCardProps {
  log: NotificationDeliveryLog | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  webhooks: Webhook[];
}

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
  const windowDays = () => props.log?.windowDays ?? 7;

  return (
    <Card padding="sm" class="sm:p-4">
      <div class="flex flex-col gap-3">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {getAlertDestinationsDeliveryLogTitle()}
            </h3>
            <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-400">
              {getAlertDestinationsDeliveryLogDescription(windowDays())}
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
            when={entries().length > 0}
            fallback={
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {getAlertDestinationsDeliveryLogEmpty()}
              </p>
            }
          >
            <ul class="max-h-80 divide-y divide-gray-200 overflow-y-auto dark:divide-gray-700">
              <For each={entries()}>
                {(entry) => (
                  <li class="flex flex-col gap-1 py-2">
                    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                      <span
                        class={`inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${outcomeBadgeClasses[entry.outcome]}`}
                      >
                        {getAlertDeliveryLogOutcomeLabel(entry.outcome)}
                      </span>
                      <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                        {destinationLabel(entry)}
                      </span>
                      <span
                        class="min-w-0 truncate text-sm text-gray-600 dark:text-gray-400"
                        title={entry.alertIds.join(', ')}
                      >
                        {alertSummary(entry)}
                      </span>
                      <span class="ml-auto flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
                        {formatRelativeTime(entry.timestamp)}
                      </span>
                    </div>
                    <Show when={!entry.success && (entry.failureClass || entry.errorMessage)}>
                      <p class="text-xs leading-5 text-red-700 dark:text-red-300">
                        <Show when={entry.failureClass}>
                          {(failureClass) => (
                            <span class="font-medium">
                              {getAlertDeliveryLogFailureClassLabel(failureClass())}
                              {entry.errorMessage ? ': ' : ''}
                            </span>
                          )}
                        </Show>
                        {entry.errorMessage}
                      </p>
                    </Show>
                  </li>
                )}
              </For>
            </ul>
          </Show>
        </Show>
      </div>
    </Card>
  );
}
